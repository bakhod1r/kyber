package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/admin/application"
	"github.com/bakhod1r/kyber/internal/admin/domain"
)

var errDown = errors.New("db down")

type fakes struct {
	admin   map[string]bool
	gateErr error
	roles   map[string]string
	retried []int64
}

func (f *fakes) IsAdmin(_ context.Context, actor string) (bool, error) {
	return f.admin[actor], f.gateErr
}
func (f *fakes) Members(context.Context) ([]domain.Member, error) {
	return []domain.Member{{ID: "u-1", Name: "Ann", Role: "owner"}}, nil
}
func (f *fakes) ChangeRole(_ context.Context, user, role string) error {
	f.roles[user] = role
	return nil
}
func (f *fakes) Projects(context.Context) ([]domain.Project, error) {
	return []domain.Project{{Key: "KYB", Members: 3}}, nil
}
func (f *fakes) DeadLetters(context.Context) ([]domain.DeadLetter, error) {
	return []domain.DeadLetter{{ID: 7, Event: "issue.created"}}, nil
}
func (f *fakes) Retry(_ context.Context, id int64) error {
	f.retried = append(f.retried, id)
	return nil
}

func TestAdministration(t *testing.T) {
	ctx := context.Background()
	f := &fakes{admin: map[string]bool{"boss": true}, roles: map[string]string{}}
	s := application.NewService(application.Deps{Gate: f, Directory: f, Projects: f, DeadLetters: f})

	if ok, err := s.Access(ctx, "boss"); !ok || err != nil {
		t.Fatalf("boss access = %v %v", ok, err)
	}
	if ok, _ := s.Access(ctx, "dev"); ok {
		t.Fatal("dev is not an administrator")
	}
	if ms, err := s.Members(ctx, "boss"); err != nil || len(ms) != 1 {
		t.Fatalf("members = %v %v", ms, err)
	}
	if err := s.ChangeRole(ctx, "boss", "u-2", "admin"); err != nil || f.roles["u-2"] != "admin" {
		t.Fatalf("change role: %v %v", err, f.roles)
	}
	if ps, err := s.Projects(ctx, "boss"); err != nil || ps[0].Key != "KYB" {
		t.Fatalf("projects = %v %v", ps, err)
	}
	if dl, err := s.DeadLetters(ctx, "boss"); err != nil || dl[0].ID != 7 {
		t.Fatalf("dead letters = %v %v", dl, err)
	}
	if err := s.Retry(ctx, "boss", 7); err != nil || f.retried[0] != 7 {
		t.Fatalf("retry: %v %v", err, f.retried)
	}

	// Everything is for administrators only.
	calls := map[string]error{}
	_, calls["members"] = s.Members(ctx, "dev")
	calls["role"] = s.ChangeRole(ctx, "dev", "u-2", "owner")
	_, calls["projects"] = s.Projects(ctx, "dev")
	_, calls["dead"] = s.DeadLetters(ctx, "dev")
	calls["retry"] = s.Retry(ctx, "dev", 7)
	for name, err := range calls {
		if !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("%s: %v", name, err)
		}
	}
	f.gateErr = errDown
	if _, err := s.Members(ctx, "boss"); !errors.Is(err, errDown) {
		t.Fatalf("gate failure: %v", err)
	}
	if ok, err := s.Access(ctx, "boss"); ok || !errors.Is(err, errDown) {
		t.Fatalf("access on failure = %v %v", ok, err)
	}
}
