// Package httpx contains small JSON HTTP helpers shared by adapters.
package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/bakhod1r/errorx"
)

var ErrBadJSON = errors.New("malformed JSON body")

// CodeFor maps an error to a registered Kyber error code; adapters pass their own mapping.
type CodeFor func(error) (code string, ok bool)

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

// Error writes an RFC 9457 problem. Known errors get their code and the error text as
// detail; unknown errors are logged and hidden behind INTERNAL.
func Error(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error, mapping CodeFor) {
	if errors.Is(err, ErrBadJSON) {
		Problem(w, r, errorx.New(CodeBadRequest, err.Error()).WithDetails(err.Error()))
		return
	}
	if code, ok := mapping(err); ok {
		Problem(w, r, errorx.New(code, err.Error()).WithDetails(err.Error()))
		return
	}
	log.ErrorContext(r.Context(), "internal error", "err", err, "path", r.URL.Path)
	Problem(w, r, errorx.New(CodeInternal, err.Error()))
}

// Problem renders an errorx error with a title localized from Accept-Language.
func Problem(w http.ResponseWriter, r *http.Request, err *errorx.AppError) {
	_ = errorx.WriteProblemRequest(w, r, err)
}
