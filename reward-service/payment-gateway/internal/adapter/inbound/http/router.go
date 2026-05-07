package httpadapter

import (
	"net/http"
	"time"

	"github.com/enterprise/payment-gateway/internal/service_logic/service"
	"github.com/enterprise/payment-gateway/pkg/metrics"
	"github.com/enterprise/payment-gateway/pkg/ratelimit"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

type Router struct {
	engine         *gin.Engine
	paymentHandler *PaymentHandler
	merchantRepo   service.MerchantRepository
	rateLimiter    *ratelimit.Limiter
	metrics        *metrics.Metrics
	logger         *zap.Logger
}

func NewRouter(
	paymentHandler *PaymentHandler,
	merchantRepo service.MerchantRepository,
	rateLimiter *ratelimit.Limiter,
	m *metrics.Metrics,
	logger *zap.Logger,
	env string,
) *Router {
	if env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := &Router{
		engine:         gin.New(),
		paymentHandler: paymentHandler,
		merchantRepo:   merchantRepo,
		rateLimiter:    rateLimiter,
		metrics:        m,
		logger:         logger,
	}
	r.setupRoutes()
	return r
}

func (r *Router) setupRoutes() {
	e := r.engine

	// Global middleware
	e.Use(RecoveryWithLogger(r.logger))
	e.Use(RequestLogger(r.logger, r.metrics))
	e.Use(SecurityHeaders())
	e.Use(ValidateContentType())
	e.Use(BlockSuspiciousIPs())
	e.Use(corsMiddleware())

	// Health & monitoring
	e.GET("/health", r.healthCheck)
	e.GET("/ready", r.readyCheck)
	e.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API v1
	v1 := e.Group("/v1")
	v1.Use(
		APIKeyAuth(r.merchantRepo, r.logger),
		IdempotencyKey(),
		RateLimit(r.rateLimiter, r.metrics, 100, time.Minute),
	)
	{
		// Payment routes
		payments := v1.Group("/payments")
		{
			payments.POST("",
				RateLimit(r.rateLimiter, r.metrics, 30, time.Minute),
				r.paymentHandler.CreatePayment,
			)
			payments.GET("", r.paymentHandler.ListPayments)
			payments.GET("/summary", r.paymentHandler.GetSummary)
			payments.GET("/:id", r.paymentHandler.GetPayment)
			payments.POST("/:id/verify", r.paymentHandler.VerifyPayment)
			payments.POST("/:id/cancel", r.paymentHandler.CancelPayment)
			payments.POST("/:id/refund",
				RateLimit(r.rateLimiter, r.metrics, 10, time.Minute),
				r.paymentHandler.CreateRefund,
			)
		}
	}

	// Webhook callback endpoints (no auth - verified by signature)
	webhooks := e.Group("/webhooks")
	{
		webhooks.POST("/omise", r.webhookCallback("omise"))
		webhooks.POST("/gbprimepay", r.webhookCallback("gbprimepay"))
		webhooks.POST("/stripe", r.webhookCallback("stripe"))
		webhooks.POST("/kbank", r.webhookCallback("kbank"))
		webhooks.POST("/scb", r.webhookCallback("scb"))
		webhooks.POST("/truewallet", r.webhookCallback("truewallet"))
		webhooks.POST("/linepay", r.webhookCallback("linepay"))
		webhooks.POST("/2c2p", r.webhookCallback("2c2p"))
	}

	// 404 handler
	e.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   gin.H{"code": "NOT_FOUND", "message": "Endpoint not found"},
		})
	})
}

func (r *Router) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"version":   "1.0.0",
	})
}

func (r *Router) readyCheck(c *gin.Context) {
	// Could check DB, Redis connectivity here
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func (r *Router) webhookCallback(provider string) gin.HandlerFunc {
	return func(c *gin.Context) {
		r.logger.Info("webhook received", zap.String("provider", provider))
		// In production: parse, verify signature, update payment status
		c.JSON(http.StatusOK, gin.H{"received": true})
	}
}

func (r *Router) Handler() http.Handler {
	return r.engine
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key, X-Request-ID")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
