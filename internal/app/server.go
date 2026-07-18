package app

import (
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func Run() error {
	cfgPath := resolveConfigPath()
	cfg := loadConfig(cfgPath)
	if envTok := os.Getenv("DIARY_LOGIN_TOKEN"); envTok != "" {
		cfg.Auth.Token = envTok
	}

	db, err := openDB(cfg.Database.Path)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := migrate(db); err != nil {
		return err
	}

	app := &App{DB: db, Token: cfg.Auth.Token, Cfg: cfg, ConfigPath: cfgPath}
	addr := cfg.Server.Address

	log.Printf("Server listening on %s\n", listenURL(addr))
	if err := http.ListenAndServe(addr, withCommonHeaders(app.routes())); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/login", a.apiLogin)
	mux.HandleFunc("POST /api/logout", a.apiLogout)
	mux.HandleFunc("GET /api/me", a.apiMe)

	mux.Handle("GET /api/entries", a.requireAuth(http.HandlerFunc(a.apiListEntries)))
	mux.Handle("POST /api/entries", a.requireAuth(http.HandlerFunc(a.apiSaveEntry)))
	mux.Handle("GET /api/entries/date/{date}", a.requireAuth(http.HandlerFunc(a.apiGetEntryByDate)))
	mux.Handle("GET /api/thoughts", a.requireAuth(http.HandlerFunc(a.apiListThoughts)))
	mux.Handle("POST /api/thoughts", a.requireAuth(http.HandlerFunc(a.apiCreateThought)))
	mux.Handle("PUT /api/thoughts/{id}", a.requireAuth(http.HandlerFunc(a.apiUpdateThought)))
	mux.Handle("DELETE /api/thoughts/{id}", a.requireAuth(http.HandlerFunc(a.apiDeleteThought)))
	mux.Handle("GET /api/search", a.requireAuth(http.HandlerFunc(a.apiSearch)))
	mux.Handle("GET /api/stats", a.requireAuth(http.HandlerFunc(a.apiStats)))
	mux.Handle("GET /api/settings", a.requireAuth(http.HandlerFunc(a.apiGetSettings)))
	mux.Handle("POST /api/settings", a.requireAuth(http.HandlerFunc(a.apiUpdateSettings)))
	mux.Handle("POST /api/generate-title", a.requireAuth(http.HandlerFunc(a.apiGenerateTitle)))
	mux.Handle("POST /api/generate-title-and-save", a.requireAuth(http.HandlerFunc(a.apiGenerateTitleAndSave)))
	mux.Handle("GET /api/export", a.requireAuth(http.HandlerFunc(a.apiExport)))
	mux.Handle("POST /api/import", a.requireAuth(http.HandlerFunc(a.apiImport)))

	fs := http.FileServer(http.Dir("frontend/dist"))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		path := filepath.Join("frontend/dist", filepath.Clean(r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, "frontend/dist/index.html")
	}))

	return mux
}
