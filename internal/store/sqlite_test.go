package store

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Stiriacus/vitrine/internal/board"
)

func openTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "vitrine.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func testCustomerView(customerID, clientName string) board.CustomerView {
	rating := 4.5
	return board.CustomerView{
		CustomerID: customerID,
		ClientName: clientName,
		Intro:      "Optional intro.",
		Language:   "en",
		Products: []board.Product{
			{
				Category:       "Printers",
				Title:          "Brother HL-L2375DW",
				Recommendation: "Good balance of price and toner cost.",
				Specs:          map[string]string{"Print engine": "Laser B/W", "Duplex": "yes"},
				Rating:         &rating,
				AffiliateLink:  "https://example.com/p/123?tag=xyz",
				ImageURL:       "https://example.com/img/123.jpg",
				Price:          "$179.00",
				Badge:          "Best Value",
			},
		},
	}
}

// fixedSlug returns a newSlug callback that always yields s.
func fixedSlug(s string) func() (string, error) {
	return func() (string, error) { return s, nil }
}

// sequence returns a newSlug callback that yields each of slugs in order,
// repeating the last one once exhausted.
func sequence(slugs ...string) func() (string, error) {
	i := 0
	return func() (string, error) {
		s := slugs[i]
		if i < len(slugs)-1 {
			i++
		}
		return s, nil
	}
}

func TestUpsert_InsertThenBySlug(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	v := testCustomerView("acme-corp", "ACME Corp")

	slug, created, err := s.Upsert(ctx, v, fixedSlug("acme-corp-a8f9b2"))
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if !created {
		t.Fatalf("expected created=true on first insert")
	}
	if slug != "acme-corp-a8f9b2" {
		t.Fatalf("slug = %q, want %q", slug, "acme-corp-a8f9b2")
	}

	got, err := s.BySlug(ctx, slug)
	if err != nil {
		t.Fatalf("BySlug: %v", err)
	}
	got.Slug = "" // Slug isn't set on the input fixture; ignore for comparison.
	v.Slug = ""
	if !reflect.DeepEqual(got, v) {
		t.Fatalf("BySlug returned %+v, want %+v", got, v)
	}
}

func TestUpsert_SameCustomerID_OneRowSameSlugNewProducts(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	v := testCustomerView("acme-corp", "ACME Corp")

	slug1, created1, err := s.Upsert(ctx, v, fixedSlug("acme-corp-a8f9b2"))
	if err != nil {
		t.Fatalf("first Upsert: %v", err)
	}
	if !created1 {
		t.Fatalf("expected created=true on first insert")
	}

	v2 := v
	v2.Products = []board.Product{{Category: "Scanner", Title: "Epson V39"}}
	slug2, created2, err := s.Upsert(ctx, v2, fixedSlug("should-not-be-used"))
	if err != nil {
		t.Fatalf("second Upsert: %v", err)
	}
	if created2 {
		t.Fatalf("expected created=false on second upsert")
	}
	if slug2 != slug1 {
		t.Fatalf("slug changed on update: got %q, want %q", slug2, slug1)
	}

	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM customer_views`).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("row count = %d, want 1", count)
	}

	got, err := s.BySlug(ctx, slug1)
	if err != nil {
		t.Fatalf("BySlug: %v", err)
	}
	if !reflect.DeepEqual(got.Products, v2.Products) {
		t.Fatalf("products = %+v, want %+v", got.Products, v2.Products)
	}
}

func TestUpsert_UpdateClientName_SlugUnchanged(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	v := testCustomerView("acme-corp", "ACME Corp")

	slug1, _, err := s.Upsert(ctx, v, fixedSlug("acme-corp-a8f9b2"))
	if err != nil {
		t.Fatalf("first Upsert: %v", err)
	}

	v.ClientName = "ACME International Corp"
	slug2, created, err := s.Upsert(ctx, v, fixedSlug("should-not-be-used"))
	if err != nil {
		t.Fatalf("second Upsert: %v", err)
	}
	if created {
		t.Fatalf("expected created=false on update")
	}
	if slug2 != slug1 {
		t.Fatalf("slug changed after client_name update: got %q, want %q", slug2, slug1)
	}

	got, err := s.BySlug(ctx, slug1)
	if err != nil {
		t.Fatalf("BySlug: %v", err)
	}
	if got.ClientName != "ACME International Corp" {
		t.Fatalf("client_name = %q, want updated value", got.ClientName)
	}
}

func TestUpsert_SlugExhausted(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	taken := testCustomerView("existing-customer", "Existing")
	if _, _, err := s.Upsert(ctx, taken, fixedSlug("taken-slug")); err != nil {
		t.Fatalf("seed Upsert: %v", err)
	}

	newCustomer := testCustomerView("new-customer", "New Customer")
	_, _, err := s.Upsert(ctx, newCustomer, fixedSlug("taken-slug"))
	if !errors.Is(err, board.ErrSlugExhausted) {
		t.Fatalf("err = %v, want ErrSlugExhausted", err)
	}
}

func TestUpsert_CollidesTwiceThenFree(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	taken := testCustomerView("existing-customer", "Existing")
	if _, _, err := s.Upsert(ctx, taken, fixedSlug("taken-slug")); err != nil {
		t.Fatalf("seed Upsert: %v", err)
	}

	newCustomer := testCustomerView("new-customer", "New Customer")
	slug, created, err := s.Upsert(ctx, newCustomer,
		sequence("taken-slug", "taken-slug", "free-slug"))
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if !created {
		t.Fatalf("expected created=true")
	}
	if slug != "free-slug" {
		t.Fatalf("slug = %q, want %q", slug, "free-slug")
	}
}

func TestBySlug_Unknown(t *testing.T) {
	s := openTestStore(t)
	_, err := s.BySlug(context.Background(), "does-not-exist")
	if !errors.Is(err, board.ErrUnknownSlug) {
		t.Fatalf("err = %v, want ErrUnknownSlug", err)
	}
}

func TestStore_PersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vitrine.db")
	ctx := context.Background()

	s1, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	v := testCustomerView("acme-corp", "ACME Corp")
	slug, _, err := s1.Upsert(ctx, v, fixedSlug("acme-corp-a8f9b2"))
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()

	got, err := s2.BySlug(ctx, slug)
	if err != nil {
		t.Fatalf("BySlug after reopen: %v", err)
	}
	if got.ClientName != v.ClientName {
		t.Fatalf("client_name after reopen = %q, want %q", got.ClientName, v.ClientName)
	}
}
