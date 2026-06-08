package web

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"subforme/backend/internal/config"
)

const (
	backupMaxUploadBytes = 128 << 20
)

func registerBackupRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/api/backup/export", requireSession(deps.SessionSecret, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if deps.ConfigDir == "" {
			http.Error(w, "config dir unavailable", http.StatusNotImplemented)
			return
		}
		filename := "subforme-backup-" + time.Now().Format("20060102-150405") + ".zip"
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
		if err := writeConfigBackup(w, deps.ConfigDir); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}))

	mux.HandleFunc("/api/backup/restore", requireSession(deps.SessionSecret, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if deps.ConfigDir == "" {
			http.Error(w, "config dir unavailable", http.StatusNotImplemented)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, backupMaxUploadBytes)
		file, _, err := r.FormFile("backup")
		if err != nil {
			http.Error(w, "missing backup file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		pendingPath := filepath.Join(deps.ConfigDir, config.PendingRestoreName)
		tmpPath := pendingPath + ".tmp"
		out, err := os.Create(tmpPath)
		if err != nil {
			http.Error(w, "create pending restore failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := io.Copy(out, file); err != nil {
			out.Close()
			_ = os.Remove(tmpPath)
			http.Error(w, "save pending restore failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := out.Close(); err != nil {
			_ = os.Remove(tmpPath)
			http.Error(w, "save pending restore failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := validateBackupZip(tmpPath); err != nil {
			_ = os.Remove(tmpPath)
			http.Error(w, "invalid backup: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err := os.Rename(tmpPath, pendingPath); err != nil {
			_ = os.Remove(tmpPath)
			http.Error(w, "activate pending restore failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"message": "备份已上传，SubForMe 正在自动重启并恢复数据。",
		})
		if deps.RestartAfterRestore != nil {
			go deps.RestartAfterRestore()
		}
	}))
}

func writeConfigBackup(w io.Writer, configDir string) error {
	zw := zip.NewWriter(w)
	defer zw.Close()

	base := filepath.Clean(configDir)
	return filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		name := entry.Name()
		if name == config.PendingRestoreName || strings.HasSuffix(name, ".tmp") {
			return nil
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." || strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "/") {
			return fmt.Errorf("invalid backup path %q", rel)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = rel
		header.Method = zip.Deflate
		fw, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(fw, in)
		closeErr := in.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func validateBackupZip(path string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer reader.Close()
	if len(reader.File) == 0 {
		return fmt.Errorf("empty backup")
	}
	for _, file := range reader.File {
		if err := validateBackupEntry(file.Name); err != nil {
			return err
		}
	}
	return nil
}

func validateBackupEntry(name string) error {
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return fmt.Errorf("unsafe path %q", name)
	}
	if strings.Contains(clean, string(filepath.Separator)+config.PendingRestoreName) || filepath.Base(clean) == config.PendingRestoreName {
		return fmt.Errorf("backup cannot contain %s", config.PendingRestoreName)
	}
	return nil
}
