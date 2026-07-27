package ingest

import (
	"context"
	"errors"
	"testing"

	"github.com/Stiriacus/vitrine/internal/board"
	"github.com/Stiriacus/vitrine/internal/slug"
)

// fakeStore is an in-memory board.Store for testing Service without SQLite.
type fakeStore struct {
	byCustomer map[string]board.CustomerView
	bySlug     map[string]string // slug -> customer_id
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		byCustomer: make(map[string]board.CustomerView),
		bySlug:     make(map[string]string),
	}
}

func (f *fakeStore) Upsert(ctx context.Context, v board.CustomerView, newSlug func() (string, error)) (string, bool, error) {
	if existing, ok := f.byCustomer[v.CustomerID]; ok {
		v.Slug = existing.Slug
		f.byCustomer[v.CustomerID] = v
		return existing.Slug, false, nil
	}

	const maxAttempts = 5
	for attempt := 0; attempt < maxAttempts; attempt++ {
		s, err := newSlug()
		if err != nil {
			return "", false, err
		}
		if _, taken := f.bySlug[s]; taken {
			continue
		}
		v.Slug = s
		f.byCustomer[v.CustomerID] = v
		f.bySlug[s] = v.CustomerID
		return s, true, nil
	}
	return "", false, board.ErrSlugExhausted
}

func (f *fakeStore) BySlug(ctx context.Context, s string) (board.CustomerView, error) {
	id, ok := f.bySlug[s]
	if !ok {
		return board.CustomerView{}, board.ErrUnknownSlug
	}
	return f.byCustomer[id], nil
}

func (f *fakeStore) Close() error { return nil }

// repeatReader is an io.Reader that cycles through b forever, so a
// slug.Generator built on top of it always yields the same suffix for a
// given client name. Used to script deterministic or colliding slugs.
type repeatReader struct{ b []byte }

func (r repeatReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = r.b[i%len(r.b)]
	}
	return len(p), nil
}

func validCustomerView(customerID, clientName string) board.CustomerView {
	return board.CustomerView{
		CustomerID: customerID,
		ClientName: clientName,
		Products: []board.Product{
			{Category: "Printers", Title: "Brother HL-L2375DW"},
		},
	}
}

func TestIngest_HappyPath(t *testing.T) {
	store := newFakeStore()
	svc := New(store, slug.New(repeatReader{b: []byte("aaaaaa")}))

	result, err := svc.Ingest(context.Background(), validCustomerView("acme-corp", "ACME Corp"))
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if !result.Created {
		t.Fatalf("expected Created=true on first ingest")
	}
	if result.Slug == "" {
		t.Fatalf("expected non-empty slug")
	}
}

func TestIngest_InvalidPayload(t *testing.T) {
	store := newFakeStore()
	svc := New(store, slug.New(repeatReader{b: []byte("aaaaaa")}))

	_, err := svc.Ingest(context.Background(), board.CustomerView{})
	if !errors.Is(err, board.ErrInvalidPayload) {
		t.Fatalf("err = %v, want ErrInvalidPayload", err)
	}
}

func TestIngest_SlugExhausted(t *testing.T) {
	store := newFakeStore()
	generator := slug.New(repeatReader{b: []byte("aaaaaa")})

	// Seed a customer that takes the slug the generator will always
	// produce for "New Customer", so every retry collides.
	seedSlug, err := generator.For("New Customer")
	if err != nil {
		t.Fatalf("seed slug: %v", err)
	}
	if _, _, err := store.Upsert(context.Background(), validCustomerView("existing-customer", "Existing"),
		func() (string, error) { return seedSlug, nil }); err != nil {
		t.Fatalf("seed Upsert: %v", err)
	}

	svc := New(store, generator)
	_, err = svc.Ingest(context.Background(), validCustomerView("new-customer", "New Customer"))
	if !errors.Is(err, board.ErrSlugExhausted) {
		t.Fatalf("err = %v, want ErrSlugExhausted", err)
	}
}

func TestIngest_ReIngest_CreatedFalse(t *testing.T) {
	store := newFakeStore()
	svc := New(store, slug.New(repeatReader{b: []byte("aaaaaa")}))
	ctx := context.Background()

	first, err := svc.Ingest(ctx, validCustomerView("acme-corp", "ACME Corp"))
	if err != nil {
		t.Fatalf("first Ingest: %v", err)
	}

	updated := validCustomerView("acme-corp", "ACME Corp")
	updated.Products = []board.Product{{Category: "Scanner", Title: "Epson V39"}}
	second, err := svc.Ingest(ctx, updated)
	if err != nil {
		t.Fatalf("second Ingest: %v", err)
	}
	if second.Created {
		t.Fatalf("expected Created=false on re-ingest")
	}
	if second.Slug != first.Slug {
		t.Fatalf("slug changed on re-ingest: got %q, want %q", second.Slug, first.Slug)
	}
}

func TestIngest_NormalizesWhitespace(t *testing.T) {
	store := newFakeStore()
	svc := New(store, slug.New(repeatReader{b: []byte("aaaaaa")}))

	v := board.CustomerView{
		CustomerID: "acme-corp",
		ClientName: "  ACME Corp  ",
		Intro:      "  hello  ",
		Products: []board.Product{
			{Category: "  Printers  ", Title: "  Brother HL-L2375DW  "},
			{Category: "   ", Title: "   "}, // whitespace-only, dropped by normalize
		},
	}

	if _, err := svc.Ingest(context.Background(), v); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	stored := store.byCustomer["acme-corp"]
	if stored.ClientName != "ACME Corp" {
		t.Fatalf("client_name = %q, want trimmed", stored.ClientName)
	}
	if stored.Intro != "hello" {
		t.Fatalf("intro = %q, want trimmed", stored.Intro)
	}
	if len(stored.Products) != 1 {
		t.Fatalf("products = %d, want 1 (whitespace-only product dropped)", len(stored.Products))
	}
	if stored.Products[0].Category != "Printers" || stored.Products[0].Title != "Brother HL-L2375DW" {
		t.Fatalf("product not trimmed: %+v", stored.Products[0])
	}
}
