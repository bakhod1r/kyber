// Package application holds the Announcements context's use cases: publishing news to a project or
// the whole workspace, and reading it.
package application

import (
	"context"
	"errors"
	"time"

	"github.com/bakhod1r/kyber/internal/announcement/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrForbidden       = errors.New("only admins may publish announcements here")
)

// Access checks the actor's rights; project "" is the whole workspace (ACL to Project/Workspace).
type Access interface {
	// Read: browse the project, or belong to the workspace.
	Read(ctx context.Context, actor, project string) error
	// Publish: administer the project, or the workspace.
	Publish(ctx context.Context, actor, project string) error
}

type Deps struct {
	Access        Access
	Announcements domain.Repository
	NewID         func() string
	Now           func() time.Time
}

type Service struct{ d Deps }

func NewService(d Deps) *Service { return &Service{d: d} }

// FeedSize bounds a list of announcements.
const FeedSize = 50

// Publish posts an announcement; its members are notified through the Published event.
func (s *Service) Publish(ctx context.Context, actor, project, title, body string) (*domain.Announcement, error) {
	if err := s.d.Access.Publish(ctx, actor, project); err != nil {
		return nil, err
	}
	a, err := domain.Publish(domain.AnnouncementID(s.d.NewID()), tenant.Workspace(ctx), project, title, body, actor, s.d.Now().UTC())
	if err != nil {
		return nil, err
	}
	return a, s.d.Announcements.Save(ctx, a)
}

// List returns the newest announcements of a project ("" = the workspace's own).
func (s *Service) List(ctx context.Context, actor, project string) ([]*domain.Announcement, error) {
	if err := s.d.Access.Read(ctx, actor, project); err != nil {
		return nil, err
	}
	return s.d.Announcements.For(ctx, project, FeedSize)
}
