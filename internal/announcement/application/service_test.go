package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/announcement/application"
	"github.com/bakhod1r/kyber/internal/announcement/domain"
	"github.com/bakhod1r/kyber/internal/announcement/infrastructure/memory"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// access: "admin" publishes anywhere, "dev" reads KYB and the workspace, others see nothing.
type access struct{}

func (access) Read(_ context.Context, actor, project string) error {
	if actor == "out" || (project != "" && project != "KYB") {
		return application.ErrProjectNotFound
	}
	return nil
}

func (a access) Publish(ctx context.Context, actor, project string) error {
	if err := a.Read(ctx, actor, project); err != nil {
		return err
	}
	if actor != "admin" {
		return application.ErrForbidden
	}
	return nil
}

func TestAnnouncements(t *testing.T) {
	ctx := tenant.With(context.Background(), "w-1")
	n := 0
	repo := memory.NewRepository()
	s := application.NewService(application.Deps{Access: access{}, Announcements: repo,
		NewID: func() string { n++; return fmt.Sprintf("a-%d", n) },
		Now:   func() time.Time { return time.Date(2026, 10, 13, 14, 0, 0, 0, time.FixedZone("UZ", 5*3600)) }})

	a, err := s.Publish(ctx, "admin", "KYB", "Release Friday", "Deploy at 16:00")
	if err != nil || a.Workspace != "w-1" || a.At.Location() != time.UTC || len(repo.Outbox()) != 1 {
		t.Fatalf("publish = %+v %v", a, err)
	}
	if _, err := s.Publish(ctx, "admin", "", "Holiday", ""); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		actor, project, title string
		want                  error
	}{
		{"dev", "KYB", "x", application.ErrForbidden},
		{"out", "", "x", application.ErrProjectNotFound},
		{"admin", "KYB", "", domain.ErrInvalidAnnouncement},
	} {
		if _, err := s.Publish(ctx, tc.actor, tc.project, tc.title, ""); !errors.Is(err, tc.want) {
			t.Errorf("%+v: %v", tc, err)
		}
	}
	if list, err := s.List(ctx, "dev", "KYB"); err != nil || len(list) != 1 || list[0].Title != "Release Friday" {
		t.Fatalf("project feed = %v %v", list, err)
	}
	if list, err := s.List(ctx, "dev", ""); err != nil || len(list) != 1 || list[0].Title != "Holiday" {
		t.Fatalf("workspace feed = %v %v", list, err)
	}
	if _, err := s.List(ctx, "out", ""); !errors.Is(err, application.ErrProjectNotFound) {
		t.Fatalf("outsider reads: %v", err)
	}
}
