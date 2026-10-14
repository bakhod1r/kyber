package server_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/server"
)

// ADR-0017: an identity provider provisions, updates and deprovisions users over SCIM 2.0.
func TestSCIM(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		idp := &client{t: t, srv: anon.srv, token: "scim-secret-token-0123456789"}
		wrong := &client{t: t, srv: anon.srv, token: "nope"}
		code, b := wrong.do("GET", "/scim/v2/Users", nil)
		expect(t, code, 401, b)

		code, cfg := idp.do("GET", "/scim/v2/ServiceProviderConfig", nil)
		expect(t, code, 200, cfg)
		if cfg["patch"].(map[string]any)["supported"] != true {
			t.Fatalf("config = %v", cfg)
		}
		code, u := idp.do("POST", "/scim/v2/Users", map[string]any{
			"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:User"}, "userName": "Ann@X.uz",
			"name": map[string]string{"givenName": "Ann", "familyName": "Lee"}, "active": true,
		})
		expect(t, code, 201, u)
		id := u["id"].(string)
		if u["userName"] != "ann@x.uz" || u["displayName"] != "Ann Lee" || u["active"] != true {
			t.Fatalf("created = %v", u)
		}
		code, b = idp.do("POST", "/scim/v2/Users", map[string]any{"userName": "ann@x.uz", "displayName": "Dup"})
		expect(t, code, 409, b)
		if b["scimType"] != "uniqueness" {
			t.Fatalf("conflict = %v", b)
		}
		code, list := idp.do("GET", `/scim/v2/Users?filter=userName%20eq%20%22ann%40x.uz%22`, nil)
		expect(t, code, 200, list)
		if list["totalResults"] != float64(1) || list["Resources"].([]any)[0].(map[string]any)["id"] != id {
			t.Fatalf("filtered = %v", list)
		}
		code, list = idp.do("GET", "/scim/v2/Users?startIndex=1&count=1", nil)
		expect(t, code, 200, list)
		if len(list["Resources"].([]any)) != 1 || list["itemsPerPage"] != float64(1) {
			t.Fatalf("page = %v", list)
		}

		// A regular member is deactivated through PATCH and loses access at once.
		bo := anon.signedIn("bo@x.uz")
		code, me := bo.do("GET", "/api/v1/me", nil)
		expect(t, code, 200, me)
		code, b = idp.do("PATCH", "/scim/v2/Users/"+me["id"].(string), map[string]any{
			"schemas":    []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
			"Operations": []map[string]any{{"op": "Replace", "value": map[string]any{"active": false}}},
		})
		expect(t, code, 200, b)
		if b["active"] != false {
			t.Fatalf("patched = %v", b)
		}
		code, b = bo.do("GET", "/api/v1/me", nil)
		expect(t, code, 401, b)

		code, b = idp.do("PUT", "/scim/v2/Users/"+id, map[string]any{"userName": "ann@x.uz", "displayName": "Ann L.", "active": true})
		expect(t, code, 200, b)
		if b["displayName"] != "Ann L." {
			t.Fatalf("replaced = %v", b)
		}
		code, b = idp.do("DELETE", "/scim/v2/Users/"+id, nil)
		if code != 204 {
			t.Fatalf("delete = %d %v", code, b)
		}
		code, b = idp.do("GET", "/scim/v2/Users/"+id, nil)
		expect(t, code, 200, b)
		if b["active"] != false {
			t.Fatalf("deleted users are deactivated, not removed: %v", b)
		}
		code, b = idp.do("GET", "/scim/v2/Users/40000000-0000-4000-8000-0000000000ff", nil)
		expect(t, code, 404, b)
	}, server.WithSCIMToken("scim-secret-token-0123456789"))
}
