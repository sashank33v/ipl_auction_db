package rooms

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSerialPerRoom(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := NewManager(ctx)
	defer m.Close()
	var active, count atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := m.Do(ctx, "a", func(context.Context) error {
				if active.Add(1) != 1 {
					t.Error("overlapping commands")
				}
				time.Sleep(time.Millisecond)
				count.Add(1)
				active.Add(-1)
				return nil
			})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if count.Load() != 32 {
		t.Fatal(count.Load())
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.actors) != 0 {
		t.Fatal("idle actor leaked")
	}
}
func TestIndependentRooms(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := NewManager(ctx)
	defer m.Close()
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() { done <- m.Do(ctx, "a", func(context.Context) error { close(entered); <-release; return nil }) }()
	<-entered
	b := make(chan error, 1)
	go func() { b <- m.Do(ctx, "b", func(context.Context) error { return nil }) }()
	select {
	case err := <-b:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(time.Second):
		t.Error("different room blocked")
	}
	close(release)
	<-done
}
func TestCanceledAndClosed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := NewManager(ctx)
	canceled, c := context.WithCancel(ctx)
	c()
	if err := m.Do(canceled, "a", func(context.Context) error { t.Error("canceled command ran"); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	cancel()
	m.Close()
	if err := m.Do(context.Background(), "a", func(context.Context) error { return nil }); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
}
func TestPanicDoesNotKillActor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := NewManager(ctx)
	defer m.Close()
	if err := m.Do(ctx, "a", func(context.Context) error { panic("test") }); err == nil {
		t.Fatal("panic not reported")
	}
	if err := m.Do(ctx, "a", func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
}
