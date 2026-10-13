// Package application holds the Attachment context's use cases: attach files to issues, list,
// download and remove them.
package application

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/bakhod1r/kyber/internal/attachment/domain"
)

var (
	ErrIssueNotFound = errors.New("issue not found")
	ErrForbidden     = errors.New("insufficient project role")
)

// Need is what the actor wants to do with an issue's attachments.
type Need int

const (
	View   Need = iota // see the issue
	Attach             // edit the issue
	Manage             // administer the project (remove anyone's files)
)

// Issues authorizes the actor on an issue and returns its canonical key (ACL to Issue Tracking).
// Hidden and missing issues give ErrIssueNotFound, missing permissions ErrForbidden.
type Issues interface {
	Authorize(ctx context.Context, actor, issue string, need Need) (string, error)
}

// Blobs stores file contents by attachment id in the workspace of ctx.
type Blobs interface {
	// Put stores r and returns its size and hex SHA-256.
	Put(ctx context.Context, id domain.ID, r io.Reader) (int64, string, error)
	Open(ctx context.Context, id domain.ID) (io.ReadCloser, error)
	Delete(ctx context.Context, id domain.ID) error
}

type Deps struct {
	Issues      Issues
	Attachments domain.Repository
	Blobs       Blobs
	NewID       func() string
	Now         func() time.Time
}

type Service struct{ d Deps }

func NewService(d Deps) *Service { return &Service{d: d} }

// Upload stores body as a new attachment of the issue. At most domain.MaxSize bytes are read.
func (s *Service) Upload(ctx context.Context, actor, issue, name, contentType string, body io.Reader) (*domain.Attachment, error) {
	key, err := s.d.Issues.Authorize(ctx, actor, issue, Attach)
	if err != nil {
		return nil, err
	}
	id := domain.ID(s.d.NewID())
	size, sum, err := s.d.Blobs.Put(ctx, id, io.LimitReader(body, domain.MaxSize+1))
	if err != nil {
		return nil, err
	}
	a, err := domain.New(id, key, name, contentType, size, sum, actor, s.d.Now().UTC())
	if err == nil {
		err = s.d.Attachments.Save(ctx, a)
	}
	if err != nil {
		_ = s.d.Blobs.Delete(ctx, id)
		return nil, err
	}
	return a, nil
}

// List gives the issue's attachments, oldest first.
func (s *Service) List(ctx context.Context, actor, issue string) ([]*domain.Attachment, error) {
	key, err := s.d.Issues.Authorize(ctx, actor, issue, View)
	if err != nil {
		return nil, err
	}
	return s.d.Attachments.ForIssue(ctx, key)
}

// load finds an attachment the actor may see; files of hidden issues stay hidden.
func (s *Service) load(ctx context.Context, actor, id string) (*domain.Attachment, error) {
	a, err := s.d.Attachments.ByID(ctx, domain.ID(id))
	if err != nil {
		return nil, err
	}
	if _, err := s.d.Issues.Authorize(ctx, actor, a.Issue, View); err != nil {
		if errors.Is(err, ErrIssueNotFound) {
			return nil, domain.ErrAttachmentNotFound
		}
		return nil, err
	}
	return a, nil
}

// Open returns the attachment and its contents; the caller closes the reader.
func (s *Service) Open(ctx context.Context, actor, id string) (*domain.Attachment, io.ReadCloser, error) {
	a, err := s.load(ctx, actor, id)
	if err != nil {
		return nil, nil, err
	}
	r, err := s.d.Blobs.Open(ctx, a.ID)
	if err != nil {
		return nil, nil, err
	}
	return a, r, nil
}

// Delete removes an attachment: the uploader may, others need to manage the project.
func (s *Service) Delete(ctx context.Context, actor, id string) error {
	a, err := s.load(ctx, actor, id)
	if err != nil {
		return err
	}
	if !a.MayDelete(actor, false) {
		if _, err := s.d.Issues.Authorize(ctx, actor, a.Issue, Manage); err != nil {
			return err
		}
	}
	if err := s.d.Attachments.Delete(ctx, a.ID); err != nil {
		return err
	}
	// The record is gone, so the file is unreachable; a leftover blob is only wasted space.
	_ = s.d.Blobs.Delete(ctx, a.ID)
	return nil
}
