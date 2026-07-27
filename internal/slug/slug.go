// Package slug generates URL-safe, capability-style slugs for boards.
package slug

import (
	"fmt"
	"io"
	"strings"
)

const (
	maxSlugifiedLen  = 32
	defaultSuffixLen = 6
)

const base62Alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var umlautReplacer = strings.NewReplacer(
	"ä", "ae",
	"ö", "oe",
	"ü", "ue",
	"ß", "ss",
)

// Slugify lowercases s, transliterates umlauts and special characters, replaces every
// remaining character outside [a-z0-9-] with '-', collapses repeated '-',
// trims leading/trailing '-' and truncates to 32 characters. An empty
// result becomes "board" so callers always get a non-empty base.
func Slugify(s string) string {
	s = strings.ToLower(s)
	s = umlautReplacer.Replace(s)

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	s = collapseDashes(b.String())
	s = strings.Trim(s, "-")

	if len(s) > maxSlugifiedLen {
		s = strings.Trim(s[:maxSlugifiedLen], "-")
	}

	if s == "" {
		return "board"
	}
	return s
}

func collapseDashes(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevDash := false
	for _, r := range s {
		if r == '-' {
			if prevDash {
				continue
			}
			prevDash = true
		} else {
			prevDash = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Generator produces slugs of the form "<slugified-client-name>-<suffix>",
// drawing suffix entropy from an injected io.Reader so tests can be
// deterministic while production uses crypto/rand.Reader.
type Generator struct {
	rand      io.Reader
	suffixLen int
}

// New returns a Generator that reads suffix entropy from r. In production r
// should be crypto/rand.Reader.
func New(r io.Reader) *Generator {
	return &Generator{rand: r, suffixLen: defaultSuffixLen}
}

// For builds a slug for clientName: a slugified base plus a random base62
// suffix, joined by '-'. Each call draws fresh entropy, so repeated calls
// for the same client name yield different slugs.
func (g *Generator) For(clientName string) (string, error) {
	suffix, err := g.suffix()
	if err != nil {
		return "", err
	}
	return Slugify(clientName) + "-" + suffix, nil
}

func (g *Generator) suffix() (string, error) {
	buf := make([]byte, g.suffixLen)
	if _, err := io.ReadFull(g.rand, buf); err != nil {
		return "", fmt.Errorf("slug: read entropy: %w", err)
	}
	out := make([]byte, g.suffixLen)
	for i, b := range buf {
		out[i] = base62Alphabet[int(b)%len(base62Alphabet)]
	}
	return string(out), nil
}
