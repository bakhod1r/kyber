package id_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/id"
)

func TestNewIsValid(t *testing.T) {
	for range 100 {
		if v := id.New(); !id.Valid(v) {
			t.Fatalf("New() = %q is not valid", v)
		}
	}
}

func TestValid(t *testing.T) {
	for _, ok := range []string{"80000000-0000-4000-8000-000000000001", "ABCDEF00-0000-4000-8000-000000000001"} {
		if !id.Valid(ok) {
			t.Errorf("Valid(%q) = false", ok)
		}
	}
	for _, bad := range []string{"", "abc", "80000000000040008000000000000001", "80000000-0000-4000-8000-00000000000g", "80000000-0000-4000-8000-0000000000011"} {
		if id.Valid(bad) {
			t.Errorf("Valid(%q) = true", bad)
		}
	}
}
