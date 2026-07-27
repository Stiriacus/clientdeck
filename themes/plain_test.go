package themes_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Stiriacus/vitrine/internal/board"
	"github.com/Stiriacus/vitrine/internal/i18n"
	"github.com/Stiriacus/vitrine/internal/render"
	"github.com/Stiriacus/vitrine/themes"
)

var update = flag.Bool("update", false, "update golden files")

func loadTheme(t *testing.T) (*render.Theme, *i18n.Bundle) {
	t.Helper()
	fsys, err := themes.Plain()
	if err != nil {
		t.Fatalf("themes.Plain: %v", err)
	}
	theme, bundle, err := render.LoadTheme(fsys)
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	return theme, bundle
}

func loadCustomerView(t *testing.T, path string) board.CustomerView {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var v board.CustomerView
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return v
}

// TestBoardGolden renders testdata/board_full.json, deliberately containing
// the hard cases (mixed spec keys, a half-star rating, a product without an
// image, a very long title/recommendation, non-ASCII characters), and
// compares it byte-for-byte against testdata/golden/board_full.html. Run
// with `go test ./themes/... -update` (or `make golden`) to regenerate it
// after an intentional template/CSS change.
func TestBoardGolden(t *testing.T) {
	theme, bundle := loadTheme(t)
	v := loadCustomerView(t, "../testdata/board_full.json")
	v.Slug = "riverside-books-office-a8f9b2"

	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	view := render.BuildBoardView(v, now, bundle, "TestCompany")

	var buf bytes.Buffer
	if err := theme.RenderBoard(&buf, view); err != nil {
		t.Fatalf("RenderBoard: %v", err)
	}

	goldenPath := filepath.Join("..", "testdata", "golden", "board_full.html")
	if *update {
		if err := os.WriteFile(goldenPath, buf.Bytes(), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if buf.String() != string(want) {
		t.Errorf("rendered board HTML does not match golden file %s (run with -update to refresh it after an intentional change)", goldenPath)
	}
}

func TestNotFoundRenders(t *testing.T) {
	theme, _ := loadTheme(t)
	var buf bytes.Buffer
	if err := theme.RenderNotFound(&buf, "en"); err != nil {
		t.Fatalf("RenderNotFound: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("404")) {
		t.Errorf("expected not-found page to contain 404, got: %s", buf.String())
	}
}

// TestXSSEscaped feeds script-injection payloads through every free-text
// field reachable from the template and asserts html/template's
// auto-escaping neutralized all of them.
func TestXSSEscaped(t *testing.T) {
	theme, bundle := loadTheme(t)
	rating := 3.0
	v := board.CustomerView{
		CustomerID: "xss-test",
		ClientName: `<script>alert(1)</script>`,
		Intro:      `<img src=x onerror=alert(0)>`,
		Slug:       "xss-test-slug",
		Products: []board.Product{
			{
				Category:       "Test",
				Title:          `"><img src=x onerror=alert(2)>`,
				Recommendation: `<script>alert(3)</script>`,
				Badge:          `<script>alert(4)</script>`,
				Specs:          map[string]string{`<script>k</script>`: `<script>alert(5)</script>`},
				Rating:         &rating,
			},
		},
	}
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	view := render.BuildBoardView(v, now, bundle, "TestCompany")

	var buf bytes.Buffer
	if err := theme.RenderBoard(&buf, view); err != nil {
		t.Fatalf("RenderBoard: %v", err)
	}
	out := buf.String()

	rawPayloads := []string{
		`<script>alert(1)</script>`,
		`<img src=x onerror=alert(0)>`,
		`"><img src=x onerror=alert(2)>`,
		`<script>alert(3)</script>`,
		`<script>alert(4)</script>`,
		`<script>k</script>`,
		`<script>alert(5)</script>`,
	}
	for _, raw := range rawPayloads {
		if bytes.Contains([]byte(out), []byte(raw)) {
			t.Errorf("unescaped payload found in output: %q", raw)
		}
	}
	if !bytes.Contains([]byte(out), []byte("&lt;script&gt;alert(1)&lt;/script&gt;")) {
		t.Errorf("expected escaped client name in output")
	}
}
