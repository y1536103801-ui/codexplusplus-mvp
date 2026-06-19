package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/gin-gonic/gin"
)

func registerCodexPlusAdminRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	if h == nil || h.Admin == nil || h.Admin.CodexPlus == nil {
		return
	}
	codexPlus := admin.Group("/codex-plus")
	{
		codexPlus.GET("/config", h.Admin.CodexPlus.GetConfig)
		codexPlus.POST("/config/validate", h.Admin.CodexPlus.ValidateConfig)
		codexPlus.POST("/config/publish", h.Admin.CodexPlus.PublishConfig)
		codexPlus.GET("/config/versions", h.Admin.CodexPlus.ListConfigVersions)
		codexPlus.POST("/config/rollback", h.Admin.CodexPlus.RollbackConfig)
		codexPlus.GET("/options", h.Admin.CodexPlus.GetOptions)
		codexPlus.GET("/users/:id/entitlement", h.Admin.CodexPlus.GetUserEntitlement)
		codexPlus.GET("/users/:id/devices", h.Admin.CodexPlus.GetUserDevices)
		codexPlus.POST("/users/:id/devices/:device_id/revoke", h.Admin.CodexPlus.RevokeUserDevice)
		codexPlus.POST("/users/:id/devices/:device_id/restore", h.Admin.CodexPlus.RestoreUserDevice)
		codexPlus.GET("/users/:id/events", h.Admin.CodexPlus.GetUserEvents)
	}
}
