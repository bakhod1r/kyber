package projectcompat_test

import (
	"context"
	"errors"
	"testing"

	accessapp "github.com/bakhod1r/kyber/internal/access/application"
	accessdomain "github.com/bakhod1r/kyber/internal/access/domain"
	"github.com/bakhod1r/kyber/internal/access/infrastructure/projectcompat"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
)

type fake struct {
	err  error
	perm accessdomain.Permission
	who  string
}

func (f *fake) Can(_ context.Context, actor, _ string, perm accessdomain.Permission, on *accessapp.Issue) error {
	f.perm, f.who = perm, actor
	return f.err
}

func TestCheck(t *testing.T) {
	boom := errors.New("boom")
	for in, want := range map[error]error{
		nil: nil, accessapp.ErrNotFound: projectdomain.ErrProjectNotFound, accessapp.ErrForbidden: projectdomain.ErrForbidden, boom: boom,
	} {
		f := &fake{err: in}
		err := projectcompat.Check(context.Background(), f, "u-1", "KYB", accessdomain.ManageSprints)
		if !errors.Is(err, want) || (want == nil) != (err == nil) {
			t.Errorf("%v → %v, want %v", in, err, want)
		}
		if f.perm != accessdomain.ManageSprints || f.who != "u-1" {
			t.Errorf("asked %s for %s", f.perm, f.who)
		}
	}
}

func TestGate(t *testing.T) {
	f := &fake{}
	g := projectcompat.NewGate(nil, f, accessdomain.ManageSprints)
	for need, want := range map[projectdomain.Permission]accessdomain.Permission{
		projectdomain.PermRead:  accessdomain.BrowseProjects,
		projectdomain.PermWrite: accessdomain.ManageSprints,
		projectdomain.PermAdmin: accessdomain.AdministerProjects,
	} {
		if err := g.Authorize(context.Background(), "u-1", "KYB", need); err != nil || f.perm != want {
			t.Errorf("need %v → %s (%v), want %s", need, f.perm, err, want)
		}
	}
}
