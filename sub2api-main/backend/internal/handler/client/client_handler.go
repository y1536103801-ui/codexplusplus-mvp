package client

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

const (
	deviceIDHeader      = "X-CodexPlus-Device-Id"
	clientVersionHeader = "X-CodexPlus-Client-Version"
)

type successEnvelope struct {
	Code      int     `json:"code"`
	Status    string  `json:"status"`
	Message   string  `json:"message"`
	Reason    *string `json:"reason"`
	ErrorCode *string `json:"error_code"`
	Data      any     `json:"data"`
}

type Handler struct {
	clientService *service.CodexPlusClientService
}

func NewClientHandler(clientService *service.CodexPlusClientService) *Handler {
	return &Handler{clientService: clientService}
}

func (h *Handler) Bootstrap(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	snapshot, err := h.clientService.Bootstrap(c.Request.Context(), service.CodexPlusBootstrapInput{
		UserID:        subject.UserID,
		DeviceID:      clientDeviceID(c),
		ClientVersion: strings.TrimSpace(c.GetHeader(clientVersionHeader)),
		RequestID:     clientRequestID(c),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	clientSuccess(c, clientBootstrapMessage(snapshot), dto.ClientBootstrapFromService(snapshot))
}

func (h *Handler) Usage(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	snapshot, err := h.clientService.ClientUsage(c.Request.Context(), service.CodexPlusUsageInput{
		UserID:    subject.UserID,
		DeviceID:  clientDeviceID(c),
		RequestID: clientRequestID(c),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	clientSuccess(c, "Usage snapshot ready.", dto.ClientUsageFromService(snapshot))
}

func (h *Handler) UpsertDevice(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req dto.ClientDeviceRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	device, err := h.clientService.UpsertDevice(c.Request.Context(), subject.UserID, service.CodexPlusDeviceInput{
		DeviceID:     req.DeviceID,
		Platform:     req.Platform,
		AppVersion:   req.AppVersion,
		CodexVersion: req.CodexVersion,
		LastSeenAt:   req.LastSeenAt,
		RequestID:    clientRequestID(c),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	clientSuccess(c, "Device registered.", dto.ClientDeviceFromService(device))
}

func (h *Handler) Redeem(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req dto.ClientRedeemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.clientService.Redeem(c.Request.Context(), subject.UserID, req.Code, req.DeviceID, clientRequestID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	clientSuccess(c, clientRedeemMessage(result), dto.ClientRedeemFromService(result))
}

func clientDeviceID(c *gin.Context) string {
	if value := strings.TrimSpace(c.GetHeader(deviceIDHeader)); value != "" {
		return value
	}
	return strings.TrimSpace(c.Query("device_id"))
}

func clientRequestID(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	ctx := c.Request.Context()
	for _, key := range []ctxkey.Key{ctxkey.RequestID, ctxkey.ClientRequestID} {
		if value, ok := ctx.Value(key).(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	for _, header := range []string{"X-Request-ID", "X-Request-Id", "X-Client-Request-ID"} {
		if value := strings.TrimSpace(c.GetHeader(header)); value != "" {
			return value
		}
	}
	return ""
}

func clientSuccess(c *gin.Context, message string, data any) {
	if strings.TrimSpace(message) == "" {
		message = "success"
	}
	c.JSON(http.StatusOK, successEnvelope{
		Code:    0,
		Status:  "success",
		Message: message,
		Data:    data,
	})
}

func clientBootstrapMessage(snapshot *service.CodexPlusBootstrapSnapshot) string {
	if snapshot == nil || strings.TrimSpace(snapshot.Service.Status) == "" {
		return "Bootstrap snapshot ready."
	}
	if snapshot.Service.Status == service.ClientServiceStatusAvailable {
		return "Codex++ Cloud is available."
	}
	return snapshot.Service.Message
}

func clientRedeemMessage(result *service.CodexPlusRedeemResult) string {
	if result == nil {
		return "Redeem request processed."
	}
	if result.RedeemStatus == "applied" {
		return "Code applied."
	}
	if strings.TrimSpace(result.Message) != "" {
		return result.Message
	}
	return "Redeem request processed."
}
