package logger

import (
	"context"
	"fmt"
	"log"

	enginelog "github.com/PlatformCore/libpackage/observability/logging"
)

type Logger struct {
	service string
	engine  enginelog.Logger
}

func New(service string) *Logger {
	return &Logger{service: service, engine: enginelog.New().With("service", service)}
}

func (l *Logger) Infof(format string, args ...any) {
	l.engine.InfoContext(context.Background(), fmt.Sprintf(format, args...))
}
func (l *Logger) Warnf(format string, args ...any) {
	l.engine.WarnContext(context.Background(), fmt.Sprintf(format, args...))
}
func (l *Logger) Errorf(format string, args ...any) {
	l.engine.ErrorContext(context.Background(), fmt.Sprintf(format, args...))
}
func (l *Logger) Fatalf(format string, args ...any) { log.Fatalf(format, args...) }
