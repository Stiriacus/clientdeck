package render

import (
	"bytes"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Stiriacus/vitrine/internal/board"
)

func fakeThemeFS() fstest.MapFS {
	return fstest.MapFS{
		"layout.html": {Data: []byte(
			`<html lang="{{.Language}}"><body>{{block "content" .}}{{end}}</body></html>`,
		)},
		"board.html": {Data: []byte(
			`{{define "content"}}<h1>{{.ClientName}}</h1><p>{{formatDate .GeneratedAt}}</p>{{end}}`,
		)},
		"notfound.html": {Data: []byte(
			`{{define "content"}}<p>not found</p>{{end}}`,
		)},
		"i18n/en.json": {Data: []byte(`{}`)},
	}
}

func TestLoadTheme_RenderBoard(t *testing.T) {
	theme, bundle, err := LoadTheme(fakeThemeFS())
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}

	v := board.BoardView{
		ClientName:  "ACME Corp",
		GeneratedAt: time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC),
		Language:    "en",
		T:           func(key string) string { return bundle.T("en", key) },
	}

	var buf bytes.Buffer
	if err := theme.RenderBoard(&buf, v); err != nil {
		t.Fatalf("RenderBoard: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "<h1>ACME Corp</h1>") {
		t.Fatalf("output missing client name: %s", out)
	}
	if !strings.Contains(out, "21 July 2026") {
		t.Fatalf("output missing formatted date: %s", out)
	}
}

func TestLoadTheme_RenderBoard_EscapesHTML(t *testing.T) {
	theme, bundle, err := LoadTheme(fakeThemeFS())
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}

	v := board.BoardView{
		ClientName: `<script>alert(1)</script>`,
		Language:   "en",
		T:          func(key string) string { return bundle.T("en", key) },
	}

	var buf bytes.Buffer
	if err := theme.RenderBoard(&buf, v); err != nil {
		t.Fatalf("RenderBoard: %v", err)
	}
	if strings.Contains(buf.String(), "<script>") {
		t.Fatalf("output not escaped: %s", buf.String())
	}
}

func TestLoadTheme_RenderNotFound(t *testing.T) {
	theme, _, err := LoadTheme(fakeThemeFS())
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}

	var buf bytes.Buffer
	if err := theme.RenderNotFound(&buf, "en"); err != nil {
		t.Fatalf("RenderNotFound: %v", err)
	}
	if !strings.Contains(buf.String(), "not found") {
		t.Fatalf("output = %s, want not-found content", buf.String())
	}
}

func TestLoadTheme_MissingTemplateFile(t *testing.T) {
	fsys := fakeThemeFS()
	delete(fsys, "board.html")

	if _, _, err := LoadTheme(fsys); err == nil {
		t.Fatalf("LoadTheme: expected error for missing board.html")
	}
}
