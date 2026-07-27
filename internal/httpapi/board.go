package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Stiriacus/vitrine/internal/board"
	"github.com/Stiriacus/vitrine/internal/render"
)

// boardCSP is served on every HTML response. The plain theme has no inline
// script/style (see themes/plain/*.html), so this can omit 'unsafe-inline'
// entirely.
const boardCSP = "default-src 'self'; img-src 'self' https: data:; style-src 'self'; script-src 'self'"

// handleBoard implements GET /c/{slug}: look up the customer by slug and
// render its board, or the theme's not-found page for an unknown slug.
func handleBoard(store board.Store, renderer *render.Service, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")

		v, err := store.BySlug(r.Context(), slug)
		switch {
		case errors.Is(err, board.ErrUnknownSlug):
			lang := acceptLanguage(r.Header.Get("Accept-Language"))
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Content-Security-Policy", boardCSP)
			w.WriteHeader(http.StatusNotFound)
			if err := renderer.RenderNotFound(w, lang); err != nil {
				logger.Error("render not-found failed", "request_id", requestIDFrom(r.Context()), "error", err)
			}
			return
		case err != nil:
			logger.Error("lookup slug failed", "request_id", requestIDFrom(r.Context()), "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", boardCSP)
		w.WriteHeader(http.StatusOK)
		if err := renderer.RenderBoard(w, v); err != nil {
			logger.Error("render board failed", "request_id", requestIDFrom(r.Context()), "customer_id", v.CustomerID, "error", err)
		}
	}
}

// acceptLanguage extracts the primary language tag from an Accept-Language
// header value. Returns "en" if the header is empty or unparseable.
func acceptLanguage(header string) string {
	if header == "" {
		return "en"
	}
	// Take the first tag, strip quality weights and whitespace.
	tag := strings.TrimSpace(strings.SplitN(header, ",", 2)[0])
	tag = strings.SplitN(tag, ";", 2)[0]
	tag = strings.TrimSpace(tag)
	// Extract the primary language subtag (first 2-3 chars before any hyphen).
	if len(tag) >= 2 {
		return strings.ToLower(tag[:2])
	}
	return "en"
}

// handleHealthz implements GET /healthz.
func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
