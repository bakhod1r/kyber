// Package httpx contains small JSON HTTP helpers shared by adapters.
package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

var ErrBadJSON = errors.New("malformed JSON body")

// StatusFor maps an error to an HTTP status; adapters pass their own mapping.
type StatusFor func(error) (int, bool)

func Decode(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	if err := dec.Decode(v); err != nil {
		return ErrBadJSON
	}
	return nil
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Error writes {"error": msg}, mapping known errors and hiding unknown ones as 500.
func Error(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error, mapping StatusFor) {
	if errors.Is(err, ErrBadJSON) {
		JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if status, ok := mapping(err); ok {
		JSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	log.ErrorContext(r.Context(), "internal error", "err", err, "path", r.URL.Path)
	JSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
}
