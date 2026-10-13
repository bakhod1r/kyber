package blobfs_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/attachment/domain"
	"github.com/bakhod1r/kyber/internal/attachment/infrastructure/blobfs"
	"github.com/bakhod1r/kyber/internal/attachment/infrastructure/blobtest"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

func TestContract(t *testing.T) { blobtest.Run(t, blobfs.New(t.TempDir())) }

func TestRejectsUnsafeIDs(t *testing.T) {
	b := blobfs.New(t.TempDir())
	ctx := tenant.With(context.Background(), tenant.Default)
	for _, id := range []domain.ID{"../../etc/passwd", ""} {
		if _, _, err := b.Put(ctx, id, strings.NewReader("x")); !errors.Is(err, domain.ErrAttachmentNotFound) {
			t.Errorf("put %q: %v", id, err)
		}
		if _, err := b.Open(ctx, id); !errors.Is(err, domain.ErrAttachmentNotFound) {
			t.Errorf("open %q: %v", id, err)
		}
		if err := b.Delete(ctx, id); !errors.Is(err, domain.ErrAttachmentNotFound) {
			t.Errorf("delete %q: %v", id, err)
		}
	}
	if _, err := b.Open(tenant.With(context.Background(), "../x"), blobtest.ID); !errors.Is(err, domain.ErrAttachmentNotFound) {
		t.Errorf("bad workspace: %v", err)
	}
}

func TestStorageFailures(t *testing.T) {
	ctx := tenant.With(context.Background(), tenant.Default)
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	// dir is a regular file: nothing can be created under it.
	if _, _, err := blobfs.New(file).Put(ctx, blobtest.ID, strings.NewReader("x")); err == nil {
		t.Fatal("put under a file succeeded")
	}
	dir := t.TempDir()
	ws := filepath.Join(dir, tenant.Default)
	if err := os.MkdirAll(ws, 0o500); err != nil { // read-only workspace directory
		t.Fatal(err)
	}
	if os.Geteuid() != 0 {
		if _, _, err := blobfs.New(dir).Put(ctx, blobtest.ID, strings.NewReader("x")); err == nil {
			t.Fatal("put into a read-only directory succeeded")
		}
	}
	// a directory where the blob should be: open succeeds as a dir, delete fails (not empty).
	if err := os.Chmod(ws, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(ws, string(blobtest.ID), "x"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := blobfs.New(dir).Delete(ctx, blobtest.ID); err == nil {
		t.Fatal("deleted a non-empty directory")
	}
	if _, _, err := blobfs.New(dir).Put(ctx, blobtest.ID, strings.NewReader("x")); err == nil {
		t.Fatal("renamed over a directory")
	}
}
