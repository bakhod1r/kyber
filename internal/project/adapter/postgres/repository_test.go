package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
	"github.com/bakhod1r/kyber/internal/project/adapter/postgres"
	"github.com/bakhod1r/kyber/internal/project/domain"
)

const (
	alice domain.UserID = "30000000-0000-4000-8000-00000000000a"
	bob   domain.UserID = "30000000-0000-4000-8000-00000000000b"
)

func TestRepository(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.New(t)
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, name, password_hash) VALUES
		($1, 'a@x.uz', 'A', 'h'), ($2, 'b@x.uz', 'B', 'h')`, string(alice), string(bob)); err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewRepository(pool)

	kyb, _ := domain.NewProject("20000000-0000-4000-8000-000000000001", domain.DefaultWorkspace, "KYB", "Kyber", alice)
	zed, _ := domain.NewProject("20000000-0000-4000-8000-000000000002", domain.DefaultWorkspace, "ZED", "Zed", bob)
	for _, p := range []*domain.Project{kyb, zed} {
		if err := repo.Create(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	dup, _ := domain.NewProject("20000000-0000-4000-8000-000000000003", domain.DefaultWorkspace, "KYB", "Dup", bob)
	if err := repo.Create(ctx, dup); !errors.Is(err, domain.ErrKeyTaken) {
		t.Fatalf("dup err = %v", err)
	}

	got, err := repo.ByKey(ctx, "KYB")
	if err != nil || got.Name() != "Kyber" || got.ID() != kyb.ID() {
		t.Fatalf("ByKey = %+v, %v", got, err)
	}
	if role, ok := got.RoleOf(alice); !ok || role != domain.RoleAdmin {
		t.Fatalf("creator role = %q %v", role, ok)
	}
	if _, err := repo.ByKey(ctx, "NOPE"); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("err = %v", err)
	}

	// Membership changes persist through Update.
	if err := repo.Update(ctx, "KYB", func(p *domain.Project) error { return p.SetMember(bob, domain.RoleViewer) }); err != nil {
		t.Fatal(err)
	}
	if ps, _ := repo.ListForUser(ctx, bob); len(ps) != 2 || ps[0].Key() != "KYB" || ps[1].Key() != "ZED" {
		t.Fatalf("ListForUser(bob) = %v", ps)
	}
	if ps, _ := repo.ListForUser(ctx, alice); len(ps) != 1 {
		t.Fatalf("ListForUser(alice) = %v", ps)
	}
	// A failing mutation leaves state untouched.
	boom := errors.New("boom")
	if err := repo.Update(ctx, "KYB", func(p *domain.Project) error { _ = p.SetMember(bob, domain.RoleAdmin); return boom }); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if p, _ := repo.ByKey(ctx, "KYB"); func() domain.Role { r, _ := p.RoleOf(bob); return r }() != domain.RoleViewer {
		t.Fatal("failed update must roll back")
	}
	if err := repo.Update(ctx, "NOPE", func(*domain.Project) error { return nil }); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("err = %v", err)
	}

	// S7 AC2: concurrent allocation yields unique sequential numbers (row lock).
	const n = 40
	nums := make(chan int, n)
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var v int
			if err := repo.Update(ctx, "KYB", func(p *domain.Project) error { v = p.NextIssueNumber(); return nil }); err != nil {
				t.Error(err)
			}
			nums <- v
		}()
	}
	wg.Wait()
	close(nums)
	seen := map[int]bool{}
	for v := range nums {
		seen[v] = true
	}
	if len(seen) != n || !seen[1] || !seen[n] {
		t.Fatalf("numbers = %v", seen)
	}
	if p, _ := repo.ByKey(ctx, "KYB"); p.IssueSeq() != n {
		t.Fatalf("persisted seq = %d", p.IssueSeq())
	}
}
