package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"subforme/backend/internal/app"
	"subforme/backend/internal/config"
	"subforme/backend/internal/db"
	"subforme/backend/internal/web"
)

func main() {
	exeDir := exeDirOrFail()
	configDir := filepath.Join(exeDir, "config")
	runtimePath := filepath.Join(configDir, "config.json")

	if _, err := os.Stat(runtimePath); err != nil {
		ensureDefaultConfig(configDir, exeDir)
	}

	runtimeConfig, err := config.LoadRuntimeConfig(runtimePath)
	if err != nil {
		log.Fatal(err)
	}

	store, err := db.Open(runtimeConfig.ConfigDir)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer store.Close()

	service := app.NewServiceWithDB(runtimeConfig.ConfigDir, runtimeConfig.XUI, store)
	bgCtx := context.Background()
	service.StartProviderUpdater(bgCtx)
	service.StartTrafficRefresher(bgCtx)

	authSvc := &app.StaticAuthService{
		Username: runtimeConfig.AdminUsername,
		Password: runtimeConfig.AdminPassword,
	}

	handler := web.NewRouter(web.Dependencies{
		SubscriptionService: service,
		ConfigService:       service,
		XUIService:          service,
		UserService:         service,
		DBService:           service,
		SessionSecret:       runtimeConfig.SessionSecret,
		FrontendDir:         runtimeConfig.FrontendDir,
		AdminUsername:       runtimeConfig.AdminUsername,
		RuntimePath:         runtimeConfig.RuntimePath,
		AuthService:         authSvc,
	})

	log.Printf("subforme backend listening on %s using config dir %s", runtimeConfig.Listen, runtimeConfig.ConfigDir)
	if err := http.ListenAndServe(runtimeConfig.Listen, handler); err != nil {
		log.Fatal(err)
	}
}

func exeDirOrFail() string {
	exePath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	return filepath.Dir(exePath)
}

func ensureDefaultConfig(configDir, exeDir string) {
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		log.Fatalf("create config dir: %v", err)
	}

	// Copy defaults from bundled directory (Docker) or from exe sibling dir
	sourceCandidates := []string{
		filepath.Join(exeDir, "defaults", "config"), // Docker image bundled copy
		filepath.Join(exeDir, "config"),             // fallback
	}
	var sourceDir string
	for _, dir := range sourceCandidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			sourceDir = dir
			break
		}
	}
	if sourceDir == "" {
		return
	}

	entries, _ := os.ReadDir(sourceDir)
	for _, e := range entries {
		dst := filepath.Join(configDir, e.Name())
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		src := filepath.Join(sourceDir, e.Name())
		if e.IsDir() {
			os.MkdirAll(dst, 0o755)
			subEntries, _ := os.ReadDir(src)
			for _, sub := range subEntries {
				srcFile := filepath.Join(src, sub.Name())
				dstFile := filepath.Join(dst, sub.Name())
				if !sub.IsDir() {
					input, _ := os.ReadFile(srcFile)
					os.WriteFile(dstFile, input, 0o644)
				}
			}
		} else {
			input, err := os.ReadFile(src)
			if err != nil {
				continue
			}
			os.WriteFile(dst, input, 0o644)
		}
	}
}
