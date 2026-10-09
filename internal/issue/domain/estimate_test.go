package domain_test

import (
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func TestParsePoints(t *testing.T) {
	tests := []struct {
		in      float64
		want    domain.Points
		wantErr bool
	}{
		{0, 0, false}, {1, 10, false}, {2.5, 25, false}, {13, 130, false}, {999, 9990, false},
		{-1, 0, true}, {999.1, 0, true}, {1.25, 0, true},
	}
	for _, tt := range tests {
		got, err := domain.ParsePoints(tt.in)
		if (err != nil) != tt.wantErr || (err == nil && got != tt.want) {
			t.Errorf("ParsePoints(%v) = %v, %v", tt.in, got, err)
		}
		if err != nil && !errors.Is(err, domain.ErrInvalidEstimate) {
			t.Errorf("err = %v", err)
		}
	}
	if domain.Points(25).Float() != 2.5 {
		t.Fatal("Float")
	}
}

func TestEstimate(t *testing.T) {
	is := newIssue(t)
	is.PullEvents()
	if _, ok := is.Estimate(); ok {
		t.Fatal("new issue must be unestimated")
	}
	p := domain.Points(30)
	is.SetEstimate(&p)
	is.SetEstimate(&p) // no-op
	is.SetEstimate(nil)
	ev := is.PullEvents()
	if len(ev) != 2 {
		t.Fatalf("events = %+v", ev)
	}
	a, b := ev[0].(domain.IssueEstimated), ev[1].(domain.IssueEstimated)
	if a.From != nil || a.To == nil || *a.To != 3 || b.From == nil || *b.From != 3 || b.To != nil {
		t.Fatalf("events = %+v %+v", a, b)
	}
	if a.EventName() != "issue.estimated" {
		t.Fatal(a.EventName())
	}
	r := domain.Rehydrate(domain.Snapshot{Estimate: &p})
	if got, ok := r.Estimate(); !ok || got != 30 {
		t.Fatalf("rehydrated estimate = %v %v", got, ok)
	}
}
