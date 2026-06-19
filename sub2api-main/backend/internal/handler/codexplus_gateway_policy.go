package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type codexPlusGatewayPolicyErrorWriter func(status int, code, message string)

func (h *GatewayHandler) enforceCodexPlusGatewayPolicy(
	c *gin.Context,
	apiKey *service.APIKey,
	subscription *service.UserSubscription,
	requestedModel string,
	endpoint string,
	writeError codexPlusGatewayPolicyErrorWriter,
) bool {
	if h == nil {
		return true
	}
	return enforceCodexPlusGatewayPolicy(c, h.codexPlusPolicyService, apiKey, subscription, requestedModel, endpoint, writeError)
}

func (h *OpenAIGatewayHandler) enforceCodexPlusGatewayPolicy(
	c *gin.Context,
	apiKey *service.APIKey,
	subscription *service.UserSubscription,
	requestedModel string,
	endpoint string,
	writeError codexPlusGatewayPolicyErrorWriter,
) bool {
	if h == nil {
		return true
	}
	return enforceCodexPlusGatewayPolicy(c, h.codexPlusPolicyService, apiKey, subscription, requestedModel, endpoint, writeError)
}

func enforceCodexPlusGatewayPolicy(
	c *gin.Context,
	policyService *service.CodexPlusGatewayPolicyService,
	apiKey *service.APIKey,
	subscription *service.UserSubscription,
	requestedModel string,
	endpoint string,
	writeError codexPlusGatewayPolicyErrorWriter,
) bool {
	if policyService == nil {
		return true
	}
	if writeError == nil {
		writeError = func(status int, code, message string) {
			c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
		}
	}
	decision, err := policyService.Evaluate(c.Request.Context(), service.CodexPlusGatewayPolicyInput{
		APIKey:         apiKey,
		User:           codexPlusGatewayPolicyUser(apiKey),
		Group:          codexPlusGatewayPolicyGroup(apiKey),
		Subscription:   subscription,
		RequestedModel: strings.TrimSpace(requestedModel),
		Endpoint:       codexPlusGatewayPolicyEndpoint(c, endpoint),
		RequestID:      codexPlusGatewayPolicyRequestID(c),
		DeviceID:       codexPlusGatewayPolicyDeviceID(c),
		Platform:       service.QuotaPlatform(c.Request.Context(), apiKey),
		Entitlement: service.CodexPlusGatewayEntitlementContext{
			Status: service.CodexPlusServiceStatusAvailable,
		},
		StrictDeviceEnforcement: false,
		CheckBilling:            true,
		Metadata: map[string]any{
			"method": c.Request.Method,
			"path":   c.Request.URL.Path,
		},
	})
	if decision != nil && (decision.Allowed || decision.Skipped) {
		return true
	}
	status, code, message := codexPlusGatewayPolicyError(err, decision)
	writeError(status, code, message)
	return false
}

func codexPlusGatewayPolicyUser(apiKey *service.APIKey) *service.User {
	if apiKey == nil {
		return nil
	}
	return apiKey.User
}

func codexPlusGatewayPolicyGroup(apiKey *service.APIKey) *service.Group {
	if apiKey == nil {
		return nil
	}
	return apiKey.Group
}

func codexPlusGatewayPolicyEndpoint(c *gin.Context, fallback string) string {
	if value := strings.TrimSpace(fallback); value != "" {
		return value
	}
	if c != nil && c.Request != nil && c.Request.URL != nil {
		return c.Request.URL.Path
	}
	return ""
}

func codexPlusGatewayPolicyRequestID(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	ctx := c.Request.Context()
	if value, ok := ctx.Value(ctxkey.RequestID).(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	if value, ok := ctx.Value(ctxkey.ClientRequestID).(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	for _, header := range []string{"X-Request-ID", "X-Request-Id", "X-Client-Request-ID"} {
		if value := strings.TrimSpace(c.GetHeader(header)); value != "" {
			return value
		}
	}
	return ""
}

func codexPlusGatewayPolicyDeviceID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	for _, header := range []string{"X-CodexPlus-Device-Id", "X-CodexPlus-Device-ID", "X-Codex-Device-ID"} {
		if value := strings.TrimSpace(c.GetHeader(header)); value != "" {
			return value
		}
	}
	return ""
}

func codexPlusGatewayPolicyError(err error, decision *service.CodexPlusGatewayPolicyDecision) (int, string, string) {
	status := http.StatusServiceUnavailable
	code := service.CodexPlusGatewayErrorConfigUnavailable
	message := "Codex++ gateway policy is temporarily unavailable."
	if decision != nil {
		if decision.HTTPStatus > 0 {
			status = decision.HTTPStatus
		}
		if strings.TrimSpace(decision.ErrorCode) != "" {
			code = strings.TrimSpace(decision.ErrorCode)
		}
		if strings.TrimSpace(decision.Reason) != "" {
			message = strings.TrimSpace(decision.Reason)
		}
	}
	if err != nil && decision == nil {
		message = fmt.Sprintf("%s: %v", message, err)
	}
	return status, code, message
}
