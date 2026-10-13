# ADR-0006: OpenTelemetry tracing

Status: accepted (2026-10-13)

## Context

Kyber logged requests (`slog`, JSON) and counted them (`/metrics`), but a slow or failing request
could not be followed through its use case, its SQL and the domain events it caused. The plan
(`docs/PLAN.md`, Observability) calls for OpenTelemetry traces.

## Decision

- `internal/platform/otel` owns tracing. `Setup` installs W3C trace-context propagation and, only
  when `OTEL_EXPORTER_OTLP_ENDPOINT` (or `…_TRACES_ENDPOINT`) is set, an SDK tracer provider that
  batches spans to an OTLP/HTTP exporter. All other settings use the standard `OTEL_*` variables.
  Without an endpoint the global provider stays the no-op one, so instrumentation costs nothing.
- **HTTP**: `Middleware` (otelhttp) opens a server span per request, continuing an incoming
  `traceparent`; probes (`/healthz`, `/readyz`, `/metrics`) are not traced. `Routed` records the
  innermost `ServeMux` pattern, so spans are named like `GET /api/v1/issues/{issueKey}` (low
  cardinality) and carry `http.route`.
- **PostgreSQL**: `QueryTracer` is the pgx tracer of every pool opened with `db.Open`: a client span
  per statement with `db.query.text` (parameterized SQL, so no values), errors recorded except
  `pgx.ErrNoRows`.
- **Outbox**: the relay opens a consumer span per event (`outbox <event name>`); handler errors
  are recorded on it.
- **Logs**: `LogHandler` adds `trace_id` and `span_id` to records logged inside a span, joining logs
  and traces.
- Domain and application layers stay free of OpenTelemetry (ADR-0005): spans come from the
  platform edges (HTTP, SQL, outbox) and travel in `context.Context`.

## Consequences

- Operators enable tracing with environment variables only; no build flags.
- Metrics stay on the existing Prometheus `/metrics`; moving them to OpenTelemetry metrics is a
  later decision.
- Outbox spans continue the trace of the request that recorded the event, through the trace
  context in the event envelope (ADR-0007).
