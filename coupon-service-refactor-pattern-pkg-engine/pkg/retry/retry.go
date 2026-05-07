package retry

import (
	"context"
	"time"

	engineretry "github.com/PlatformCore/libpackage/resilience/retry"
)

func Do(ctx context.Context, fn func(context.Context) error) error {
	cfg := engineretry.DefaultConfig()
	cfg.MaxAttempts = 3
	cfg.InitialDelay = 100 * time.Millisecond
	res := engineretry.Do(ctx, cfg, fn)
	return res.Err
}
