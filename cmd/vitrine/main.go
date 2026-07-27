// Command vitrine runs the vitrine HTTP service: config → store →
// theme → services → router → server, with graceful shutdown on
// SIGINT/SIGTERM. VITRINE_DEV support (disk-backed, reloading themes and
// the /c/demo route) is kept entirely in this file. Internal/ code never
// sees a dev flag, only the fs.FS and services it's handed.
package main

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
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
		logger.Warn("VITRINE_DEV enabled: templates and static assets reload from disk on every request. Do not use in production")
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		logger.Error("failed to open store", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	themeFS, err := loadThemeFS(cfg)
	if err != nil {
		logger.Error("failed to load theme", "error", err)
		os.Exit(1)
	}
	staticFS, err := fs.Sub(themeFS, "static")
	if err != nil {
		logger.Error("failed to load theme static assets", "error", err)
		os.Exit(1)
	}

	var (
		renderer board.Renderer
		bundle   *i18n.Bundle
	)
	if cfg.Dev {
		// In dev mode, load the bundle once so reloadingRenderer can use it.
		// Templates are still re-parsed on every request.
		i18nFS, err := fs.Sub(themeFS, "i18n")
		if err != nil {
			logger.Error("failed to locate i18n directory in theme", "error", err)
			os.Exit(1)
		}
		b, err := i18n.Load(i18nFS)
		if err != nil {
			logger.Error("failed to load i18n bundle", "error", err)
			os.Exit(1)
		}
		bundle = b
		renderer = &reloadingRenderer{fsys: themeFS, bundle: bundle}
	} else {
		theme, b, err := render.LoadTheme(themeFS)
		if err != nil {
			logger.Error("failed to parse theme templates", "error", err)
			os.Exit(1)
		}
		bundle = b
		renderer = theme
	}
	renderSvc := render.NewService(renderer, bundle, cfg.PoweredBy)

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
		StaticFS:      staticFS,
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
		logger.Info("starting vitrine", "addr", cfg.Addr, "theme", cfg.Theme)
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

// loadThemeFS resolves cfg.Theme to an fs.FS rooted at the theme directory
// itself. In dev mode it reads from disk (themes/<name>), so edits are
// picked up on the next request; otherwise it uses the embedded "plain"
// theme, the only one built into the binary.
func loadThemeFS(cfg config.Config) (fs.FS, error) {
	if cfg.Dev {
		return fs.Sub(os.DirFS("themes"), cfg.Theme)
	}
	if cfg.Theme != "plain" {
		return nil, fmt.Errorf("unknown embedded theme %q (only \"plain\" is built in; set VITRINE_DEV=true to load a theme from disk)", cfg.Theme)
	}
	return themes.Plain()
}

// reloadingRenderer implements board.Renderer by parsing the theme fresh on
// every call, so VITRINE_DEV picks up template/CSS edits without a
// rebuild or restart. It uses a pre-loaded i18n bundle so translations don't
// need to be in the template re-parse path.
type reloadingRenderer struct {
	fsys   fs.FS
	bundle *i18n.Bundle
}

func (r *reloadingRenderer) RenderBoard(w io.Writer, v board.BoardView) error {
	theme, _, err := render.LoadTheme(r.fsys)
	if err != nil {
		return err
	}
	return theme.RenderBoard(w, v)
}

func (r *reloadingRenderer) RenderNotFound(w io.Writer, lang string) error {
	theme, _, err := render.LoadTheme(r.fsys)
	if err != nil {
		return err
	}
	return theme.RenderNotFound(w, lang)
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
