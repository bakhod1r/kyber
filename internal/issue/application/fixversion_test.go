package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/application"
	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func TestFixVersions(t *testing.T) {
	ctx := context.Background()
	s := seed(t, 3)
	edit := func(key string, versions ...string) error {
		is, _ := s.Get(ctx, dev, key)
		_, err := s.Edit(ctx, dev, key, application.EditIssue{Version: is.Version(), FixVersions: &versions})
		return err
	}
	if err := edit("KYB-1", "v-1", "v-2"); err != nil {
		t.Fatal(err)
	}
	if err := edit("KYB-2", "v-1"); err != nil {
		t.Fatal(err)
	}
	if err := edit("KYB-3", "v-9"); !errors.Is(err, application.ErrInvalidVersion) {
		t.Fatalf("unknown version: %v", err)
	}
	if got := order(t, s, application.ListQuery{FixVersion: "v-1"}); got != "KYB-1,KYB-2" {
		t.Fatalf("in v-1 = %s", got)
	}
	in, err := s.InVersion(ctx, dev, "KYB", "v-2")
	if err != nil || len(in) != 1 || in[0].Key().String() != "KYB-1" {
		t.Fatalf("InVersion = %v %v", in, err)
	}

	// Releasing moves only unresolved issues; deleting moves all.
	_, _ = s.Transition(ctx, dev, "KYB-2", "done", "")
	n, err := s.MoveFixVersion(ctx, dev, "KYB", "v-1", "v-2", true)
	if err != nil || n != 1 {
		t.Fatalf("moved %d %v", n, err)
	}
	if got := order(t, s, application.ListQuery{FixVersion: "v-1"}); got != "KYB-2" {
		t.Fatalf("resolved issue stays: %s", got)
	}
	if n, _ := s.MoveFixVersion(ctx, dev, "KYB", "v-1", "", false); n != 1 {
		t.Fatalf("removed %d", n)
	}
	if got := order(t, s, application.ListQuery{FixVersion: "v-1"}); got != "" {
		t.Fatalf("v-1 still has %s", got)
	}
	is, _ := s.Get(ctx, dev, "KYB-1")
	if fv := is.FixVersions(); len(fv) != 1 || fv[0] != domain.VersionID("v-2") {
		t.Fatalf("KYB-1 versions = %v", fv)
	}
}
