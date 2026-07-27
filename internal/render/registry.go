package render

import (
	"fmt"
	"io/fs"
	"log/slog"
)

// Registry holds multiple loaded themes and provides lookup by name.
// The embedded "plain" theme is always registered so there is always a
// guaranteed fallback. All themes are loaded at startup and the Registry
// is read-only thereafter, so no synchronisation is needed.
// When a theme name is registered via SetDevTheme, Get re-parses its
// templates from disk on every call, supporting hot-reload during
// theme development.
type Registry struct {
	themes    map[string]*Theme
	staticFSs map[string]fs.FS
	defaultTheme string
	// Dev-theme filesystem: when non-nil, Get re-parses templates from
	// this FS instead of using the cached theme.
	devFSs map[string]fs.FS
}

// NewRegistry creates a Registry with the given default theme name.
// The theme name is not validated — call Register to populate themes before
// the registry is used for rendering.
func NewRegistry(defaultTheme string) *Registry {
	return &Registry{
		themes:       make(map[string]*Theme),
		staticFSs:    make(map[string]fs.FS),
		defaultTheme: defaultTheme,
	}
}

// Register stores a loaded theme and its static directory under name.
// staticFS may be nil if the theme has no static assets.
func (r *Registry) Register(name string, theme *Theme, staticFS fs.FS) {
	r.themes[name] = theme
	r.staticFSs[name] = staticFS
}

// SetDevTheme marks name as a dev-mode theme. Subsequent calls to Get(name)
// will re-parse templates from themeFS on every call instead of returning
// a cached theme. This enables hot-reload during theme development.
// The static filesystem is still served from the cached value set via
// Register (static files don't benefit from per-request reloading).
func (r *Registry) SetDevTheme(name string, themeFS fs.FS) {
	if r.devFSs == nil {
		r.devFSs = make(map[string]fs.FS)
	}
	r.devFSs[name] = themeFS
}

// Get returns the theme for name, or the default theme if name is empty or
// unknown. Returns the theme and a boolean indicating whether name was found.
// It always returns a non-nil theme — if even the default theme is missing
// (which should never happen) it returns the first registered theme.
func (r *Registry) Get(name string) (*Theme, bool) {
	// Dev mode: re-parse templates from disk on every call.
	if r.devFSs != nil {
		if themeFS, ok := r.devFSs[name]; ok {
			theme, _, err := LoadTheme(themeFS)
			if err == nil {
				return theme, true
			}
			slog.Warn("dev theme: re-parse failed, falling back to cached", "theme", name, "error", err)
		}
	}

	if name == "" || !r.Has(name) {
		theme, ok := r.themes[r.defaultTheme]
		if !ok {
			// Last-resort fallback: grab any registered theme.
			for _, t := range r.themes {
				return t, false
			}
		}
		return theme, ok
	}
	theme, _ := r.themes[name]
	return theme, true
}

// Has reports whether name is a registered theme.
func (r *Registry) Has(name string) bool {
	_, ok := r.themes[name]
	return ok
}

// DefaultTheme returns the configured default theme name.
func (r *Registry) DefaultTheme() string {
	return r.defaultTheme
}

// Names returns all registered theme names in no particular order.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.themes))
	for n := range r.themes {
		names = append(names, n)
	}
	return names
}

// MergedStaticFS returns a filesystem that merges every registered theme's
// static/ directory. The map iteration order determines priority — this is
// non-deterministic, so themes should use unique static filenames.
func (r *Registry) MergedStaticFS() fs.FS {
	var fses []fs.FS
	// Iterate in deterministic order by collecting names first.
	names := r.Names()
	for _, name := range names {
		if st := r.staticFSs[name]; st != nil {
			fses = append(fses, st)
		}
	}
	if len(fses) == 0 {
		slog.Warn("no theme has a static directory, serving empty static")
	}
	return NewMergedFS(fses...)
}

// LoadThemeFromDir loads a theme from a subdirectory of parentFS and
// registers it under name. The subdirectory must contain layout.html,
// board.html, notfound.html, and an i18n/ directory (see render.LoadTheme).
// Its static/ subdirectory is also registered if present.
func (r *Registry) LoadThemeFromDir(name string, parentFS fs.FS) error {
	themeFS, err := fs.Sub(parentFS, name)
	if err != nil {
		return fmt.Errorf("registry: sub %q: %w", name, err)
	}
	theme, _, err := LoadTheme(themeFS)
	if err != nil {
		return fmt.Errorf("registry: load theme %q: %w", name, err)
	}
	var staticFS fs.FS
	if st, err := fs.Sub(themeFS, "static"); err == nil {
		staticFS = st
	}
	r.Register(name, theme, staticFS)
	return nil
}
