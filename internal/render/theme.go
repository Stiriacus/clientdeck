package render

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"time"
	"unicode/utf8"

	"github.com/Stiriacus/vitrine/internal/board"
	"github.com/Stiriacus/vitrine/internal/i18n"
)

// notFoundData is the minimal data passed to the not-found template so it can
// use {{.T}} and {{.Language}} like the board template.
type notFoundData struct {
	Language string
	T        func(string) string
}

// Theme parses a theme's templates from an fs.FS and renders board.BoardView
// values against them. It implements board.Renderer.
type Theme struct {
	board    *template.Template
	notFound *template.Template
	bundle   *i18n.Bundle
}

var funcMap = template.FuncMap{
	"stars":         stars,
	"hasSpecs":      hasSpecs,
	"formatDate":    formatDate,
	"truncateTitle": truncateTitle,
}

// LoadTheme parses layout.html+board.html into one template tree and
// layout.html+notfound.html into another, so board.html and notfound.html
// can each define their own "content" block without colliding on the name.
// It also loads the i18n translation bundle from the theme's i18n/
// subdirectory. fsys must be rooted at the theme directory itself (e.g.
// themes/plain), not at themes/.
func LoadTheme(fsys fs.FS) (*Theme, *i18n.Bundle, error) {
	boardTmpl, err := template.New("layout.html").Funcs(funcMap).ParseFS(fsys, "layout.html", "board.html")
	if err != nil {
		return nil, nil, fmt.Errorf("render: load theme: %w", err)
	}
	notFoundTmpl, err := template.New("layout.html").Funcs(funcMap).ParseFS(fsys, "layout.html", "notfound.html")
	if err != nil {
		return nil, nil, fmt.Errorf("render: load theme: %w", err)
	}

	i18nFS, err := fs.Sub(fsys, "i18n")
	if err != nil {
		return nil, nil, fmt.Errorf("render: theme missing i18n/ directory: %w", err)
	}
	bundle, err := i18n.Load(i18nFS)
	if err != nil {
		return nil, nil, fmt.Errorf("render: load i18n: %w", err)
	}

	return &Theme{board: boardTmpl, notFound: notFoundTmpl, bundle: bundle}, bundle, nil
}

// Bundle returns the theme's i18n translation bundle.
func (t *Theme) Bundle() *i18n.Bundle {
	return t.bundle
}

// RenderBoard renders v through the theme's board template.
func (t *Theme) RenderBoard(w io.Writer, v board.BoardView) error {
	return t.board.ExecuteTemplate(w, "layout.html", v)
}

// RenderNotFound renders the theme's 404 page in the given language.
func (t *Theme) RenderNotFound(w io.Writer, lang string) error {
	data := notFoundData{
		Language: lang,
		T:        func(key string) string { return t.bundle.T(lang, key) },
	}
	return t.notFound.ExecuteTemplate(w, "layout.html", data)
}

func hasSpecs(keys []string) bool {
	return len(keys) > 0
}

func formatDate(t time.Time) string {
	return t.Format("2 January 2006")
}

// truncateTitle shortens s to max runes. If truncated, "…" is appended.
func truncateTitle(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	runes := []rune(s)
	return string(runes[:limit]) + "…"
}
