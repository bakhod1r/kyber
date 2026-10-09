package auth_test

import (
	"context"
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/auth"
)

func TestActor(t *testing.T) {
	if auth.Actor(context.Background()) != "" {
		t.Fatal("empty context must have no actor")
	}
	if got := auth.Actor(auth.WithActor(context.Background(), "u-1")); got != "u-1" {
		t.Fatalf("actor = %q", got)
	}
}
