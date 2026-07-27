// Package ingest implements the write path shared by the webhook and its
// alias: validate an incoming CustomerView, normalize it, and hand it to a
// board.Store for upsert.
package ingest

import (
	"context"
	"strings"
	"time"

	"github.com/Stiriacus/vitrine/internal/board"
	"github.com/Stiriacus/vitrine/internal/slug"
)

// Result reports the outcome of an Ingest call.
type Result struct {
	Slug    string
	Created bool
}

// Service is the domain-level entry point for ingesting a CustomerView. It
// does not check the webhook secret. That is a transport concern handled by
// the HTTP middleware, not the domain.
type Service struct {
	store board.Store
	slugs *slug.Generator
	clock func() time.Time
}

// New returns a Service backed by store, minting new slugs via slugs.
func New(store board.Store, slugs *slug.Generator) *Service {
	return &Service{store: store, slugs: slugs, clock: time.Now}
}

// Ingest validates and normalizes v, then upserts it into the store. On a
// new customer_id, slugs.For(v.ClientName) supplies the slug (retried by the
// store on collision); on an existing customer_id the stored slug is kept.
func (s *Service) Ingest(ctx context.Context, v board.CustomerView) (Result, error) {
	if err := v.Validate(); err != nil {
		return Result{}, err
	}
	v = normalize(v)

	newSlug := func() (string, error) { return s.slugs.For(v.ClientName) }
	slugStr, created, err := s.store.Upsert(ctx, v, newSlug)
	if err != nil {
		return Result{}, err
	}
	return Result{Slug: slugStr, Created: created}, nil
}

// normalize trims all string fields and drops products left empty by
// whitespace-only input that passed Validate's non-empty check.
func normalize(v board.CustomerView) board.CustomerView {
	v.ClientName = strings.TrimSpace(v.ClientName)
	v.Intro = strings.TrimSpace(v.Intro)

	products := make([]board.Product, 0, len(v.Products))
	for _, p := range v.Products {
		p.Category = strings.TrimSpace(p.Category)
		p.Title = strings.TrimSpace(p.Title)
		p.Recommendation = strings.TrimSpace(p.Recommendation)
		p.Price = strings.TrimSpace(p.Price)
		p.Badge = strings.TrimSpace(p.Badge)
		if p.Category == "" || p.Title == "" {
			continue
		}
		products = append(products, p)
	}
	v.Products = products
	return v
}
