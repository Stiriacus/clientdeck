package render

import (
	"io"
	"time"

	"github.com/Stiriacus/vitrine/internal/board"
	"github.com/Stiriacus/vitrine/internal/i18n"
)

// Service is the domain-level entry point for rendering a stored
// CustomerView: it builds the BoardView view model and hands it to the
// configured theme.
type Service struct {
	theme     board.Renderer
	bundle    *i18n.Bundle
	clock     func() time.Time
	poweredBy string
}

// NewService returns a Service that renders through theme using bundle for
// translations.
func NewService(theme board.Renderer, bundle *i18n.Bundle, poweredBy string) *Service {
	return &Service{theme: theme, bundle: bundle, clock: time.Now, poweredBy: poweredBy}
}

// RenderBoard builds v's BoardView as of now and renders it via the theme.
func (s *Service) RenderBoard(w io.Writer, v board.CustomerView) error {
	return s.theme.RenderBoard(w, BuildBoardView(v, s.clock(), s.bundle, s.poweredBy))
}

// RenderNotFound renders the theme's not-found page in the given language.
func (s *Service) RenderNotFound(w io.Writer, lang string) error {
	return s.theme.RenderNotFound(w, lang)
}
