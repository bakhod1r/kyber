// Package blobtest is the contract every blob store must pass.
package blobtest

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/attachment/application"
	"github.com/bakhod1r/kyber/internal/attachment/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

const (
	ID             domain.ID = "93000000-0000-4000-8000-000000000001"
	OtherWorkspace           = "00000000-0000-4000-8000-0000000000ae"
)

func Run(t *testing.T, b application.Blobs) {
	ctx := tenant.With(context.Background(), tenant.Default)
	n, sum, err := b.Put(ctx, ID, strings.NewReader("hello"))
	if err != nil || n != 5 || sum != "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Fatalf("put = %d %s %v", n, sum, err)
	}
	r, err := b.Open(ctx, ID)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(r)
	r.Close()
	if string(data) != "hello" {
		t.Fatalf("read %q", data)
	}
	if _, err := b.Open(tenant.With(context.Background(), OtherWorkspace), ID); !errors.Is(err, domain.ErrAttachmentNotFound) {
		t.Fatalf("other workspace: %v", err)
	}
	if err := b.Delete(ctx, ID); err != nil {
		t.Fatal(err)
	}
	if err := b.Delete(ctx, ID); err != nil {
		t.Fatalf("delete twice: %v", err)
	}
	if _, err := b.Open(ctx, ID); !errors.Is(err, domain.ErrAttachmentNotFound) {
		t.Fatalf("deleted: %v", err)
	}
	if _, _, err := b.Put(ctx, ID, iotestErr{}); err == nil {
		t.Fatal("put swallowed a read error")
	}
}

type iotestErr struct{}

func (iotestErr) Read([]byte) (int, error) { return 0, errors.New("broken upload") }
