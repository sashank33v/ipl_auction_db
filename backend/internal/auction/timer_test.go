package auction

import (
	"context"
	"testing"
	"time"
)

func TestRunTimerTicksAndStops(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	calls := 0
	RunTimer(ctx, time.Millisecond, func(context.Context) {
		calls++
		if calls == 3 {
			cancel()
		}
	})
	if calls != 3 {
		t.Fatalf("got %d ticks, want 3", calls)
	}
}

func TestRunTimerCancelledBeforeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	RunTimer(ctx, time.Millisecond, func(context.Context) { t.Fatal("cancelled worker executed a tick") })
}
