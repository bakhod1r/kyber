package server_test

import (
	"fmt"
	"net/url"
	"testing"
)

// Labels: tag issues, filter the list and JQL by label, and autocomplete the project's labels.
func TestLabels(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		dev := anon.signedIn("dev@x.uz")
		eve := anon.signedIn("eve@x.uz")
		dev.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		for i, labels := range [][]string{{"ui", "backend"}, {"ui"}, nil} {
			_, is := dev.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": fmt.Sprint("Issue ", i), "type": "task"})
			if labels == nil {
				continue
			}
			code, b := dev.do("PATCH", "/api/v1/issues/"+is["key"].(string), map[string]any{"version": is["version"], "labels": labels})
			expect(t, code, 200, b)
		}
		_, is := dev.do("GET", "/api/v1/issues/KYB-1", nil)
		if fmt.Sprint(is["labels"]) != "[backend ui]" {
			t.Fatalf("labels = %v", is["labels"])
		}
		_, plain := dev.do("GET", "/api/v1/issues/KYB-3", nil)
		if fmt.Sprint(plain["labels"]) != "[]" {
			t.Fatalf("no labels = %v", plain["labels"])
		}
		code, b := dev.do("PATCH", "/api/v1/issues/KYB-3", map[string]any{"version": plain["version"], "labels": []string{"two words"}})
		expect(t, code, 422, b)

		_, list := dev.do("GET", "/api/v1/projects/KYB/issues?label=ui", nil)
		if n := len(list["items"].([]any)); n != 2 {
			t.Fatalf("label filter = %d", n)
		}
		code, got := dev.do("GET", "/api/v1/projects/KYB/labels", nil)
		expect(t, code, 200, got)
		if fmt.Sprint(got["items"]) != "[backend ui]" {
			t.Fatalf("project labels = %v", got["items"])
		}
		code, b = eve.do("GET", "/api/v1/projects/KYB/labels", nil)
		expect(t, code, 404, b)

		_, found := dev.do("GET", "/api/v1/search?jql="+url.QueryEscape(`project = KYB AND labels = backend`), nil)
		if n := len(found["items"].([]any)); n != 1 {
			t.Fatalf("jql labels = %v", found)
		}
	})
}
