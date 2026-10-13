package memory

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"sync"

	"github.com/bakhod1r/kyber/internal/attachment/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// Blobs keeps file contents in memory.
type Blobs struct {
	mu    sync.Mutex
	files map[key][]byte
}

func NewBlobs() *Blobs { return &Blobs{files: map[key][]byte{}} }

func (b *Blobs) Put(ctx context.Context, id domain.ID, r io.Reader) (int64, string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return 0, "", err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.files[key{tenant.Workspace(ctx), id}] = data
	sum := sha256.Sum256(data)
	return int64(len(data)), hex.EncodeToString(sum[:]), nil
}

func (b *Blobs) Open(ctx context.Context, id domain.ID) (io.ReadCloser, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	data, ok := b.files[key{tenant.Workspace(ctx), id}]
	if !ok {
		return nil, domain.ErrAttachmentNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (b *Blobs) Delete(ctx context.Context, id domain.ID) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.files, key{tenant.Workspace(ctx), id})
	return nil
}
