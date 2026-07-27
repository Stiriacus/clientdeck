# Themes

`plain/` is vitrine's built-in theme, embedded into the binary at compile time
via `//go:embed`. It is always available as a fallback and requires no files on
disk at runtime.

## Custom themes

You can create your own theme in a new directory under `themes/`, e.g.
`themes/frost/`. A custom theme lives **only on your disk** (it is git-ignored
by default) and is loaded at runtime through one of two mechanisms:

| Scenario | Env vars | Reload |
|---|---|---|
| **Development** (hot-reload templates) | `VITRINE_DEV=true VITRINE_THEME=mytheme` | Every request |
| **Production** (Docker volume) | `VITRINE_THEMES_DIR=/themes` | Startup only |

## Required files

Every theme directory must contain:

```
themes/<name>/
├── layout.html       # base HTML structure with {{block "content" .}}
├── board.html        # {{define "content"}} for the board page
├── notfound.html     # {{define "content"}} for the 404 page
├── i18n/
│   ├── en.json       # at minimum, English translations
│   └── de.json       # additional languages (optional)
└── static/           # CSS, JS, fonts (optional but recommended)
```

See `docs/theming.md` for the complete theme contract, the `BoardView` data
model, and security requirements.

## Validating a custom theme

Run the local-theme validation test from the repository root:

```bash
go test ./themes/ -run TestLocalThemes -v
```

This discovers all theme directories under `themes/` (except `plain`, which is
tested separately), checks that required files exist, parses templates, and
loads translations. It produces no output in CI (where no local themes exist),
so it will never break your build.

If you have themes stored outside `themes/` (e.g. a Docker volume mount), you
can also validate a single directory programmatically:

```go
import "github.com/Stiriacus/vitrine/themes"

if err := themes.ValidateDir("/path/to/my-theme"); err != nil {
    log.Fatal(err)
}
```
