package render

import (
	"io"
	"log/slog"
	"time"

	"github.com/Stiriacus/vitrine/internal/board"
)

// Service is the domain-level entry point for rendering a stored
// CustomerView: it builds the BoardView view model and hands it to the
// customer's configured theme, falling back to the default theme when
// none is set or the configured theme is unknown.
type Service struct {
	registry  *Registry
	clock     func() time.Time
	poweredBy string
}

// NewService returns a Service that dispatches rendering through registry.
func NewService(registry *Registry, poweredBy string) *Service {
	return &Service{registry: registry, clock: time.Now, poweredBy: poweredBy}
}

// RenderBoard builds v's BoardView as of now and renders it via the
// customer's configured theme (v.Theme). If v.Theme is empty or refers to
// an unknown theme, the registry's default theme is used.
func (s *Service) RenderBoard(w io.Writer, v board.CustomerView) error {
	themeName := v.Theme
	if themeName == "" || !s.registry.Has(themeName) {
		if themeName != "" {
			slog.Warn("unknown theme, falling back to default",
				"theme", themeName,
				"customer_id", v.CustomerID,
			)
		}
		themeName = s.registry.DefaultTheme()
	}
	theme, _ := s.registry.Get(themeName)
	bv := BuildBoardView(v, s.clock(), theme.Bundle(), s.poweredBy)
	bv.Theme = themeName
	return theme.RenderBoard(w, bv)
}

// RenderNotFound renders the default theme's not-found page in the given
// language. The resolved theme name is available as {{.Theme}} in the template.
func (s *Service) RenderNotFound(w io.Writer, lang string) error {
	theme, _ := s.registry.Get("")
	return theme.RenderNotFoundWithTheme(w, lang, s.registry.DefaultTheme())
}
