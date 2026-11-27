package auction

import (
	"context"
	"time"
)

// RunTimer executes a tick immediately and then periodically until shutdown.
// The callback must honor cancellation. Persisted database deadlines, not this
// ticker, decide whether a player can be sold or marked unsold.
func RunTimer(ctx context.Context, interval time.Duration, tick func(context.Context)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
