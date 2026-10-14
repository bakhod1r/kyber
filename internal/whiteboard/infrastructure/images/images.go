// Package images keeps whiteboard pictures (S96) in the blob store chat files use (disk, memory or S3).
package images

import (
	"context"
	"errors"
	"io"

	"github.com/bakhod1r/kyber/internal/platform/blob"
	"github.com/bakhod1r/kyber/internal/whiteboard/domain"
)

type Store struct{ blobs blob.Store }

func New(blobs blob.Store) *Store { return &Store{blobs: blobs} }

func (s *Store) Put(ctx context.Context, key string, r io.Reader) error {
	return s.blobs.Put(ctx, key, r)
}

func (s *Store) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	r, err := s.blobs.Open(ctx, key)
	if errors.Is(err, blob.ErrNotFound) {
		return nil, domain.ErrImageMissing
	}
	return r, err
}
