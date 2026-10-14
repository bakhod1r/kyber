package application_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/blob"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
	app "github.com/bakhod1r/kyber/internal/whiteboard/application"
	"github.com/bakhod1r/kyber/internal/whiteboard/domain"
	"github.com/bakhod1r/kyber/internal/whiteboard/infrastructure/images"
	"github.com/bakhod1r/kyber/internal/whiteboard/infrastructure/memory"
)

// S96: members upload pictures to a board; only members read them back; only real images are kept.
func TestBoardImages(t *testing.T) {
	n := 0
	blobs := images.New(blob.NewMemory())
	svc := app.NewService(app.Deps{Boards: memory.NewRepository(), Conversations: convs{"dev": {"lead", "dev"}}, Images: blobs,
		NewID: func() string { n++; return fmt.Sprintf("00000000-0000-4000-8000-%012d", n) }, Now: func() time.Time { return time.Unix(0, 0) }})
	ctx := tenant.With(context.Background(), "w1")
	b, _ := svc.Create(ctx, "lead", "dev", "Arch")
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 100)...)

	id, err := svc.UploadImage(ctx, "dev", string(b.ID), bytes.NewReader(png))
	if err != nil || id == "" {
		t.Fatalf("upload = %q %v", id, err)
	}
	r, typ, err := svc.Image(ctx, "lead", string(b.ID), id)
	if err != nil || typ != "image/png" {
		t.Fatalf("read = %q %v", typ, err)
	}
	got, _ := io.ReadAll(r)
	r.Close()
	if !bytes.Equal(got, png) {
		t.Fatal("content changed")
	}
	if _, _, err := svc.Image(ctx, "eve", string(b.ID), id); !errors.Is(err, domain.ErrWhiteboardMissing) {
		t.Fatalf("outsider reads: %v", err)
	}
	if _, err := svc.UploadImage(ctx, "eve", string(b.ID), bytes.NewReader(png)); !errors.Is(err, domain.ErrWhiteboardMissing) {
		t.Fatalf("outsider uploads: %v", err)
	}
	if _, err := svc.UploadImage(ctx, "dev", string(b.ID), strings.NewReader("<svg onload='x'/>")); !errors.Is(err, domain.ErrNotImage) {
		t.Fatalf("svg: %v", err)
	}
	huge := io.MultiReader(bytes.NewReader(png), bytes.NewReader(make([]byte, domain.MaxImage)))
	if _, err := svc.UploadImage(ctx, "dev", string(b.ID), huge); !errors.Is(err, domain.ErrNotImage) {
		t.Fatalf("too big: %v", err)
	}
	if _, _, err := svc.Image(ctx, "dev", string(b.ID), "../x"); !errors.Is(err, domain.ErrImageMissing) {
		t.Fatalf("bad id: %v", err)
	}
	if _, _, err := svc.Image(ctx, "dev", string(b.ID), "00000000-0000-4000-8000-000000000999"); !errors.Is(err, domain.ErrImageMissing) {
		t.Fatalf("missing: %v", err)
	}
	// Without a store, images are off.
	plain := app.NewService(app.Deps{Boards: memory.NewRepository(), Conversations: convs{"dev": {"dev"}}, NewID: func() string { return "x" }, Now: time.Now})
	if _, err := plain.UploadImage(ctx, "dev", "x", bytes.NewReader(png)); !errors.Is(err, app.ErrNoImages) {
		t.Fatalf("no store: %v", err)
	}
}
