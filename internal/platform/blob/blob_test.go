package blob_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/blob"
	"github.com/bakhod1r/kyber/internal/platform/blob/blobtest"
)

func TestMemory(t *testing.T) { blobtest.Run(t, blob.NewMemory()) }

func TestDisk(t *testing.T) {
	s, err := blob.NewDisk(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	blobtest.Run(t, s)
}
