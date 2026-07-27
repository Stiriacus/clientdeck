package board

import (
	"errors"
	"strings"
	"testing"
)

func validCustomerView() CustomerView {
	return CustomerView{
		CustomerID: "acme-corp",
		ClientName: "ACME Corp",
		Intro:      "Optional intro.",
		Products: []Product{
			{
				Category:       "Printers",
				Title:          "Brother HL-L2375DW",
				Recommendation: "Good balance of price and toner cost.",
				Specs:          map[string]string{"Print engine": "Laser B/W"},
				Rating:         ratingPtr(4.5),
				AffiliateLink:  "https://example.com/p/123?tag=xyz",
				ImageURL:       "https://example.com/img/123.jpg",
				Price:          "$179.00",
				Badge:          "Best Value",
			},
		},
	}
}

func ratingPtr(f float64) *float64 { return &f }

func TestValidate_Valid(t *testing.T) {
	if err := validCustomerView().Validate(); err != nil {
		t.Fatalf("expected valid payload to pass, got: %v", err)
	}
}

func TestValidate_Rules(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*CustomerView)
		wantField string
	}{
		{
			name:      "missing customer_id",
			mutate:    func(v *CustomerView) { v.CustomerID = "" },
			wantField: "customer_id",
		},
		{
			name:      "missing client_name",
			mutate:    func(v *CustomerView) { v.ClientName = "" },
			wantField: "client_name",
		},
		{
			name:      "client_name too long",
			mutate:    func(v *CustomerView) { v.ClientName = strings.Repeat("a", maxClientNameLen+1) },
			wantField: "client_name",
		},
		{
			name:      "no products",
			mutate:    func(v *CustomerView) { v.Products = nil },
			wantField: "products",
		},
		{
			name: "too many products",
			mutate: func(v *CustomerView) {
				p := v.Products[0]
				products := make([]Product, maxProducts+1)
				for i := range products {
					products[i] = p
				}
				v.Products = products
			},
			wantField: "products",
		},
		{
			name:      "missing category",
			mutate:    func(v *CustomerView) { v.Products[0].Category = "" },
			wantField: "products[0].category",
		},
		{
			name:      "missing title",
			mutate:    func(v *CustomerView) { v.Products[0].Title = "" },
			wantField: "products[0].title",
		},
		{
			name:      "title too long",
			mutate:    func(v *CustomerView) { v.Products[0].Title = strings.Repeat("a", maxTitleLen+1) },
			wantField: "products[0].title",
		},
		{
			name: "recommendation too long",
			mutate: func(v *CustomerView) {
				v.Products[0].Recommendation = strings.Repeat("a", maxRecommendationLen+1)
			},
			wantField: "products[0].recommendation",
		},
		{
			name: "too many specs",
			mutate: func(v *CustomerView) {
				specs := make(map[string]string, maxSpecsPerProduct+1)
				for i := 0; i < maxSpecsPerProduct+1; i++ {
					specs[strings.Repeat("k", i+1)] = "v"
				}
				v.Products[0].Specs = specs
			},
			wantField: "products[0].specs",
		},
		{
			name:      "rating below range",
			mutate:    func(v *CustomerView) { v.Products[0].Rating = ratingPtr(-0.1) },
			wantField: "products[0].rating",
		},
		{
			name:      "rating above range",
			mutate:    func(v *CustomerView) { v.Products[0].Rating = ratingPtr(5.1) },
			wantField: "products[0].rating",
		},
		{
			name:      "affiliate_link javascript scheme",
			mutate:    func(v *CustomerView) { v.Products[0].AffiliateLink = "javascript:alert(1)" },
			wantField: "products[0].affiliate_link",
		},
		{
			name:      "affiliate_link data scheme",
			mutate:    func(v *CustomerView) { v.Products[0].AffiliateLink = "data:text/html,<script>alert(1)</script>" },
			wantField: "products[0].affiliate_link",
		},
		{
			name:      "affiliate_link file scheme",
			mutate:    func(v *CustomerView) { v.Products[0].AffiliateLink = "file:///etc/passwd" },
			wantField: "products[0].affiliate_link",
		},
		{
			name:      "image_url javascript scheme",
			mutate:    func(v *CustomerView) { v.Products[0].ImageURL = "javascript:alert(1)" },
			wantField: "products[0].image_url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := validCustomerView()
			tt.mutate(&v)

			err := v.Validate()
			if err == nil {
				t.Fatalf("expected validation error, got nil")
			}
			if !errors.Is(err, ErrInvalidPayload) {
				t.Errorf("expected error to wrap ErrInvalidPayload, got: %v", err)
			}
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("expected *ValidationError, got %T: %v", err, err)
			}
			if ve.Field != tt.wantField {
				t.Errorf("Field = %q, want %q", ve.Field, tt.wantField)
			}
		})
	}
}

func TestValidate_RatingBoundariesAccepted(t *testing.T) {
	for _, r := range []float64{0.0, 5.0} {
		v := validCustomerView()
		v.Products[0].Rating = ratingPtr(r)
		if err := v.Validate(); err != nil {
			t.Errorf("rating %v should be valid, got: %v", r, err)
		}
	}
}

func TestValidate_RatingNilIsValid(t *testing.T) {
	v := validCustomerView()
	v.Products[0].Rating = nil
	if err := v.Validate(); err != nil {
		t.Errorf("nil rating should be valid, got: %v", err)
	}
}
