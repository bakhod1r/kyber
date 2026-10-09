package app_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/bakhod1r/kyber/internal/project/adapter/memory"
	"github.com/bakhod1r/kyber/internal/project/app"
	"github.com/bakhod1r/kyber/internal/project/domain"
)

func newService() *app.Service {
	n := 0
	return app.NewService(memory.NewRepository(), func() string { n++; return "p-" + string(rune('0'+n)) })
}

func TestCreateAndGet(t *testing.T) {
	ctx := context.Background()
	s := newService()
	p, err := s.Create(ctx, "KYB", "Kyber")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, "KYB")
	if err != nil || got.ID() != p.ID() || got.Name() != "Kyber" {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	if _, err := s.Get(ctx, "NOPE"); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestCreateRejectsDuplicateAndInvalid(t *testing.T) {
	ctx := context.Background()
	s := newService()
	_, _ = s.Create(ctx, "KYB", "Kyber")
	if _, err := s.Create(ctx, "KYB", "Other"); !errors.Is(err, domain.ErrKeyTaken) {
		t.Fatalf("dup err = %v", err)
	}
	if _, err := s.Create(ctx, "x", "Other"); !errors.Is(err, domain.ErrInvalidKey) {
		t.Fatalf("invalid err = %v", err)
	}
}

func TestListSortedByKey(t *testing.T) {
	ctx := context.Background()
	s := newService()
	_, _ = s.Create(ctx, "ZED", "Z")
	_, _ = s.Create(ctx, "ABC", "A")
	ps, err := s.List(ctx)
	if err != nil || len(ps) != 2 || ps[0].Key() != "ABC" || ps[1].Key() != "ZED" {
		t.Fatalf("List = %v, %v", ps, err)
	}
}

func TestNextIssueNumberIsSequentialUnderConcurrency(t *testing.T) {
	ctx := context.Background()
	s := newService()
	_, _ = s.Create(ctx, "KYB", "Kyber")

	const n = 50
	seen := make(chan int, n)
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			num, err := s.NextIssueNumber(ctx, "KYB")
			if err != nil {
				t.Error(err)
			}
			seen <- num
		}()
	}
	wg.Wait()
	close(seen)
	uniq := map[int]bool{}
	for v := range seen {
		uniq[v] = true
	}
	for i := 1; i <= n; i++ {
		if !uniq[i] {
			t.Fatalf("missing number %d", i)
		}
	}
	if _, err := s.NextIssueNumber(ctx, "NOPE"); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("err = %v", err)
	}
}
