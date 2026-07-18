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
		`CREATE TABLE IF NOT EXISTS thoughts (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            uid TEXT NOT NULL DEFAULT '',
            content TEXT NOT NULL,
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL
        );`,
		`CREATE INDEX IF NOT EXISTS idx_thoughts_updated_at ON thoughts(updated_at DESC, id DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_thoughts_created_at ON thoughts(created_at);`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	if _, err := db.Exec(`ALTER TABLE thoughts ADD COLUMN uid TEXT NOT NULL DEFAULT ''`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
			return fmt.Errorf("add thought uid column: %w", err)
		}
	}
	if _, err := db.Exec(`UPDATE thoughts SET uid=lower(hex(randomblob(16))) WHERE uid IS NULL OR uid=''`); err != nil {
		return fmt.Errorf("backfill thought uid: %w", err)
	}
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_thoughts_uid ON thoughts(uid)`); err != nil {
		return fmt.Errorf("create thought uid index: %w", err)
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
	if err := normalizeExistingEntryTimestamps(db); err != nil {
		log.Printf("[WARN] normalize timestamps: %v", err)
	}
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_entries_day ON entries(day)`); err != nil {
		return fmt.Errorf("create unique day index: %w", err)
	}
	return nil
}

func normalizeExistingEntryTimestamps(db *sql.DB) error {
	rows, err := db.Query(`SELECT id, day, created_at, updated_at FROM entries WHERE day IS NOT NULL AND day!=''`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type rowData struct {
		id      int64
		day     string
		created string
		updated string
	}
	var rowsData []rowData
	for rows.Next() {
		var r rowData
		if err := rows.Scan(&r.id, &r.day, &r.created, &r.updated); err != nil {
			return err
		}
		rowsData = append(rowsData, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, r := range rowsData {
		created := normalizeTimestampWithDay(r.created, r.day)
		updated := normalizeTimestampWithDay(r.updated, r.day)
		if _, err := db.Exec(`UPDATE entries SET created_at=?, updated_at=? WHERE id=?`, created, updated, r.id); err != nil {
			return err
		}
	}
	return nil
}

func normalizeTimestampWithDay(value, day string) string {
	t, ok := parseTimestampWithDay(value, day)
	if !ok {
		dayTime, err := time.ParseInLocation("2006-01-02 15:04:05", day+" 20:00:00", time.Local)
		if err != nil {
			return time.Now().UTC().Format(time.RFC3339Nano)
		}
		return dayTime.UTC().Format(time.RFC3339Nano)
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTimestampWithDay(value, day string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}

	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t, true
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local); err == nil {
		return t, true
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04", value, time.Local); err == nil {
		return t, true
	}
	if t, err := time.ParseInLocation("2006-01-02", value, time.Local); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 20, 0, 0, 0, time.Local), true
	}
	if t, err := time.ParseInLocation("15:04:05", value, time.Local); err == nil {
		dayTime, err := time.ParseInLocation("2006-01-02", day, time.Local)
		if err != nil {
			return time.Time{}, false
		}
		return time.Date(dayTime.Year(), dayTime.Month(), dayTime.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.Local), true
	}
	if t, err := time.ParseInLocation("15:04", value, time.Local); err == nil {
		dayTime, err := time.ParseInLocation("2006-01-02", day, time.Local)
		if err != nil {
			return time.Time{}, false
		}
		return time.Date(dayTime.Year(), dayTime.Month(), dayTime.Day(), t.Hour(), t.Minute(), 0, 0, time.Local), true
	}
	return time.Time{}, false
}

func scanEntry(scanner interface{ Scan(dest ...any) error }, e *Entry) error {
	var createdStr, updatedStr string
	var ambientWeathers string
	if err := scanner.Scan(&e.ID, &e.Title, &e.Content, &createdStr, &updatedStr, &e.Day, &e.EditCount, &e.Mood, &e.Fulfill, &e.BaseWeather, &ambientWeathers, &e.AutoTitle); err != nil {
		return err
	}
	e.Created, _ = time.Parse(time.RFC3339Nano, normalizeTimestampWithDay(createdStr, e.Day))
	e.Updated, _ = time.Parse(time.RFC3339Nano, normalizeTimestampWithDay(updatedStr, e.Day))
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

func (a *App) searchEntries(q string, offset, limit int, baseWeather string, ambientWeathers []string, moods []int, fulfillments []int) ([]*Entry, error) {
	clauses := []string{}
	args := []any{}
	if q != "" {
		like := "%" + q + "%"
		clauses = append(clauses, "(title LIKE ? OR content LIKE ?)")
		args = append(args, like, like)
	}

	if baseWeather != "" {
		clauses = append(clauses, "base_weather = ?")
		args = append(args, baseWeather)
	}
	if len(ambientWeathers) > 0 {
		parts := make([]string, 0, len(ambientWeathers))
		for _, value := range ambientWeathers {
			parts = append(parts, "ambient_weathers LIKE ?")
			args = append(args, "%\""+value+"\"%")
		}
		clauses = append(clauses, "("+strings.Join(parts, " OR ")+")")
	}
	if len(moods) > 0 {
		placeholders := strings.TrimRight(strings.Repeat("?,", len(moods)), ",")
		clauses = append(clauses, "mood IN ("+placeholders+")")
		for _, value := range moods {
			args = append(args, value)
		}
	}
	if len(fulfillments) > 0 {
		placeholders := strings.TrimRight(strings.Repeat("?,", len(fulfillments)), ",")
		clauses = append(clauses, "fulfillment IN ("+placeholders+")")
		for _, value := range fulfillments {
			args = append(args, value)
		}
	}

	if len(clauses) == 0 {
		return []*Entry{}, nil
	}

	query := fmt.Sprintf(`
        SELECT id, title, content, created_at, updated_at, day, edit_count, mood, fulfillment, base_weather, ambient_weathers, auto_title
        FROM entries
        WHERE %s
        ORDER BY day DESC
		LIMIT ? OFFSET ?
    `, strings.Join(clauses, " AND "))
	args = append(args, limit, offset)

	rows, err := a.DB.Query(query, args...)
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
