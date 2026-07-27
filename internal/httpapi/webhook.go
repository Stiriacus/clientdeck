package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/Stiriacus/vitrine/internal/board"
	"github.com/Stiriacus/vitrine/internal/ingest"
)

// maxBodyBytes bounds a webhook request body; larger bodies get 413 before
// any JSON parsing is attempted.
const maxBodyBytes = 1 << 20 // 1 MiB

// webhookResponse is the frozen success body from docs/api.md.
// IgnoredFields is omitted from the JSON entirely when empty.
type webhookResponse struct {
	CustomerID    string   `json:"customer_id"`
	Slug          string   `json:"slug"`
	URL           string   `json:"url"`
	Created       bool     `json:"created"`
	IgnoredFields []string `json:"ignored_fields,omitempty"`
}

// knownTopLevelFields and knownProductFields list the wire keys board.CustomerView
// and board.Product actually bind, so ignoredFields can report anything else.
var knownTopLevelFields = map[string]bool{
	"customer_id": true, "client_name": true, "intro": true, "language": true, "theme": true, "products": true,
}

var knownProductFields = map[string]bool{
	"category": true, "title": true, "recommendation": true, "specs": true,
	"rating": true, "affiliate_link": true, "image_url": true, "price": true, "badge": true,
	"highlights": true, "pros": true, "cons": true,
}

// handleWebhook implements POST /api/v1/views (and its /webhook alias):
// decode, report unknown fields, ingest, respond with the assigned slug/URL.
// The webhook secret is checked by withAuth before this handler ever runs.
func handleWebhook(ingestSvc *ingest.Service, baseURL string, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		data, err := io.ReadAll(r.Body)
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "request body exceeds 1 MiB")
				return
			}
			writeError(w, http.StatusBadRequest, "invalid_payload", "failed to read request body")
			return
		}

		var v board.CustomerView
		if err := json.Unmarshal(data, &v); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_payload", "malformed JSON: "+err.Error())
			return
		}

		ignored, err := ignoredFields(data)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_payload", "malformed JSON: "+err.Error())
			return
		}

		result, err := ingestSvc.Ingest(r.Context(), v)
		if err != nil {
			status, code := mapIngestError(err)
			logger.Warn("ingest failed",
				"request_id", requestIDFrom(r.Context()),
				"customer_id", v.CustomerID,
				"error", err,
			)
			writeError(w, status, code, err.Error())
			return
		}

		logger.Info("ingest ok",
			"request_id", requestIDFrom(r.Context()),
			"customer_id", v.CustomerID,
			"created", result.Created,
		)

		resp := webhookResponse{
			CustomerID:    v.CustomerID,
			Slug:          result.Slug,
			URL:           strings.TrimRight(baseURL, "/") + "/c/" + result.Slug,
			Created:       result.Created,
			IgnoredFields: ignored,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// ignoredFields diffs the raw request body against the fields board.CustomerView
// and board.Product actually bind, returning a sorted "products[i].field"-style
// list of everything else. Accepted, ignored, and reported back per 1.1.
// Callers must have already confirmed data unmarshals cleanly into
// board.CustomerView, so the shallower re-decoding here cannot fail on a
// well-formed payload.
func ignoredFields(data []byte) ([]string, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, err
	}

	var ignored []string
	for k := range top {
		if !knownTopLevelFields[k] {
			ignored = append(ignored, k)
		}
	}

	if raw, ok := top["products"]; ok {
		var products []map[string]json.RawMessage
		if err := json.Unmarshal(raw, &products); err != nil {
			return nil, err
		}
		for i, p := range products {
			for k := range p {
				if !knownProductFields[k] {
					ignored = append(ignored, fmt.Sprintf("products[%d].%s", i, k))
				}
			}
		}
	}

	sort.Strings(ignored)
	return ignored, nil
}
