package app

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

func (a *App) generateTitle(content, date string) string {
	fallback := fallbackTitle(content, date)
	if strings.TrimSpace(content) == "" || !a.Cfg.LLM.Enabled {
		return fallback
	}
	title, err := a.summarizeTitleWithLLM(content)
	if err != nil {
		log.Printf("[WARN] llm title failed: %v", err)
		return fallback
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return fallback
	}
	return title
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
		endpoint += "/responses"
	}
	body := map[string]any{
		"model":        model,
		"instructions": prompt,
		"input":        content,
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
	var out struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
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
