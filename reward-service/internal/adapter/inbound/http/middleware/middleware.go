package middleware

import (
	"net/http"
	"strings"

	_ "github.com/PlatformCore/libpackage/middleware/auth"
	_ "github.com/PlatformCore/libpackage/middleware/logging"
	_ "github.com/PlatformCore/libpackage/middleware/metrics"
	_ "github.com/PlatformCore/libpackage/middleware/recovery"
	_ "github.com/PlatformCore/libpackage/middleware/requestid"
	_ "github.com/PlatformCore/libpackage/middleware/securityheaders"
	_ "github.com/PlatformCore/libpackage/middleware/timeout"

	"github.com/gin-gonic/gin"
)

func RequireAnyRole(roles ...string) gin.HandlerFunc {
	allowed := map[string]struct{}{}
	for _, role := range roles {
		allowed[strings.ToUpper(strings.TrimSpace(role))] = struct{}{}
	}
	return func(c *gin.Context) {
		roleHeader := c.GetHeader("X-Admin-Role")
		if strings.TrimSpace(roleHeader) == "" {
			c.Next()
			return
		}
		for _, role := range strings.Split(roleHeader, ",") {
			if _, ok := allowed[strings.ToUpper(strings.TrimSpace(role))]; ok {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	}
}
