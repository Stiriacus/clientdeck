// Command vitrine runs the vitrine HTTP service: config → store →
// themes → services → router → server, with graceful shutdown on
// SIGINT/SIGTERM. Themes are loaded at startup from both the embedded
// "plain" theme and any directories found in VITRINE_THEMES_DIR.
// VITRINE_DEV enables per-request template reloading for theme development
// (keep the /c/demo route). Internal/ code never sees a dev flag, only
// the fs.FS and services it's handed.
package main

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Stiriacus/vitrine/internal/board"
	"github.com/Stiriacus/vitrine/internal/config"
	"github.com/Stiriacus/vitrine/internal/httpapi"
	"github.com/Stiriacus/vitrine/internal/i18n"
	"github.com/Stiriacus/vitrine/internal/ingest"
	"github.com/Stiriacus/vitrine/internal/render"
	"github.com/Stiriacus/vitrine/internal/slug"
	"github.com/Stiriacus/vitrine/internal/store"
	"github.com/Stiriacus/vitrine/themes"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)
	if cfg.Dev {
		logger.Warn("VITRINE_DEV enabled: templates reload from disk on every request. Do not use in production")
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		logger.Error("failed to open store", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	// Build the theme registry.
	registry := buildRegistry(cfg, logger)

	renderSvc := render.NewService(registry, cfg.PoweredBy)

	slugs := slug.New(cryptorand.Reader)
	ingestSvc := ingest.New(st, slugs)

	mux := http.NewServeMux()
	if cfg.Dev && cfg.DemoPayload != "" {
		logger.Warn("VITRINE_DEMO_PAYLOAD set: serving /c/demo without auth or the store", "path", cfg.DemoPayload)
		mux.HandleFunc("GET /c/demo", handleDemo(cfg.DemoPayload, renderSvc))
	}
	mux.Handle("/", httpapi.NewRouter(httpapi.Deps{
		Store:         st,
		Ingest:        ingestSvc,
		Render:        renderSvc,
		StaticFS:      registry.MergedStaticFS(),
		WebhookSecret: cfg.WebhookSecret,
		BaseURL:       cfg.BaseURL,
		Logger:        logger,
	}))

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting vitrine", "addr", cfg.Addr, "theme", cfg.Theme, "themes_dir", cfg.ThemesDir)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil {
			logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
	case sig := <-stop:
		logger.Info("shutting down", "signal", sig.String())
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
			os.Exit(1)
		}
	}
}

// buildRegistry loads all available themes and returns a Registry.
// It always loads the embedded "plain" theme as a guaranteed fallback.
// In dev mode, the configured theme is loaded from disk with per-request
// template reloading. When VITRINE_THEMES_DIR is set (production), every
// subdirectory is loaded as a theme once at startup.
func buildRegistry(cfg config.Config, logger *slog.Logger) *render.Registry {
	registry := render.NewRegistry(cfg.Theme)

	// 1. Always embed "plain" as a guaranteed fallback.
	plainFS, err := themes.Plain()
	if err != nil {
		logger.Error("failed to access embedded plain theme", "error", err)
		os.Exit(1)
	}
	plainTheme, _, err := render.LoadTheme(plainFS)
	if err != nil {
		logger.Error("failed to parse embedded plain theme", "error", err)
		os.Exit(1)
	}
	plainStatic, _ := fs.Sub(plainFS, "static")
	registry.Register("plain", plainTheme, plainStatic)
	logger.Info("loaded embedded theme", "name", "plain")

	// 2. Dev mode: load the configured theme from disk with hot-reload.
	if cfg.Dev {
		themesRoot := "themes"
		if cfg.ThemesDir != "" {
			themesRoot = cfg.ThemesDir
		}
		// Load i18n once (not hot-reloaded).
		themeFS, err := fs.Sub(os.DirFS(themesRoot), cfg.Theme)
		if err != nil {
			logger.Error("dev: theme directory not found", "name", cfg.Theme, "root", themesRoot, "error", err)
			os.Exit(1)
		}
		i18nSub, err := fs.Sub(themeFS, "i18n")
		if err != nil {
			logger.Error("dev: theme missing i18n directory", "name", cfg.Theme, "error", err)
			os.Exit(1)
		}
		_, err = i18n.Load(i18nSub)
		if err != nil {
			logger.Error("dev: failed to load i18n bundle", "name", cfg.Theme, "error", err)
			os.Exit(1)
		}
		// Cache the static FS so it's not re-read on every request.
		devStatic, _ := fs.Sub(themeFS, "static")
		registry.Register(cfg.Theme, devTheme(cfg.Theme, themeFS, logger), devStatic)
		registry.SetDevTheme(cfg.Theme, themeFS)
		logger.Info("dev: loaded theme with hot-reload", "name", cfg.Theme, "root", themesRoot)
		return registry
	}

	// 3. Production: scan VITRINE_THEMES_DIR for additional themes.
	if cfg.ThemesDir != "" {
		entries, err := os.ReadDir(cfg.ThemesDir)
		if err != nil {
			logger.Warn("cannot read themes directory, using only embedded themes",
				"path", cfg.ThemesDir, "error", err)
			return registry
		}
		for _, entry := range entries {
			if !entry.IsDir() || entry.Name() == "plain" {
				continue
			}
			themePath := filepath.Join(cfg.ThemesDir, entry.Name())
			if err := themes.ValidateDir(themePath); err != nil {
				logger.Warn("theme validation failed, skipping", "name", entry.Name(), "error", err)
				continue
			}
			if err := registry.LoadThemeFromDir(entry.Name(), os.DirFS(cfg.ThemesDir)); err != nil {
				logger.Warn("failed to load theme, skipping", "name", entry.Name(), "error", err)
				continue
			}
			logger.Info("loaded theme from disk", "name", entry.Name())
		}
	}

	return registry
}

// devTheme wraps a theme name so its static filesystem can be cached while
// templates are reloaded on every request via Registry.SetDevTheme.
// The returned theme is used for static FS registration only; the actual
// template rendering uses the dev-mode re-parse path in the Registry.
func devTheme(name string, themeFS fs.FS, logger *slog.Logger) *render.Theme {
	theme, _, err := render.LoadTheme(themeFS)
	if err != nil {
		logger.Error("dev: failed to parse theme", "name", name, "error", err)
		os.Exit(1)
	}
	return theme
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
}

// handleDemo serves VITRINE_DEMO_PAYLOAD at /c/demo: re-read, validate
// and render the file on every request, bypassing the store and the
// webhook secret entirely. Only registered when Dev is set alongside it.
func handleDemo(path string, renderer *render.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(path)
		if err != nil {
			http.Error(w, fmt.Sprintf("demo payload: read %s: %v", path, err), http.StatusInternalServerError)
			return
		}
		var v board.CustomerView
		if err := json.Unmarshal(data, &v); err != nil {
			http.Error(w, fmt.Sprintf("demo payload: invalid JSON: %v", err), http.StatusInternalServerError)
			return
		}
		if err := v.Validate(); err != nil {
			http.Error(w, fmt.Sprintf("demo payload: %v", err), http.StatusBadRequest)
			return
		}
		v.Slug = "demo"

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := renderer.RenderBoard(w, v); err != nil {
			http.Error(w, fmt.Sprintf("render: %v", err), http.StatusInternalServerError)
		}
	}
}
