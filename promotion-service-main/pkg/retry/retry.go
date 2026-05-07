package retry

import (
	"context"
	retrypkg "github.com/PlatformCore/libpackage/resilience/retry"
	"time"
)

func Do(ctx context.Context, fn func(context.Context) error) error {
	cfg := retrypkg.DefaultConfig()
	cfg.MaxAttempts = 3
	cfg.InitialDelay = 50 * time.Millisecond
	cfg.MaxDelay = time.Second
	return retrypkg.Do(ctx, cfg, fn).Err
}
