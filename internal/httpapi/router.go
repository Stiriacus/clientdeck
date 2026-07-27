// Package httpapi implements vitrine's HTTP surface: the ingest webhook,
// the public board pages, static theme assets and healthz. Handlers stay
// thin: parse, call a domain service, map errors to the status table in
// docs/api.md. All actual behavior lives in internal/ingest and
// internal/render.
package httpapi

import (
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/Stiriacus/vitrine/internal/board"
	"github.com/Stiriacus/vitrine/internal/ingest"
	"github.com/Stiriacus/vitrine/internal/render"
)

// Deps collects everything the router needs to wire its routes.
type Deps struct {
	Store         board.Store
	Ingest        *ingest.Service
	Render        *render.Service
	StaticFS      fs.FS // rooted at the theme's static/ directory
	WebhookSecret string
	BaseURL       string
	Logger        *slog.Logger
}

// NewRouter builds the full request handler: routes plus the middleware
// chain (RequestID → logging → recover globally; auth only on the write
// routes, since slug is itself the read capability).
func NewRouter(d Deps) http.Handler {
	mux := http.NewServeMux()

	webhook := withAuth(d.WebhookSecret)(handleWebhook(d.Ingest, d.BaseURL, d.Logger))
	mux.Handle("POST /api/v1/views", webhook)
	mux.Handle("POST /webhook", webhook) // alias, shorter for n8n

	mux.Handle("GET /c/{slug}", handleBoard(d.Store, d.Render, d.Logger))
	mux.Handle("GET /static/", cacheControl(http.StripPrefix("/static/", http.FileServerFS(d.StaticFS))))
	mux.HandleFunc("GET /healthz", handleHealthz)

	var h http.Handler = mux
	h = withRecover(d.Logger)(h)
	h = withLogging(d.Logger)(h)
	h = withRequestID(h)
	return h
}

// cacheControl marks theme assets as immutable: they're embedded per-build,
// so a given URL's content never changes without a new binary.
func cacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		next.ServeHTTP(w, r)
	})
}
