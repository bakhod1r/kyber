package argon2_test

import (
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/identity/adapter/argon2"
)

func TestHashVerify(t *testing.T) {
	h := argon2.New()
	hash, err := h.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Fatalf("hash format = %q", hash)
	}
	if !h.Verify(hash, "correct horse battery") {
		t.Fatal("valid password rejected")
	}
	if h.Verify(hash, "wrong horse battery") {
		t.Fatal("invalid password accepted")
	}
	other, _ := h.Hash("correct horse battery")
	if other == hash {
		t.Fatal("salt must be random")
	}
	for _, bad := range []string{"", "plain", "$argon2id$v=19$m=x$a$b", "$argon2id$v=19$m=65536,t=1,p=2$!!$!!"} {
		if h.Verify(bad, "x") {
			t.Fatalf("malformed hash %q accepted", bad)
		}
	}
}
