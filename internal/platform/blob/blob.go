// Package blob stores files (chat attachments, huddle recordings) by key. Keys are chosen by Kyber (ids),
// never by users; content is opaque bytes.
package blob

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

var (
	ErrNotFound   = errors.New("blob not found")
	ErrInvalidKey = errors.New("invalid blob key")
)

// Store is the port; adapters: disk (self-hosted), memory (dev and tests). An S3/MinIO adapter fits the same port.
type Store interface {
	Put(ctx context.Context, key string, r io.Reader) error
	Open(ctx context.Context, key string) (io.ReadCloser, error) // ErrNotFound
	Delete(ctx context.Context, key string) error                // idempotent
}

var validKey = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

type Memory struct {
	mu    sync.Mutex
	blobs map[string][]byte
}

func NewMemory() *Memory { return &Memory{blobs: map[string][]byte{}} }

func (m *Memory) Put(_ context.Context, key string, r io.Reader) error {
	if !validKey.MatchString(key) {
		return ErrInvalidKey
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blobs[key] = b
	return nil
}

func (m *Memory) Open(_ context.Context, key string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.blobs[key]
	if !ok {
		return nil, ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (m *Memory) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.blobs, key)
	return nil
}

// Disk keeps blobs under a directory, two levels deep by key prefix; writes go to a temporary file first,
// so a reader never sees half a file.
type Disk struct{ dir string }

func NewDisk(dir string) (*Disk, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return &Disk{dir: dir}, nil
}

func (d *Disk) path(key string) (string, error) {
	if !validKey.MatchString(key) {
		return "", ErrInvalidKey
	}
	k := key + "__"
	return filepath.Join(d.dir, k[:2], key), nil
}

func (d *Disk) Put(_ context.Context, key string, r io.Reader) error {
	p, err := d.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".upload-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after the rename
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), p)
}

func (d *Disk) Open(_ context.Context, key string) (io.ReadCloser, error) {
	p, err := d.path(key)
	if err != nil {
		return nil, ErrNotFound
	}
	f, err := os.Open(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return f, err
}

func (d *Disk) Delete(_ context.Context, key string) error {
	p, err := d.path(key)
	if err != nil {
		return nil
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
