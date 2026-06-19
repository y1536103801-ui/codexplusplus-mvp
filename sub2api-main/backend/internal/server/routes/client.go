package routes

import (
	clienthandler "github.com/Wei-Shaw/sub2api/internal/handler/client"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

func RegisterClientRoutes(
	v1 *gin.RouterGroup,
	h *clienthandler.Handler,
	jwtAuth middleware.JWTAuthMiddleware,
	settingService *service.SettingService,
) {
	if v1 == nil || h == nil {
		return
	}
	authenticated := v1.Group("")
	authenticated.Use(gin.HandlerFunc(jwtAuth))
	authenticated.Use(middleware.BackendModeUserGuard(settingService))

	client := authenticated.Group("/client")
	{
		client.GET("/bootstrap", h.Bootstrap)
		client.GET("/usage", h.Usage)
		client.POST("/devices", h.UpsertDevice)
		client.POST("/redeem", h.Redeem)
	}
}
