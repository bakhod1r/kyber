package application

import (
	"bytes"
	"context"
	"io"

	"github.com/bakhod1r/kyber/internal/whiteboard/domain"
)

// UploadImage stores a picture for a board (S96) and returns its id; the member then adds an image element
// showing it. Only PNG, JPEG, GIF and WebP up to domain.MaxImage are kept.
func (s *Service) UploadImage(ctx context.Context, actor, board string, r io.Reader) (string, error) {
	if s.d.Images == nil {
		return "", ErrNoImages
	}
	b, _, err := s.load(ctx, actor, board)
	if err != nil {
		return "", err
	}
	data, err := io.ReadAll(io.LimitReader(r, domain.MaxImage+1))
	if err != nil {
		return "", err
	}
	if len(data) > domain.MaxImage {
		return "", domain.ErrNotImage
	}
	if _, err := domain.ImageType(data); err != nil {
		return "", err
	}
	id := s.d.NewID()
	return id, s.d.Images.Put(ctx, domain.ImageKey(b.ID, id), bytes.NewReader(data))
}

// Image opens a board's picture for a member, with its media type.
func (s *Service) Image(ctx context.Context, actor, board, image string) (io.ReadCloser, string, error) {
	if s.d.Images == nil {
		return nil, "", ErrNoImages
	}
	b, _, err := s.load(ctx, actor, board)
	if err != nil {
		return nil, "", err
	}
	if !domain.ValidImageID(image) {
		return nil, "", domain.ErrImageMissing
	}
	r, err := s.d.Images.Open(ctx, domain.ImageKey(b.ID, image))
	if err != nil {
		return nil, "", err
	}
	head := make([]byte, 16)
	n, _ := io.ReadFull(r, head)
	typ, err := domain.ImageType(head[:n])
	if err != nil {
		r.Close()
		return nil, "", err
	}
	return struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(head[:n]), r), r}, typ, nil
}
