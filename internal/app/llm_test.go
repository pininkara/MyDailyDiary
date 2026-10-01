package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestLLMTitleAPIFormats(t *testing.T) {
	const content = "今天整理了书架，心情很好。"
	const date = "2024-05-12"
	const prompt = "概括这篇日记，只返回标题。"
	const model = "test-model"
	const geminiResponse = `{"candidates":[{"content":{"parts":[{"thought":true,"text":"思考过程"},{"text":"“书架"},{"text":"与好心情”"}]},"finishReason":"STOP"}],"promptFeedback":{"blockReason":"BLOCK_REASON_UNSPECIFIED"}}`
	const openAIResponse = `{"output":[{"content":[{"type":"output_text","text":"“书架与好心情”"}]}]}`
	tests := []struct {
		name, format, basePath, model, endpoint, response string
		stream                                            bool
	}{
		{"default", "", "", model, "/v1/responses", openAIResponse, false},
		{"openai-v1", "openai", "/v1/", model, "/v1/responses", openAIResponse, false},
		{"openai-endpoint", "openai", "/v1/responses", model, "/v1/responses", openAIResponse, false},
		{"openai-stream", "openai", "", model, "/v1/responses", "data: {\"type\":\"response.output_text.delta\",\"delta\":\"书架与好心情\"}\n\ndata: [DONE]\n\n", true},
		{"gemini-root", "gemini", "", model, "/v1beta/models/test-model:generateContent", geminiResponse, false},
		{"gemini-v1beta", "gemini", "/v1beta/", model, "/v1beta/models/test-model:generateContent", geminiResponse, false},
		{"gemini-v1", "gemini", "/v1", model, "/v1/models/test-model:generateContent", geminiResponse, false},
		{"gemini-prefixed-model", "gemini", "/proxy/v1beta", "models/test-model", "/proxy/v1beta/models/test-model:generateContent", geminiResponse, false},
		{"gemini-endpoint", "gemini", "/v1beta/models/test-model:generateContent", model, "/v1beta/models/test-model:generateContent", geminiResponse, false},
		{"gemini-normalized-format", " Gemini ", "", model, "/v1beta/models/test-model:generateContent", geminiResponse, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != http.MethodPost || r.URL.Path != tt.endpoint {
					t.Errorf("request = %s %s, want POST %s", r.Method, r.URL.Path, tt.endpoint)
				}
				if r.URL.RawQuery != "" {
					t.Errorf("unexpected query: %s", r.URL.RawQuery)
				}
				if r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Accept") != "application/json" {
					t.Error("expected JSON request and response headers")
				}
				var body struct {
					Model        string `json:"model"`
					Instructions string `json:"instructions"`
					Stream       *bool  `json:"stream"`
					Input        []struct {
						Role    string `json:"role"`
						Content []struct {
							Type string `json:"type"`
							Text string `json:"text"`
						} `json:"content"`
					} `json:"input"`
					SystemInstruction struct {
						Parts []struct {
							Text string `json:"text"`
						} `json:"parts"`
					} `json:"systemInstruction"`
					Contents []struct {
						Role  string `json:"role"`
						Parts []struct {
							Text string `json:"text"`
						} `json:"parts"`
					} `json:"contents"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				var input string
				if strings.EqualFold(strings.TrimSpace(tt.format), "gemini") {
					if r.Header.Get("x-goog-api-key") != "test-key" || r.Header.Get("Authorization") != "" {
						t.Error("expected Gemini API key header")
					}
					if len(body.SystemInstruction.Parts) != 1 || body.SystemInstruction.Parts[0].Text != prompt {
						t.Errorf("Gemini system instruction = %+v", body.SystemInstruction)
					}
					if len(body.Contents) != 1 || body.Contents[0].Role != "user" || len(body.Contents[0].Parts) != 1 {
						t.Errorf("Gemini contents = %+v", body.Contents)
					} else {
						input = body.Contents[0].Parts[0].Text
					}
					if body.Model != "" || len(body.Input) != 0 || body.Stream != nil {
						t.Error("Gemini request contains OpenAI fields")
					}
				} else {
					if r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("x-goog-api-key") != "" {
						t.Error("expected OpenAI bearer authorization")
					}
					if body.Model != model || body.Instructions != prompt || body.Stream == nil || *body.Stream {
						t.Errorf("incorrect OpenAI model, instructions or stream flag")
					}
					if len(body.Input) != 1 || body.Input[0].Role != "user" || len(body.Input[0].Content) != 1 || body.Input[0].Content[0].Type != "input_text" {
						t.Errorf("OpenAI input = %+v", body.Input)
					} else {
						input = body.Input[0].Content[0].Text
					}
					if len(body.Contents) != 0 || len(body.SystemInstruction.Parts) != 0 {
						t.Error("OpenAI request contains Gemini fields")
					}
				}
				if !strings.Contains(input, date) || !strings.Contains(input, content) {
					t.Errorf("input does not include the diary date and content: %q", input)
				}
				w.Header().Set("Content-Type", "application/json")
				if tt.stream {
					w.Header().Set("Content-Type", "text/event-stream")
				}
				io.WriteString(w, tt.response)
			}))
			defer server.Close()
			app := &App{Cfg: &Config{}}
			app.Cfg.LLM.Enabled = true
			app.Cfg.LLM.APIFormat = tt.format
			app.Cfg.LLM.BaseURL = server.URL + tt.basePath
			app.Cfg.LLM.APIKey = " test-key "
			app.Cfg.LLM.Model = tt.model
			app.Cfg.LLM.Prompt = prompt
			title, err := app.generateTitleWithError(content, date)
			if err != nil || title != "书架与好心情" {
				t.Fatalf("title = %q, err = %v", title, err)
			}
			if calls.Load() != 1 {
				t.Errorf("API calls = %d, want 1", calls.Load())
			}
		})
	}
}

func TestReadGeminiJSONFailures(t *testing.T) {
	tests := []struct {
		name, response, wantError string
	}{
		{"empty", `{}`, "no text"},
		{"empty-title", `{"candidates":[{"content":{"parts":[{"text":"“”"}]}}]}`, "no text"},
		{"thought-only", `{"candidates":[{"content":{"parts":[{"thought":true,"text":"private reasoning"}]}}]}`, "no text"},
		{"blocked-prompt", `{"promptFeedback":{"blockReason":"SAFETY"}}`, "prompt blocked: SAFETY"},
		{"blocked-candidate", `{"candidates":[{"finishReason":"SAFETY"}]}`, "finishReason: SAFETY"},
		{"api-error", `{"error":{"message":"quota exceeded"}}`, "quota exceeded"},
		{"invalid-json", `{`, "EOF"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, err := readGeminiJSON(strings.NewReader(tt.response))
			if title != "" || err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("title = %q, err = %v, want error containing %q", title, err, tt.wantError)
			}
		})
	}
}

func TestGeminiRetriesAndFallback(t *testing.T) {
	for _, recover := range []bool{true, false} {
		t.Run(fmt.Sprintf("recover=%t", recover), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				call := calls.Add(1)
				if recover && call == 2 {
					io.WriteString(w, `{"candidates":[{"content":{"parts":[{"text":"恢复后的标题"}]}}]}`)
					return
				}
				http.Error(w, `{"error":{"message":"quota exceeded"}}`, http.StatusTooManyRequests)
			}))
			defer server.Close()
			app := &App{Cfg: &Config{}}
			app.Cfg.LLM.Enabled = true
			app.Cfg.LLM.APIFormat = "gemini"
			app.Cfg.LLM.BaseURL = server.URL
			app.Cfg.LLM.APIKey = "test-key"
			app.Cfg.LLM.Model = "test-model"
			content, date := "今天散步很开心。", "2024-05-12"
			title, err := app.generateTitleWithError(content, date)
			if recover {
				if err != nil || title != "恢复后的标题" || calls.Load() != 2 {
					t.Fatalf("title = %q, err = %v, calls = %d", title, err, calls.Load())
				}
			} else if err == nil || !strings.Contains(err.Error(), "llm status 429") || title != fallbackTitle(content, date) || calls.Load() != 3 {
				t.Fatalf("title = %q, err = %v, calls = %d", title, err, calls.Load())
			}
		})
	}
}

func TestTitleDateInManualAndBackgroundGeneration(t *testing.T) {
	const date = "2024-05-12"
	const content = "整理书架"
	for _, format := range []string{"openai", "gemini"} {
		t.Run(format, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				data, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if !strings.Contains(string(data), date) || !strings.Contains(string(data), content) {
					t.Errorf("missing diary date or content: %s", data)
				}
				if format == "gemini" {
					io.WriteString(w, `{"candidates":[{"content":{"parts":[{"text":"书架与好心情"}]}}]}`)
				} else {
					io.WriteString(w, `{"output_text":"书架与好心情"}`)
				}
			}))
			defer server.Close()
			app := newThoughtTestApp(t)
			app.Cfg = &Config{}
			app.Cfg.LLM.Enabled = true
			app.Cfg.LLM.APIFormat = format
			app.Cfg.LLM.BaseURL = server.URL
			app.Cfg.LLM.APIKey = "test-key"
			app.Cfg.LLM.Model = "test-model"
			for _, handler := range []http.HandlerFunc{app.apiGenerateTitle, app.apiGenerateTitleAndSave} {
				response := httptest.NewRecorder()
				handler(response, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"content":"`+content+`","date":"`+date+`"}`)))
				if response.Code != http.StatusOK {
					t.Fatalf("status = %d, body = %s", response.Code, response.Body)
				}
			}
			if _, err := app.DB.Exec(`UPDATE entries SET title='旧标题' WHERE day=?`, date); err != nil {
				t.Fatal(err)
			}
			app.generateTitleInBackground(date, content)
			entry, err := app.getEntryByDate(date)
			if err != nil {
				t.Fatal(err)
			}
			if entry == nil || entry.Title != "书架与好心情" || !entry.AutoTitle || calls.Load() != 3 {
				t.Fatalf("entry = %+v, API calls = %d", entry, calls.Load())
			}
		})
	}
}

func TestLLMAPIFormatConfigAndSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[llm]\nenabled = true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	app := &App{Cfg: loadConfig(path), ConfigPath: path}
	if app.Cfg.LLM.APIFormat != "openai" {
		t.Fatalf("old config format = %q, want openai", app.Cfg.LLM.APIFormat)
	}
	if err := os.WriteFile(path, []byte("[llm]\napi_format = ' Gemini '\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if format := loadConfig(path).LLM.APIFormat; format != "gemini" {
		t.Fatalf("loaded API format = %q, want gemini", format)
	}
	for _, tt := range []struct {
		body, wantFormat string
		status           int
	}{
		{`{"llm_api_format":" Gemini "}`, "gemini", http.StatusOK},
		{`{"username":"old client"}`, "gemini", http.StatusOK},
		{`{"llm_api_format":"invalid","username":"should not be applied"}`, "gemini", http.StatusBadRequest},
		{`{"llm_api_format":"openai"}`, "openai", http.StatusOK},
	} {
		response := httptest.NewRecorder()
		app.apiUpdateSettings(response, httptest.NewRequest(http.MethodPost, "/api/settings", strings.NewReader(tt.body)))
		if response.Code != tt.status {
			t.Fatalf("settings status = %d, want %d; body = %s", response.Code, tt.status, response.Body)
		}
		if app.Cfg.LLM.APIFormat != tt.wantFormat || loadConfig(path).LLM.APIFormat != tt.wantFormat {
			t.Fatalf("format not preserved in memory and TOML: want %q", tt.wantFormat)
		}
		if tt.status == http.StatusBadRequest && app.Cfg.UI.Username != "old client" {
			t.Fatal("invalid format changed other settings")
		}
		getResponse := httptest.NewRecorder()
		app.apiGetSettings(getResponse, httptest.NewRequest(http.MethodGet, "/api/settings", nil))
		var settings struct {
			APIFormat string `json:"llm_api_format"`
		}
		if err := json.Unmarshal(getResponse.Body.Bytes(), &settings); err != nil {
			t.Fatal(err)
		}
		if settings.APIFormat != tt.wantFormat {
			t.Fatalf("settings format = %q, want %q", settings.APIFormat, tt.wantFormat)
		}
	}
}

func TestUnsupportedLLMAPIFormat(t *testing.T) {
	app := &App{Cfg: &Config{}}
	app.Cfg.LLM.APIFormat = "invalid"
	if _, err := app.summarizeTitleWithLLMOnce("content", "2024-05-12"); err == nil || !strings.Contains(err.Error(), "unsupported llm api_format") {
		t.Fatalf("unexpected error: %v", err)
	}
}
