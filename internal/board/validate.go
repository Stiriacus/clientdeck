package board

import (
	"fmt"
	"net/url"
	"regexp"
)

const (
	maxClientNameLen     = 120
	maxTitleLen          = 200
	maxRecommendationLen = 2000
	maxProducts          = 100
	maxSpecsPerProduct   = 20
	maxHighlights        = 5
	maxHighlightLen      = 160
	maxPros              = 10
	maxCons              = 10
	maxProConLen         = 300
)

// languagePattern accepts BCP 47 primary language tags: 2–3 lowercase letters
// optionally followed by a hyphen and a region subtag.
var languagePattern = regexp.MustCompile(`^[a-z]{2,3}(-[A-Z]{2})?$`)

// Validate checks v against the rules frozen in docs/api.md and returns the
// first violation found as a *ValidationError, or nil if v is well-formed.
func (v CustomerView) Validate() error {
	if v.CustomerID == "" {
		return &ValidationError{Field: "customer_id", Reason: "must not be empty"}
	}
	if v.ClientName == "" {
		return &ValidationError{Field: "client_name", Reason: "must not be empty"}
	}
	if len(v.ClientName) > maxClientNameLen {
		return &ValidationError{Field: "client_name", Reason: fmt.Sprintf("must be at most %d characters", maxClientNameLen)}
	}
	if v.Language != "" && !languagePattern.MatchString(v.Language) {
		return &ValidationError{Field: "language", Reason: "must be a valid language code (e.g. \"en\", \"fr\")"}
	}
	if len(v.Products) == 0 {
		return &ValidationError{Field: "products", Reason: "must contain at least one product"}
	}
	if len(v.Products) > maxProducts {
		return &ValidationError{Field: "products", Reason: fmt.Sprintf("must contain at most %d products", maxProducts)}
	}

	for i, p := range v.Products {
		if err := p.validate(i); err != nil {
			return err
		}
	}
	return nil
}

func (p Product) validate(index int) error {
	prefix := fmt.Sprintf("products[%d]", index)

	if p.Category == "" {
		return &ValidationError{Field: prefix + ".category", Reason: "must not be empty"}
	}
	if p.Title == "" {
		return &ValidationError{Field: prefix + ".title", Reason: "must not be empty"}
	}
	if len(p.Title) > maxTitleLen {
		return &ValidationError{Field: prefix + ".title", Reason: fmt.Sprintf("must be at most %d characters", maxTitleLen)}
	}
	if len(p.Recommendation) > maxRecommendationLen {
		return &ValidationError{Field: prefix + ".recommendation", Reason: fmt.Sprintf("must be at most %d characters", maxRecommendationLen)}
	}
	if len(p.Specs) > maxSpecsPerProduct {
		return &ValidationError{Field: prefix + ".specs", Reason: fmt.Sprintf("must contain at most %d entries", maxSpecsPerProduct)}
	}
	if p.Rating != nil && (*p.Rating < 0 || *p.Rating > 5) {
		return &ValidationError{Field: prefix + ".rating", Reason: "must be between 0.0 and 5.0"}
	}
	if len(p.Highlights) > maxHighlights {
		return &ValidationError{Field: prefix + ".highlights", Reason: fmt.Sprintf("must contain at most %d entries", maxHighlights)}
	}
	for i, h := range p.Highlights {
		if len(h) > maxHighlightLen {
			return &ValidationError{Field: fmt.Sprintf("%s.highlights[%d]", prefix, i), Reason: fmt.Sprintf("must be at most %d characters", maxHighlightLen)}
		}
	}
	if len(p.Pros) > maxPros {
		return &ValidationError{Field: prefix + ".pros", Reason: fmt.Sprintf("must contain at most %d entries", maxPros)}
	}
	for i, s := range p.Pros {
		if len(s) > maxProConLen {
			return &ValidationError{Field: fmt.Sprintf("%s.pros[%d]", prefix, i), Reason: fmt.Sprintf("must be at most %d characters", maxProConLen)}
		}
	}
	if len(p.Cons) > maxCons {
		return &ValidationError{Field: prefix + ".cons", Reason: fmt.Sprintf("must contain at most %d entries", maxCons)}
	}
	for i, s := range p.Cons {
		if len(s) > maxProConLen {
			return &ValidationError{Field: fmt.Sprintf("%s.cons[%d]", prefix, i), Reason: fmt.Sprintf("must be at most %d characters", maxProConLen)}
		}
	}
	if err := validateURL(prefix+".affiliate_link", p.AffiliateLink); err != nil {
		return err
	}
	if err := validateURL(prefix+".image_url", p.ImageURL); err != nil {
		return err
	}
	return nil
}

// validateURL rejects anything but http/https so schemes like javascript:,
// data: and file: never reach a template or an href.
func validateURL(field, raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return &ValidationError{Field: field, Reason: "must use http or https scheme"}
	}
	return nil
}
