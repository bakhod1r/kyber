package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/audit/application"
	"github.com/bakhod1r/kyber/internal/audit/domain"
	"github.com/bakhod1r/kyber/internal/audit/infrastructure/memory"
)

type fakeAdmins map[string]error // actor → Administer result

func (f fakeAdmins) Administer(_ context.Context, actor, _ string) error { return f[actor] }

type fakeNames map[string]string

func (f fakeNames) DisplayName(_ context.Context, id string) (string, error) {
	if id == "u-err" {
		return "", errors.New("boom")
	}
	return f[id], nil
}

type failing struct{}

func (failing) Append(context.Context, domain.Entry) error { return errors.New("disk full") }
func (failing) ForProject(context.Context, string, int) ([]domain.Entry, error) {
	return nil, errors.New("disk full")
}

var t0 = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

func svc(repo domain.Repository) *application.Service {
	return application.NewService(repo, fakeAdmins{"u-dev": application.ErrForbidden, "u-alien": application.ErrNotFound}, fakeNames{"u-lead": "Lead"}, func() time.Time { return t0 })
}

func TestRecordAndLog(t *testing.T) {
	ctx := context.Background()
	s := svc(memory.NewRepository())
	if err := s.Record(ctx, "u-lead", "PUT /api/v1/projects/{key}/workflow", "/api/v1/projects/KYB/workflow", "KYB", 200); err != nil {
		t.Fatal(err)
	}
	if err := s.Record(ctx, "", "POST /x", "/x", "KYB", 200); !errors.Is(err, domain.ErrInvalidEntry) {
		t.Fatalf("anonymous err = %v", err)
	}
	lines, err := s.Log(ctx, "u-lead", "KYB", 0)
	if err != nil || len(lines) != 1 || lines[0].ActorName != "Lead" || !lines[0].At.Equal(t0) || lines[0].Status != 200 {
		t.Fatalf("log = %+v, %v", lines, err)
	}
	for who, want := range map[string]error{"u-dev": application.ErrForbidden, "u-alien": application.ErrNotFound} {
		if _, err := s.Log(ctx, who, "KYB", 10); !errors.Is(err, want) {
			t.Errorf("%s err = %v", who, err)
		}
	}
	_ = s.Record(ctx, "u-err", "POST /x", "/x", "KYB", 201)
	if _, err := s.Log(ctx, "u-lead", "KYB", 10); err == nil {
		t.Fatal("name lookup failure must surface")
	}
}

func TestStoreFailures(t *testing.T) {
	s := svc(failing{})
	if err := s.Record(context.Background(), "u-lead", "POST /x", "/x", "KYB", 200); err == nil {
		t.Fatal("append failure must surface")
	}
	if _, err := s.Log(context.Background(), "u-lead", "KYB", 10); err == nil {
		t.Fatal("read failure must surface")
	}
}
