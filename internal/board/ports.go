package board

import (
	"context"
	"io"
)

// Store persists and retrieves CustomerViews.
type Store interface {
	// Upsert creates or updates the CustomerView identified by v.CustomerID.
	// For a new customer, newSlug is invoked (and retried on collision) to
	// obtain a slug; for an existing customer the stored slug is kept and
	// newSlug is not called. Returns ErrSlugExhausted if newSlug fails to
	// produce a unique slug within the retry budget.
	Upsert(ctx context.Context, v CustomerView, newSlug func() (string, error)) (slug string, created bool, err error)

	// BySlug looks up a CustomerView by its public slug. It returns
	// ErrUnknownSlug if no customer has that slug.
	BySlug(ctx context.Context, slug string) (CustomerView, error)

	Close() error
}

// Renderer renders a BoardView, or the not-found page, as HTML.
type Renderer interface {
	RenderBoard(w io.Writer, v BoardView) error
	RenderNotFound(w io.Writer, lang string) error
}
