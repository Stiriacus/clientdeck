# Theming

vitrine renders boards through a swappable `Renderer` interface. The
embedded default theme is "plain" (`themes/plain/`), plain, unbranded, no
JS and no external fonts. This document describes the contract that a
custom (including proprietary) theme is built against.

## The `Renderer` interface

```go
// internal/board/ports.go
type Renderer interface {
    RenderBoard(w io.Writer, v BoardView) error
    RenderNotFound(w io.Writer, lang string) error
}
```

`internal/render.Theme` (`internal/render/theme.go`) is the bundled
implementation: it parses templates from an `fs.FS` and thereby implements
`Renderer`. A custom theme can either reuse this exact format (see below) or
implement `Renderer` entirely on its own, e.g. to use a different template
engine.

## The path of least effort: a custom `html/template` theme

1. Create a new directory, e.g. `themes/mytheme/`, with at least:
   - `layout.html` that defines the base HTML structure and a
     `{{block "content" .}}` slot
   - `board.html` defines `{{define "content"}}` for the board
   - `notfound.html` defines `{{define "content"}}` for the 404 page
   - `i18n/`, translation files (at minimum `en.json`); `LoadTheme` returns
     an error without this directory. Each file is `{lang}.json` with
     flat key→message pairs. See the plain theme's `i18n/` for the
     required key set.
   - optionally `static/` for CSS/JS/fonts
2. `render.LoadTheme(fsys fs.FS)` parses `layout.html`+`board.html` and
   `layout.html`+`notfound.html` as two separate template trees (kept
   separate so both pages can independently define their own `"content"`
   block without colliding). `fsys` must point at the theme directory
   itself (e.g. `themes/mytheme`), not at `themes/`.
3. For development: `VITRINE_DEV=true VITRINE_THEME=mytheme` reads
   templates and `static/` fresh from disk on every request
   (`os.DirFS("themes")` + `fs.Sub(..., "mytheme")`). No rebuild needed,
   see `README.md`.
4. For production: mount your theme directories via Docker volume and set
   `VITRINE_THEMES_DIR=/themes`. Every subdirectory is loaded as a theme
   at startup — no `VITRINE_DEV` required. The embedded `"plain"` theme
   is always available as fallback. Multiple themes can coexist; customers
   are assigned a theme via the `"theme"` field in the webhook payload.

## The view model

All values a theme has available, fully precomputed so templates don't
need map lookups with fallback logic:

```go
// internal/board/view.go
type BoardView struct {
    ClientName  string
    Slug        string
    Intro       string
    GeneratedAt time.Time
    Language    string              // BCP 47, e.g. "en" or "fr", set on <html lang>
    Theme       string              // resolved theme name, e.g. "plain" or "frost"
    T           func(string) string // translation lookup: {{call $.T "key"}}
    PoweredBy   string              // from VITRINE_POWERED_BY, shown in footer
    Categories  []CategoryView
}

type CategoryView struct {
    Name     string        // "Printers"
    Anchor   string        // "printers", tab ID / URL fragment
    SpecKeys []string      // union of all spec keys in this category, stably sorted
    Products []ProductView
}

type ProductView struct {
    Title, Recommendation    string
    Price, Badge             string
    ImageURL, AffiliateLink  string
    Rating                   float64
    Stars                    []StarState       // exactly 5: full | half | empty
    SpecCells                []string          // same length/order as CategoryView.SpecKeys, "—" for missing
    Specs                    map[string]string // raw spec map for accordion <dl> rendering
    Highlights               []string          // key facts shown on card surface
    Pros                     []string          // advantages shown in accordion
    Cons                     []string          // disadvantages shown in accordion
}
```

Rules a theme can rely on (details/derivation in
`internal/render/viewmodel.go`):

- **`SpecKeys`** is the order of first appearance across all products in the
  category, deterministic, independent of the (not guaranteed) JSON object
  order in the request.
- **`SpecCells`** is already aligned to `SpecKeys`: same length, same order,
  `"—"` at every position where a product doesn't have that spec key. Use
  this for comparison-table rendering; for accordion-style rendering (one
  product at a time), iterate over `Specs` directly as a `{{range $k, $v}}`
  definition list.
- **`Specs`** is the product's raw spec map (shallow copy). Use it inside
  `<details>` accordions; `SpecCells`/`SpecKeys` remain available for
  cross-product comparison tables if the theme prefers that pattern.
- **`Highlights`** are 1–5 short key facts shown on the card surface so the
  customer can scan quickly. When the operator does not set them explicitly,
  the renderer auto-derives the first two spec entries (alphabetically sorted)
  as a fallback. So every card always has at least a quick summary.
- **`Pros` and `Cons`** are optional lists intended for AI-generated content.
  They are shown inside the product detail accordion and are empty (no
  heading rendered) when not provided.
- **`Stars`** always has exactly 5 entries (`StarFull`, `StarHalf`,
  `StarEmpty`). Even without a rating in the request the slice is nil
  (no star block rendered at all). Whether a star block is rendered is
  decided by the theme via `{{if .Stars}}`.
- **`Language`** is the board's BCP 47 language tag (default `"en"`). Set it
  on `<html lang="{{.Language}}">`.
- **`Theme`** is the resolved theme name (e.g. `"plain"`, `"frost"`). Use
  `{{.Theme}}` for conditional rendering or per-theme static asset paths
  if needed, though all themes share `/static/` via a merged filesystem.
- **`PoweredBy`** is the branding string from `VITRINE_POWERED_BY`.
- **`T`** is the translation function: `{{call $.T "key"}}` in templates
  (note the `call` builtin. Struct field functions can't be invoked directly
  in Go templates). Fallback chain: requested language → `"en"` → raw key.
- **Category order** = order of first appearance in the request's
  `products` array. The operator controls tab order via n8n, not via the
  theme.
- **`Anchor`** is `slug.Slugify(category)`, with `-2`, `-3`, … appended on a
  collision, suitable for tabs without JavaScript (`:target` or radio-button
  pattern, see below).

## Security requirements for a custom theme

These points are not optional. They're part of vitrine's security
model, not just a styling question of the default theme:

- **Never `template.HTML` or a `raw`/`safeHTML` template function.** All
  strings from the request (customer name, recommendation text, specs, …)
  go through `html/template`'s auto-escaping. A theme that bypasses this
  safeguard opens stored XSS via every free-text field in the payload.
- **Tabs must work without JavaScript** (CSS `:target` or radio-button
  pattern); JS may only enhance the interaction, never be required for it.
- **Respect `prefers-reduced-motion: reduce`** for every animation.
- **No external requests** other than the `image_url`s supplied by the
  operator (`<img loading="lazy" referrerpolicy="no-referrer">`). No
  tracking, no CDN embeds. The binary should be able to run without
  outbound connectivity.
- **Matching `Content-Security-Policy`.** vitrine itself sets
  `default-src 'self'; img-src 'self' https: data:; style-src 'self'; script-src 'self'`
  on every HTML response (`internal/httpapi/board.go`). A custom theme must
  therefore not use inline `<script>`/`<style>`, and must serve everything
  via `/static/*`.

## Custom functions in the template

The bundled `FuncMap` (`internal/render/theme.go`) is deliberately minimal:
`stars`, `hasSpecs`, `formatDate`. A custom `Renderer` (not `render.Theme`)
can bring its own `FuncMap`. The same rule applies: no function that
returns raw, unescaped HTML.

## Reference implementation

`themes/plain/` is the complete, embedded example: `layout.html`,
`board.html`, `notfound.html` plus `static/style.css`. `themes/plain_test.go`
also shows how a theme is tested against golden files and XSS payloads
(`make golden` rewrites the golden files after an intentional change).

## Validating custom themes

vitrine ships with a validation helper and a test runner for local
(non-embedded) themes. The test discovers every theme directory under
`themes/` (except `plain`, which is tested separately) and validates it
with the same `render.LoadTheme` path used at runtime — full `FuncMap`,
template parsing, and i18n loading.

```bash
# Validate all local themes
go test ./themes/ -run TestLocalThemes -v
```

This test passes trivially in CI (where no local themes exist), so it will
never break your build. When a theme is invalid, the test prints a specific
error message and the file that caused it.

You can also validate a single directory programmatically:

```go
import "github.com/Stiriacus/vitrine/themes"

if err := themes.ValidateDir("/path/to/my-theme"); err != nil {
    log.Fatal(err)
}
```

See `themes/README.md` for the directory structure and how to set up a
custom theme.
