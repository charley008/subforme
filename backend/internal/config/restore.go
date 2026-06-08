package config

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const PendingRestoreName = "pending_restore.zip"

func ApplyPendingRestore(configDir string) (bool, error) {
	pendingPath := filepath.Join(configDir, PendingRestoreName)
	if _, err := os.Stat(pendingPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if err := extractRestoreZip(pendingPath, configDir); err != nil {
		return false, err
	}
	if err := os.Remove(pendingPath); err != nil {
		return false, err
	}
	return true, nil
}

func extractRestoreZip(zipPath, targetDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	base, err := filepath.Abs(targetDir)
	if err != nil {
		return err
	}
	for _, file := range reader.File {
		clean, err := safeRestorePath(base, file.Name)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(clean, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(clean), 0o755); err != nil {
			return err
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(clean, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
		if err != nil {
			in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeInErr := in.Close()
		closeOutErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeInErr != nil {
			return closeInErr
		}
		if closeOutErr != nil {
			return closeOutErr
		}
	}
	return nil
}

func safeRestorePath(base, name string) (string, error) {
	cleanName := filepath.Clean(filepath.FromSlash(name))
	if cleanName == "." || cleanName == ".." || filepath.IsAbs(cleanName) || strings.HasPrefix(cleanName, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe restore path %q", name)
	}
	if filepath.Base(cleanName) == PendingRestoreName {
		return "", fmt.Errorf("restore archive cannot contain %s", PendingRestoreName)
	}
	out := filepath.Join(base, cleanName)
	absOut, err := filepath.Abs(out)
	if err != nil {
		return "", err
	}
	if absOut != base && !strings.HasPrefix(absOut, base+string(filepath.Separator)) {
		return "", fmt.Errorf("restore path escapes config dir: %q", name)
	}
	return absOut, nil
}
