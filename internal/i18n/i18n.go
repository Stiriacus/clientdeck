// Package i18n provides a lightweight translation catalog loaded from JSON
// files bundled with a theme. It supports a fallback chain: requested language
// → default language ("en") → raw key (so missing translations are visible,
// not silent).
package i18n

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// Bundle holds translation messages keyed by language code (e.g. "en", "fr").
type Bundle struct {
	messages map[string]map[string]string // lang → key → value
}

// Load reads all *.json files from an fs.FS (typically a theme's i18n/
// directory). Each file must be named <lang>.json and contain a flat
// JSON object of translation keys → messages.
func Load(fsys fs.FS) (*Bundle, error) {
	b := &Bundle{messages: make(map[string]map[string]string)}

	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("i18n: read dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		lang := strings.TrimSuffix(entry.Name(), ".json")
		if lang == "" {
			continue
		}

		data, err := fs.ReadFile(fsys, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("i18n: read %s: %w", entry.Name(), err)
		}

		var msgs map[string]string
		if err := json.Unmarshal(data, &msgs); err != nil {
			return nil, fmt.Errorf("i18n: parse %s: %w", entry.Name(), err)
		}

		b.messages[lang] = msgs
	}

	if len(b.messages) == 0 {
		return nil, fmt.Errorf("i18n: no translation files found")
	}

	return b, nil
}

// T looks up key in the given language. Falls back to "en" if the language
// or key is missing, then to the key itself as a last resort.
func (b *Bundle) T(lang, key string) string {
	// Try requested language.
	if msgs, ok := b.messages[lang]; ok {
		if msg, ok := msgs[key]; ok {
			return msg
		}
	}
	// Fall back to English.
	if lang != "en" {
		if msgs, ok := b.messages["en"]; ok {
			if msg, ok := msgs[key]; ok {
				return msg
			}
		}
	}
	// Last resort: return the key itself so untranslated strings are visible.
	return key
}

// Supported returns true if lang is one of the loaded languages.
func (b *Bundle) Supported(lang string) bool {
	_, ok := b.messages[lang]
	return ok
}

// Languages returns the sorted list of loaded language codes.
func (b *Bundle) Languages() []string {
	langs := make([]string, 0, len(b.messages))
	for lang := range b.messages {
		langs = append(langs, lang)
	}
	return langs
}
