package app

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const thoughtTimeLayout = "2006-01-02T15:04:05.000000000Z07:00"

var errThoughtNotFound = errors.New("thought not found")

type thoughtCursor struct {
	UpdatedAt string `json:"updated_at"`
	ID        int64  `json:"id"`
}

type thoughtPage struct {
	Items      []*Thought `json:"items"`
	NextCursor string     `json:"next_cursor,omitempty"`
	HasMore    bool       `json:"has_more"`
}

func scanThought(scanner interface{ Scan(dest ...any) error }, thought *Thought) error {
	var createdAt, updatedAt string
	if err := scanner.Scan(&thought.ID, &thought.UID, &thought.Content, &createdAt, &updatedAt); err != nil {
		return err
	}
	var err error
	thought.Created, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return fmt.Errorf("parse thought created_at: %w", err)
	}
	thought.Updated, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return fmt.Errorf("parse thought updated_at: %w", err)
	}
	return nil
}

func encodeThoughtCursor(thought *Thought) string {
	cursor := thoughtCursor{
		UpdatedAt: thought.Updated.UTC().Format(thoughtTimeLayout),
		ID:        thought.ID,
	}
	payload, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeThoughtCursor(value string) (*thoughtCursor, error) {
	if value == "" {
		return nil, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor")
	}
	var cursor thoughtCursor
	if err := json.Unmarshal(payload, &cursor); err != nil || cursor.ID <= 0 {
		return nil, fmt.Errorf("invalid cursor")
	}
	parsed, err := time.Parse(time.RFC3339Nano, cursor.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor")
	}
	cursor.UpdatedAt = parsed.UTC().Format(thoughtTimeLayout)
	return &cursor, nil
}

func (a *App) listThoughts(limit int, cursor *thoughtCursor) (thoughtPage, error) {
	query := `SELECT id, uid, content, created_at, updated_at FROM thoughts`
	args := make([]any, 0, 4)
	if cursor != nil {
		query += ` WHERE updated_at < ? OR (updated_at = ? AND id < ?)`
		args = append(args, cursor.UpdatedAt, cursor.UpdatedAt, cursor.ID)
	}
	query += ` ORDER BY updated_at DESC, id DESC LIMIT ?`
	args = append(args, limit+1)

	rows, err := a.DB.Query(query, args...)
	if err != nil {
		return thoughtPage{}, err
	}
	defer rows.Close()

	items := make([]*Thought, 0, limit+1)
	for rows.Next() {
		var thought Thought
		if err := scanThought(rows, &thought); err != nil {
			return thoughtPage{}, err
		}
		items = append(items, &thought)
	}
	if err := rows.Err(); err != nil {
		return thoughtPage{}, err
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	page := thoughtPage{Items: items, HasMore: hasMore}
	if hasMore && len(items) > 0 {
		page.NextCursor = encodeThoughtCursor(items[len(items)-1])
	}
	return page, nil
}

func (a *App) listAllThoughts() ([]*Thought, error) {
	rows, err := a.DB.Query(`SELECT id, uid, content, created_at, updated_at FROM thoughts ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	thoughts := make([]*Thought, 0)
	for rows.Next() {
		var thought Thought
		if err := scanThought(rows, &thought); err != nil {
			return nil, err
		}
		thoughts = append(thoughts, &thought)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return thoughts, nil
}

func (a *App) upsertImportedThought(uid, content string, createdAt, updatedAt time.Time) error {
	created := createdAt.UTC().Format(thoughtTimeLayout)
	updated := updatedAt.UTC().Format(thoughtTimeLayout)

	var id int64
	err := a.DB.QueryRow(`SELECT id FROM thoughts WHERE uid=? LIMIT 1`, uid).Scan(&id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if errors.Is(err, sql.ErrNoRows) {
		_, err = a.DB.Exec(
			`INSERT INTO thoughts(uid, content, created_at, updated_at) VALUES(?,?,?,?)`,
			uid, content, created, updated,
		)
		return err
	}

	_, err = a.DB.Exec(`UPDATE thoughts SET content=?, updated_at=? WHERE id=?`, content, updated, id)
	return err
}

func (a *App) createThought(content string) (*Thought, error) {
	now := time.Now().UTC().Format(thoughtTimeLayout)
	result, err := a.DB.Exec(
		`INSERT INTO thoughts(uid, content, created_at, updated_at) VALUES(lower(hex(randomblob(16))),?,?,?)`,
		content, now, now,
	)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return a.getThought(id)
}

func (a *App) getThought(id int64) (*Thought, error) {
	row := a.DB.QueryRow(`SELECT id, uid, content, created_at, updated_at FROM thoughts WHERE id=?`, id)
	var thought Thought
	if err := scanThought(row, &thought); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errThoughtNotFound
		}
		return nil, err
	}
	return &thought, nil
}

func (a *App) updateThought(id int64, content string) (*Thought, error) {
	updatedAt := time.Now().UTC().Format(thoughtTimeLayout)
	result, err := a.DB.Exec(`UPDATE thoughts SET content=?, updated_at=? WHERE id=?`, content, updatedAt, id)
	if err != nil {
		return nil, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		return nil, errThoughtNotFound
	}
	return a.getThought(id)
}

func (a *App) deleteThought(id int64) error {
	result, err := a.DB.Exec(`DELETE FROM thoughts WHERE id=?`, id)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errThoughtNotFound
	}
	return nil
}

func parseThoughtID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid thought id")
	}
	return id, nil
}

func decodeThoughtContent(r *http.Request) (string, error) {
	var request struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return "", fmt.Errorf("invalid JSON")
	}
	content := strings.TrimSpace(request.Content)
	if content == "" {
		return "", fmt.Errorf("content is required")
	}
	return content, nil
}

func writeThoughtError(w http.ResponseWriter, err error) {
	if errors.Is(err, errThoughtNotFound) {
		http.Error(w, "Thought not found", http.StatusNotFound)
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func (a *App) apiListThoughts(w http.ResponseWriter, r *http.Request) {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	cursor, err := decodeThoughtCursor(strings.TrimSpace(r.URL.Query().Get("cursor")))
	if err != nil {
		http.Error(w, "Invalid cursor", http.StatusBadRequest)
		return
	}
	page, err := a.listThoughts(limit, cursor)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(page)
}

func (a *App) apiCreateThought(w http.ResponseWriter, r *http.Request) {
	content, err := decodeThoughtContent(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	thought, err := a.createThought(content)
	if err != nil {
		writeThoughtError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(thought)
}

func (a *App) apiUpdateThought(w http.ResponseWriter, r *http.Request) {
	id, err := parseThoughtID(r)
	if err != nil {
		http.Error(w, "Invalid thought id", http.StatusBadRequest)
		return
	}
	content, err := decodeThoughtContent(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	thought, err := a.updateThought(id, content)
	if err != nil {
		writeThoughtError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(thought)
}

func (a *App) apiDeleteThought(w http.ResponseWriter, r *http.Request) {
	id, err := parseThoughtID(r)
	if err != nil {
		http.Error(w, "Invalid thought id", http.StatusBadRequest)
		return
	}
	if err := a.deleteThought(id); err != nil {
		writeThoughtError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
