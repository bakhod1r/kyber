package accessacl_test

import (
	"context"
	"errors"
	"testing"

	accessapp "github.com/bakhod1r/kyber/internal/access/app"
	accessdomain "github.com/bakhod1r/kyber/internal/access/domain"
	"github.com/bakhod1r/kyber/internal/issue/adapter/accessacl"
	issueapp "github.com/bakhod1r/kyber/internal/issue/app"
	"github.com/bakhod1r/kyber/internal/issue/domain"
)

type fake struct {
	err  error
	seen *accessapp.Issue
	perm accessdomain.Permission
}

func (f *fake) Can(_ context.Context, _, _ string, perm accessdomain.Permission, on *accessapp.Issue) error {
	f.seen, f.perm = on, perm
	return f.err
}

func TestTranslates(t *testing.T) {
	key, _ := domain.NewIssueKey("KYB", 1)
	is := domain.Rehydrate(domain.Snapshot{ID: "i", Key: key, Reporter: "r", Assignee: "a", Status: "done"})
	boom := errors.New("boom")
	for in, want := range map[error]error{nil: nil, accessapp.ErrNotFound: issueapp.ErrProjectNotFound, accessapp.ErrForbidden: issueapp.ErrForbidden, boom: boom} {
		f := &fake{err: in}
		err := accessacl.New(f).Can(context.Background(), "u", "KYB", issueapp.PermEdit, is)
		if !errors.Is(err, want) || (want == nil) != (err == nil) {
			t.Errorf("%v → %v, want %v", in, err, want)
		}
		if f.perm != accessdomain.EditIssues || *f.seen != (accessapp.Issue{Reporter: "r", Assignee: "a", Status: "done"}) {
			t.Errorf("passed %s %+v", f.perm, f.seen)
		}
	}
	f := &fake{}
	_ = accessacl.New(f).Can(context.Background(), "u", "KYB", issueapp.PermBrowse, nil)
	if f.seen != nil {
		t.Fatal("project-level checks pass no issue")
	}
}
