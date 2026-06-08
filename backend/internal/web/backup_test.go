package web

import (
	"archive/zip"
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"subforme/backend/internal/config"
)

func TestBackupExportReturnsZip(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"listen":":0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	router := NewRouter(Dependencies{SessionSecret: testSessionSecret, ConfigDir: dir})
	req := httptest.NewRequest(http.MethodGet, "/api/backup/export", nil)
	addSessionCookie(req)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	reader, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatalf("expected zip body: %v", err)
	}
	if len(reader.File) != 1 || reader.File[0].Name != "config.json" {
		t.Fatalf("unexpected zip entries: %#v", reader.File)
	}
}

func TestBackupRestoreStoresPendingArchive(t *testing.T) {
	dir := t.TempDir()
	var zipBody bytes.Buffer
	zw := zip.NewWriter(&zipBody)
	fw, err := zw.Create("config.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte(`{"listen":":1"}`)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("backup", "backup.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(part, bytes.NewReader(zipBody.Bytes())); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}

	router := NewRouter(Dependencies{SessionSecret: testSessionSecret, ConfigDir: dir})
	req := httptest.NewRequest(http.MethodPost, "/api/backup/restore", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	addSessionCookie(req)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dir, config.PendingRestoreName)); err != nil {
		t.Fatalf("expected pending restore archive: %v", err)
	}
}
