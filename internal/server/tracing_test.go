package server_test

import (
	"context"
	"testing"

	gotel "go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/bakhod1r/kyber/internal/platform/otel"
)

// Tracing: API requests become server spans named after their route; SQL runs inside them.
func TestTracing(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	shutdown, err := otel.Install(context.Background(), exp, "kyber-test", "v0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = shutdown(context.Background())
		gotel.SetTracerProvider(noop.NewTracerProvider())
	})
	forEachBackend(t, func(t *testing.T, anon *client) {
		exp.Reset()
		lead := anon.signedIn("lead@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		code, b := lead.do("GET", "/api/v1/projects/KYB/goals", nil)
		expect(t, code, 200, b)
		_ = gotel.GetTracerProvider().(*sdktrace.TracerProvider).ForceFlush(context.Background())
		names := map[string]bool{}
		for _, s := range exp.GetSpans() {
			names[s.Name] = true
		}
		if !names["GET /api/v1/projects/{key}/goals"] || !names["POST /api/v1/projects"] {
			t.Fatalf("spans = %v", names)
		}
	})
}
