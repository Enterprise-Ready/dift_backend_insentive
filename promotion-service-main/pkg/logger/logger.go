package logger

import (
	gologger "github.com/PlatformCore/libpackage/observability/logging"
	"log/slog"
	"os"
)

func New(service string) *slog.Logger {
	if service == "" {
		service = "promotion-service"
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", service)
}

func EngineLogger(service string) gologger.Logger {
	return gologger.FromSlog(New(service))
}
