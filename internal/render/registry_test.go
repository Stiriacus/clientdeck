package render

import (
	"bytes"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Stiriacus/vitrine/internal/board"
)

func TestRegistry_NewRegistry(t *testing.T) {
	r := NewRegistry("plain")
	if r.DefaultTheme() != "plain" {
		t.Fatalf("DefaultTheme() = %q, want %q", r.DefaultTheme(), "plain")
	}
	if len(r.Names()) != 0 {
		t.Fatalf("Names() = %v, want empty", r.Names())
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry("plain")
	theme, _, err := LoadTheme(fakeThemeFS())
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	r.Register("plain", theme, nil)

	got, ok := r.Get("plain")
	if !ok {
		t.Fatal("Get('plain') returned ok=false")
	}
	if got == nil {
		t.Fatal("Get('plain') returned nil theme")
	}

	var buf bytes.Buffer
	bv := board.BoardView{ClientName: "Test", GeneratedAt: time.Now(), Language: "en", T: func(string) string { return "" }}
	if err := got.RenderBoard(&buf, bv); err != nil {
		t.Fatalf("RenderBoard: %v", err)
	}
	if !strings.Contains(buf.String(), "<h1>Test</h1>") {
		t.Fatalf("output missing client name: %s", buf.String())
	}
}

func TestRegistry_GetEmptyName(t *testing.T) {
	r := NewRegistry("plain")
	theme, _, _ := LoadTheme(fakeThemeFS())
	r.Register("plain", theme, nil)

	got, ok := r.Get("")
	if !ok {
		t.Fatal("Get('') returned ok=false")
	}
	if got == nil {
		t.Fatal("Get('') returned nil theme")
	}
}

func TestRegistry_GetUnknownName(t *testing.T) {
	r := NewRegistry("plain")
	theme, _, _ := LoadTheme(fakeThemeFS())
	r.Register("plain", theme, nil)

	got, ok := r.Get("nonexistent")
	if !ok {
		t.Fatal("Get('nonexistent') returned ok=false — default theme should resolve")
	}
	if got == nil {
		t.Fatal("Get('nonexistent') returned nil theme")
	}
}

func TestRegistry_GetUnknownNameFallsBackToFirst(t *testing.T) {
	r := NewRegistry("missing")
	theme, _, _ := LoadTheme(fakeThemeFS())
	r.Register("only-theme", theme, nil)

	got, ok := r.Get("nonexistent")
	if ok {
		t.Fatal("Get('nonexistent') returned ok=true")
	}
	if got == nil {
		t.Fatal("Get('nonexistent') returned nil")
	}

	var buf bytes.Buffer
	bv := board.BoardView{ClientName: "Fallback", GeneratedAt: time.Now(), Language: "en", T: func(string) string { return "" }}
	if err := got.RenderBoard(&buf, bv); err != nil {
		t.Fatalf("RenderBoard on fallback: %v", err)
	}
}

func TestRegistry_GetEmptyRegistry(t *testing.T) {
	r := NewRegistry("nonexistent")
	got, _ := r.Get("anything")
	if got != nil {
		t.Fatal("Get on empty registry should return nil")
	}
}

func TestRegistry_Has(t *testing.T) {
	r := NewRegistry("plain")
	theme, _, _ := LoadTheme(fakeThemeFS())
	r.Register("plain", theme, nil)

	if !r.Has("plain") {
		t.Fatal("Has('plain') = false after Register")
	}
	if r.Has("unknown") {
		t.Fatal("Has('unknown') = true for unregistered theme")
	}
}

func TestRegistry_Names(t *testing.T) {
	r := NewRegistry("a")
	theme, _, _ := LoadTheme(fakeThemeFS())
	r.Register("a", theme, nil)
	r.Register("b", theme, nil)

	names := r.Names()
	if len(names) != 2 {
		t.Fatalf("Names() = %v, want 2 entries", names)
	}
	m := make(map[string]bool)
	for _, n := range names {
		m[n] = true
	}
	if !m["a"] || !m["b"] {
		t.Fatalf("Names() missing expected entries: %v", names)
	}
}

func TestRegistry_MergedStaticFS(t *testing.T) {
	r := NewRegistry("plain")
	theme, _, _ := LoadTheme(fakeThemeFS())

	staticA := fstest.MapFS{"a.css": {Data: []byte("/* a */")}}
	staticB := fstest.MapFS{"b.css": {Data: []byte("/* b */")}}
	r.Register("a", theme, staticA)
	r.Register("b", theme, staticB)

	merged := r.MergedStaticFS()

	f, err := merged.Open("a.css")
	if err != nil {
		t.Fatalf("MergedStaticFS.Open('a.css'): %v", err)
	}
	f.Close()

	f, err = merged.Open("b.css")
	if err != nil {
		t.Fatalf("MergedStaticFS.Open('b.css'): %v", err)
	}
	f.Close()

	if _, err := merged.Open("nonexistent.css"); err == nil {
		t.Fatal("MergedStaticFS.Open('nonexistent.css') should fail")
	}
}

func TestRegistry_LoadThemeFromDir(t *testing.T) {
	dir := fstest.MapFS{
		"mytheme/layout.html":   {Data: []byte(`<html>{{block "content" .}}{{end}}</html>`)},
		"mytheme/board.html":    {Data: []byte(`{{define "content"}}<h1>Custom</h1>{{end}}`)},
		"mytheme/notfound.html": {Data: []byte(`{{define "content"}}<p>404</p>{{end}}`)},
		"mytheme/i18n/en.json":  {Data: []byte(`{}`)},
	}

	r := NewRegistry("custom")
	if err := r.LoadThemeFromDir("mytheme", dir); err != nil {
		t.Fatalf("LoadThemeFromDir: %v", err)
	}
	if !r.Has("mytheme") {
		t.Fatal("mytheme not registered")
	}

	got, ok := r.Get("mytheme")
	if !ok {
		t.Fatal("Get('mytheme') returned ok=false")
	}
	var buf bytes.Buffer
	bv := board.BoardView{ClientName: "X", GeneratedAt: time.Now(), Language: "en", T: func(string) string { return "" }}
	if err := got.RenderBoard(&buf, bv); err != nil {
		t.Fatalf("RenderBoard: %v", err)
	}
}

func TestRegistry_LoadThemeFromDir_NotFound(t *testing.T) {
	dir := fstest.MapFS{"other/layout.html": {Data: []byte(`<html></html>`)}} // no i18n -> error
	r := NewRegistry("plain")
	if err := r.LoadThemeFromDir("nonexistent", dir); err == nil {
		t.Fatal("LoadThemeFromDir on nonexistent dir should return error")
	}
}

func TestRegistry_DevMode(t *testing.T) {
	fsys := fakeThemeFS()
	theme, _, err := LoadTheme(fsys)
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}

	r := NewRegistry("plain")
	r.Register("plain", theme, nil)

	devFS := fstest.MapFS{
		"layout.html":   {Data: []byte(`<html>{{block "content" .}}{{end}}</html>`)},
		"board.html":    {Data: []byte(`{{define "content"}}<h1>DEV MODE</h1>{{end}}`)},
		"notfound.html": {Data: []byte(`{{define "content"}}<p>dev 404</p>{{end}}`)},
		"i18n/en.json":  {Data: []byte(`{}`)},
	}
	r.SetDevTheme("plain", devFS)

	got, ok := r.Get("plain")
	if !ok {
		t.Fatal("Get('plain') with dev mode returned ok=false")
	}

	var buf bytes.Buffer
	bv := board.BoardView{ClientName: "Test", GeneratedAt: time.Now(), Language: "en", T: func(string) string { return "" }}
	if err := got.RenderBoard(&buf, bv); err != nil {
		t.Fatalf("RenderBoard: %v", err)
	}
	if !strings.Contains(buf.String(), "DEV MODE") {
		t.Fatalf("dev mode output should contain 'DEV MODE', got: %s", buf.String())
	}
}

func TestRegistry_DevMode_FallbackOnParseError(t *testing.T) {
	theme, _, err := LoadTheme(fakeThemeFS())
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}

	r := NewRegistry("plain")
	r.Register("plain", theme, nil)

	r.SetDevTheme("plain", fstest.MapFS{})

	got, ok := r.Get("plain")
	if !ok {
		t.Fatal("Get('plain') with broken dev FS returned ok=false")
	}
	if got == nil {
		t.Fatal("Get('plain') with broken dev FS returned nil")
	}

	var buf bytes.Buffer
	bv := board.BoardView{ClientName: "Fallback", GeneratedAt: time.Now(), Language: "en", T: func(string) string { return "" }}
	if err := got.RenderBoard(&buf, bv); err != nil {
		t.Fatalf("RenderBoard: %v", err)
	}
	if !strings.Contains(buf.String(), "Fallback") {
		t.Fatalf("should use cached theme on parse error, got: %s", buf.String())
	}
}
