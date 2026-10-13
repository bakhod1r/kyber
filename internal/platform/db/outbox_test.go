package db_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	gotel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/db"
	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type created struct{ Key string }

func (created) EventName() string { return "thing.created" }

type bad struct{ C chan int }

func (bad) EventName() string { return "bad" }

// C6: every event is written with its envelope.
func TestWriteOutboxRecordsTheEnvelope(t *testing.T) {
	pool := dbtest.New(t)
	gotel.SetTextMapPropagator(propagation.TraceContext{})
	ctx, span := sdktrace.NewTracerProvider().Tracer("t").Start(auth.WithActor(tenant.With(context.Background(), tenant.Default), "u-1"), "req")
	defer span.End()
	if err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error { return db.WriteOutbox(ctx, tx, []created{{Key: "KYB-1"}}) }); err != nil {
		t.Fatal(err)
	}
	var name, actor, trace string
	var version int
	if err := pool.QueryRow(ctx, `SELECT name, actor_id, trace_parent, schema_version FROM outbox`).Scan(&name, &actor, &trace, &version); err != nil {
		t.Fatal(err)
	}
	if name != "thing.created" || actor != "u-1" || version != 1 || trace == "" || trace[3:35] != span.SpanContext().TraceID().String() {
		t.Fatalf("row = %s %s %q v%d", name, actor, trace, version)
	}
	if err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error { return db.WriteOutbox(ctx, tx, []bad{{}}) }); err == nil {
		t.Fatal("an unmarshalable event was written")
	}
}
