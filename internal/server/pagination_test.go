package server_test

import (
	"fmt"
	"testing"
)

// Issue lists page with limit/offset, so large projects are not sent whole.
func TestIssueListPaging(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		dev := anon.signedIn("dev@x.uz")
		dev.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		for i := range 5 {
			dev.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": fmt.Sprint("Issue ", i), "type": "task"})
		}
		code, page := dev.do("GET", "/api/v1/projects/KYB/issues?limit=2&offset=2", nil)
		expect(t, code, 200, page)
		items := page["items"].([]any)
		if len(items) != 2 || items[0].(map[string]any)["key"] != "KYB-3" {
			t.Fatalf("page = %v", items)
		}
		for _, bad := range []string{"limit=0", "limit=501", "offset=-1", "limit=x"} {
			code, b := dev.do("GET", "/api/v1/projects/KYB/issues?"+bad, nil)
			expect(t, code, 422, b)
		}
	})
}
