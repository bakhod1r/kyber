package server_test

import (
	"io"
	"strings"
	"testing"
)

// S48: versions (Jira's fix versions) — plan issues into a release, release it moving unfinished
// work on, and read the release notes. Only project admins manage versions.
func TestS48Versions(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		eve := anon.signedIn("eve@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "OPS", "name": "Ops"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "dev@x.uz", "role": "member"})

		code, b := dev.do("POST", "/api/v1/projects/KYB/versions", map[string]string{"name": "1.0"})
		expect(t, code, 403, b)
		code, b = eve.do("POST", "/api/v1/projects/KYB/versions", map[string]string{"name": "1.0"})
		expect(t, code, 404, b)
		code, v1 := lead.do("POST", "/api/v1/projects/KYB/versions", map[string]any{"name": "1.0", "description": "First release", "release_date": "2026-12-01T00:00:00Z"})
		expect(t, code, 201, v1)
		code, b = lead.do("POST", "/api/v1/projects/KYB/versions", map[string]string{"name": "1.0"})
		expect(t, code, 409, b)
		_, v2 := lead.do("POST", "/api/v1/projects/KYB/versions", map[string]string{"name": "1.1"})
		_, ops := lead.do("POST", "/api/v1/projects/OPS/versions", map[string]string{"name": "o"})
		id1, id2 := v1["id"].(string), v2["id"].(string)

		for i, typ := range []string{"story", "bug", "task"} {
			_, is := dev.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": []string{"Sign in", "Crash", "Docs"}[i], "type": typ})
			code, b := dev.do("PATCH", "/api/v1/issues/"+is["key"].(string), map[string]any{"version": is["version"], "fix_versions": []string{id1}})
			expect(t, code, 200, b)
		}
		_, is := dev.do("GET", "/api/v1/issues/KYB-1", nil)
		code, b = dev.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{"version": is["version"], "fix_versions": []string{ops["id"].(string)}})
		expect(t, code, 422, b)
		dev.do("POST", "/api/v1/issues/KYB-1/transitions", map[string]string{"to": "done"})
		dev.do("POST", "/api/v1/issues/KYB-3/transitions", map[string]string{"to": "done", "resolution": "wont_do"})

		_, list := dev.do("GET", "/api/v1/projects/KYB/issues?fix_version="+id1, nil)
		if n := len(list["items"].([]any)); n != 3 {
			t.Fatalf("issues in 1.0 = %d", n)
		}
		_, vs := dev.do("GET", "/api/v1/projects/KYB/versions", nil)
		first := vs["items"].([]any)[0].(map[string]any)
		if first["name"] != "1.0" || first["issues"] != float64(3) || first["done"] != float64(2) || first["unresolved"] != float64(1) {
			t.Fatalf("versions = %v", vs)
		}

		// Release 1.0, moving the unresolved bug to 1.1.
		code, b = dev.do("POST", "/api/v1/versions/"+id1+"/release", nil)
		expect(t, code, 403, b)
		code, b = lead.do("POST", "/api/v1/versions/"+id1+"/release", map[string]string{"move_to": id1})
		expect(t, code, 422, b)
		code, b = lead.do("POST", "/api/v1/versions/"+id1+"/release", map[string]string{"move_to": id2})
		expect(t, code, 200, b)
		if b["moved"] != float64(1) || b["version"].(map[string]any)["state"] != "released" {
			t.Fatalf("release = %v", b)
		}
		_, crash := dev.do("GET", "/api/v1/issues/KYB-2", nil)
		if fv := crash["fix_versions"].([]any); len(fv) != 1 || fv[0] != id2 {
			t.Fatalf("KYB-2 fix versions = %v", fv)
		}
		code, b = lead.do("POST", "/api/v1/versions/"+id1+"/release", nil)
		expect(t, code, 409, b)

		res := dev.raw("GET", "/api/v1/versions/"+id1+"/release-notes", nil, nil)
		notes, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if !strings.Contains(string(notes), "# Release notes — 1.0") || !strings.Contains(string(notes), "- [KYB-1] Sign in") ||
			strings.Contains(string(notes), "Crash") {
			t.Fatalf("notes:\n%s", notes)
		}
		code, b = dev.do("GET", "/api/v1/versions/"+id1+"/issues", nil)
		expect(t, code, 200, b)
		if code, b := eve.do("GET", "/api/v1/versions/"+id1+"/issues", nil); code != 404 {
			t.Fatalf("outsider: %d %v", code, b)
		}

		// Archive hides it from new issues; restore brings it back released.
		code, b = lead.do("POST", "/api/v1/versions/"+id1+"/archive", nil)
		expect(t, code, 200, b)
		_, is = dev.do("GET", "/api/v1/issues/KYB-2", nil)
		code, b = dev.do("PATCH", "/api/v1/issues/KYB-2", map[string]any{"version": is["version"], "fix_versions": []string{id1, id2}})
		expect(t, code, 422, b)
		code, b = lead.do("POST", "/api/v1/versions/"+id1+"/restore", nil)
		expect(t, code, 200, b)
		if b["state"] != "released" {
			t.Fatalf("restored = %v", b)
		}
		code, b = lead.do("POST", "/api/v1/versions/"+id1+"/unrelease", nil)
		expect(t, code, 200, b)

		// Edit with optimistic locking.
		code, b = lead.do("PATCH", "/api/v1/versions/"+id2, map[string]any{"version": 99, "name": "2.0"})
		expect(t, code, 409, b)
		code, b = lead.do("PATCH", "/api/v1/versions/"+id2, map[string]any{"version": v2["version"], "name": "2.0"})
		expect(t, code, 200, b)

		// Delete 2.0 moving its issue back to 1.0.
		code, b = lead.do("DELETE", "/api/v1/versions/"+id2+"?move_to="+id1, nil)
		expect(t, code, 204, b)
		_, crash = dev.do("GET", "/api/v1/issues/KYB-2", nil)
		if fv := crash["fix_versions"].([]any); len(fv) != 1 || fv[0] != id1 {
			t.Fatalf("after delete = %v", fv)
		}
		code, b = lead.do("DELETE", "/api/v1/versions/"+id2, nil)
		expect(t, code, 404, b)
	})
}
