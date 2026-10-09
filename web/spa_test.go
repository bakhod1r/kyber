package web_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/bakhod1r/kyber/web"
)

func get(t *testing.T, h http.Handler, path string) (int, string, http.Header) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	b, _ := io.ReadAll(rec.Body)
	return rec.Code, string(b), rec.Header()
}

func TestSPAHandler(t *testing.T) {
	h := web.Handler(fstest.MapFS{
		"index.html":      {Data: []byte("<html>app</html>")},
		"assets/app-1.js": {Data: []byte("console.log(1)")},
		"logo.svg":        {Data: []byte("<svg/>")},
	})

	code, body, hdr := get(t, h, "/")
	if code != 200 || body != "<html>app</html>" || hdr.Get("Cache-Control") != "no-cache" {
		t.Fatalf("/ = %d %q %v", code, body, hdr)
	}
	code, body, hdr = get(t, h, "/assets/app-1.js")
	if code != 200 || body != "console.log(1)" || !strings.Contains(hdr.Get("Cache-Control"), "immutable") {
		t.Fatalf("asset = %d %q %v", code, body, hdr)
	}
	// Client-side routes fall back to index.html.
	if code, body, _ := get(t, h, "/projects/KYB"); code != 200 || body != "<html>app</html>" {
		t.Fatalf("client route = %d %q", code, body)
	}
	// Missing assets are real 404s, not the app shell.
	if code, _, _ := get(t, h, "/assets/missing.js"); code != 404 {
		t.Fatalf("missing asset = %d", code)
	}
	if code, _, _ := get(t, h, "/logo.svg"); code != 200 {
		t.Fatalf("logo = %d", code)
	}
	if csp := hdr.Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'self'") || !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Fatalf("CSP = %q", csp)
	}
	if hdr.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("nosniff header missing")
	}
}

func TestSPAHandlerWithoutBuild(t *testing.T) {
	h := web.Handler(fstest.MapFS{".gitkeep": {}})
	code, body, _ := get(t, h, "/")
	if code != 503 || !strings.Contains(body, "npm run build") {
		t.Fatalf("unbuilt = %d %q", code, body)
	}
}
