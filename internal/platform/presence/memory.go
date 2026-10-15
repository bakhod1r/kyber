package presence

import (
	"context"
	"slices"
	"sync"
	"time"
)

type seen struct {
	replica string
	Seat
}

// Memory is the in-process store (dev mode, one replica, tests).
type Memory struct {
	mu    sync.Mutex
	seats map[seen]time.Time
}

func NewMemory() *Memory { return &Memory{seats: map[seen]time.Time{}} }

func (m *Memory) Beat(_ context.Context, replica string, seats []Seat, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k := range m.seats {
		if k.replica == replica {
			delete(m.seats, k)
		}
	}
	for _, s := range seats {
		m.seats[seen{replica, s}] = at
	}
	return nil
}

func (m *Memory) Online(_ context.Context, ws string, users []string, since time.Time) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []string{}
	for _, u := range users {
		if slices.Contains(out, u) {
			continue
		}
		for k, at := range m.seats {
			if k.Workspace == ws && k.User == u && at.After(since) {
				out = append(out, u)
				break
			}
		}
	}
	return out, nil
}

func (m *Memory) Expire(_ context.Context, before time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for k, at := range m.seats {
		if !at.After(before) {
			delete(m.seats, k)
			n++
		}
	}
	return n, nil
}

var _ Store = (*Memory)(nil)
