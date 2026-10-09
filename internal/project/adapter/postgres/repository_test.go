package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
	"github.com/bakhod1r/kyber/internal/project/adapter/postgres"
	"github.com/bakhod1r/kyber/internal/project/app"
	"github.com/bakhod1r/kyber/internal/project/domain"
)

func TestRepository(t *testing.T) {
	ctx := context.Background()
	repo := postgres.NewRepository(dbtest.New(t))
	ids := []string{"20000000-0000-4000-8000-000000000001", "20000000-0000-4000-8000-000000000002", "20000000-0000-4000-8000-000000000003"}
	i := 0
	svc := app.NewService(repo, func() string { i++; return ids[i-1] })

	if _, err := svc.Create(ctx, "ZED", "Zed"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, "KYB", "Kyber"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, "KYB", "Dup"); !errors.Is(err, domain.ErrKeyTaken) {
		t.Fatalf("dup err = %v", err)
	}
	p, err := svc.Get(ctx, "KYB")
	if err != nil || p.Name() != "Kyber" || string(p.ID()) != ids[1] {
		t.Fatalf("Get = %+v, %v", p, err)
	}
	if _, err := svc.Get(ctx, "NOPE"); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("err = %v", err)
	}
	ps, _ := svc.List(ctx)
	if len(ps) != 2 || ps[0].Key() != "KYB" {
		t.Fatalf("List = %v", ps)
	}

	// S7 AC2: concurrent allocation yields unique sequential numbers (row lock).
	const n = 40
	got := make(chan int, n)
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := svc.NextIssueNumber(ctx, "KYB")
			if err != nil {
				t.Error(err)
			}
			got <- v
		}()
	}
	wg.Wait()
	close(got)
	seen := map[int]bool{}
	for v := range got {
		seen[v] = true
	}
	if len(seen) != n || !seen[1] || !seen[n] {
		t.Fatalf("numbers = %v", seen)
	}
	if _, err := svc.NextIssueNumber(ctx, "NOPE"); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("err = %v", err)
	}
	// Persisted sequence survives reload (restart).
	p, _ = svc.Get(ctx, "KYB")
	if v, _ := svc.NextIssueNumber(ctx, "KYB"); v != n+1 {
		t.Fatalf("next after reload = %d", v)
	}
}
