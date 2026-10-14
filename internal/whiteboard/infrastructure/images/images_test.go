package images_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/blob"
	"github.com/bakhod1r/kyber/internal/whiteboard/domain"
	"github.com/bakhod1r/kyber/internal/whiteboard/infrastructure/images"
)

func TestStore(t *testing.T) {
	ctx := context.Background()
	s := images.New(blob.NewMemory())
	if err := s.Put(ctx, "wb_b_i", strings.NewReader("png")); err != nil {
		t.Fatal(err)
	}
	r, err := s.Open(ctx, "wb_b_i")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r)
	if string(b) != "png" {
		t.Fatalf("read %q", b)
	}
	if _, err := s.Open(ctx, "wb_b_missing"); !errors.Is(err, domain.ErrImageMissing) {
		t.Fatalf("missing = %v", err)
	}
}
