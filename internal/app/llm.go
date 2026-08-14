package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

func (a *App) generateTitle(content, date string) string {
	title, _ := a.generateTitleWithError(content, date)
	return title
}

func (a *App) generateTitleWithError(content, date string) (string, error) {
	fallback := fallbackTitle(content, date)
	if strings.TrimSpace(content) == "" || !a.Cfg.LLM.Enabled {
		return fallback, nil
	}
	title, err := a.summarizeTitleWithLLM(content)
	if err != nil {
		log.Printf("[WARN] llm title failed: %v", err)
		return fallback, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return fallback, nil
	}
	return title, nil
}

func (a *App) generateTitleInBackground(dateStr, content string) {
	if strings.TrimSpace(content) == "" || !a.Cfg.LLM.Enabled {
		return
	}
	title, err := a.summarizeTitleWithLLM(content)
	if err != nil {
		log.Printf("[WARN] background llm title failed: %v", err)
		return
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return
	}
	res, err := a.DB.Exec(`
        UPDATE entries
        SET title=?, updated_at=?
        WHERE day=? AND content=? AND auto_title=1
    `, title, time.Now().UTC(), dateStr, content)
	if err != nil {
		log.Printf("[WARN] background title update failed: %v", err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		log.Printf("[INFO] background title skipped for %s: entry changed or title became manual", dateStr)
	}
}

func (a *App) summarizeTitleWithLLM(content string) (string, error) {
	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		title, err := a.summarizeTitleWithLLMOnce(content)
		if err == nil {
			return title, nil
		}
		lastErr = err
		if attempt < maxAttempts {
			log.Printf("[WARN] llm title attempt %d/%d failed: %v", attempt, maxAttempts, err)
			time.Sleep(time.Duration(attempt) * 250 * time.Millisecond)
		}
	}
	return "", lastErr
}

func (a *App) summarizeTitleWithLLMOnce(content string) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(a.Cfg.LLM.BaseURL), "/")
	apiKey := strings.TrimSpace(a.Cfg.LLM.APIKey)
	model := strings.TrimSpace(a.Cfg.LLM.Model)
	if baseURL == "" || apiKey == "" || model == "" {
		return "", fmt.Errorf("llm config incomplete")
	}
	prompt := strings.TrimSpace(a.Cfg.LLM.Prompt)
	if prompt == "" {
		prompt = "请为下面这篇日记生成一个标题，只返回标题，不要解释。"
	}

	endpoint := baseURL
	if !strings.HasSuffix(endpoint, "/responses") {
		if strings.HasSuffix(endpoint, "/v1") {
			endpoint += "/responses"
		} else {
			endpoint += "/v1/responses"
		}
	}
	body := map[string]any{
		"model":        model,
		"instructions": prompt,
		"input":        content,
		"stream":       true,
		"temperature":  0.3,
	}
	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(string(b)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("llm status %d: %s", resp.StatusCode, string(data))
	}
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		return readResponsesStream(resp.Body)
	}
	return readResponsesJSON(resp.Body)
}

func readResponsesJSON(r io.Reader) (string, error) {
	var out struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.NewDecoder(r).Decode(&out); err != nil {
		return "", err
	}
	text := strings.TrimSpace(out.OutputText)
	if text == "" {
		for _, item := range out.Output {
			for _, c := range item.Content {
				if c.Text != "" {
					text = strings.TrimSpace(c.Text)
					break
				}
			}
			if text != "" {
				break
			}
		}
	}
	if text == "" {
		return "", fmt.Errorf("llm returned no text")
	}
	return strings.Trim(text, " \t\r\n\"'“”‘’#：:"), nil
}

func readResponsesStream(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	var text strings.Builder
	var dataLines []string

	processEvent := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		data := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]
		if data == "[DONE]" {
			return nil
		}
		var event struct {
			Type    string `json:"type"`
			Delta   string `json:"delta"`
			Message string `json:"message"`
			Error   *struct {
				Message string `json:"message"`
			} `json:"error"`
			Response *struct {
				Error *struct {
					Message string `json:"message"`
				} `json:"error"`
			} `json:"response"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return fmt.Errorf("decode llm stream event: %w", err)
		}
		switch event.Type {
		case "response.output_text.delta":
			text.WriteString(event.Delta)
		case "error", "response.failed", "response.incomplete":
			message := strings.TrimSpace(event.Message)
			if message == "" && event.Error != nil {
				message = strings.TrimSpace(event.Error.Message)
			}
			if message == "" && event.Response != nil && event.Response.Error != nil {
				message = strings.TrimSpace(event.Response.Error.Message)
			}
			if message == "" {
				message = data
			}
			return fmt.Errorf("llm stream %s: %s", event.Type, message)
		}
		return nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			if err := processEvent(); err != nil {
				return "", err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read llm stream: %w", err)
	}
	if err := processEvent(); err != nil {
		return "", err
	}
	result := strings.TrimSpace(text.String())
	if result == "" {
		return "", fmt.Errorf("llm returned no text")
	}
	return strings.Trim(result, " \t\r\n\"'“”‘’#：:"), nil
}
