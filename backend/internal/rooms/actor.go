package rooms

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"
)

type actor struct {
	queue   chan command
	pending int // protected by Manager.mu
}

func (m *Manager) run(key string, a *actor) {
	defer m.wg.Done()
	for job := range a.queue {
		err := m.execute(job)
		m.mu.Lock()
		a.pending--
		if a.pending == 0 {
			delete(m.actors, key)
			close(a.queue)
		}
		m.mu.Unlock()
		job.result <- err
	}
}

func (m *Manager) execute(job command) (err error) {
	ctx, cancel := context.WithCancel(job.ctx)
	stop := context.AfterFunc(m.ctx, cancel)
	defer func() {
		stop()
		cancel()
		if p := recover(); p != nil {
			log.Printf("room command panic: %v\n%s", p, debug.Stack())
			err = fmt.Errorf("room command failed")
		}
	}()
	if err = m.ctx.Err(); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	return job.run(ctx)
}
