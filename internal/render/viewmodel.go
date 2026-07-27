// Package render turns a board.CustomerView into a board.BoardView and
// renders it through a theme loaded from an fs.FS.
package render

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/Stiriacus/vitrine/internal/board"
	"github.com/Stiriacus/vitrine/internal/i18n"
	"github.com/Stiriacus/vitrine/internal/slug"
)

// BuildBoardView assembles the rendering-ready BoardView for v as of now:
// products are grouped into categories in order of first appearance, each
// category's SpecKeys are unioned across its products (ties within a single
// product's spec map broken alphabetically, since map order is otherwise
// nondeterministic), and each product's rating is precomputed into a
// 5-entry Stars slice. The i18n bundle provides the T function set on the
// BoardView so templates can look up translations for the board's language.
func BuildBoardView(v board.CustomerView, now time.Time, bundle *i18n.Bundle, poweredBy string) board.BoardView {
	var order []string
	byCategory := make(map[string][]board.Product)
	for _, p := range v.Products {
		if _, ok := byCategory[p.Category]; !ok {
			order = append(order, p.Category)
		}
		byCategory[p.Category] = append(byCategory[p.Category], p)
	}

	usedAnchors := make(map[string]bool)
	categories := make([]board.CategoryView, 0, len(order))
	for _, name := range order {
		categories = append(categories, buildCategory(name, byCategory[name], usedAnchors))
	}

	lang := v.Language
	if lang == "" {
		lang = "en"
	}

	return board.BoardView{
		ClientName:  v.ClientName,
		Slug:        v.Slug,
		Intro:       v.Intro,
		GeneratedAt: now,
		Language:    lang,
		T:           func(key string) string { return bundle.T(lang, key) },
		PoweredBy:   poweredBy,
		Categories:  categories,
	}
}

func buildCategory(name string, products []board.Product, usedAnchors map[string]bool) board.CategoryView {
	specKeys := unionSpecKeys(products)
	productViews := make([]board.ProductView, 0, len(products))
	for _, p := range products {
		productViews = append(productViews, buildProduct(p, specKeys))
	}
	return board.CategoryView{
		Name:     name,
		Anchor:   uniqueAnchor(name, usedAnchors),
		SpecKeys: specKeys,
		Products: productViews,
	}
}

// unionSpecKeys returns the union of every product's spec keys, in order of
// first appearance across products. A single map's own key order is not
// guaranteed by Go, so keys newly introduced by the same product are sorted
// alphabetically before being appended. That keeps the result stable across
// runs without depending on cross-category ordering.
func unionSpecKeys(products []board.Product) []string {
	seen := make(map[string]bool)
	var keys []string
	for _, p := range products {
		newKeys := make([]string, 0, len(p.Specs))
		for k := range p.Specs {
			if !seen[k] {
				newKeys = append(newKeys, k)
			}
		}
		sort.Strings(newKeys)
		for _, k := range newKeys {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	return keys
}

func buildProduct(p board.Product, specKeys []string) board.ProductView {
	cells := make([]string, len(specKeys))
	for i, k := range specKeys {
		if val, ok := p.Specs[k]; ok {
			cells[i] = val
		} else {
			cells[i] = "—"
		}
	}

	highlights := p.Highlights
	// Auto-derive highlights from the first two spec entries when none
	// are explicitly set, so every card shows at least a quick summary.
	if len(highlights) == 0 && len(p.Specs) > 0 {
		type kv struct{ k, v string }
		var pairs []kv
		for k, v := range p.Specs {
			pairs = append(pairs, kv{k, v})
		}
		sort.Slice(pairs, func(i, j int) bool { return pairs[i].k < pairs[j].k })
		for i := 0; i < len(pairs) && len(highlights) < 2; i++ {
			highlights = append(highlights, pairs[i].k+": "+pairs[i].v)
		}
	}

	// Defensive copy of the specs map so the template can iterate over it.
	var specs map[string]string
	if p.Specs != nil {
		specs = make(map[string]string, len(p.Specs))
		for k, v := range p.Specs {
			specs[k] = v
		}
	}

	pv := board.ProductView{
		Title:          p.Title,
		Recommendation: p.Recommendation,
		Price:          p.Price,
		Badge:          p.Badge,
		ImageURL:       p.ImageURL,
		AffiliateLink:  p.AffiliateLink,
		SpecCells:      cells,
		Specs:          specs,
		Highlights:     highlights,
		Pros:           p.Pros,
		Cons:           p.Cons,
	}
	if p.Rating != nil {
		pv.Rating = *p.Rating
		pv.Stars = stars(*p.Rating)
	}
	return pv
}

// uniqueAnchor slugifies name and, if that anchor is already taken by an
// earlier category in the same board, appends "-2", "-3", ... until unique.
func uniqueAnchor(name string, used map[string]bool) string {
	base := slug.Slugify(name)
	anchor := base
	for i := 2; used[anchor]; i++ {
		anchor = fmt.Sprintf("%s-%d", base, i)
	}
	used[anchor] = true
	return anchor
}

// stars renders rating (clamped to 0.0-5.0) as exactly 5 StarStates, with a
// half star when the value rounded to the nearest 0.5 lands on a half step.
func stars(rating float64) []board.StarState {
	if rating < 0 {
		rating = 0
	}
	if rating > 5 {
		rating = 5
	}
	rounded := math.Round(rating*2) / 2
	full := int(rounded)
	half := rounded-float64(full) >= 0.5

	result := make([]board.StarState, 0, 5)
	for i := 0; i < full; i++ {
		result = append(result, board.StarFull)
	}
	if half {
		result = append(result, board.StarHalf)
	}
	for len(result) < 5 {
		result = append(result, board.StarEmpty)
	}
	return result
}
