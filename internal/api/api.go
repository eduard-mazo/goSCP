// Package api implements the goSCP HTTP/JSON API.
package api

import (
	"encoding/json"
	"io"
	"net/http"

	"goscp/internal/storage"
)

// API holds the dependencies shared by every handler.
type API struct {
	Store          *storage.Store
	MaxUploadBytes int64
}

// New constructs an API.
func New(store *storage.Store, maxUpload int64) *API {
	return &API{Store: store, MaxUploadBytes: maxUpload}
}

// errorResponse is the canonical error envelope.
type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

// jsonDecode strictly decodes a single JSON value from r.
func jsonDecode(r io.Reader, v any) error {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
