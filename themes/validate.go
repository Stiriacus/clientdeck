// Package themes embeds vitrine's built-in themes and provides validation
// helpers for custom themes stored on disk.
package themes

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Stiriacus/vitrine/internal/render"
)

// ValidateDir checks that a directory on disk contains a valid vitrine theme.
// It delegates to render.LoadTheme, which parses templates with the full
// FuncMap and loads the i18n bundle — the exact same path used at runtime.
// Static assets are optional — their absence is not an error.
//
// This is meant for theme authors to validate their work locally; it is
// intentionally not part of CI. Callers can inspect a non-nil error with
// errors.Is for specific failure reasons.
func ValidateDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("validate theme: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("validate theme: %s is not a directory", path)
	}

	dirFS := os.DirFS(path)

	// Check required template files exist before handing off to LoadTheme,
	// so we can give clearer error messages.
	for _, name := range []string{"layout.html", "board.html", "notfound.html"} {
		if _, err := fs.Stat(dirFS, name); err != nil {
			return fmt.Errorf("validate theme: missing required file %s/%s: %w", filepath.Base(path), name, err)
		}
	}

	// Delegate to the real loader — same FuncMap, same template rules.
	if _, _, err := render.LoadTheme(dirFS); err != nil {
		return fmt.Errorf("validate theme: %w", err)
	}

	// Warn about a missing static/ directory — not an error, but helpful.
	if _, err := fs.Stat(dirFS, "static"); errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "theme %s: warning: no static/ directory (optional)\n", filepath.Base(path))
	}

	return nil
}
