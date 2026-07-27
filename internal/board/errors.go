package board

import (
	"errors"
	"fmt"
)

// Sentinel errors returned by Store and Service implementations. Callers
// should compare against these with errors.Is.
var (
	ErrUnknownSlug    = errors.New("unknown slug")
	ErrInvalidPayload = errors.New("invalid payload")
	ErrSlugExhausted  = errors.New("slug exhausted")
	ErrUnauthorized   = errors.New("unauthorized")
)

// ValidationError reports which field of a CustomerView failed validation
// and why. It always wraps ErrInvalidPayload, so errors.Is(err,
// ErrInvalidPayload) succeeds while errors.As(err, &ValidationError{})
// still exposes the offending field.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Reason)
}

func (e *ValidationError) Unwrap() error {
	return ErrInvalidPayload
}
