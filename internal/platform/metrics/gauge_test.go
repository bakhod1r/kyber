package metrics_test

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/metrics"
)

func TestGauges(t *testing.T) {
	m := metrics.NewHTTP()
	n := 3.0
	m.Gauge("kyber_calls_active", "Live calls.", func() float64 { return n })
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body, _ := io.ReadAll(rec.Body)
	for _, want := range []string{"# HELP kyber_calls_active Live calls.", "# TYPE kyber_calls_active gauge", "kyber_calls_active 3"} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
}
