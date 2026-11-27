package rooms

import (
	"context"
	"sync"
)

// Manager serializes commands per auction (one auction per room). Idle actors
// are removed immediately. The database remains the authoritative state.
type Manager struct {
	ctx    context.Context
	mu     sync.Mutex
	actors map[string]*actor
	wg     sync.WaitGroup
	closed bool
}

func NewManager(ctx context.Context) *Manager {
	return &Manager{ctx: ctx, actors: make(map[string]*actor)}
}

func (m *Manager) Do(ctx context.Context, key string, fn func(context.Context) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if key == "" || fn == nil {
		return ErrClosed
	}
	job := command{ctx: ctx, run: fn, result: make(chan error, 1)}
	m.mu.Lock()
	if m.closed || m.ctx.Err() != nil {
		m.mu.Unlock()
		return ErrClosed
	}
	a := m.actors[key]
	if a == nil {
		a = &actor{queue: make(chan command, 64)}
		m.actors[key] = a
		m.wg.Add(1)
		go m.run(key, a)
	}
	select {
	case a.queue <- job:
		a.pending++
	default:
		m.mu.Unlock()
		return ErrBusy
	}
	m.mu.Unlock()
	// Wait for the command to finish even when canceled, so captured result
	// variables are never accessed concurrently by caller and worker.
	return <-job.result
}

// Close prevents new work and waits for accepted commands to finish. Call
// after canceling the application's root context.
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	m.wg.Wait()
}
