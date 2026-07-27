// Package themes embeds vitrine's built-in themes so the binary needs no
// files on disk at runtime (see internal/config's VITRINE_DEV for the
// disk-backed alternative used during theme development).
package themes

import (
	"embed"
	"io/fs"
)

//go:embed plain/layout.html plain/board.html plain/notfound.html plain/static plain/i18n
//go:embed frost/layout.html frost/board.html frost/notfound.html frost/static frost/i18n
var themesFS embed.FS

// Plain returns an fs.FS rooted at the "plain" theme directory itself, as
// required by render.LoadTheme and for serving /static/* from the same tree.
func Plain() (fs.FS, error) {
	return fs.Sub(themesFS, "plain")
}

// Frost returns an fs.FS rooted at the "frost" theme directory itself.
func Frost() (fs.FS, error) {
	return fs.Sub(themesFS, "frost")
}
