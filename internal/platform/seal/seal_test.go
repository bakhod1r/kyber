package seal_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/seal"
)

func key(b byte) string { return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{b}, 32)) }

func TestSealAndOpen(t *testing.T) {
	box, err := seal.New(key(1))
	if err != nil {
		t.Fatal(err)
	}
	a := box.Seal("s3cret")
	b := box.Seal("s3cret")
	if bytes.Equal(a, b) || bytes.Contains(a, []byte("s3cret")) {
		t.Fatal("sealing must be randomised and opaque")
	}
	if got, err := box.Open(a); err != nil || got != "s3cret" {
		t.Fatalf("open = %q, %v", got, err)
	}
	other, _ := seal.New(key(2))
	if _, err := other.Open(a); !errors.Is(err, seal.ErrOpen) {
		t.Fatalf("wrong key: %v", err)
	}
	if _, err := box.Open([]byte{1, 2}); !errors.Is(err, seal.ErrOpen) {
		t.Fatalf("short: %v", err)
	}
	a[len(a)-1] ^= 1
	if _, err := box.Open(a); !errors.Is(err, seal.ErrOpen) {
		t.Fatalf("tampered: %v", err)
	}
}

func TestNew(t *testing.T) {
	if box, err := seal.New(""); box != nil || err != nil {
		t.Fatalf("no key = %v, %v; want nil, nil", box, err)
	}
	for _, k := range []string{"not base64!", base64.StdEncoding.EncodeToString([]byte("short"))} {
		if _, err := seal.New(k); err == nil {
			t.Errorf("%q accepted", k)
		}
	}
}
