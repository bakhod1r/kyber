package domain_test

import (
	"errors"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func TestRankBetweenBasics(t *testing.T) {
	first := domain.RankBetween("", "")
	if first == "" {
		t.Fatal("first rank must not be empty")
	}
	after := domain.RankBetween(first, "")
	before := domain.RankBetween("", first)
	if !(before < first && first < after) {
		t.Fatalf("order: %q %q %q", before, first, after)
	}
	mid := domain.RankBetween(first, after)
	if !(first < mid && mid < after) {
		t.Fatalf("mid %q not between %q and %q", mid, first, after)
	}
}

// Property: inserting at random positions always yields strictly increasing,
// valid ranks without renumbering existing ones.
func TestRankRandomInsertsStayOrdered(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	ranks := []domain.Rank{}
	for i := range 3000 {
		pos := r.IntN(len(ranks) + 1)
		var lo, hi domain.Rank
		if pos > 0 {
			lo = ranks[pos-1]
		}
		if pos < len(ranks) {
			hi = ranks[pos]
		}
		n := domain.RankBetween(lo, hi)
		if (lo != "" && !(lo < n)) || (hi != "" && !(n < hi)) {
			t.Fatalf("insert %d: %q not between %q and %q", i, n, lo, hi)
		}
		if _, err := domain.ParseRank(string(n)); err != nil {
			t.Fatalf("generated invalid rank %q: %v", n, err)
		}
		ranks = slices.Insert(ranks, pos, n)
	}
	if !slices.IsSorted(ranks) {
		t.Fatal("ranks not sorted")
	}
}

// Appending to the bottom (the common case for new issues) keeps ranks short.
func TestRankAppendStaysShort(t *testing.T) {
	var last domain.Rank
	for range 10_000 {
		last = domain.RankBetween(last, "")
	}
	if len(last) > 6 {
		t.Fatalf("after 10k appends rank is %d chars: %q", len(last), last)
	}
}

func TestParseRank(t *testing.T) {
	for _, bad := range []string{"", "a", "A", "0", "a00", "a-", "a 1", "A00000000000000000000000000"} {
		if _, err := domain.ParseRank(bad); !errors.Is(err, domain.ErrInvalidRank) {
			t.Errorf("ParseRank(%q) err = %v", bad, err)
		}
	}
	for _, ok := range []string{"a0", "a0V", "b12", "Zz"} {
		if r, err := domain.ParseRank(ok); err != nil || string(r) != ok {
			t.Errorf("ParseRank(%q) = %q, %v", ok, r, err)
		}
	}
}

// Prepending repeatedly (moving issues to the top) crosses into negative
// integers and must stay ordered and short.
func TestRankPrependStaysOrdered(t *testing.T) {
	first := domain.RankBetween("", "")
	for i := range 10_000 {
		n := domain.RankBetween("", first)
		if !(n < first) {
			t.Fatalf("prepend %d: %q !< %q", i, n, first)
		}
		if _, err := domain.ParseRank(string(n)); err != nil {
			t.Fatalf("invalid %q: %v", n, err)
		}
		first = n
	}
	if len(first) > 6 {
		t.Fatalf("after 10k prepends rank is %d chars: %q", len(first), first)
	}
}

// Dense inserts between two adjacent ranks keep working (fraction grows).
func TestRankDenseBetween(t *testing.T) {
	lo, hi := domain.Rank("a0"), domain.Rank("a1")
	for i := range 200 {
		m := domain.RankBetween(lo, hi)
		if !(lo < m && m < hi) {
			t.Fatalf("step %d: %q not in (%q, %q)", i, m, lo, hi)
		}
		if i%2 == 0 {
			lo = m
		} else {
			hi = m
		}
	}
}
