package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/Stiriacus/vitrine/internal/board"
)

// errorResponse is the frozen error body shape from docs/api.md:
// {"error":"<code>","message":"<human readable>"}.
type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{Error: code, Message: message})
}

// mapIngestError classifies an error returned by ingest.Service.Ingest into
// the (status, code) pairs frozen in docs/api.md. It does not handle
// transport-level errors (auth, body size, malformed JSON). Those are
// mapped where they occur, before the domain layer is ever called.
func mapIngestError(err error) (status int, code string) {
	var verr *board.ValidationError
	switch {
	case errors.As(err, &verr):
		if isURLField(verr.Field) {
			return http.StatusBadRequest, "invalid_url"
		}
		return http.StatusBadRequest, "invalid_payload"
	case errors.Is(err, board.ErrSlugExhausted):
		return http.StatusInternalServerError, "slug_exhausted"
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}

// isURLField reports whether field (as set on board.ValidationError) names
// one of the two URL-typed payload fields, which get the more specific
// invalid_url error code instead of invalid_payload.
func isURLField(field string) bool {
	return strings.HasSuffix(field, ".affiliate_link") || strings.HasSuffix(field, ".image_url")
}
