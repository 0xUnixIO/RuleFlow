package services

import (
	"context"
	"time"
)

func runPeriodicTask(ctx context.Context, interval time.Duration, runImmediately bool, task func(context.Context)) {
	go func() {
		if runImmediately {
			task(ctx)
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				task(ctx)
			}
		}
	}()
}
