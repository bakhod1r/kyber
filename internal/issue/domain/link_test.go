package domain_test

import (
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func TestNewLink(t *testing.T) {
	a := mkIssue(t, "a", "KYB", 1, domain.TypeTask)
	b := mkIssue(t, "b", "OPS", 2, domain.TypeBug) // links may cross projects (same workspace)
	l, err := domain.NewLink("l-1", domain.LinkBlocks, a, b, "u-1")
	if err != nil {
		t.Fatal(err)
	}
	if l.ID != "l-1" || l.From != "a" || l.To != "b" || l.FromKey.String() != "KYB-1" || l.ToKey.String() != "OPS-2" {
		t.Fatalf("link = %+v", l)
	}
	ev := l.Linked()
	if ev.EventName() != "issue.linked" || ev.By != "u-1" || ev.Type != domain.LinkBlocks {
		t.Fatalf("event = %+v", ev)
	}
	if un := l.Unlinked("u-2"); un.EventName() != "issue.unlinked" || un.By != "u-2" || un.ToKey != l.ToKey {
		t.Fatalf("unlinked = %+v", un)
	}
	if _, err := domain.NewLink("l", domain.LinkBlocks, a, a, "u"); !errors.Is(err, domain.ErrInvalidLink) {
		t.Fatalf("self link err = %v", err)
	}
	if _, err := domain.NewLink("l", "causes", a, b, "u"); !errors.Is(err, domain.ErrInvalidLink) {
		t.Fatalf("unknown type err = %v", err)
	}
}

// Each link reads differently from its two ends, as in Jira.
func TestLinkPhrases(t *testing.T) {
	a := mkIssue(t, "a", "KYB", 1, domain.TypeTask)
	b := mkIssue(t, "b", "KYB", 2, domain.TypeTask)
	cases := map[domain.LinkType][2]string{
		domain.LinkBlocks:     {"blocks", "is blocked by"},
		domain.LinkRelates:    {"relates to", "relates to"},
		domain.LinkDuplicates: {"duplicates", "is duplicated by"},
		domain.LinkClones:     {"clones", "is cloned by"},
	}
	for typ, want := range cases {
		l, _ := domain.NewLink("l", typ, a, b, "u")
		if l.PhraseFrom("a") != want[0] || l.PhraseFrom("b") != want[1] {
			t.Errorf("%s: %q / %q", typ, l.PhraseFrom("a"), l.PhraseFrom("b"))
		}
		if l.Other("a") != "b" || l.Other("b") != "a" || l.OtherKey("a").String() != "KYB-2" || l.OtherKey("b").String() != "KYB-1" {
			t.Errorf("%s: other end wrong", typ)
		}
	}
	if len(domain.LinkTypes()) != 4 {
		t.Fatal("link types")
	}
	// "relates to" has no direction: a→b and b→a are the same link.
	r1, _ := domain.NewLink("1", domain.LinkRelates, a, b, "u")
	r2, _ := domain.NewLink("2", domain.LinkRelates, b, a, "u")
	if !r1.SameAs(r2) {
		t.Fatal("relates is symmetric")
	}
	k1, _ := domain.NewLink("1", domain.LinkBlocks, a, b, "u")
	k2, _ := domain.NewLink("2", domain.LinkBlocks, b, a, "u")
	k3, _ := domain.NewLink("3", domain.LinkBlocks, a, b, "u")
	if k1.SameAs(k2) || !k1.SameAs(k3) || k1.SameAs(r1) {
		t.Fatal("directional links")
	}
}

func TestIssueKeyJSONRoundTrip(t *testing.T) {
	key, _ := domain.NewIssueKey("KYB", 12)
	raw, _ := key.MarshalText()
	var back domain.IssueKey
	if err := back.UnmarshalText(raw); err != nil || back != key {
		t.Fatalf("round trip = %v, %v", back, err)
	}
	if err := back.UnmarshalText([]byte("nope")); err == nil {
		t.Fatal("invalid key must fail")
	}
}
