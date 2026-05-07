package observability

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Context keys
type contextKey string

const (
	contextKeyTraceID   contextKey = "trace_id"
	contextKeyRequestID contextKey = "request_id"
	contextKeyUserID    contextKey = "user_id"
	contextKeyService   contextKey = "service"
)

// Logger wraps slog.Logger with trace-aware methods
type Logger struct {
	base    *slog.Logger
	service string
}

// NewLogger creates a structured logger
func NewLogger(service string, level slog.Level) *Logger {
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	base := slog.New(handler).With(
		"service", service,
		"version", getVersion(),
	)

	return &Logger{
		base:    base,
		service: service,
	}
}

func getVersion() string {
	if v := os.Getenv("SERVICE_VERSION"); v != "" {
		return v
	}
	return "dev"
}

// FromContext extracts trace metadata from context and returns enriched logger
func (l *Logger) FromContext(ctx context.Context) *slog.Logger {
	logger := l.base

	if traceID, ok := ctx.Value(contextKeyTraceID).(string); ok && traceID != "" {
		logger = logger.With("trace_id", traceID)
	}
	if requestID, ok := ctx.Value(contextKeyRequestID).(string); ok && requestID != "" {
		logger = logger.With("request_id", requestID)
	}
	if userID, ok := ctx.Value(contextKeyUserID).(string); ok && userID != "" {
		logger = logger.With("user_id", userID)
	}

	return logger
}

// WithTraceID injects trace ID into context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, contextKeyTraceID, traceID)
}

// WithUserID injects user ID into context
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, contextKeyUserID, userID)
}

// TraceIDFromContext extracts trace ID from context
func TraceIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(contextKeyTraceID).(string); ok {
		return v
	}
	return ""
}

// GinTracingMiddleware injects trace/request IDs into context and response headers
func GinTracingMiddleware(logger *Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Extract or generate trace ID
		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = uuid.NewString()
		}
		requestID := uuid.NewString()

		// Set in context
		ctx := WithTraceID(c.Request.Context(), traceID)
		ctx = context.WithValue(ctx, contextKeyRequestID, requestID)
		c.Request = c.Request.WithContext(ctx)

		// Set headers for downstream services
		c.Header("X-Trace-ID", traceID)
		c.Header("X-Request-ID", requestID)

		c.Next()

		// Log request completion
		latency := time.Since(start)
		status := c.Writer.Status()

		logFn := logger.base.Info
		if status >= 500 {
			logFn = logger.base.Error
		} else if status >= 400 {
			logFn = logger.base.Warn
		}

		logFn("http_request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", status,
			"latency_ms", latency.Milliseconds(),
			"trace_id", traceID,
			"request_id", requestID,
			"client_ip", c.ClientIP(),
			"user_agent", c.Request.UserAgent(),
		)
	}
}

// Op represents a named operation for timing/logging
type Op struct {
	Name      string
	TraceID   string
	StartTime time.Time
	logger    *slog.Logger
}

// StartOp begins a named operation with timing
func (l *Logger) StartOp(ctx context.Context, name string, fields ...any) *Op {
	traceID := TraceIDFromContext(ctx)
	baseFields := []any{"operation", name, "trace_id", traceID}
	baseFields = append(baseFields, fields...)
	l.base.Debug("operation_started", baseFields...)

	return &Op{
		Name:      name,
		TraceID:   traceID,
		StartTime: time.Now(),
		logger:    l.base,
	}
}

// End logs the completion of an operation
func (op *Op) End(err error, fields ...any) {
	latency := time.Since(op.StartTime)
	baseFields := []any{
		"operation", op.Name,
		"trace_id", op.TraceID,
		"latency_ms", latency.Milliseconds(),
	}
	baseFields = append(baseFields, fields...)

	if err != nil {
		baseFields = append(baseFields, "error", err.Error())
		op.logger.Error("operation_failed", baseFields...)
	} else {
		op.logger.Info("operation_completed", baseFields...)
	}
}

// Global default logger
var defaultLogger *Logger

func init() {
	defaultLogger = NewLogger("reward-service", slog.LevelInfo)
}

func Default() *Logger {
	return defaultLogger
}
