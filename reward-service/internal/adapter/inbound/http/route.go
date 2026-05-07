package http

import (
	"net/http"
	"os"
	"strings"

	"reward-service/internal/adapter/inbound/http/middleware"
	"reward-service/pkg/health"

	"github.com/gin-gonic/gin"

	redeemPort "reward-service/internal/interface/http"
)

type Handlers struct {
	Redeem redeemPort.RewardRedeemHTTPPort
}

func RegisterRoutes(router *gin.Engine, h Handlers) {
	redeemHandler := NewRedeemHandler(h.Redeem)

	api := router.Group("/api")
	{
		api.GET("/v1/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, health.Controller("reward-service", "v1"))
		})
		api.POST("/redeem", redeemHandler.Redeem)
	}

	admin := router.Group("/api/v1/admin")
	admin.Use(middleware.RequireAnyRole("SUPER_ADMIN", "ADMIN", "EDITOR"))
	{
		admin.POST("/rewards/redeem", redeemHandler.Redeem)
		admin.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"admin": true, "health": health.Controller("reward-service", "v1")})
		})
	}

	router.POST("/internal/admin/control", func(c *gin.Context) {
		secret := strings.TrimSpace(os.Getenv("ADMIN_CONTROL_SHARED_SECRET"))
		if secret != "" && c.GetHeader("X-Admin-Secret") != secret {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		var req map[string]any
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
			return
		}
		action, _ := req["action"].(string)
		if strings.TrimSpace(action) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action_required"})
			return
		}
		c.JSON(http.StatusAccepted, gin.H{"accepted": true, "service": "reward-service", "action": action})
	})
}
