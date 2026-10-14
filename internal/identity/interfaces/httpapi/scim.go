package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/bakhod1r/kyber/internal/identity/application"
	"github.com/bakhod1r/kyber/internal/identity/domain"
)

// SCIM 2.0 user provisioning (RFC 7643/7644, ADR-0017) for identity providers such as Okta,
// Entra ID or Keycloak. One shared bearer token (KYBER_SCIM_TOKEN) authenticates the provider.

const (
	scimUser  = "urn:ietf:params:scim:schemas:core:2.0:User"
	scimList  = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	scimError = "urn:ietf:params:scim:api:messages:2.0:Error"
)

// RegisterSCIM mounts /scim/v2 when token is set.
func (h *Handler) RegisterSCIM(mux *http.ServeMux, token string) {
	if token == "" {
		return
	}
	guard := func(fn http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
				scimFail(w, http.StatusUnauthorized, "", "a valid SCIM bearer token is required")
				return
			}
			fn(w, r)
		}
	}
	mux.HandleFunc("GET /scim/v2/ServiceProviderConfig", guard(h.scimConfig))
	mux.HandleFunc("GET /scim/v2/Users", guard(h.scimList))
	mux.HandleFunc("POST /scim/v2/Users", guard(h.scimCreate))
	mux.HandleFunc("GET /scim/v2/Users/{id}", guard(h.scimGet))
	mux.HandleFunc("PUT /scim/v2/Users/{id}", guard(h.scimReplace))
	mux.HandleFunc("PATCH /scim/v2/Users/{id}", guard(h.scimPatch))
	mux.HandleFunc("DELETE /scim/v2/Users/{id}", guard(h.scimDelete))
}

func scimJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func scimFail(w http.ResponseWriter, status int, scimType, detail string) {
	body := map[string]any{"schemas": []string{scimError}, "status": strconv.Itoa(status), "detail": detail}
	if scimType != "" {
		body["scimType"] = scimType
	}
	scimJSON(w, status, body)
}

// scimErr maps domain errors to SCIM errors; anything unknown is logged and hidden.
func (h *Handler) scimErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		scimFail(w, http.StatusNotFound, "", "user not found")
	case errors.Is(err, domain.ErrEmailTaken):
		scimFail(w, http.StatusConflict, "uniqueness", "a user with this userName exists")
	case errors.Is(err, domain.ErrInvalidEmail), errors.Is(err, domain.ErrEmptyName), errors.Is(err, domain.ErrInvalidName), errors.Is(err, errSCIMBody):
		scimFail(w, http.StatusBadRequest, "invalidValue", err.Error())
	default:
		h.log.ErrorContext(r.Context(), "scim", "err", err)
		scimFail(w, http.StatusInternalServerError, "", "internal error")
	}
}

var errSCIMBody = errors.New("invalid SCIM request body")

func scimOut(u *domain.User) map[string]any {
	return map[string]any{
		"schemas": []string{scimUser}, "id": string(u.ID()), "userName": u.Email().String(), "displayName": u.Name(),
		"name":   map[string]string{"formatted": u.Name()},
		"emails": []map[string]any{{"value": u.Email().String(), "primary": true}},
		"active": u.Active(), "meta": map[string]string{"resourceType": "User"},
	}
}

// scimIn is the part of a SCIM User Kyber stores.
type scimIn struct {
	UserName    string `json:"userName"`
	DisplayName string `json:"displayName"`
	Name        struct {
		Formatted  string `json:"formatted"`
		GivenName  string `json:"givenName"`
		FamilyName string `json:"familyName"`
	} `json:"name"`
	Active *bool `json:"active"`
}

func (in scimIn) displayName() string {
	switch {
	case in.DisplayName != "":
		return in.DisplayName
	case in.Name.Formatted != "":
		return in.Name.Formatted
	}
	if n := strings.TrimSpace(in.Name.GivenName + " " + in.Name.FamilyName); n != "" {
		return n
	}
	return strings.Split(in.UserName, "@")[0]
}

func decodeSCIM(r *http.Request, v any) error {
	if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<16)).Decode(v); err != nil {
		return errSCIMBody
	}
	return nil
}

func (h *Handler) scimConfig(w http.ResponseWriter, _ *http.Request) {
	no := map[string]bool{"supported": false}
	scimJSON(w, http.StatusOK, map[string]any{
		"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"},
		"patch":   map[string]bool{"supported": true}, "filter": map[string]any{"supported": true, "maxResults": 200},
		"bulk": map[string]any{"supported": false, "maxOperations": 0, "maxPayloadSize": 0}, "changePassword": no, "sort": no, "etag": no,
		"authenticationSchemes": []map[string]string{{"type": "oauthbearertoken", "name": "Bearer token", "description": "KYBER_SCIM_TOKEN"}},
	})
}

var userNameEq = regexp.MustCompile(`(?i)^\s*userName\s+eq\s+"([^"]*)"\s*$`)

func (h *Handler) scimList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	email := ""
	if f := q.Get("filter"); f != "" {
		m := userNameEq.FindStringSubmatch(f)
		if m == nil {
			scimFail(w, http.StatusBadRequest, "invalidFilter", `only 'userName eq "…"' filters are supported`)
			return
		}
		email = m[1]
	}
	start, _ := strconv.Atoi(q.Get("startIndex"))
	count, err := strconv.Atoi(q.Get("count"))
	if err != nil || count > 200 || count < 0 {
		count = 100
	}
	start = max(start, 1)
	list, total, err := h.svc.DirectoryUsers(r.Context(), email, start-1, count)
	if err != nil {
		h.scimErr(w, r, err)
		return
	}
	res := make([]map[string]any, 0, len(list))
	for _, u := range list {
		res = append(res, scimOut(u))
	}
	scimJSON(w, http.StatusOK, map[string]any{"schemas": []string{scimList}, "totalResults": total, "startIndex": start, "itemsPerPage": len(res), "Resources": res})
}

func (h *Handler) scimCreate(w http.ResponseWriter, r *http.Request) {
	var in scimIn
	if err := decodeSCIM(r, &in); err != nil {
		h.scimErr(w, r, err)
		return
	}
	u, err := h.svc.ProvisionUser(r.Context(), in.UserName, in.displayName(), in.Active == nil || *in.Active)
	if err != nil {
		h.scimErr(w, r, err)
		return
	}
	scimJSON(w, http.StatusCreated, scimOut(u))
}

func (h *Handler) scimGet(w http.ResponseWriter, r *http.Request) {
	u, err := h.svc.DirectoryUser(r.Context(), r.PathValue("id"))
	if err != nil {
		h.scimErr(w, r, err)
		return
	}
	scimJSON(w, http.StatusOK, scimOut(u))
}

func (h *Handler) scimUpdate(w http.ResponseWriter, r *http.Request, c application.DirectoryChange) {
	u, err := h.svc.UpdateDirectoryUser(r.Context(), r.PathValue("id"), c)
	if err != nil {
		h.scimErr(w, r, err)
		return
	}
	scimJSON(w, http.StatusOK, scimOut(u))
}

// scimReplace (PUT) applies the name and active flag; the userName (email) is the identity and
// does not change.
func (h *Handler) scimReplace(w http.ResponseWriter, r *http.Request) {
	var in scimIn
	if err := decodeSCIM(r, &in); err != nil {
		h.scimErr(w, r, err)
		return
	}
	name, active := in.displayName(), in.Active == nil || *in.Active
	h.scimUpdate(w, r, application.DirectoryChange{Name: &name, Active: &active})
}

// scimPatch understands replace operations on active and the display name, with or without a path.
func (h *Handler) scimPatch(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Operations []struct {
			Op    string          `json:"op"`
			Path  string          `json:"path"`
			Value json.RawMessage `json:"value"`
		} `json:"Operations"`
	}
	if err := decodeSCIM(r, &in); err != nil {
		h.scimErr(w, r, err)
		return
	}
	var c application.DirectoryChange
	for _, op := range in.Operations {
		if !strings.EqualFold(op.Op, "replace") && !strings.EqualFold(op.Op, "add") {
			continue
		}
		fields := map[string]json.RawMessage{}
		if op.Path != "" {
			fields[op.Path] = op.Value
		} else if json.Unmarshal(op.Value, &fields) != nil {
			h.scimErr(w, r, errSCIMBody)
			return
		}
		for path, raw := range fields {
			switch strings.ToLower(path) {
			case "active":
				var b bool
				if json.Unmarshal(raw, &b) != nil {
					var s string // some providers send "False"
					_ = json.Unmarshal(raw, &s)
					b = strings.EqualFold(s, "true")
				}
				c.Active = &b
			case "displayname", "name.formatted":
				var s string
				if json.Unmarshal(raw, &s) == nil {
					c.Name = &s
				}
			}
		}
	}
	h.scimUpdate(w, r, c)
}

// scimDelete deactivates: the user's issues and comments keep their author.
func (h *Handler) scimDelete(w http.ResponseWriter, r *http.Request) {
	inactive := false
	if _, err := h.svc.UpdateDirectoryUser(r.Context(), r.PathValue("id"), application.DirectoryChange{Active: &inactive}); err != nil {
		h.scimErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
