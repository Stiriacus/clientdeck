package board

import (
	"errors"
	"testing"
)

func TestValidationError_WrapsErrInvalidPayload(t *testing.T) {
	err := &ValidationError{Field: "client_name", Reason: "must not be empty"}

	if !errors.Is(err, ErrInvalidPayload) {
		t.Errorf("expected ValidationError to satisfy errors.Is(err, ErrInvalidPayload)")
	}

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected errors.As to find *ValidationError")
	}
	if ve.Field != "client_name" || ve.Reason != "must not be empty" {
		t.Errorf("unexpected ValidationError fields: %+v", ve)
	}
}

func TestSentinelErrors_AreDistinct(t *testing.T) {
	sentinels := []error{ErrUnknownSlug, ErrInvalidPayload, ErrSlugExhausted, ErrUnauthorized}
	for i, a := range sentinels {
		for j, b := range sentinels {
			if i != j && errors.Is(a, b) {
				t.Errorf("sentinel %v should not match %v", a, b)
			}
		}
	}
}
