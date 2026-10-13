// Package blobfs stores attachment contents as files: <dir>/<workspace>/<id>.
package blobfs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/bakhod1r/kyber/internal/attachment/domain"
	kid "github.com/bakhod1r/kyber/internal/platform/id"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type Blobs struct{ dir string }

func New(dir string) *Blobs { return &Blobs{dir: dir} }

// path places a blob; ids and workspaces are UUIDs, so nothing can escape dir.
func (b *Blobs) path(ctx context.Context, id domain.ID) (string, error) {
	ws := tenant.Workspace(ctx)
	if !kid.Valid(string(id)) || !kid.Valid(ws) {
		return "", domain.ErrAttachmentNotFound
	}
	return filepath.Join(b.dir, ws, string(id)), nil
}

// Put writes to a temporary file and renames it into place, so readers never see a partial file.
func (b *Blobs) Put(ctx context.Context, id domain.ID, r io.Reader) (int64, string, error) {
	p, err := b.path(ctx, id)
	if err != nil {
		return 0, "", err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return 0, "", err
	}
	f, err := os.CreateTemp(filepath.Dir(p), ".upload-*")
	if err != nil {
		return 0, "", err
	}
	defer os.Remove(f.Name()) // no-op after the rename
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), r)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(f.Name(), p)
	}
	if err != nil {
		return 0, "", err
	}
	return n, hex.EncodeToString(h.Sum(nil)), nil
}

func (b *Blobs) Open(ctx context.Context, id domain.ID) (io.ReadCloser, error) {
	p, err := b.path(ctx, id)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, domain.ErrAttachmentNotFound
	}
	return f, err
}

func (b *Blobs) Delete(ctx context.Context, id domain.ID) error {
	p, err := b.path(ctx, id)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
