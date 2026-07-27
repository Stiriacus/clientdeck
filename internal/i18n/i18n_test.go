package i18n

import (
	"testing"
	"testing/fstest"
)

func TestLoad_Success(t *testing.T) {
	fsys := fstest.MapFS{
		"en.json": {Data: []byte(`{"hello": "Hello", "world": "World"}`)},
		"fr.json": {Data: []byte(`{"hello": "Bonjour"}`)},
	}

	b, err := Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Direct lookup.
	if got := b.T("en", "hello"); got != "Hello" {
		t.Fatalf("T(en, hello) = %q, want %q", got, "Hello")
	}
	if got := b.T("fr", "hello"); got != "Bonjour" {
		t.Fatalf("T(fr, hello) = %q, want %q", got, "Bonjour")
	}

	// Fallback to English when key missing in requested language.
	if got := b.T("fr", "world"); got != "World" {
		t.Fatalf("T(fr, world) = %q, want English fallback %q", got, "World")
	}

	// Fallback to key itself when completely missing.
	if got := b.T("en", "missing"); got != "missing" {
		t.Fatalf("T(en, missing) = %q, want key itself %q", got, "missing")
	}

	// Unknown language falls back to English.
	if got := b.T("es", "hello"); got != "Hello" {
		t.Fatalf("T(es, hello) = %q, want English fallback %q", got, "Hello")
	}

	// Supported check.
	if !b.Supported("en") {
		t.Fatal("expected en to be supported")
	}
	if !b.Supported("fr") {
		t.Fatal("expected fr to be supported")
	}
	if b.Supported("es") {
		t.Fatal("expected es to NOT be supported")
	}
}

func TestLoad_Empty(t *testing.T) {
	fsys := fstest.MapFS{}
	_, err := Load(fsys)
	if err == nil {
		t.Fatal("expected error for empty i18n dir")
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	fsys := fstest.MapFS{
		"en.json": {Data: []byte(`not json`)},
	}
	_, err := Load(fsys)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
