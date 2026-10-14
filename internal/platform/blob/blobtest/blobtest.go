// Package blobtest is the contract every blob.Store adapter must pass.
package blobtest

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/blob"
)

// Run checks put, open, delete and key validation.
func Run(t *testing.T, s blob.Store) {
	ctx := context.Background()
	if err := s.Put(ctx, "k-1", strings.NewReader("hello")); err != nil {
		t.Fatal(err)
	}
	r, err := s.Open(ctx, "k-1")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r)
	r.Close()
	if string(b) != "hello" {
		t.Fatalf("read %q", b)
	}
	if err := s.Delete(ctx, "k-1"); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"k-1", "../etc/passwd", ""} {
		if _, err := s.Open(ctx, key); !errors.Is(err, blob.ErrNotFound) {
			t.Fatalf("open %q = %v", key, err)
		}
	}
	if err := s.Put(ctx, "../escape", strings.NewReader("x")); !errors.Is(err, blob.ErrInvalidKey) {
		t.Fatalf("path traversal = %v", err)
	}
	if err := s.Delete(ctx, "missing"); err != nil {
		t.Fatalf("delete missing = %v (idempotent)", err)
	}
}
