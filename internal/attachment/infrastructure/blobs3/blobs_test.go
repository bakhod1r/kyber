package blobs3_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"

	"github.com/bakhod1r/kyber/internal/attachment/domain"
	"github.com/bakhod1r/kyber/internal/attachment/infrastructure/blobs3"
	"github.com/bakhod1r/kyber/internal/attachment/infrastructure/blobtest"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// fakeS3 serves an in-memory S3 API with one bucket, "kyber".
func fakeS3(t *testing.T) blobs3.Config {
	t.Helper()
	backend := s3mem.New()
	if err := backend.CreateBucket("kyber"); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(gofakes3.New(backend).Server())
	t.Cleanup(srv.Close)
	return blobs3.Config{Endpoint: strings.TrimPrefix(srv.URL, "http://"), Bucket: "kyber", Region: "us-east-1", AccessKey: "k", SecretKey: "s"}
}

func TestContract(t *testing.T) {
	b, err := blobs3.New(context.Background(), fakeS3(t))
	if err != nil {
		t.Fatal(err)
	}
	blobtest.Run(t, b)
}

func TestLargeUploadStreamsInParts(t *testing.T) {
	b, err := blobs3.New(context.Background(), fakeS3(t))
	if err != nil {
		t.Fatal(err)
	}
	ctx := tenant.With(context.Background(), tenant.Default)
	n, _, err := b.Put(ctx, blobtest.ID, strings.NewReader(strings.Repeat("x", 12<<20)))
	if err != nil || n != 12<<20 {
		t.Fatalf("put = %d, %v", n, err)
	}
}

func TestRejectsUnsafeIDs(t *testing.T) {
	b, _ := blobs3.New(context.Background(), fakeS3(t))
	ctx := tenant.With(context.Background(), tenant.Default)
	if _, _, err := b.Put(ctx, "../x", strings.NewReader("x")); !errors.Is(err, domain.ErrAttachmentNotFound) {
		t.Errorf("put: %v", err)
	}
	if _, err := b.Open(ctx, "../x"); !errors.Is(err, domain.ErrAttachmentNotFound) {
		t.Errorf("open: %v", err)
	}
	if err := b.Delete(ctx, "../x"); !errors.Is(err, domain.ErrAttachmentNotFound) {
		t.Errorf("delete: %v", err)
	}
}

func TestConfigErrors(t *testing.T) {
	ctx := context.Background()
	c := fakeS3(t)
	missing := c
	missing.Bucket = "nope"
	if _, err := blobs3.New(ctx, missing); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("missing bucket: %v", err)
	}
	if _, err := blobs3.New(ctx, blobs3.Config{Endpoint: "bad endpoint/", Bucket: "kyber"}); err == nil {
		t.Fatal("bad endpoint accepted")
	}
	down := c
	down.Endpoint = "127.0.0.1:1"
	if _, err := blobs3.New(ctx, down); err == nil {
		t.Fatal("unreachable endpoint accepted")
	}
}
