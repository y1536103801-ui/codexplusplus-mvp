package admin

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type CodexPlusHandler struct {
	adminService *service.CodexPlusAdminService
}

type codexPlusPublishRequest struct {
	Config       service.CodexPlusConfig `json:"config"`
	ChangeReason string                  `json:"change_reason"`
}

type codexPlusRollbackRequest struct {
	ConfigVersion string `json:"config_version"`
	ChangeReason  string `json:"change_reason"`
}

type codexPlusDeviceActionRequest struct {
	Reason string `json:"reason"`
}

func NewCodexPlusHandler(adminService *service.CodexPlusAdminService) *CodexPlusHandler {
	return &CodexPlusHandler{adminService: adminService}
}

func (h *CodexPlusHandler) GetConfig(c *gin.Context) {
	cfg, err := h.adminService.GetCurrentConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *CodexPlusHandler) ValidateConfig(c *gin.Context) {
	var cfg service.CodexPlusConfig
	if err := decodeStrictJSON(c, &cfg); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.adminService.ValidateConfig(c.Request.Context(), &cfg); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"valid": true})
}

func (h *CodexPlusHandler) PublishConfig(c *gin.Context) {
	var req codexPlusPublishRequest
	if err := decodeStrictJSON(c, &req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	actor := "admin:" + strconv.FormatInt(getAdminIDFromContext(c), 10)
	cfg, err := h.adminService.PublishConfig(c.Request.Context(), req.Config, actor, req.ChangeReason)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *CodexPlusHandler) ListConfigVersions(c *gin.Context) {
	versions, err := h.adminService.ListConfigVersions(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, versions)
}

func (h *CodexPlusHandler) RollbackConfig(c *gin.Context) {
	var req codexPlusRollbackRequest
	if err := decodeStrictJSON(c, &req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	actor := "admin:" + strconv.FormatInt(getAdminIDFromContext(c), 10)
	cfg, err := h.adminService.RollbackConfig(c.Request.Context(), req.ConfigVersion, actor, req.ChangeReason)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *CodexPlusHandler) GetOptions(c *gin.Context) {
	opts, err := h.adminService.GetOptions(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, opts)
}

func (h *CodexPlusHandler) GetUserEntitlement(c *gin.Context) {
	userID, ok := parseCodexPlusUserID(c)
	if !ok {
		return
	}
	entitlement, err := h.adminService.GetUserEntitlement(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, entitlement)
}

func (h *CodexPlusHandler) GetUserDevices(c *gin.Context) {
	userID, ok := parseCodexPlusUserID(c)
	if !ok {
		return
	}
	limit, ok := parseCodexPlusDeviceLimit(c)
	if !ok {
		return
	}
	devices, err := h.adminService.ListUserDevices(c.Request.Context(), userID, service.CodexPlusDeviceListFilter{
		Statuses: c.QueryArray("status"),
		Limit:    limit,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, devices)
}

func (h *CodexPlusHandler) RevokeUserDevice(c *gin.Context) {
	userID, ok := parseCodexPlusUserID(c)
	if !ok {
		return
	}
	deviceID, ok := parseCodexPlusDeviceID(c)
	if !ok {
		return
	}
	actor, ok := requireCodexPlusAdminActor(c)
	if !ok {
		return
	}
	var req codexPlusDeviceActionRequest
	if err := decodeOptionalStrictJSON(c, &req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	device, err := h.adminService.RevokeUserDevice(c.Request.Context(), service.CodexPlusDeviceAdminActionInput{
		UserID:   userID,
		DeviceID: deviceID,
		Actor:    actor,
		Reason:   req.Reason,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, device)
}

func (h *CodexPlusHandler) RestoreUserDevice(c *gin.Context) {
	userID, ok := parseCodexPlusUserID(c)
	if !ok {
		return
	}
	deviceID, ok := parseCodexPlusDeviceID(c)
	if !ok {
		return
	}
	actor, ok := requireCodexPlusAdminActor(c)
	if !ok {
		return
	}
	var req codexPlusDeviceActionRequest
	if err := decodeOptionalStrictJSON(c, &req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	device, err := h.adminService.RestoreUserDevice(c.Request.Context(), service.CodexPlusDeviceAdminActionInput{
		UserID:   userID,
		DeviceID: deviceID,
		Actor:    actor,
		Reason:   req.Reason,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, device)
}

func (h *CodexPlusHandler) GetUserEvents(c *gin.Context) {
	userID, ok := parseCodexPlusUserID(c)
	if !ok {
		return
	}
	events, err := h.adminService.GetUserEvents(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, events)
}

func parseCodexPlusUserID(c *gin.Context) (int64, bool) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return 0, false
	}
	return userID, true
}

func parseCodexPlusDeviceID(c *gin.Context) (string, bool) {
	deviceID := strings.TrimSpace(c.Param("device_id"))
	if deviceID == "" {
		response.BadRequest(c, "Invalid device ID")
		return "", false
	}
	if len(deviceID) > 128 {
		response.BadRequest(c, "Invalid device ID")
		return "", false
	}
	return deviceID, true
}

func parseCodexPlusDeviceLimit(c *gin.Context) (int, bool) {
	raw := strings.TrimSpace(c.Query("limit"))
	if raw == "" {
		return 0, true
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 || limit > 500 {
		response.BadRequest(c, "Invalid limit")
		return 0, false
	}
	return limit, true
}

func requireCodexPlusAdminActor(c *gin.Context) (string, bool) {
	adminID := getAdminIDFromContext(c)
	if adminID <= 0 {
		response.Forbidden(c, "Admin authentication required")
		return "", false
	}
	return "admin:" + strconv.FormatInt(adminID, 10), true
}

func decodeStrictJSON(c *gin.Context, out any) error {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var extra struct{}
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("request body must contain a single JSON document")
		}
		return err
	}
	return nil
}

func decodeOptionalStrictJSON(c *gin.Context, out any) error {
	if c.Request == nil || c.Request.Body == nil || c.Request.ContentLength == 0 {
		return nil
	}
	return decodeStrictJSON(c, out)
}
