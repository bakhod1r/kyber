package httpapi

import (
	"net/http"

	"github.com/bakhod1r/kyber/internal/issue/application"
	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

// fieldsOut is the issue's custom field values by field id (clients show those of current fields).
func fieldsOut(is *domain.Issue) map[string]string {
	out := map[string]string{}
	for id, v := range is.Fields() {
		out[string(id)] = v
	}
	return out
}

type fieldDTO struct {
	ID         string   `json:"id"`
	ProjectKey string   `json:"project_key"`
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Options    []string `json:"options"`
	Version    int      `json:"version"`
}

func fieldOut(f *domain.CustomField) fieldDTO {
	opts := f.Options()
	if opts == nil {
		opts = []string{}
	}
	return fieldDTO{ID: string(f.ID()), ProjectKey: f.Project(), Name: f.Name(), Type: string(f.Type()), Options: opts, Version: f.Version()}
}

type fieldIn struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Options []string `json:"options"`
	Version int      `json:"version"`
}

func (in fieldIn) spec() application.FieldSpec {
	return application.FieldSpec{Name: in.Name, Type: in.Type, Options: in.Options}
}

func (h *Handler) customFields(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.CustomFields(r.Context(), auth.Actor(r.Context()), r.PathValue("key"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	out := make([]fieldDTO, 0, len(list))
	for _, f := range list {
		out = append(out, fieldOut(f))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) createCustomField(w http.ResponseWriter, r *http.Request) {
	var in fieldIn
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	f, err := h.svc.CreateCustomField(r.Context(), auth.Actor(r.Context()), r.PathValue("key"), in.spec())
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, fieldOut(f))
}

func (h *Handler) editCustomField(w http.ResponseWriter, r *http.Request) {
	var in fieldIn
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	f, err := h.svc.EditCustomField(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), in.Version, in.spec())
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, fieldOut(f))
}

func (h *Handler) deleteCustomField(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteCustomField(r.Context(), auth.Actor(r.Context()), r.PathValue("id")); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
