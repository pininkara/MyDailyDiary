package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func newThoughtTestApp(t *testing.T) *App {
	t.Helper()
	db, err := openDB(filepath.Join(t.TempDir(), "diary.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	return &App{DB: db}
}

func TestThoughtPaginationAndOrdering(t *testing.T) {
	app := newThoughtTestApp(t)
	timestamp := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC).Format(thoughtTimeLayout)
	for i := 1; i <= 25; i++ {
		if _, err := app.DB.Exec(
			`INSERT INTO thoughts(uid, content, created_at, updated_at) VALUES(?,?,?,?)`,
			fmt.Sprintf("thought-%d", i), fmt.Sprintf("Thought %d", i), timestamp, timestamp,
		); err != nil {
			t.Fatal(err)
		}
	}

	var allIDs []int64
	var cursor *thoughtCursor
	for {
		page, err := app.listThoughts(10, cursor)
		if err != nil {
			t.Fatal(err)
		}
		for _, thought := range page.Items {
			allIDs = append(allIDs, thought.ID)
		}
		if !page.HasMore {
			break
		}
		cursor, err = decodeThoughtCursor(page.NextCursor)
		if err != nil {
			t.Fatal(err)
		}
	}

	if len(allIDs) != 25 {
		t.Fatalf("got %d thoughts, want 25", len(allIDs))
	}
	for i, id := range allIDs {
		want := int64(25 - i)
		if id != want {
			t.Fatalf("position %d: got id %d, want %d", i, id, want)
		}
	}

	updated, err := app.updateThought(1, "Updated thought")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Content != "Updated thought" {
		t.Fatalf("unexpected updated content: %q", updated.Content)
	}
	firstPage, err := app.listThoughts(10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if firstPage.Items[0].ID != 1 {
		t.Fatalf("updated thought was not moved to the top: got id %d", firstPage.Items[0].ID)
	}

	if err := app.deleteThought(1); err != nil {
		t.Fatal(err)
	}
	if _, err := app.getThought(1); err != errThoughtNotFound {
		t.Fatalf("get deleted thought: got %v, want %v", err, errThoughtNotFound)
	}
}

func TestThoughtAPIValidationAndAuth(t *testing.T) {
	app := newThoughtTestApp(t)
	handler := app.routes()

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/thoughts", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status: got %d, want %d", unauthorized.Code, http.StatusUnauthorized)
	}

	do := func(method, target string, body []byte) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(method, target, bytes.NewReader(body))
		request.AddCookie(&http.Cookie{Name: "diary_auth", Value: "ok"})
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}

	empty := do(http.MethodPost, "/api/thoughts", []byte(`{"content":"   "}`))
	if empty.Code != http.StatusBadRequest {
		t.Fatalf("empty content status: got %d, want %d", empty.Code, http.StatusBadRequest)
	}

	createdResponse := do(http.MethodPost, "/api/thoughts", []byte(`{"content":"  First thought  "}`))
	if createdResponse.Code != http.StatusCreated {
		t.Fatalf("create status: got %d, want %d; body=%s", createdResponse.Code, http.StatusCreated, createdResponse.Body)
	}
	var created Thought
	if err := json.NewDecoder(createdResponse.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.Content != "First thought" {
		t.Fatalf("created content: got %q", created.Content)
	}

	badCursor := do(http.MethodGet, "/api/thoughts?cursor=not-a-cursor", nil)
	if badCursor.Code != http.StatusBadRequest {
		t.Fatalf("invalid cursor status: got %d, want %d", badCursor.Code, http.StatusBadRequest)
	}

	missing := do(http.MethodPut, "/api/thoughts/999", []byte(`{"content":"Missing"}`))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing thought status: got %d, want %d", missing.Code, http.StatusNotFound)
	}

	deleted := do(http.MethodDelete, fmt.Sprintf("/api/thoughts/%d", created.ID), nil)
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete status: got %d, want %d", deleted.Code, http.StatusNoContent)
	}
}

func TestThoughtExportImportRoundTrip(t *testing.T) {
	source := newThoughtTestApp(t)
	createdAt := time.Date(2026, 7, 18, 8, 15, 30, 123456789, time.UTC)
	updatedAt := createdAt.Add(2 * time.Hour)
	if err := source.upsertImportedThought("thought-one", "First exported thought", createdAt, updatedAt); err != nil {
		t.Fatal(err)
	}
	if err := source.upsertImportedThought("thought-two", "Second exported thought", createdAt, updatedAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	exported := httptest.NewRecorder()
	source.apiExport(exported, httptest.NewRequest(http.MethodGet, "/api/export", nil))
	if exported.Code != http.StatusOK {
		t.Fatalf("export status: got %d, want %d; body=%s", exported.Code, http.StatusOK, exported.Body)
	}
	var exportPayload struct {
		Thoughts []struct {
			UID       string `json:"uid"`
			Content   string `json:"content"`
			CreatedAt string `json:"created_at"`
			UpdatedAt string `json:"updated_at"`
		} `json:"thoughts"`
	}
	if err := json.Unmarshal(exported.Body.Bytes(), &exportPayload); err != nil {
		t.Fatal(err)
	}
	if len(exportPayload.Thoughts) != 2 {
		t.Fatalf("exported thoughts: got %d, want 2", len(exportPayload.Thoughts))
	}
	if exportPayload.Thoughts[0].Content != "First exported thought" {
		t.Fatalf("unexpected first exported thought: %q", exportPayload.Thoughts[0].Content)
	}
	if exportPayload.Thoughts[0].UID != "thought-one" || exportPayload.Thoughts[1].UID != "thought-two" {
		t.Fatalf("exported thought UIDs were not preserved: %+v", exportPayload.Thoughts)
	}

	destination := newThoughtTestApp(t)
	importBackup := func() {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("file", "backup.json")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(exported.Body.Bytes()); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}

		request := httptest.NewRequest(http.MethodPost, "/api/import", &body)
		request.Header.Set("Content-Type", writer.FormDataContentType())
		response := httptest.NewRecorder()
		destination.apiImport(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("import status: got %d, want %d; body=%s", response.Code, http.StatusOK, response.Body)
		}
		var result struct {
			ThoughtsImported int `json:"thoughts_imported"`
		}
		if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		if result.ThoughtsImported != 2 {
			t.Fatalf("thoughts imported: got %d, want 2", result.ThoughtsImported)
		}
	}

	importBackup()
	importBackup()

	importedThoughts, err := destination.listAllThoughts()
	if err != nil {
		t.Fatal(err)
	}
	if len(importedThoughts) != 2 {
		t.Fatalf("thoughts after repeated import: got %d, want 2", len(importedThoughts))
	}
	if !importedThoughts[0].Created.Equal(createdAt) || !importedThoughts[0].Updated.Equal(updatedAt) {
		t.Fatalf(
			"timestamps changed during import: created=%s updated=%s",
			importedThoughts[0].Created,
			importedThoughts[0].Updated,
		)
	}
}

func TestThoughtMigrationBackfillsUID(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE thoughts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		content TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`); err != nil {
		t.Fatal(err)
	}
	timestamp := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC).Format(thoughtTimeLayout)
	if _, err := db.Exec(
		`INSERT INTO thoughts(content, created_at, updated_at) VALUES(?,?,?)`,
		"Legacy thought", timestamp, timestamp,
	); err != nil {
		t.Fatal(err)
	}

	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	var uid string
	if err := db.QueryRow(`SELECT uid FROM thoughts LIMIT 1`).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	if uid == "" {
		t.Fatal("migration did not backfill thought UID")
	}
}

func TestThoughtImportAcceptsLegacyBackup(t *testing.T) {
	app := newThoughtTestApp(t)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "legacy.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(`{"entries":[]}`)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/import", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	app.apiImport(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("legacy import status: got %d, want %d; body=%s", response.Code, http.StatusOK, response.Body)
	}
}
