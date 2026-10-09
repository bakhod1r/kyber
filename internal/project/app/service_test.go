package app_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/bakhod1r/kyber/internal/project/adapter/memory"
	"github.com/bakhod1r/kyber/internal/project/app"
	"github.com/bakhod1r/kyber/internal/project/domain"
)

const (
	alice domain.UserID = "u-alice"
	bob   domain.UserID = "u-bob"
	carol domain.UserID = "u-carol"
)

// fakeDirectory is an in-memory app.UserDirectory.
type fakeDirectory map[string]app.UserInfo

func (d fakeDirectory) ByEmail(_ context.Context, email string) (app.UserInfo, error) {
	u, ok := d[email]
	if !ok {
		return app.UserInfo{}, app.ErrUnknownUser
	}
	return u, nil
}

func (d fakeDirectory) ByID(_ context.Context, id domain.UserID) (app.UserInfo, error) {
	for _, u := range d {
		if u.ID == id {
			return u, nil
		}
	}
	return app.UserInfo{}, app.ErrUnknownUser
}

func newService(t *testing.T) *app.Service {
	t.Helper()
	n := 0
	dir := fakeDirectory{
		"alice@x.uz": {ID: alice, Email: "alice@x.uz", Name: "Alice"},
		"bob@x.uz":   {ID: bob, Email: "bob@x.uz", Name: "Bob"},
		"carol@x.uz": {ID: carol, Email: "carol@x.uz", Name: "Carol"},
	}
	return app.NewService(memory.NewRepository(), dir, func() string { n++; return fmt.Sprintf("p-%d", n) })
}

func TestCreateGetList(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	if _, err := s.Create(ctx, alice, "KYB", "Kyber"); err != nil {
		t.Fatal(err)
	}
	_, _ = s.Create(ctx, bob, "OPS", "Ops")
	if _, err := s.Create(ctx, bob, "KYB", "Dup"); !errors.Is(err, domain.ErrKeyTaken) {
		t.Fatalf("dup err = %v", err)
	}
	if _, err := s.Create(ctx, bob, "x", "Bad"); !errors.Is(err, domain.ErrInvalidKey) {
		t.Fatalf("invalid err = %v", err)
	}
	if p, err := s.Get(ctx, alice, "KYB"); err != nil || p.Name() != "Kyber" {
		t.Fatalf("Get = %+v, %v", p, err)
	}
	if _, err := s.Get(ctx, bob, "KYB"); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("non-member err = %v", err)
	}
	if _, err := s.Get(ctx, alice, "NOPE"); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("missing err = %v", err)
	}
	ps, _ := s.List(ctx, alice)
	if len(ps) != 1 || ps[0].Key() != "KYB" {
		t.Fatalf("List(alice) = %v", ps)
	}
}

func TestSetMember(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	_, _ = s.Create(ctx, alice, "KYB", "Kyber")

	if err := s.SetMember(ctx, alice, "KYB", "bob@x.uz", "member"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMember(ctx, bob, "KYB", "carol@x.uz", "viewer"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("member adding err = %v", err)
	}
	if err := s.SetMember(ctx, carol, "KYB", "carol@x.uz", "admin"); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("outsider err = %v", err)
	}
	if err := s.SetMember(ctx, alice, "KYB", "ghost@x.uz", "viewer"); !errors.Is(err, app.ErrUnknownUser) {
		t.Fatalf("unknown email err = %v", err)
	}
	if err := s.SetMember(ctx, alice, "KYB", "carol@x.uz", "owner"); !errors.Is(err, domain.ErrInvalidRole) {
		t.Fatalf("bad role err = %v", err)
	}
	if err := s.SetMember(ctx, alice, "KYB", "alice@x.uz", "viewer"); !errors.Is(err, domain.ErrLastAdmin) {
		t.Fatalf("last admin err = %v", err)
	}
	if err := s.SetMember(ctx, alice, "KYB", "bob@x.uz", "viewer"); err != nil {
		t.Fatalf("role change: %v", err)
	}

	ms, err := s.Members(ctx, bob, "KYB")
	if err != nil || len(ms) != 2 {
		t.Fatalf("Members = %+v, %v", ms, err)
	}
	if ms[0].Email != "alice@x.uz" || ms[0].Role != domain.RoleAdmin || ms[1].Name != "Bob" || ms[1].Role != domain.RoleViewer {
		t.Fatalf("members = %+v", ms)
	}
	if _, err := s.Members(ctx, carol, "KYB"); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("outsider members err = %v", err)
	}
}

func TestAuthorize(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	_, _ = s.Create(ctx, alice, "KYB", "Kyber")
	_ = s.SetMember(ctx, alice, "KYB", "carol@x.uz", "viewer")
	if err := s.Authorize(ctx, carol, "KYB", domain.PermRead); err != nil {
		t.Fatal(err)
	}
	if err := s.Authorize(ctx, carol, "KYB", domain.PermWrite); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("err = %v", err)
	}
	if err := s.Authorize(ctx, bob, "KYB", domain.PermRead); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestNextIssueNumberIsSequentialUnderConcurrency(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	_, _ = s.Create(ctx, alice, "KYB", "Kyber")
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
	if len(uniq) != n || !uniq[1] || !uniq[n] {
		t.Fatalf("numbers = %v", uniq)
	}
	if _, err := s.NextIssueNumber(ctx, "NOPE"); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("err = %v", err)
	}
}
