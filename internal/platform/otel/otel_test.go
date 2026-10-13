package otel_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	gotel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/bakhod1r/kyber/internal/platform/otel"
)

func env(kv ...string) func(string) string {
	m := map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return func(k string) string { return m[k] }
}

func TestEnabled(t *testing.T) {
	for _, tc := range []struct {
		env  func(string) string
		want bool
	}{
		{env(), false},
		{env("OTEL_EXPORTER_OTLP_ENDPOINT", "http://collector:4318"), true},
		{env("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "http://collector:4318/v1/traces"), true},
		{env("OTEL_EXPORTER_OTLP_ENDPOINT", "http://c", "OTEL_SDK_DISABLED", "TRUE"), false},
		{env("OTEL_EXPORTER_OTLP_ENDPOINT", "http://c", "OTEL_TRACES_EXPORTER", "none"), false},
	} {
		if got := otel.Enabled(tc.env); got != tc.want {
			t.Errorf("Enabled = %v, want %v", got, tc.want)
		}
	}
}

// record installs an in-memory exporter as the global provider for one test.
func record(t *testing.T) (*tracetest.InMemoryExporter, func()) {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	shutdown, err := otel.Install(context.Background(), exp, "kyber-test", "v0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { gotel.SetTracerProvider(noop.NewTracerProvider()) })
	t.Cleanup(func() { _ = shutdown(context.Background()) })
	return exp, func() { _ = gotel.GetTracerProvider().(*sdktrace.TracerProvider).ForceFlush(context.Background()) }
}

func TestSetup(t *testing.T) {
	shutdown, err := otel.Setup(context.Background(), "kyber", "v1", env())
	if err != nil || shutdown(context.Background()) != nil {
		t.Fatalf("disabled setup: %v", err)
	}
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:1")
	shutdown, err = otel.Setup(context.Background(), "kyber", "v1", env("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:1"))
	t.Cleanup(func() { gotel.SetTracerProvider(noop.NewTracerProvider()) })
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // nothing to flush; do not wait on the unreachable collector
	_ = shutdown(ctx)
}

func TestInstallRejectsBadResource(t *testing.T) {
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "not-a-pair")
	if _, err := otel.Install(context.Background(), tracetest.NewInMemoryExporter(), "kyber", "v1"); err == nil {
		t.Fatal("malformed OTEL_RESOURCE_ATTRIBUTES accepted")
	}
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:1")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "not-a-pair")
	if _, err := otel.Setup(context.Background(), "kyber", "v1", env("OTEL_EXPORTER_OTLP_ENDPOINT", "x")); err == nil {
		t.Fatal("setup with a malformed resource succeeded")
	}
}

func TestMiddlewareNamesSpansAfterTheInnermostRoute(t *testing.T) {
	exp, flush := record(t)
	api := http.NewServeMux()
	api.HandleFunc("GET /api/v1/issues/{issueKey}", func(w http.ResponseWriter, r *http.Request) {
		if !trace.SpanContextFromContext(r.Context()).IsValid() {
			t.Error("handler runs outside the request span")
		}
	})
	outer := http.NewServeMux()
	outer.Handle("/api/v1/", otel.Routed(api))
	outer.HandleFunc("GET /healthz", func(http.ResponseWriter, *http.Request) {})
	h := otel.Middleware(otel.Routed(outer))

	parent := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	for _, path := range []string{"/api/v1/issues/KYB-1", "/nowhere", "/healthz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("traceparent", parent)
		h.ServeHTTP(httptest.NewRecorder(), req)
	}
	// Without Routed the outer pattern is not recorded and the span keeps the method.
	otel.Middleware(http.NotFoundHandler()).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/x", nil))
	flush()

	spans := exp.GetSpans()
	if len(spans) != 3 {
		t.Fatalf("spans = %d, want 3 (probes are not traced)", len(spans))
	}
	if s := spans[0]; s.Name != "GET /api/v1/issues/{issueKey}" || s.Parent.TraceID().String() != "4bf92f3577b34da6a3ce929d0e0e4736" || s.SpanKind != trace.SpanKindServer {
		t.Fatalf("api span = %q parent %v kind %v", s.Name, s.Parent.TraceID(), s.SpanKind)
	}
	found := false
	for _, a := range spans[0].Attributes {
		if a.Key == "http.route" && a.Value.AsString() == "/api/v1/issues/{issueKey}" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no http.route on %v", spans[0].Attributes)
	}
	if spans[1].Name != "GET" || spans[2].Name != "POST" {
		t.Fatalf("unrouted spans = %q %q", spans[1].Name, spans[2].Name)
	}
}

func TestQueryTracer(t *testing.T) {
	exp, flush := record(t)
	var qt otel.QueryTracer
	ctx := qt.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: "  select id from issues where id = $1"})
	qt.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: pgx.ErrNoRows})
	ctx = qt.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: "UPDATE issues SET title = $1 " + strings.Repeat("x", 3000)})
	qt.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: errors.New("deadlock")})
	flush()
	spans := exp.GetSpans()
	if len(spans) != 2 || spans[0].Name != "db SELECT" || spans[0].Status.Code == codes.Error || spans[1].Name != "db UPDATE" || spans[1].Status.Code != codes.Error {
		t.Fatalf("spans = %+v", spans)
	}
	for _, a := range spans[1].Attributes {
		if a.Key == "db.query.text" && len(a.Value.AsString()) != 2000 {
			t.Fatalf("query text not truncated: %d", len(a.Value.AsString()))
		}
	}
}

func TestLogHandlerAddsTraceIDs(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(otel.LogHandler{Handler: slog.NewJSONHandler(&buf, nil)}).With("app", "kyber")
	grouped := log.WithGroup("g")
	tp := sdktrace.NewTracerProvider()
	ctx, span := tp.Tracer("t").Start(context.Background(), "op")
	log.InfoContext(ctx, "inside")
	span.End()
	log.InfoContext(context.Background(), "outside")
	grouped.InfoContext(ctx, "grouped")
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	var in, out map[string]any
	_ = json.Unmarshal([]byte(lines[0]), &in)
	_ = json.Unmarshal([]byte(lines[1]), &out)
	if in["trace_id"] != span.SpanContext().TraceID().String() || in["span_id"] != span.SpanContext().SpanID().String() || in["app"] != "kyber" {
		t.Fatalf("inside = %v", in)
	}
	if _, ok := out["trace_id"]; ok {
		t.Fatalf("outside = %v", out)
	}
	if !strings.Contains(lines[2], span.SpanContext().TraceID().String()) {
		t.Fatalf("grouped = %s", lines[2])
	}
}

func TestSetupReportsExporterErrors(t *testing.T) {
	boom := errors.New("no exporter")
	if _, err := otel.Installing(context.Background(), "kyber", "v1")(nil, boom); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}
