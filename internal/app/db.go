package app

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

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
            updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
            edit_count INTEGER NOT NULL DEFAULT 0,
            mood INTEGER NOT NULL DEFAULT 3,
            fulfillment INTEGER NOT NULL DEFAULT 3,
            base_weather TEXT NOT NULL DEFAULT '',
            ambient_weathers TEXT NOT NULL DEFAULT '[]',
            auto_title INTEGER NOT NULL DEFAULT 0
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
	if _, err := db.Exec(`ALTER TABLE entries ADD COLUMN edit_count INTEGER NOT NULL DEFAULT 0`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
			return fmt.Errorf("add edit_count column: %w", err)
		}
	}
	if _, err := db.Exec(`ALTER TABLE entries ADD COLUMN mood INTEGER NOT NULL DEFAULT 3`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
			return fmt.Errorf("add mood column: %w", err)
		}
	}
	if _, err := db.Exec(`ALTER TABLE entries ADD COLUMN fulfillment INTEGER NOT NULL DEFAULT 3`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
			return fmt.Errorf("add fulfillment column: %w", err)
		}
	}
	if _, err := db.Exec(`ALTER TABLE entries ADD COLUMN auto_title INTEGER NOT NULL DEFAULT 0`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
			return fmt.Errorf("add auto_title column: %w", err)
		}
	}
	if _, err := db.Exec(`ALTER TABLE entries ADD COLUMN base_weather TEXT NOT NULL DEFAULT ''`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
			return fmt.Errorf("add base_weather column: %w", err)
		}
	}
	if _, err := db.Exec(`ALTER TABLE entries ADD COLUMN ambient_weathers TEXT NOT NULL DEFAULT '[]'`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
			return fmt.Errorf("add ambient_weathers column: %w", err)
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

func scanEntry(scanner interface{ Scan(dest ...any) error }, e *Entry) error {
	var createdStr, updatedStr string
	var ambientWeathers string
	if err := scanner.Scan(&e.ID, &e.Title, &e.Content, &createdStr, &updatedStr, &e.Day, &e.EditCount, &e.Mood, &e.Fulfill, &e.BaseWeather, &ambientWeathers, &e.AutoTitle); err != nil {
		return err
	}
	e.Created, _ = time.Parse(time.RFC3339Nano, createdStr)
	e.Updated, _ = time.Parse(time.RFC3339Nano, updatedStr)
	e.AmbientWeathers = decodeAmbientWeathers(ambientWeathers)
	return nil
}

func (a *App) listAllEntries() ([]*Entry, error) {
	rows, err := a.DB.Query(`SELECT id, title, content, created_at, updated_at, day, edit_count, mood, fulfillment, base_weather, ambient_weathers, auto_title FROM entries ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*Entry
	for rows.Next() {
		var e Entry
		if err := scanEntry(rows, &e); err != nil {
			return nil, err
		}
		list = append(list, &e)
	}
	return list, nil
}

func (a *App) upsertByDateWithUpdated(dateStr, title, content string, mood, fulfill int, baseWeather string, ambientWeathers []string, autoTitle, contentChanged bool, updatedAt *time.Time) error {
	if len(dateStr) != 10 {
		return fmt.Errorf("bad date")
	}
	dayLocal, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
	if err != nil {
		return err
	}
	mood = normalizeRating(mood)
	fulfill = normalizeRating(fulfill)
	baseWeather, ok := normalizeBaseWeather(baseWeather)
	if !ok {
		return fmt.Errorf("invalid base weather")
	}
	ambientWeathers, ok = normalizeAmbientWeathers(ambientWeathers)
	if !ok {
		return fmt.Errorf("invalid ambient weathers")
	}
	ambientWeathersJSON := encodeAmbientWeathers(ambientWeathers)
	createdUTC := dayLocal.UTC()
	existing, _ := a.getEntryByDate(dateStr)
	autoTitleInt := 0
	if autoTitle {
		autoTitleInt = 1
	}
	if existing == nil {
		upd := time.Now().UTC()
		if updatedAt != nil {
			upd = updatedAt.UTC()
		}
		_, err = a.DB.Exec(`INSERT INTO entries(day, title, content, created_at, updated_at, edit_count, mood, fulfillment, base_weather, ambient_weathers, auto_title) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, dateStr, title, content, createdUTC, upd, 1, mood, fulfill, baseWeather, ambientWeathersJSON, autoTitleInt)
		return err
	}
	upd := time.Now().UTC()
	if updatedAt != nil {
		upd = updatedAt.UTC()
	}
	if contentChanged {
		_, err = a.DB.Exec(`UPDATE entries SET title=?, content=?, updated_at=?, edit_count=edit_count+1, mood=?, fulfillment=?, base_weather=?, ambient_weathers=?, auto_title=? WHERE id=?`, title, content, upd, mood, fulfill, baseWeather, ambientWeathersJSON, autoTitleInt, existing.ID)
		return err
	}
	_, err = a.DB.Exec(`UPDATE entries SET title=?, content=?, updated_at=?, mood=?, fulfillment=?, base_weather=?, ambient_weathers=?, auto_title=? WHERE id=?`, title, content, upd, mood, fulfill, baseWeather, ambientWeathersJSON, autoTitleInt, existing.ID)
	return err
}

func (a *App) getEntryByDate(date string) (*Entry, error) {
	rows, err := a.DB.Query(`SELECT id, title, content, created_at, updated_at, day, edit_count, mood, fulfillment, base_weather, ambient_weathers, auto_title FROM entries WHERE day=? LIMIT 1`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if rows.Next() {
		var e Entry
		if err := scanEntry(rows, &e); err != nil {
			return nil, err
		}
		return &e, nil
	}
	return nil, nil
}

func (a *App) listEntries(offset, limit int) ([]*Entry, error) {
	rows, err := a.DB.Query(`SELECT id, title, content, created_at, updated_at, day, edit_count, mood, fulfillment, base_weather, ambient_weathers, auto_title FROM entries ORDER BY day DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*Entry
	for rows.Next() {
		var e Entry
		if err := scanEntry(rows, &e); err != nil {
			return nil, err
		}
		list = append(list, &e)
	}
	return list, nil
}

func (a *App) searchEntries(q string, limit int) ([]*Entry, error) {
	like := "%" + q + "%"
	rows, err := a.DB.Query(`
        SELECT id, title, content, created_at, updated_at, day, edit_count, mood, fulfillment, base_weather, ambient_weathers, auto_title
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
		if err := scanEntry(rows, &e); err != nil {
			return nil, err
		}
		list = append(list, &e)
	}
	return list, nil
}

func (a *App) listEntriesByRange(from, to time.Time, offset, limit int) ([]*Entry, error) {
	rows, err := a.DB.Query(`
        SELECT id, title, content, created_at, updated_at, day, edit_count, mood, fulfillment, base_weather, ambient_weathers, auto_title
        FROM entries
        WHERE day >= ? AND day < ?
        ORDER BY day DESC
        LIMIT ? OFFSET ?
    `, from.Format("2006-01-02"), to.Format("2006-01-02"), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*Entry
	for rows.Next() {
		var e Entry
		if err := scanEntry(rows, &e); err != nil {
			return nil, err
		}
		list = append(list, &e)
	}
	return list, nil
}

func (a *App) listEntriesByRangeAll(from, to time.Time) ([]*Entry, error) {
	rows, err := a.DB.Query(`
        SELECT id, title, content, created_at, updated_at, day, edit_count, mood, fulfillment, base_weather, ambient_weathers, auto_title
        FROM entries
        WHERE day >= ? AND day < ?
        ORDER BY day ASC
    `, from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*Entry
	for rows.Next() {
		var e Entry
		if err := scanEntry(rows, &e); err != nil {
			return nil, err
		}
		list = append(list, &e)
	}
	return list, nil
}
