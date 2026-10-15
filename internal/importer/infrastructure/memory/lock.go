package memory

import (
	"context"
	"sync"

	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// ImportLock is the in-process import lock (single-replica and dev mode).
type ImportLock struct {
	mu   sync.Mutex
	held map[string]chan struct{} // closed when released
}

func NewImportLock() *ImportLock { return &ImportLock{held: map[string]chan struct{}{}} }

func (l *ImportLock) Lock(ctx context.Context, project string) (func(), error) {
	key := tenant.Workspace(ctx) + "/" + project
	for {
		l.mu.Lock()
		done, busy := l.held[key]
		if !busy {
			mine := make(chan struct{})
			l.held[key] = mine
			l.mu.Unlock()
			var once sync.Once
			return func() {
				once.Do(func() {
					l.mu.Lock()
					delete(l.held, key)
					l.mu.Unlock()
					close(mine)
				})
			}, nil
		}
		l.mu.Unlock()
		select {
		case <-done:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}
