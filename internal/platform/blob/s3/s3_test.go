package s3_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"

	"github.com/bakhod1r/kyber/internal/platform/blob/blobtest"
	"github.com/bakhod1r/kyber/internal/platform/blob/s3"
)

func TestContract(t *testing.T) {
	backend := s3mem.New()
	if err := backend.CreateBucket("kyber"); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(gofakes3.New(backend).Server())
	t.Cleanup(srv.Close)
	s, err := s3.New(context.Background(), s3.Config{Endpoint: strings.TrimPrefix(srv.URL, "http://"), Bucket: "kyber", Region: "us-east-1",
		AccessKey: "k", SecretKey: "s", Prefix: "chat/"})
	if err != nil {
		t.Fatal(err)
	}
	blobtest.Run(t, s)
	// Keys live under the prefix, next to other users of the bucket.
	_ = s.Put(context.Background(), "k-2", strings.NewReader("x"))
	if _, err := backend.HeadObject("kyber", "chat/k-2"); err != nil {
		t.Fatalf("object not under the prefix: %v", err)
	}
}
