// Package board holds vitrine's core domain types: the CustomerView
// payload ingested via the API, and the BoardView contract that themes
// render against. It also declares the Store and Renderer ports.
package board

import "time"

// CustomerView is the domain representation of a customer's product board,
// as received via POST /api/v1/views and persisted by a Store. JSON tags
// mirror the frozen wire contract in docs/api.md.
type CustomerView struct {
	CustomerID string    `json:"customer_id"`
	ClientName string    `json:"client_name"`
	Intro      string    `json:"intro,omitempty"`
	Language   string    `json:"language,omitempty"`
	Products   []Product `json:"products"`

	// Slug is assigned by the Store on first insert and kept stable across
	// updates. It is never populated from an incoming payload.
	Slug string `json:"-"`
}

// Product is a single recommended item within a category.
type Product struct {
	Category       string            `json:"category"`
	Title          string            `json:"title"`
	Recommendation string            `json:"recommendation,omitempty"`
	Specs          map[string]string `json:"specs,omitempty"`
	// Rating is nil when omitted from the payload, distinguishing "no
	// rating" from an explicit 0.0.
	Rating        *float64 `json:"rating,omitempty"`
	AffiliateLink string   `json:"affiliate_link,omitempty"`
	ImageURL      string   `json:"image_url,omitempty"`
	Price         string   `json:"price,omitempty"`
	Badge         string   `json:"badge,omitempty"`
	// Highlights are 1–5 short key facts shown on the card surface so the
	// customer can scan quickly. When empty, the first two spec entries are
	// used as a fallback.
	Highlights []string `json:"highlights,omitempty"`
	// Pros and Cons are optional AI-generated advantage/disadvantage lists
	// shown inside the product detail accordion.
	Pros []string `json:"pros,omitempty"`
	Cons []string `json:"cons,omitempty"`
}

// BoardView is the fully prepared view model handed to a Renderer. It is
// the frozen contract external themes render against (docs/theming.md).
type BoardView struct {
	ClientName  string
	Slug        string
	Intro       string
	GeneratedAt time.Time
	Language    string
	T           func(string) string
	PoweredBy   string
	Categories  []CategoryView
}

// CategoryView groups the products belonging to one tab of the board.
type CategoryView struct {
	Name     string // e.g. "Printers"
	Anchor   string // tab id / URL fragment, e.g. "printers"
	SpecKeys []string
	Products []ProductView
}

// ProductView is a Product prepared for rendering: rating is precomputed
// into Stars, and SpecCells is aligned to CategoryView.SpecKeys so templates
// never need map lookups with fallbacks.
type ProductView struct {
	Title, Recommendation   string
	Price, Badge            string
	ImageURL, AffiliateLink string
	Rating                  float64
	Stars                   []StarState       // always exactly 5 entries
	SpecCells               []string          // same length/order as CategoryView.SpecKeys
	Specs                   map[string]string // raw spec map for accordion <dl> rendering
	Highlights              []string          // key facts shown on card surface
	Pros                    []string          // advantages shown in accordion
	Cons                    []string          // disadvantages shown in accordion
}

// StarState is the rendered state of one star in a rating widget.
type StarState string

const (
	StarFull  StarState = "full"
	StarHalf  StarState = "half"
	StarEmpty StarState = "empty"
)
