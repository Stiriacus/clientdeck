package render

import (
	"reflect"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Stiriacus/vitrine/internal/board"
	"github.com/Stiriacus/vitrine/internal/i18n"
)

func testBundle(t *testing.T) *i18n.Bundle {
	t.Helper()
	fsys := fstest.MapFS{
		"en.json": {Data: []byte(`{"key": "value"}`)},
	}
	b, err := i18n.Load(fsys)
	if err != nil {
		t.Fatalf("i18n.Load: %v", err)
	}
	return b
}

func ratingPtr(f float64) *float64 { return &f }

func TestBuildBoardView_CategoryOrder(t *testing.T) {
	v := board.CustomerView{
		ClientName: "ACME Corp",
		Products: []board.Product{
			{Category: "Scanner", Title: "Epson V39"},
			{Category: "Printers", Title: "Brother HL-L2375DW"},
			{Category: "Scanner", Title: "Canon LiDE 300"},
		},
	}

	bundle := testBundle(t)
	got := BuildBoardView(v, time.Now(), bundle, "TestCompany")
	if len(got.Categories) != 2 {
		t.Fatalf("categories = %d, want 2", len(got.Categories))
	}
	if got.Categories[0].Name != "Scanner" || got.Categories[1].Name != "Printers" {
		t.Fatalf("category order = [%s, %s], want [Scanner, Printers] (first-appearance order)",
			got.Categories[0].Name, got.Categories[1].Name)
	}
	if len(got.Categories[0].Products) != 2 {
		t.Fatalf("Scanner products = %d, want 2", len(got.Categories[0].Products))
	}
}

func TestBuildBoardView_SpecKeyUnionAndCells(t *testing.T) {
	v := board.CustomerView{
		ClientName: "ACME Corp",
		Products: []board.Product{
			{
				Category: "Printers",
				Title:    "Brother HL-L2375DW",
				Specs:    map[string]string{"Duplex": "yes", "Pages/min": "34"},
			},
			{
				Category: "Printers",
				Title:    "HP LaserJet M110",
				Specs:    map[string]string{"Pages/min": "20", "Color printing": "no"},
			},
		},
	}

	bundle := testBundle(t)
	got := BuildBoardView(v, time.Now(), bundle, "TestCompany")
	cat := got.Categories[0]

	wantKeys := []string{"Duplex", "Pages/min", "Color printing"}
	if !reflect.DeepEqual(cat.SpecKeys, wantKeys) {
		t.Fatalf("SpecKeys = %v, want %v", cat.SpecKeys, wantKeys)
	}

	// Product 2 has no "Duplex" entry -> placeholder.
	p2 := cat.Products[1]
	wantCells := []string{"—", "20", "no"}
	if !reflect.DeepEqual(p2.SpecCells, wantCells) {
		t.Fatalf("product 2 SpecCells = %v, want %v", p2.SpecCells, wantCells)
	}
}

func TestBuildBoardView_CategoryWithoutSpecs(t *testing.T) {
	v := board.CustomerView{
		ClientName: "ACME Corp",
		Products: []board.Product{
			{Category: "Accessories", Title: "USB cable"},
		},
	}

	bundle := testBundle(t)
	got := BuildBoardView(v, time.Now(), bundle, "TestCompany")
	if len(got.Categories[0].SpecKeys) != 0 {
		t.Fatalf("SpecKeys = %v, want empty", got.Categories[0].SpecKeys)
	}
	if len(got.Categories[0].Products[0].SpecCells) != 0 {
		t.Fatalf("SpecCells = %v, want empty", got.Categories[0].Products[0].SpecCells)
	}
}

func TestBuildBoardView_Rating(t *testing.T) {
	cases := []struct {
		name   string
		rating *float64
		want   []board.StarState
	}{
		{"3.5 rounds to three full + half", ratingPtr(3.5),
			[]board.StarState{board.StarFull, board.StarFull, board.StarFull, board.StarHalf, board.StarEmpty}},
		{"0.0 explicit is all empty", ratingPtr(0),
			[]board.StarState{board.StarEmpty, board.StarEmpty, board.StarEmpty, board.StarEmpty, board.StarEmpty}},
		{"5.0 is all full", ratingPtr(5),
			[]board.StarState{board.StarFull, board.StarFull, board.StarFull, board.StarFull, board.StarFull}},
	}

	bundle := testBundle(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := board.CustomerView{
				ClientName: "ACME Corp",
				Products: []board.Product{
					{Category: "Printers", Title: "X", Rating: tc.rating},
				},
			}
			got := BuildBoardView(v, time.Now(), bundle, "TestCompany")
			pv := got.Categories[0].Products[0]
			if !reflect.DeepEqual(pv.Stars, tc.want) {
				t.Fatalf("Stars = %v, want %v", pv.Stars, tc.want)
			}
			if pv.Rating != *tc.rating {
				t.Fatalf("Rating = %v, want %v", pv.Rating, *tc.rating)
			}
		})
	}
}

func TestBuildBoardView_MissingRatingHasNoStars(t *testing.T) {
	v := board.CustomerView{
		ClientName: "ACME Corp",
		Products: []board.Product{
			{Category: "Printers", Title: "X"},
		},
	}
	bundle := testBundle(t)
	got := BuildBoardView(v, time.Now(), bundle, "TestCompany")
	pv := got.Categories[0].Products[0]
	if len(pv.Stars) != 0 {
		t.Fatalf("Stars = %v, want empty for missing rating (no star block)", pv.Stars)
	}
}

func TestBuildBoardView_DuplicateAnchors(t *testing.T) {
	v := board.CustomerView{
		ClientName: "ACME Corp",
		Products: []board.Product{
			{Category: "Foo!", Title: "A"},
			{Category: "Foo?", Title: "B"},
			{Category: "Foo#", Title: "C"},
		},
	}

	bundle := testBundle(t)
	got := BuildBoardView(v, time.Now(), bundle, "TestCompany")
	anchors := make(map[string]bool)
	for _, cat := range got.Categories {
		if anchors[cat.Anchor] {
			t.Fatalf("duplicate anchor %q across categories", cat.Anchor)
		}
		anchors[cat.Anchor] = true
	}
	want := []string{"foo", "foo-2", "foo-3"}
	for i, cat := range got.Categories {
		if cat.Anchor != want[i] {
			t.Fatalf("anchors = %v, want %v", collectAnchors(got.Categories), want)
		}
	}
}

func collectAnchors(cats []board.CategoryView) []string {
	out := make([]string, len(cats))
	for i, c := range cats {
		out[i] = c.Anchor
	}
	return out
}

func TestBuildBoardView_TopLevelFields(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	v := board.CustomerView{
		CustomerID: "acme-corp",
		ClientName: "ACME Corp",
		Intro:      "Welcome",
		Slug:       "acme-corp-a8f9b2",
		Products: []board.Product{
			{Category: "Printers", Title: "X"},
		},
	}

	bundle := testBundle(t)
	got := BuildBoardView(v, now, bundle, "TestCompany")
	if got.ClientName != v.ClientName || got.Slug != v.Slug || got.Intro != v.Intro || !got.GeneratedAt.Equal(now) {
		t.Fatalf("top-level fields not carried through: %+v", got)
	}
	if got.Language != "en" {
		t.Fatalf("Language = %q, want \"en\" (default)", got.Language)
	}
	if got.T == nil {
		t.Fatalf("T function is nil")
	}
}
