package app

import (
	"github.com/gin-gonic/gin"
	adapterhttp "promotion-service/internal/adapter/inbound/http"
)

func wireHTTPMiddleware(router *gin.Engine) *gin.Engine {
	router.Use(adapterhttp.RequestIDMiddleware(), adapterhttp.AccessLogMiddleware(), adapterhttp.RecoveryMiddleware())
	return router
}
