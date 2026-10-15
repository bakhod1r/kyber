package realip_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/realip"
)

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		in      string
		n       int
		wantErr bool
	}{
		{"", 0, false},
		{"10.0.0.0/8", 1, false},
		{" 10.0.0.0/8 , 192.168.1.1 ,::1/128", 3, false},
		{"not-an-ip", 0, true},
		{"10.0.0.0/99", 0, true},
	} {
		got, err := realip.Parse(tc.in)
		if (err != nil) != tc.wantErr || len(got) != tc.n {
			t.Fatalf("Parse(%q) = %v, %v", tc.in, got, err)
		}
	}
}

func TestMiddleware(t *testing.T) {
	trusted, _ := realip.Parse("10.0.0.0/8")
	for _, tc := range []struct {
		name, peer, xff, want string
	}{
		{"untrusted peer keeps its address", "203.0.113.1:5000", "198.51.100.9", "203.0.113.1:5000"},
		{"trusted peer without header", "10.0.0.2:5000", "", "10.0.0.2:5000"},
		{"trusted peer, one hop", "10.0.0.2:5000", "198.51.100.9", "198.51.100.9:0"},
		{"rightmost untrusted wins over a forged left part", "10.0.0.2:5000", "1.1.1.1, 198.51.100.9", "198.51.100.9:0"},
		{"proxy chain is skipped", "10.0.0.2:5000", "198.51.100.9, 10.0.0.7", "198.51.100.9:0"},
		{"several headers are one list", "10.0.0.2:5000", "198.51.100.9|10.0.0.7", "198.51.100.9:0"},
		{"garbage entry stops the walk", "10.0.0.2:5000", "198.51.100.9, junk", "10.0.0.2:5000"},
		{"all trusted: leftmost", "10.0.0.2:5000", "10.0.0.5, 10.0.0.7", "10.0.0.5:0"},
		{"ipv6 client", "10.0.0.2:5000", "2001:db8::1", "[2001:db8::1]:0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			h := realip.Middleware(trusted, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = r.RemoteAddr }))
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tc.peer
			if tc.xff != "" {
				for _, v := range splitBar(tc.xff) {
					req.Header.Add("X-Forwarded-For", v)
				}
			}
			h.ServeHTTP(httptest.NewRecorder(), req)
			if got != tc.want {
				t.Fatalf("RemoteAddr = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMiddlewareWithoutTrustedIsIdentity(t *testing.T) {
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	if h := realip.Middleware(nil, next); h == nil {
		t.Fatal("nil handler")
	}
}

func splitBar(s string) []string {
	var out []string
	start := 0
	for i := range len(s) {
		if s[i] == '|' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}
