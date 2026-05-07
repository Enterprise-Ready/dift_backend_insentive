package paymentengine

import (
	"context"
	"time"

	"github.com/PlatformCore/libpackage/resilience/retry"
)

type GatewayEngine struct{ DefaultTTL time.Duration }

func NewGatewayEngine() *GatewayEngine { return &GatewayEngine{DefaultTTL: 15 * time.Minute} }
func (e *GatewayEngine) WithRetry(ctx context.Context, fn func(context.Context) error) error {
	_ = retry.Policy{}
	return fn(ctx)
}
func (e *GatewayEngine) ExpiryFromNow() time.Time { return time.Now().Add(e.DefaultTTL) }
