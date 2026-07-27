package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// withRequestID assigns each request a random id, echoes it as
// X-Request-Id and makes it available to downstream middleware/handlers via
// the request context.
func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := newRequestID()
		w.Header().Set("X-Request-Id", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func newRequestID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(buf)
}

func requestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// responseRecorder captures the status code a handler writes so the logging
// middleware can report it after the handler returns.
type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// withLogging logs one structured line per request. Per the logging rules
// in ROADMAP.md: a board slug must never appear at info level (path is
// redacted for /c/* requests), and appears at debug only truncated to 4
// characters.
func withLogging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			logger.Info("request",
				"request_id", requestIDFrom(r.Context()),
				"method", r.Method,
				"path", redactedPath(r.URL.Path),
				"status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
			)
			if s := slugFromPath(r.URL.Path); s != "" {
				logger.Debug("request path detail",
					"request_id", requestIDFrom(r.Context()),
					"slug", truncateSlug(s),
				)
			}
		})
	}
}

func redactedPath(path string) string {
	if slugFromPath(path) != "" {
		return "/c/<redacted>"
	}
	return path
}

func slugFromPath(path string) string {
	const prefix = "/c/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	return strings.TrimPrefix(path, prefix)
}

func truncateSlug(s string) string {
	const n = 4
	r := []rune(s)
	if len(r) <= n {
		return string(r) + "…"
	}
	return string(r[:n]) + "…"
}

// withRecover turns a panic anywhere downstream into a 500 response instead
// of crashing the server, and logs the recovered value.
func withRecover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered",
						"request_id", requestIDFrom(r.Context()),
						"panic", rec,
					)
					writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// withAuth rejects requests whose X-Webhook-Secret header does not match
// secret. Missing and wrong secrets return an identical 401 response, so a
// caller cannot distinguish "no header" from "wrong header".
func withAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := r.Header.Get("X-Webhook-Secret")
			if subtle.ConstantTimeCompare([]byte(got), []byte(secret)) != 1 {
				writeError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid X-Webhook-Secret header")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
