package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (a *App) apiLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if subtleEqual(req.Token, a.Token) {
		http.SetCookie(w, &http.Cookie{
			Name:     "diary_auth",
			Value:    "ok",
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   7 * 24 * 3600,
		})
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}
	http.Error(w, "Invalid token", http.StatusUnauthorized)
}

func (a *App) apiLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "diary_auth",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
	w.WriteHeader(http.StatusOK)
}

func (a *App) apiMe(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("diary_auth")
	if err == nil && cookie.Value == "ok" {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"authenticated": true, "username": a.Cfg.UI.Username, "avatar_url": a.Cfg.UI.AvatarURL})
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"authenticated": false})
}

func (a *App) apiListEntries(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	fromStr := strings.TrimSpace(r.URL.Query().Get("from"))
	toStr := strings.TrimSpace(r.URL.Query().Get("to"))

	var entries []*Entry
	var err error
	if fromStr != "" || toStr != "" {
		if fromStr == "" || toStr == "" {
			http.Error(w, "from/to required", http.StatusBadRequest)
			return
		}
		from, errFrom := parseDateOnly(fromStr)
		to, errTo := parseDateOnly(toStr)
		if errFrom != nil || errTo != nil || to.Before(from) {
			http.Error(w, "Invalid date range", http.StatusBadRequest)
			return
		}
		toExclusive := to.AddDate(0, 0, 1)
		entries, err = a.listEntriesByRange(from, toExclusive, offset, limit)
	} else {
		entries, err = a.listEntries(offset, limit)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type responseEntry struct {
		Entry
		Snippet string `json:"snippet"`
	}
	out := make([]responseEntry, len(entries))
	for i, e := range entries {
		out[i] = responseEntry{Entry: *e, Snippet: makeSnippet(e.Content, 120)}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (a *App) apiGetEntryByDate(w http.ResponseWriter, r *http.Request) {
	date := r.PathValue("date")
	if len(date) != 10 {
		http.Error(w, "Invalid date format", http.StatusBadRequest)
		return
	}
	entry, err := a.getEntryByDate(date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if entry == nil {
		http.Error(w, "Entry not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

func (a *App) apiSaveEntry(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Date            string   `json:"date"`
		Content         string   `json:"content"`
		Title           string   `json:"title"`
		Mood            int      `json:"mood"`
		Fulfill         int      `json:"fulfillment"`
		BaseWeather     string   `json:"base_weather"`
		AmbientWeathers []string `json:"ambient_weathers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	req.Date = strings.TrimSpace(req.Date)
	req.Content = strings.TrimSpace(req.Content)
	req.Title = strings.TrimSpace(req.Title)

	if len(req.Date) != 10 {
		http.Error(w, "Invalid date", http.StatusBadRequest)
		return
	}

	existing, err := a.getEntryByDate(req.Date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Mood = normalizeRating(req.Mood)
	req.Fulfill = normalizeRating(req.Fulfill)
	baseWeather, ok := normalizeBaseWeather(req.BaseWeather)
	if !ok {
		http.Error(w, "Invalid base weather", http.StatusBadRequest)
		return
	}
	ambientWeathers, ok := normalizeAmbientWeathers(req.AmbientWeathers)
	if !ok {
		http.Error(w, "Invalid ambient weathers", http.StatusBadRequest)
		return
	}

	contentChanged := existing == nil || req.Content != existing.Content
	titleChanged := false
	if existing != nil {
		titleChanged = req.Title != existing.Title
	} else {
		titleChanged = req.Title != ""
	}
	autoTitle := existing != nil && existing.AutoTitle
	shouldGenerateTitle := false
	contentForTitle := req.Content
	dateForTitle := req.Date

	if existing == nil && req.Title == "" {
		req.Title = fallbackTitle(req.Content, req.Date)
		autoTitle = true
		shouldGenerateTitle = strings.TrimSpace(req.Content) != ""
	} else if existing != nil && titleChanged {
		autoTitle = false
	} else if existing != nil && contentChanged && existing.AutoTitle {
		autoTitle = true
		shouldGenerateTitle = strings.TrimSpace(req.Content) != ""
	}

	if err := a.upsertByDateWithUpdated(req.Date, req.Title, req.Content, req.Mood, req.Fulfill, baseWeather, ambientWeathers, autoTitle, contentChanged, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	entry, err := a.getEntryByDate(req.Date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if entry == nil {
		http.Error(w, "Entry not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(entry)

	if shouldGenerateTitle {
		go a.generateTitleInBackground(dateForTitle, contentForTitle)
	}
}

func (a *App) apiSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	parseList := func(key string) []string {
		values := r.URL.Query()[key]
		if len(values) == 0 {
			return nil
		}
		out := make([]string, 0, len(values))
		for _, value := range values {
			for _, item := range strings.Split(value, ",") {
				item = strings.TrimSpace(item)
				if item != "" {
					out = append(out, item)
				}
			}
		}
		return out
	}

	baseWeather := strings.TrimSpace(r.URL.Query().Get("base_weather"))
	if baseWeather != "" {
		var ok bool
		baseWeather, ok = normalizeBaseWeather(baseWeather)
		if !ok {
			http.Error(w, "Invalid base weather", http.StatusBadRequest)
			return
		}
	}

	ambientWeathers, ok := normalizeAmbientWeathers(parseList("ambient"))
	if !ok {
		http.Error(w, "Invalid ambient weathers", http.StatusBadRequest)
		return
	}

	parseRatings := func(key string) ([]int, error) {
		values := parseList(key)
		if len(values) == 0 {
			return nil, nil
		}
		seen := map[int]struct{}{}
		out := make([]int, 0, len(values))
		for _, value := range values {
			parsed, err := strconv.Atoi(value)
			if err != nil || !isRatingValue(parsed) {
				return nil, fmt.Errorf("invalid rating")
			}
			if _, exists := seen[parsed]; exists {
				continue
			}
			seen[parsed] = struct{}{}
			out = append(out, parsed)
		}
		sort.Ints(out)
		return out, nil
	}

	moods, err := parseRatings("mood")
	if err != nil {
		http.Error(w, "Invalid mood", http.StatusBadRequest)
		return
	}
	fulfillments, err := parseRatings("fulfillment")
	if err != nil {
		http.Error(w, "Invalid fulfillment", http.StatusBadRequest)
		return
	}

	hasFilters := baseWeather != "" || len(ambientWeathers) > 0 || len(moods) > 0 || len(fulfillments) > 0
	if q == "" && !hasFilters {
		json.NewEncoder(w).Encode([]*Entry{})
		return
	}

	results, err := a.searchEntries(q, offset, limit, baseWeather, ambientWeathers, moods, fulfillments)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (a *App) apiStats(w http.ResponseWriter, r *http.Request) {
	end := time.Now()
	start := end.AddDate(0, 0, -364)
	entries, err := a.listEntriesByRangeAll(start, end.AddDate(0, 0, 1))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	dayEnd := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location()).AddDate(0, 0, 1)
	last7Start := dayEnd.AddDate(0, 0, -7)
	last30Start := dayEnd.AddDate(0, 0, -30)

	resp := map[string]any{
		"weeks": buildContributionWeeks(entries, end),
		"periods": map[string]PeriodStats{
			"last30": buildPeriodStats(entries, last30Start, dayEnd),
			"last7":  buildPeriodStats(entries, last7Start, dayEnd),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (a *App) apiGetSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"username":     a.Cfg.UI.Username,
		"avatar_url":   a.Cfg.UI.AvatarURL,
		"token_mask":   maskToken(a.Cfg.Auth.Token),
		"llm_enabled":  a.Cfg.LLM.Enabled,
		"llm_base_url": a.Cfg.LLM.BaseURL,
		"llm_model":    a.Cfg.LLM.Model,
		"llm_prompt":   a.Cfg.LLM.Prompt,
		"llm_key_mask": maskToken(a.Cfg.LLM.APIKey),
	})
}

func (a *App) apiGenerateTitle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string `json:"content"`
		Date    string `json:"date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	req.Content = strings.TrimSpace(req.Content)
	req.Date = strings.TrimSpace(req.Date)
	if req.Date == "" {
		// optional: use today
		req.Date = time.Now().Format("2006-01-02")
	}
	title := a.generateTitle(req.Content, req.Date)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"title": title})
}

func (a *App) apiGenerateTitleAndSave(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string `json:"content"`
		Date    string `json:"date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	req.Content = strings.TrimSpace(req.Content)
	req.Date = strings.TrimSpace(req.Date)
	if req.Date == "" {
		req.Date = time.Now().Format("2006-01-02")
	}

	// Generate title via existing logic (may call LLM)
	title := a.generateTitle(req.Content, req.Date)

	// Fetch existing entry to preserve other fields
	existing, err := a.getEntryByDate(req.Date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	mood := 3
	fulfill := 3
	baseWeather := ""
	ambient := []string{}
	if existing != nil {
		mood = existing.Mood
		fulfill = existing.Fulfill
		baseWeather = existing.BaseWeather
		ambient = existing.AmbientWeathers
	}

	now := time.Now().UTC()
	// Save with autoTitle = true, contentChanged = false (we're only updating title)
	if err := a.upsertByDateWithUpdated(req.Date, title, req.Content, mood, fulfill, baseWeather, ambient, true, false, &now); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entry, err := a.getEntryByDate(req.Date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if entry == nil {
		http.Error(w, "Entry not found after save", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

func (a *App) apiUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username   string `json:"username"`
		AvatarURL  string `json:"avatar_url"`
		Token      string `json:"token"`
		LLMEnabled bool   `json:"llm_enabled"`
		LLMBaseURL string `json:"llm_base_url"`
		LLMAPIKey  string `json:"llm_api_key"`
		LLMModel   string `json:"llm_model"`
		LLMPrompt  string `json:"llm_prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	a.Cfg.UI.Username = strings.TrimSpace(req.Username)
	a.Cfg.UI.AvatarURL = strings.TrimSpace(req.AvatarURL)

	newToken := strings.TrimSpace(req.Token)
	if newToken != "" {
		a.Cfg.Auth.Token = newToken
		a.Token = newToken
	}

	a.Cfg.LLM.Enabled = req.LLMEnabled
	a.Cfg.LLM.BaseURL = strings.TrimSpace(req.LLMBaseURL)
	a.Cfg.LLM.Model = strings.TrimSpace(req.LLMModel)
	a.Cfg.LLM.Prompt = strings.TrimSpace(req.LLMPrompt)
	newLLMKey := strings.TrimSpace(req.LLMAPIKey)
	if newLLMKey != "" {
		a.Cfg.LLM.APIKey = newLLMKey
	}

	if err := saveConfig(a.ConfigPath, a.Cfg); err != nil {
		http.Error(w, "Failed to save config", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (a *App) apiExport(w http.ResponseWriter, r *http.Request) {
	entries, err := a.listAllEntries()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	thoughts, err := a.listAllThoughts()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type jsonEntry struct {
		Date            string   `json:"date"`
		Title           string   `json:"title"`
		Content         string   `json:"content"`
		UpdatedAt       string   `json:"updated_at"`
		BaseWeather     string   `json:"base_weather,omitempty"`
		AmbientWeathers []string `json:"ambient_weathers,omitempty"`
	}
	out := make([]jsonEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, jsonEntry{
			Date:            e.Created.Local().Format("2006-01-02"),
			Title:           e.Title,
			Content:         e.Content,
			UpdatedAt:       e.Updated.UTC().Format(time.RFC3339),
			BaseWeather:     e.BaseWeather,
			AmbientWeathers: e.AmbientWeathers,
		})
	}
	type jsonThought struct {
		UID       string `json:"uid"`
		Content   string `json:"content"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}
	thoughtsOut := make([]jsonThought, 0, len(thoughts))
	for _, thought := range thoughts {
		thoughtsOut = append(thoughtsOut, jsonThought{
			UID:       thought.UID,
			Content:   thought.Content,
			CreatedAt: thought.Created.UTC().Format(time.RFC3339Nano),
			UpdatedAt: thought.Updated.UTC().Format(time.RFC3339Nano),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=diary-export-%s.json", time.Now().Format("20060102")))
	json.NewEncoder(w).Encode(map[string]any{"entries": out, "thoughts": thoughtsOut})
}

func (a *App) apiImport(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Parse form failed", http.StatusBadRequest)
		return
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file uploaded", http.StatusBadRequest)
		return
	}
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		http.Error(w, "Read failed", http.StatusInternalServerError)
		return
	}

	var payload struct {
		Entries  []json.RawMessage `json:"entries"`
		Thoughts []json.RawMessage `json:"thoughts"`
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		http.Error(w, "JSON parse failed", http.StatusBadRequest)
		return
	}

	count := 0
	for _, raw := range payload.Entries {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		norm := func(s string) string { return strings.ToLower(strings.ReplaceAll(s, "_", "")) }
		get := func(key string) string {
			for k, v := range m {
				if norm(k) == key {
					if vs, ok := v.(string); ok {
						return vs
					}
				}
			}
			return ""
		}
		dateStr := strings.TrimSpace(get("date"))
		title := strings.TrimSpace(get("title"))
		content := strings.TrimSpace(get("content"))
		updatedStr := strings.TrimSpace(get("updatedat"))
		baseWeather, _ := getStringAny(m, "baseweather")
		ambientWeathers := getStringSliceAny(m, "ambientweathers")

		if len(dateStr) != 10 {
			continue
		}
		var updPtr *time.Time
		if updatedStr != "" {
			if t, err := time.Parse(time.RFC3339, updatedStr); err == nil {
				tt := t.UTC()
				updPtr = &tt
			} else if t2, err2 := time.Parse(time.RFC3339Nano, updatedStr); err2 == nil {
				tt := t2.UTC()
				updPtr = &tt
			} else if t3, err3 := time.ParseInLocation("2006-01-02 15:04:05", updatedStr, time.Local); err3 == nil {
				tt := t3.UTC()
				updPtr = &tt
			}
		}
		normalizedBaseWeather, okBase := normalizeBaseWeather(baseWeather)
		if !okBase {
			normalizedBaseWeather = ""
		}
		normalizedAmbientWeathers, okAmbient := normalizeAmbientWeathers(ambientWeathers)
		if !okAmbient {
			normalizedAmbientWeathers = nil
		}
		if err := a.upsertByDateWithUpdated(dateStr, title, content, 3, 3, normalizedBaseWeather, normalizedAmbientWeathers, false, true, updPtr); err == nil {
			count++
		}
	}

	thoughtCount := 0
	for _, raw := range payload.Thoughts {
		var thoughtData map[string]any
		if err := json.Unmarshal(raw, &thoughtData); err != nil {
			continue
		}
		content, _ := getStringAny(thoughtData, "content")
		uid, _ := getStringAny(thoughtData, "uid")
		createdStr, _ := getStringAny(thoughtData, "createdat")
		updatedStr, _ := getStringAny(thoughtData, "updatedat")
		if uid == "" || content == "" || createdStr == "" {
			continue
		}
		createdAt, err := time.Parse(time.RFC3339Nano, createdStr)
		if err != nil {
			continue
		}
		updatedAt := createdAt
		if updatedStr != "" {
			parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedStr)
			if err != nil {
				continue
			}
			updatedAt = parsedUpdatedAt
		}
		if err := a.upsertImportedThought(uid, content, createdAt, updatedAt); err == nil {
			thoughtCount++
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"imported": count, "thoughts_imported": thoughtCount})
}
