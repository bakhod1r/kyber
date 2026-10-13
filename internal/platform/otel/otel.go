// Package otel wires OpenTelemetry tracing: the global tracer provider (OTLP over HTTP), W3C trace
// context propagation, a server span per HTTP request named after its route, a client span per
// SQL statement, and trace ids on log records. Without an OTLP endpoint tracing stays off and
// every hook is a no-op.
package otel

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const scope = "github.com/bakhod1r/kyber"

// Tracer is the application's tracer from the global provider (a no-op until Setup enables tracing).
func Tracer() trace.Tracer { return otel.Tracer(scope) }

// Enabled reports whether the environment asks for trace export (the standard OTEL_* variables).
func Enabled(getenv func(string) string) bool {
	if strings.EqualFold(getenv("OTEL_SDK_DISABLED"), "true") || strings.EqualFold(getenv("OTEL_TRACES_EXPORTER"), "none") {
		return false
	}
	return getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" || getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") != ""
}

// Setup installs W3C trace context propagation and, when Enabled, a tracer provider exporting
// over OTLP/HTTP (endpoint, headers and sampler come from the standard OTEL_* variables).
// The returned shutdown flushes pending spans.
func Setup(ctx context.Context, service, version string, getenv func(string) string) (shutdown func(context.Context) error, err error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	if !Enabled(getenv) {
		return func(context.Context) error { return nil }, nil
	}
	return installing(ctx, service, version)(otlptracehttp.New(ctx))
}

// installing installs the exporter a constructor returned, or reports its error.
func installing(ctx context.Context, service, version string) func(sdktrace.SpanExporter, error) (func(context.Context) error, error) {
	return func(exp sdktrace.SpanExporter, err error) (func(context.Context) error, error) {
		if err != nil {
			return nil, err
		}
		return Install(ctx, exp, service, version)
	}
}

// Install makes a provider batching spans to exp the global one.
func Install(ctx context.Context, exp sdktrace.SpanExporter, service, version string) (func(context.Context) error, error) {
	res, err := resource.New(ctx, resource.WithFromEnv(), resource.WithTelemetrySDK(),
		resource.WithAttributes(attribute.String("service.name", service), attribute.String("service.version", version)))
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp), sdktrace.WithResource(res))
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

// ---- HTTP

type route struct{ pattern string }

type routeKey struct{}

// probes are not traced: they are frequent and say nothing about requests.
var probes = map[string]bool{"/healthz": true, "/readyz": true, "/metrics": true}

// Middleware gives every request a server span (continuing an incoming traceparent) named after
// the route its innermost Routed mux matched, e.g. "GET /api/v1/issues/{issueKey}".
func Middleware(next http.Handler) http.Handler {
	named := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rt := &route{}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), routeKey{}, rt)))
		span := trace.SpanFromContext(r.Context())
		if rt.pattern == "" {
			span.SetName(r.Method)
			return
		}
		span.SetName(rt.pattern)
		span.SetAttributes(attribute.String("http.route", rt.pattern[strings.IndexByte(rt.pattern, ' ')+1:]))
	})
	return otelhttp.NewHandler(named, "http", otelhttp.WithFilter(func(r *http.Request) bool { return !probes[r.URL.Path] }))
}

// Routed records the pattern mux matched for the request's span; the innermost mux wins.
func Routed(mux http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r)
		if rt, ok := r.Context().Value(routeKey{}).(*route); ok && rt.pattern == "" {
			rt.pattern = r.Pattern
		}
	})
}

// ---- PostgreSQL

// QueryTracer gives every SQL statement a client span. Statements are parameterized, so the
// recorded text carries no values.
type QueryTracer struct{}

const maxQueryText = 2000

func (QueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	sql := strings.TrimSpace(data.SQL)
	op, _, _ := strings.Cut(sql, " ")
	if len(sql) > maxQueryText {
		sql = sql[:maxQueryText]
	}
	ctx, _ = Tracer().Start(ctx, "db "+strings.ToUpper(op), trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String("db.system.name", "postgresql"), attribute.String("db.query.text", sql)))
	return ctx
}

func (QueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	span := trace.SpanFromContext(ctx)
	if data.Err != nil && !errors.Is(data.Err, pgx.ErrNoRows) {
		span.RecordError(data.Err)
		span.SetStatus(codes.Error, data.Err.Error())
	}
	span.End()
}

// ---- logs

// LogHandler adds trace_id and span_id to records logged inside a span.
type LogHandler struct{ slog.Handler }

func (h LogHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(slog.String("trace_id", sc.TraceID().String()), slog.String("span_id", sc.SpanID().String()))
	}
	return h.Handler.Handle(ctx, r)
}

func (h LogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return LogHandler{h.Handler.WithAttrs(attrs)}
}
func (h LogHandler) WithGroup(name string) slog.Handler { return LogHandler{h.Handler.WithGroup(name)} }
