package enginebundle

import (
	"net/http"
	"os"
	"strings"

	_ "github.com/PlatformCore/libpackage/middleware/adaptive"
	_ "github.com/PlatformCore/libpackage/middleware/auth"
	_ "github.com/PlatformCore/libpackage/middleware/metrics"
	_ "github.com/PlatformCore/libpackage/middleware/recovery"
	_ "github.com/PlatformCore/libpackage/middleware/requestid"
	_ "github.com/PlatformCore/libpackage/middleware/retry"
	_ "github.com/PlatformCore/libpackage/middleware/timeout"
	_ "github.com/PlatformCore/libpackage/middleware/tracing"
	_ "github.com/PlatformCore/libpackage/middleware/validation"

	"reward-service/pkg/health"
)

type Config struct {
	ServiceName string
	Enabled     bool
	Profile     string
}

type Bundle struct{ Config Config }

func LoadEngineUnifiedConfigFromEnv(serviceName string) Config {
	profile := strings.TrimSpace(os.Getenv("ENGINE_PROFILE"))
	if profile == "" {
		profile = "reward-enterprise"
	}
	return Config{ServiceName: serviceName, Enabled: true, Profile: profile}
}

func NewEngineUnifiedBundle(cfg Config) *Bundle { return &Bundle{Config: cfg} }

func DefaultHTTPMiddlewares() []func(http.Handler) http.Handler { return nil }

type HealthStatus = health.Status

func HealthController(serviceName, version string) HealthStatus {
	return health.Controller(serviceName, version)
}
