package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	_ "modernc.org/sqlite"
)

// App holds app-wide dependencies
type App struct {
	DB         *sql.DB
	Token      string
	Cfg        *Config
	ConfigPath string
}

type Entry struct {
	ID      int64     `json:"id"`
	Title   string    `json:"title"`
	Content string    `json:"content"`
	Created time.Time `json:"created_at"`
	Updated time.Time `json:"updated_at"`
	Day     string    `json:"day"` // calendar day YYYY-MM-DD
}

func main() {
	// Load config (DIARY_CONFIG -> data/config.toml -> config.toml)
	cfgPath := os.Getenv("DIARY_CONFIG")
	if cfgPath == "" {
		if _, err := os.Stat("data/config.toml"); err == nil {
			cfgPath = "data/config.toml"
		} else {
			cfgPath = "config.toml"
		}
	}
	cfg := loadConfig(cfgPath)
	// env override for token if provided
	if envTok := os.Getenv("DIARY_LOGIN_TOKEN"); envTok != "" {
		cfg.Auth.Token = envTok
	}

	// Open SQLite database (created if missing)
	db, err := openDB(cfg.Database.Path)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := migrate(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	app := &App{DB: db, Token: cfg.Auth.Token, Cfg: cfg, ConfigPath: cfgPath}

	mux := http.NewServeMux()

	// API Routes
	mux.HandleFunc("POST /api/login", app.apiLogin)
	mux.HandleFunc("POST /api/logout", app.apiLogout)
	mux.HandleFunc("GET /api/me", app.apiMe)

	// Protected API Routes
	mux.Handle("GET /api/entries", app.requireAuth(http.HandlerFunc(app.apiListEntries)))
	mux.Handle("POST /api/entries", app.requireAuth(http.HandlerFunc(app.apiSaveEntry)))
	mux.Handle("GET /api/entries/date/{date}", app.requireAuth(http.HandlerFunc(app.apiGetEntryByDate)))
	mux.Handle("GET /api/search", app.requireAuth(http.HandlerFunc(app.apiSearch)))
	mux.Handle("GET /api/settings", app.requireAuth(http.HandlerFunc(app.apiGetSettings)))
	mux.Handle("POST /api/settings", app.requireAuth(http.HandlerFunc(app.apiUpdateSettings)))
	mux.Handle("GET /api/export", app.requireAuth(http.HandlerFunc(app.apiExport)))
	mux.Handle("POST /api/import", app.requireAuth(http.HandlerFunc(app.apiImport)))

	// Static Frontend (SPA fallback)
	// We serve the 'frontend/dist' directory.
	// If a file exists, serve it. If not, and it's not an API call, serve index.html.
	fs := http.FileServer(http.Dir("frontend/dist"))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		// Check if file exists in dist
		path := filepath.Join("frontend/dist", filepath.Clean(r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		// Fallback to index.html for SPA routing
		http.ServeFile(w, r, "frontend/dist/index.html")
	}))

	addr := cfg.Server.Address
	log.Printf("Server listening on http://localhost%s\n", addr)
	if err := http.ListenAndServe(addr, withCommonHeaders(mux)); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server: %v", err)
	}
}

func withCommonHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS for development (allow localhost:5173)
		// In production, we serve same origin, but for dev we might need this if running separate vite server
		// For simplicity in this refactor, we assume we might run vite separately during dev.
		// But strictly speaking, if we proxy or build, we don't need CORS.
		// Let's add basic CORS for dev convenience if needed, or just rely on Vite proxy.
		// We'll rely on Vite proxy for dev, so no loose CORS needed.

		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// Auth Middleware
func (a *App) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("diary_auth")
		if err == nil && cookie.Value == "ok" {
			next.ServeHTTP(w, r)
			return
		}
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

// API Handlers

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

	entries, err := a.listEntries(offset, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Helper to create snippet
	type J struct {
		Entry
		Snippet string `json:"snippet"`
	}
	out := make([]J, len(entries))
	for i, e := range entries {
		out[i] = J{Entry: *e, Snippet: makeSnippet(e.Content, 120)}
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
		// Return 404 or just empty? 404 is better for REST
		http.Error(w, "Entry not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

func (a *App) apiSaveEntry(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Date    string `json:"date"`
		Content string `json:"content"`
		Title   string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	req.Date = strings.TrimSpace(req.Date)
	req.Content = strings.TrimSpace(req.Content)
	req.Title = strings.TrimSpace(req.Title)

	if req.Title == "" {
		// auto title
		rn := []rune(req.Content)
		max := 16
		if len(rn) < max {
			max = len(rn)
		}
		if max > 0 {
			req.Title = string(rn[:max])
		} else {
			req.Title = req.Date
		}
	}

	if len(req.Date) != 10 {
		http.Error(w, "Invalid date", http.StatusBadRequest)
		return
	}

	if err := a.upsertByDateWithUpdated(req.Date, req.Title, req.Content, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "saved"})
}

func (a *App) apiSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		json.NewEncoder(w).Encode([]*Entry{})
		return
	}

	results, err := a.searchEntries(q, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Date matching logic from original
	if from, to, ok := dateRangeFromQuery(q); ok {
		dr, _ := a.listEntriesByRange(from, to, 200)
		seen := map[int64]struct{}{}
		for _, e := range results {
			seen[e.ID] = struct{}{}
		}
		for _, e := range dr {
			if _, ok := seen[e.ID]; !ok {
				results = append(results, e)
				seen[e.ID] = struct{}{}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (a *App) apiGetSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"username":   a.Cfg.UI.Username,
		"avatar_url": a.Cfg.UI.AvatarURL,
		// Don't send token back for security, or send masked if needed?
		// Original sent masked.
		"token_mask": maskToken(a.Cfg.Auth.Token),
	})
}

func (a *App) apiUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username  string `json:"username"`
		AvatarURL string `json:"avatar_url"`
		Token     string `json:"token"`
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

	type JSONEntry struct {
		Date      string `json:"date"`
		Title     string `json:"title"`
		Content   string `json:"content"`
		UpdatedAt string `json:"updated_at"`
	}
	out := make([]JSONEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, JSONEntry{
			Date:      e.Created.Local().Format("2006-01-02"),
			Title:     e.Title,
			Content:   e.Content,
			UpdatedAt: e.Updated.UTC().Format(time.RFC3339),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=diary-export-%s.json", time.Now().Format("20060102")))
	json.NewEncoder(w).Encode(map[string]any{"entries": out})
}

func (a *App) apiImport(w http.ResponseWriter, r *http.Request) {
	// Expecting multipart upload with "file"
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
		Entries []json.RawMessage `json:"entries"`
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		http.Error(w, "JSON parse failed", http.StatusBadRequest)
		return
	}

	// Reuse existing logic
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
			} else {
				if t3, err3 := time.ParseInLocation("2006-01-02 15:04:05", updatedStr, time.Local); err3 == nil {
					tt := t3.UTC()
					updPtr = &tt
				}
			}
		}
		if err := a.upsertByDateWithUpdated(dateStr, title, content, updPtr); err == nil {
			count++
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"imported": count})
}

// --- Helpers & DB Logic (Preserved) ---

func makeSnippet(s string, n int) string {
	if n <= 0 {
		return ""
	}
	s = strings.ReplaceAll(s, "\n", " ")
	r := []rune(s)
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}

func subtleEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

func openDB(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, db.Ping()
}

func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
			updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		);`,
		`CREATE INDEX IF NOT EXISTS idx_entries_created_at ON entries(created_at);`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	if _, err := db.Exec(`ALTER TABLE entries ADD COLUMN day TEXT`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
			return fmt.Errorf("add day column: %w", err)
		}
	}
	if _, err := db.Exec(`UPDATE entries SET day = DATE(created_at,'localtime') WHERE day IS NULL OR day=''`); err != nil {
		return fmt.Errorf("backfill day: %w", err)
	}
	if _, err := db.Exec(`UPDATE entries SET created_at = day || 'T00:00:00Z' WHERE day IS NOT NULL AND day!='' AND created_at NOT LIKE day || '%'`); err != nil {
		log.Printf("[WARN] normalize created_at: %v", err)
	}
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_entries_day ON entries(day)`); err != nil {
		return fmt.Errorf("create unique day index: %w", err)
	}
	return nil
}

type Config struct {
	Server struct {
		Address string `toml:"address"`
	} `toml:"server"`
	Auth struct {
		Token string `toml:"token"`
	} `toml:"auth"`
	Database struct {
		Path string `toml:"path"`
	} `toml:"database"`
	UI struct {
		AvatarURL string `toml:"avatar_url"`
		Username  string `toml:"username"`
	} `toml:"ui"`
}

func loadConfig(path string) *Config {
	cfg := &Config{}
	cfg.Server.Address = ":8080"
	cfg.Auth.Token = "changeme"
	cfg.Database.Path = "data/diary.db"
	cfg.UI.AvatarURL = ""
	cfg.UI.Username = ""

	b, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[INFO] config not found (%s), using defaults", path)
		return cfg
	}
	if err := toml.Unmarshal(b, cfg); err != nil {
		log.Printf("[WARN] parse config failed: %v; using defaults", err)
	}
	return cfg
}

func saveConfig(path string, cfg *Config) error {
	b, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

func maskToken(tok string) string {
	if len(tok) <= 6 {
		return "******"
	}
	return tok[:2] + strings.Repeat("*", len(tok)-4) + tok[len(tok)-2:]
}

func (a *App) listAllEntries() ([]*Entry, error) {
	rows, err := a.DB.Query(`SELECT id, title, content, created_at, updated_at, day FROM entries ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*Entry
	for rows.Next() {
		var e Entry
		var createdStr, updatedStr string
		if err := rows.Scan(&e.ID, &e.Title, &e.Content, &createdStr, &updatedStr, &e.Day); err != nil {
			return nil, err
		}
		e.Created, _ = time.Parse(time.RFC3339Nano, createdStr)
		e.Updated, _ = time.Parse(time.RFC3339Nano, updatedStr)
		list = append(list, &e)
	}
	return list, nil
}

func (a *App) upsertByDateWithUpdated(dateStr, title, content string, updatedAt *time.Time) error {
	if len(dateStr) != 10 {
		return fmt.Errorf("bad date")
	}
	dayLocal, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
	if err != nil {
		return err
	}
	createdUTC := dayLocal.UTC()
	existing, _ := a.getEntryByDate(dateStr)
	if existing == nil {
		upd := createdUTC
		if updatedAt != nil {
			upd = updatedAt.UTC()
		}
		_, err = a.DB.Exec(`INSERT INTO entries(day, title, content, created_at, updated_at) VALUES(?,?,?,?,?)`, dateStr, title, content, createdUTC, upd)
		return err
	}
	upd := time.Now().UTC()
	if updatedAt != nil {
		upd = updatedAt.UTC()
	}
	_, err = a.DB.Exec(`UPDATE entries SET title=?, content=?, updated_at=? WHERE id=?`, title, content, upd, existing.ID)
	return err
}

func (a *App) getEntryByDate(date string) (*Entry, error) {
	rows, err := a.DB.Query(`SELECT id, title, content, created_at, updated_at, day FROM entries WHERE day=? LIMIT 1`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if rows.Next() {
		var e Entry
		var createdStr, updatedStr string
		if err := rows.Scan(&e.ID, &e.Title, &e.Content, &createdStr, &updatedStr, &e.Day); err != nil {
			return nil, err
		}
		e.Created, _ = time.Parse(time.RFC3339Nano, createdStr)
		e.Updated, _ = time.Parse(time.RFC3339Nano, updatedStr)
		return &e, nil
	}
	return nil, nil
}

func (a *App) listEntries(offset, limit int) ([]*Entry, error) {
	rows, err := a.DB.Query(`SELECT id, title, content, created_at, updated_at, day FROM entries ORDER BY day DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*Entry
	for rows.Next() {
		var e Entry
		var createdStr, updatedStr string
		if err := rows.Scan(&e.ID, &e.Title, &e.Content, &createdStr, &updatedStr, &e.Day); err != nil {
			return nil, err
		}
		e.Created, _ = time.Parse(time.RFC3339Nano, createdStr)
		e.Updated, _ = time.Parse(time.RFC3339Nano, updatedStr)
		list = append(list, &e)
	}
	return list, nil
}

func (a *App) searchEntries(q string, limit int) ([]*Entry, error) {
	like := "%" + q + "%"
	rows, err := a.DB.Query(`
		SELECT id, title, content, created_at, updated_at, day
		FROM entries
		WHERE title LIKE ? OR content LIKE ?
		ORDER BY day DESC
		LIMIT ?
	`, like, like, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*Entry
	for rows.Next() {
		var e Entry
		var createdStr, updatedStr string
		if err := rows.Scan(&e.ID, &e.Title, &e.Content, &createdStr, &updatedStr, &e.Day); err != nil {
			return nil, err
		}
		e.Created, _ = time.Parse(time.RFC3339Nano, createdStr)
		e.Updated, _ = time.Parse(time.RFC3339Nano, updatedStr)
		list = append(list, &e)
	}
	return list, nil
}

func (a *App) listEntriesByRange(from, to time.Time, limit int) ([]*Entry, error) {
	rows, err := a.DB.Query(`
		SELECT id, title, content, created_at, updated_at, day
		FROM entries
		WHERE day >= ? AND day < ?
		ORDER BY day DESC
		LIMIT ?
	`, from.Format("2006-01-02"), to.Format("2006-01-02"), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*Entry
	for rows.Next() {
		var e Entry
		var createdStr, updatedStr string
		if err := rows.Scan(&e.ID, &e.Title, &e.Content, &createdStr, &updatedStr, &e.Day); err != nil {
			return nil, err
		}
		e.Created, _ = time.Parse(time.RFC3339Nano, createdStr)
		e.Updated, _ = time.Parse(time.RFC3339Nano, updatedStr)
		list = append(list, &e)
	}
	return list, nil
}

func dateRangeFromQuery(q string) (time.Time, time.Time, bool) {
	s := strings.TrimSpace(q)
	if s == "" {
		return time.Time{}, time.Time{}, false
	}
	reCn := regexp.MustCompile(`^\s*(\d{4})\s*年\s*(\d{1,2})\s*月\s*(\d{1,2})\s*日?\s*$`)
	if m := reCn.FindStringSubmatch(s); m != nil {
		y := atoi(m[1])
		mo := atoi(m[2])
		d := atoi(m[3])
		from := time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.Local)
		to := from.AddDate(0, 0, 1)
		return from, to, true
	}
	reDash := regexp.MustCompile(`^\s*(\d{4})[-/](\d{1,2})[-/](\d{1,2})\s*$`)
	if m := reDash.FindStringSubmatch(s); m != nil {
		y := atoi(m[1])
		mo := atoi(m[2])
		d := atoi(m[3])
		from := time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.Local)
		to := from.AddDate(0, 0, 1)
		return from, to, true
	}
	re8 := regexp.MustCompile(`^\s*(\d{4})(\d{2})(\d{2})\s*$`)
	if m := re8.FindStringSubmatch(s); m != nil {
		y := atoi(m[1])
		mo := atoi(m[2])
		d := atoi(m[3])
		from := time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.Local)
		to := from.AddDate(0, 0, 1)
		return from, to, true
	}
	reMonth := regexp.MustCompile(`^\s*(?:(\d{4})\s*年\s*)?(\d{1,2})\s*月\s*$`)
	if m := reMonth.FindStringSubmatch(s); m != nil {
		year := time.Now().Year()
		if m[1] != "" {
			year = atoi(m[1])
		}
		mo := atoi(m[2])
		from := time.Date(year, time.Month(mo), 1, 0, 0, 0, 0, time.Local)
		to := from.AddDate(0, 1, 0)
		return from, to, true
	}
	reYear := regexp.MustCompile(`^\s*(\d{4})\s*年?\s*$`)
	if m := reYear.FindStringSubmatch(s); m != nil {
		year := atoi(m[1])
		from := time.Date(year, 1, 1, 0, 0, 0, 0, time.Local)
		to := from.AddDate(1, 0, 0)
		return from, to, true
	}
	reDay := regexp.MustCompile(`^\s*(\d{1,2})\s*日\s*$`)
	if m := reDay.FindStringSubmatch(s); m != nil {
		now := time.Now()
		d := atoi(m[1])
		from := time.Date(now.Year(), now.Month(), d, 0, 0, 0, 0, time.Local)
		to := from.AddDate(0, 0, 1)
		return from, to, true
	}
	return time.Time{}, time.Time{}, false
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
