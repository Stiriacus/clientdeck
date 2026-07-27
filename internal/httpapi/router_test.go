package httpapi_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Stiriacus/vitrine/internal/board"
	"github.com/Stiriacus/vitrine/internal/httpapi"
	"github.com/Stiriacus/vitrine/internal/ingest"
	"github.com/Stiriacus/vitrine/internal/render"
	"github.com/Stiriacus/vitrine/internal/slug"
	"github.com/Stiriacus/vitrine/internal/store"
	"github.com/Stiriacus/vitrine/themes"
)

const (
	testSecret  = "01234567890123456789012345678901" // 32 chars
	testBaseURL = "https://boards.example.com"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newTestRouter wires the full stack: real SQLite in t.TempDir(), the real
// slug generator and the embedded plain theme, exactly as main.go does, so
// these tests exercise the actual production wiring.
func newTestRouter(t *testing.T) http.Handler {
	t.Helper()

	st, err := store.Open(filepath.Join(t.TempDir(), "vitrine.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	themeFS, err := themes.Plain()
	if err != nil {
		t.Fatalf("themes.Plain: %v", err)
	}
	theme, _, err := render.LoadTheme(themeFS)
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	staticFS, err := fs.Sub(themeFS, "static")
	if err != nil {
		t.Fatalf("fs.Sub static: %v", err)
	}

	slugs := slug.New(rand.Reader)
	ingestSvc := ingest.New(st, slugs)

	registry := render.NewRegistry("plain")
	registry.Register("plain", theme, staticFS)
	renderSvc := render.NewService(registry, "TestCompany")

	return httpapi.NewRouter(httpapi.Deps{
		Store:         st,
		Ingest:        ingestSvc,
		Render:        renderSvc,
		StaticFS:      registry.MergedStaticFS(),
		WebhookSecret: testSecret,
		BaseURL:       testBaseURL,
		Logger:        discardLogger(),
	})
}

func postJSON(t *testing.T, h http.Handler, path, secret string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set("X-Webhook-Secret", secret)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func getPath(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeWebhookResponse(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response %s: %v", rec.Body.String(), err)
	}
	return body
}

func validPayload(customerID string) []byte {
	v := board.CustomerView{
		CustomerID: customerID,
		ClientName: "ACME Corp",
		Products: []board.Product{
			{Category: "Printers", Title: "Brother HL-L2375DW"},
		},
	}
	data, _ := json.Marshal(v)
	return data
}

// TestContractFlow walks the numbered acceptance scenarios from ROADMAP.md
// Phase 8 in order, since several steps depend on state (the slug) produced
// by an earlier one.
func TestContractFlow(t *testing.T) {
	h := newTestRouter(t)

	// 1. POST valid, correct secret -> 200, slug+url, created:true.
	rec := postJSON(t, h, "/api/v1/views", testSecret, validPayload("acme-corp"))
	if rec.Code != http.StatusOK {
		t.Fatalf("case 1: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := decodeWebhookResponse(t, rec)
	slugValue, _ := body["slug"].(string)
	if slugValue == "" {
		t.Fatalf("case 1: missing slug in %v", body)
	}
	if url, _ := body["url"].(string); url == "" {
		t.Fatalf("case 1: missing url in %v", body)
	}
	if created, _ := body["created"].(bool); !created {
		t.Fatalf("case 1: created = %v, want true", body["created"])
	}

	// 2. Same customer_id again -> 200, same slug, created:false.
	rec = postJSON(t, h, "/api/v1/views", testSecret, validPayload("acme-corp"))
	if rec.Code != http.StatusOK {
		t.Fatalf("case 2: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body2 := decodeWebhookResponse(t, rec)
	if body2["slug"] != slugValue {
		t.Fatalf("case 2: slug = %v, want unchanged %q", body2["slug"], slugValue)
	}
	if created, _ := body2["created"].(bool); created {
		t.Fatalf("case 2: created = %v, want false", body2["created"])
	}

	// 3. POST without secret header -> 401.
	recNoSecret := postJSON(t, h, "/api/v1/views", "", validPayload("acme-corp"))
	if recNoSecret.Code != http.StatusUnauthorized {
		t.Fatalf("case 3: status = %d, want 401", recNoSecret.Code)
	}

	// 4. POST with wrong secret -> 401, identical body to case 3.
	recWrongSecret := postJSON(t, h, "/api/v1/views", "wrong-secret-wrong-secret-wrong", validPayload("acme-corp"))
	if recWrongSecret.Code != http.StatusUnauthorized {
		t.Fatalf("case 4: status = %d, want 401", recWrongSecret.Code)
	}
	if recWrongSecret.Body.String() != recNoSecret.Body.String() {
		t.Fatalf("case 4: body = %s, want identical to missing-secret body %s", recWrongSecret.Body.String(), recNoSecret.Body.String())
	}

	// 5. Malformed JSON -> 400 invalid_payload.
	rec = postJSON(t, h, "/api/v1/views", testSecret, []byte(`{"customer_id": "broken"`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("case 5: status = %d, want 400", rec.Code)
	}
	if errBody := decodeWebhookResponse(t, rec); errBody["error"] != "invalid_payload" {
		t.Fatalf("case 5: error = %v, want invalid_payload", errBody["error"])
	}

	// 6. Unknown field products[0].color -> 200, ignored_fields reports it,
	// the rest of the payload is still stored.
	raw := []byte(`{
		"customer_id": "acme-unknown-field",
		"client_name": "ACME Corp",
		"products": [{"category": "Printers", "title": "Brother HL-L2375DW", "color": "blue"}]
	}`)
	rec = postJSON(t, h, "/api/v1/views", testSecret, raw)
	if rec.Code != http.StatusOK {
		t.Fatalf("case 6: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body6 := decodeWebhookResponse(t, rec)
	ignored, _ := body6["ignored_fields"].([]any)
	if len(ignored) != 1 || ignored[0] != "products[0].color" {
		t.Fatalf("case 6: ignored_fields = %v, want [products[0].color]", body6["ignored_fields"])
	}

	// 7. affiliate_link with javascript: scheme -> 400 invalid_url.
	raw = []byte(`{
		"customer_id": "acme-bad-url",
		"client_name": "ACME Corp",
		"products": [{"category": "Printers", "title": "Brother HL-L2375DW", "affiliate_link": "javascript:alert(1)"}]
	}`)
	rec = postJSON(t, h, "/api/v1/views", testSecret, raw)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("case 7: status = %d, want 400", rec.Code)
	}
	if errBody := decodeWebhookResponse(t, rec); errBody["error"] != "invalid_url" {
		t.Fatalf("case 7: error = %v, want invalid_url", errBody["error"])
	}

	// 8. 2 MiB body -> 413.
	huge := bytes.Repeat([]byte("a"), 2<<20)
	rec = postJSON(t, h, "/api/v1/views", testSecret, huge)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("case 8: status = %d, want 413", rec.Code)
	}

	// 9. GET /c/<slug from case 1> -> 200 text/html, contains client name and tabs.
	rec = getPath(t, h, "/c/"+slugValue)
	if rec.Code != http.StatusOK {
		t.Fatalf("case 9: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("case 9: Content-Type = %q, want text/html", ct)
	}
	if !strings.Contains(rec.Body.String(), "ACME Corp") {
		t.Fatalf("case 9: body missing client name: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Printers") {
		t.Fatalf("case 9: body missing category tab: %s", rec.Body.String())
	}

	// 10. GET /c/does-not-exist -> 404, theme's HTML not-found page.
	rec = getPath(t, h, "/c/does-not-exist")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("case 10: status = %d, want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("case 10: Content-Type = %q, want text/html", ct)
	}

	// 11. GET /healthz -> 200.
	rec = getPath(t, h, "/healthz")
	if rec.Code != http.StatusOK {
		t.Fatalf("case 11: status = %d, want 200", rec.Code)
	}

	// 12. POST /webhook alias behaves like case 1.
	rec = postJSON(t, h, "/webhook", testSecret, validPayload("acme-via-alias"))
	if rec.Code != http.StatusOK {
		t.Fatalf("case 12: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body12 := decodeWebhookResponse(t, rec)
	if created, _ := body12["created"].(bool); !created {
		t.Fatalf("case 12: created = %v, want true", body12["created"])
	}
}

// panicStore is an injected board.Store fake whose every method panics, used
// to exercise the recover middleware (case 13).
type panicStore struct{}

func (panicStore) Upsert(context.Context, board.CustomerView, func() (string, error)) (string, bool, error) {
	panic("injected panic: Upsert")
}

func (panicStore) BySlug(context.Context, string) (board.CustomerView, error) {
	panic("injected panic: BySlug")
}

func (panicStore) Close() error { return nil }

var _ board.Store = panicStore{}

// TestPanicRecovery covers case 13: a panicking handler yields 500 and the
// server keeps serving subsequent requests.
func TestPanicRecovery(t *testing.T) {
	themeFS, err := themes.Plain()
	if err != nil {
		t.Fatalf("themes.Plain: %v", err)
	}
	theme, _, err := render.LoadTheme(themeFS)
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	staticFS, err := fs.Sub(themeFS, "static")
	if err != nil {
		t.Fatalf("fs.Sub static: %v", err)
	}

	registry := render.NewRegistry("plain")
	registry.Register("plain", theme, staticFS)
	h := httpapi.NewRouter(httpapi.Deps{
		Store:         panicStore{},
		Ingest:        ingest.New(panicStore{}, slug.New(rand.Reader)),
		Render:        render.NewService(registry, "TestCompany"),
		StaticFS:      registry.MergedStaticFS(),
		WebhookSecret: testSecret,
		BaseURL:       testBaseURL,
		Logger:        discardLogger(),
	})

	rec := getPath(t, h, "/c/anything")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}

	// The server must still be alive for the next request.
	rec = getPath(t, h, "/healthz")
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz after panic: status = %d, want 200", rec.Code)
	}
}

// TestStaticAssets covers the /static/{path...} contract from 1.2: served
// from the theme's embed.FS with a long-lived, immutable Cache-Control.
func TestStaticAssets(t *testing.T) {
	h := newTestRouter(t)

	rec := getPath(t, h, "/static/style.css")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	want := "public, max-age=31536000, immutable"
	if got := rec.Header().Get("Cache-Control"); got != want {
		t.Fatalf("Cache-Control = %q, want %q", got, want)
	}
}
