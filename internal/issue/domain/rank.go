package domain

import (
	"errors"
	"strings"
)

// ErrInvalidRank means a rank string is not a valid fractional index.
var ErrInvalidRank = errors.New("invalid rank")

// Rank orders issues in a backlog. It is a fractional index (base-62, ASCII
// ordered, compare with COLLATE "C"): a new rank can always be generated between
// any two others, so moving an issue rewrites only that issue.
//
// Format (rocicorp/fractional-indexing): an integer part whose head character
// encodes its length ('a'..'z' = 2..27 chars ascending, 'Z'..'A' descending for
// the negative side), followed by an optional fraction without trailing zeros.
type Rank string

const rankDigits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var smallestInteger = "A" + strings.Repeat("0", 26)

// ParseRank validates a persisted or client-supplied rank.
func ParseRank(s string) (Rank, error) {
	if s == "" || s == smallestInteger {
		return "", ErrInvalidRank
	}
	i, ok := integerPart(s)
	if !ok {
		return "", ErrInvalidRank
	}
	for k := 1; k < len(s); k++ {
		if strings.IndexByte(rankDigits, s[k]) < 0 {
			return "", ErrInvalidRank
		}
	}
	if f := s[len(i):]; f != "" && f[len(f)-1] == '0' {
		return "", ErrInvalidRank
	}
	return Rank(s), nil
}

// RankBetween returns a rank strictly between lo and hi; "" means unbounded.
// Callers must pass lo < hi (both valid) when both are set.
func RankBetween(lo, hi Rank) Rank {
	a, b := string(lo), string(hi)
	switch {
	case a == "" && b == "":
		return "a0"
	case a == "":
		ib, _ := integerPart(b)
		if ib == smallestInteger {
			return Rank(ib + midpoint("", b[len(ib):]))
		}
		if ib < b {
			return Rank(ib)
		}
		if d, ok := decrementInteger(ib); ok {
			return Rank(d)
		}
		return Rank(ib + midpoint("", b[len(ib):]))
	case b == "":
		ia, _ := integerPart(a)
		if i, ok := incrementInteger(ia); ok {
			return Rank(i)
		}
		return Rank(ia + midpoint(a[len(ia):], ""))
	}
	ia, _ := integerPart(a)
	ib, _ := integerPart(b)
	if ia == ib {
		return Rank(ia + midpoint(a[len(ia):], b[len(ib):]))
	}
	if i, ok := incrementInteger(ia); ok && i < b {
		return Rank(i)
	}
	return Rank(ia + midpoint(a[len(ia):], ""))
}

// midpoint returns a fraction strictly between fractions a < b ("" b = +inf).
func midpoint(a, b string) string {
	if b != "" {
		n := 0
		for n < len(b) {
			ca := byte('0')
			if n < len(a) {
				ca = a[n]
			}
			if ca != b[n] {
				break
			}
			n++
		}
		if n > 0 {
			rest := ""
			if n < len(a) {
				rest = a[n:]
			}
			return b[:n] + midpoint(rest, b[n:])
		}
	}
	da := 0
	if a != "" {
		da = strings.IndexByte(rankDigits, a[0])
	}
	db := len(rankDigits)
	if b != "" {
		db = strings.IndexByte(rankDigits, b[0])
	}
	if db-da > 1 {
		return string(rankDigits[(da+db+1)/2])
	}
	if b != "" && len(b) > 1 {
		return b[:1]
	}
	rest := ""
	if len(a) > 1 {
		rest = a[1:]
	}
	return string(rankDigits[da]) + midpoint(rest, "")
}

func integerLength(head byte) (int, bool) {
	switch {
	case head >= 'a' && head <= 'z':
		return int(head-'a') + 2, true
	case head >= 'A' && head <= 'Z':
		return int('Z'-head) + 2, true
	}
	return 0, false
}

func integerPart(s string) (string, bool) {
	n, ok := integerLength(s[0])
	if !ok || n > len(s) {
		return "", false
	}
	return s[:n], true
}

func incrementInteger(x string) (string, bool) {
	head, digs := x[0], []byte(x[1:])
	carry := true
	for i := len(digs) - 1; carry && i >= 0; i-- {
		d := strings.IndexByte(rankDigits, digs[i]) + 1
		if d == len(rankDigits) {
			digs[i] = rankDigits[0]
		} else {
			digs[i] = rankDigits[d]
			carry = false
		}
	}
	if !carry {
		return string(head) + string(digs), true
	}
	switch head {
	case 'Z':
		return "a" + string(rankDigits[0]), true
	case 'z':
		return "", false
	}
	h := head + 1
	if h > 'a' {
		digs = append(digs, rankDigits[0])
	} else {
		digs = digs[:len(digs)-1]
	}
	return string(h) + string(digs), true
}

func decrementInteger(x string) (string, bool) {
	head, digs := x[0], []byte(x[1:])
	last := rankDigits[len(rankDigits)-1]
	borrow := true
	for i := len(digs) - 1; borrow && i >= 0; i-- {
		d := strings.IndexByte(rankDigits, digs[i]) - 1
		if d == -1 {
			digs[i] = last
		} else {
			digs[i] = rankDigits[d]
			borrow = false
		}
	}
	if !borrow {
		return string(head) + string(digs), true
	}
	switch head {
	case 'a':
		return "Z" + string(last), true
	case 'A':
		return "", false
	}
	h := head - 1
	if h < 'Z' {
		digs = append(digs, last)
	} else {
		digs = digs[:len(digs)-1]
	}
	return string(h) + string(digs), true
}
