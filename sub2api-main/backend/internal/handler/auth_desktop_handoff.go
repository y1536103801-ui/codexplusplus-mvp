package handler

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/ent/pendingauthsession"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	desktopLoginAuthorizePath       = "/auth/desktop/authorize"
	desktopLoginProviderType        = "email"
	desktopLoginProviderKey         = "codexplus-desktop"
	desktopLoginFlowStepPending     = "desktop_pending"
	desktopLoginFlowStepCompleted   = "desktop_completed"
	desktopLoginPollIntervalSeconds = 2

	desktopLoginSessionCreateFailed = "CLIENT_AUTH_DESKTOP_SESSION_CREATE_FAILED"
	desktopLoginSessionInvalid      = "CLIENT_AUTH_DESKTOP_SESSION_INVALID"
	desktopLoginTargetMismatch      = "CLIENT_AUTH_DESKTOP_TARGET_MISMATCH"
	desktopLoginServiceNotReady     = "CLIENT_AUTH_DESKTOP_SERVICE_NOT_READY"
	desktopLoginSessionUpdateFailed = "CLIENT_AUTH_DESKTOP_SESSION_UPDATE_FAILED"
)

type desktopLoginStartRequest struct {
	DeviceID   string `json:"device_id,omitempty"`
	DeviceName string `json:"device_name,omitempty"`
}

type desktopLoginStartResponse struct {
	SessionToken        string    `json:"session_token"`
	PollToken           string    `json:"poll_token"`
	AuthorizeURL        string    `json:"authorize_url"`
	VerificationCode    string    `json:"verification_code"`
	ExpiresAt           time.Time `json:"expires_at"`
	PollIntervalSeconds int       `json:"poll_interval_seconds"`
}

type desktopLoginCompleteRequest struct {
	SessionToken string `json:"session_token" binding:"required"`
}

type desktopLoginPollRequest struct {
	SessionToken string `json:"session_token" binding:"required"`
	PollToken    string `json:"poll_token" binding:"required"`
}

type desktopLoginCompleteResponse struct {
	Status string `json:"status"`
}

type desktopLoginPollResponse struct {
	Status       string    `json:"status"`
	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresIn    int       `json:"expires_in,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	User         *dto.User `json:"user,omitempty"`
}

// StartDesktopLogin creates a browser-approved desktop login session.
// POST /api/v1/auth/desktop/start
func (h *AuthHandler) StartDesktopLogin(c *gin.Context) {
	var req desktopLoginStartRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	pendingSvc, err := h.pendingIdentityService()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	sessionToken, err := generateOAuthPendingBrowserSession()
	if err != nil {
		response.ErrorFrom(c, infraerrors.InternalServer(desktopLoginSessionCreateFailed, "failed to create desktop login session").WithCause(err))
		return
	}
	pollToken, err := generateOAuthPendingBrowserSession()
	if err != nil {
		response.ErrorFrom(c, infraerrors.InternalServer(desktopLoginSessionCreateFailed, "failed to create desktop login session").WithCause(err))
		return
	}
	verificationCode, err := generateDesktopLoginVerificationCode()
	if err != nil {
		response.ErrorFrom(c, infraerrors.InternalServer(desktopLoginSessionCreateFailed, "failed to create desktop login session").WithCause(err))
		return
	}

	session, err := pendingSvc.CreatePendingSession(c.Request.Context(), service.CreatePendingAuthSessionInput{
		SessionToken: sessionToken,
		Intent:       oauthIntentLogin,
		Identity: service.PendingAuthIdentityKey{
			ProviderType:    desktopLoginProviderType,
			ProviderKey:     desktopLoginProviderKey,
			ProviderSubject: sessionToken,
		},
		BrowserSessionKey: pollToken,
		LocalFlowState: map[string]any{
			"step":        desktopLoginFlowStepPending,
			"client":      "codex-plus-desktop",
			"device_id":   strings.TrimSpace(req.DeviceID),
			"device_name": strings.TrimSpace(req.DeviceName),
			"verify_code": verificationCode,
			"created_at":  time.Now().UTC().Format(time.RFC3339),
		},
	})
	if err != nil {
		response.ErrorFrom(c, infraerrors.InternalServer(desktopLoginSessionCreateFailed, "failed to create desktop login session").WithCause(err))
		return
	}

	h.recordDesktopLoginEvent(c, "desktop_login_started", nil, req.DeviceID, map[string]any{
		"status":                "pending",
		"expires_at":            session.ExpiresAt.UTC().Format(time.RFC3339),
		"poll_interval_seconds": desktopLoginPollIntervalSeconds,
		"has_authorize_url":     true,
		"client_ip":             ip.GetClientIP(c),
	})

	response.Success(c, desktopLoginStartResponse{
		SessionToken:        session.SessionToken,
		PollToken:           pollToken,
		AuthorizeURL:        h.buildDesktopLoginAuthorizeURL(c, session.SessionToken, verificationCode),
		VerificationCode:    verificationCode,
		ExpiresAt:           session.ExpiresAt,
		PollIntervalSeconds: desktopLoginPollIntervalSeconds,
	})
}

// CompleteDesktopLogin approves a pending desktop login using the browser's authenticated JWT.
// POST /api/v1/auth/desktop/complete
func (h *AuthHandler) CompleteDesktopLogin(c *gin.Context) {
	var req desktopLoginCompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	user, err := h.userService.GetByID(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := ensureLoginUserActive(user); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	pendingSvc, err := h.pendingIdentityService()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	session, err := pendingSvc.GetSessionForBrowserApproval(c.Request.Context(), req.SessionToken)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := ensureDesktopLoginSession(sessionIntentProvider{
		intent:       session.Intent,
		providerType: session.ProviderType,
		providerKey:  session.ProviderKey,
	}); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if session.TargetUserID != nil && *session.TargetUserID > 0 && *session.TargetUserID != user.ID {
		response.ErrorFrom(c, infraerrors.Conflict(desktopLoginTargetMismatch, "desktop login session has already been approved by another user"))
		return
	}

	now := time.Now().UTC()
	localFlowState := clonePendingMap(session.LocalFlowState)
	localFlowState["step"] = desktopLoginFlowStepCompleted
	localFlowState["approved_at"] = now.Format(time.RFC3339)
	localFlowState["approved_user_id"] = user.ID

	client := h.entClient()
	if client == nil {
		response.ErrorFrom(c, infraerrors.ServiceUnavailable(desktopLoginServiceNotReady, "pending auth service is not ready"))
		return
	}
	_, err = client.PendingAuthSession.UpdateOneID(session.ID).
		Where(
			pendingauthsession.ConsumedAtIsNil(),
			pendingauthsession.ExpiresAtGTE(now),
		).
		SetTargetUserID(user.ID).
		SetResolvedEmail(user.Email).
		SetLocalFlowState(localFlowState).
		Save(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, infraerrors.InternalServer(desktopLoginSessionUpdateFailed, "failed to approve desktop login session").WithCause(err))
		return
	}

	userID := user.ID
	h.recordDesktopLoginEvent(c, "desktop_login_completed", &userID, desktopLoginDeviceID(session.LocalFlowState), map[string]any{
		"status":        "completed",
		"approved_at":   now.Format(time.RFC3339),
		"has_token":     false,
		"redacted_auth": true,
	})

	response.Success(c, desktopLoginCompleteResponse{Status: "completed"})
}

// PollDesktopLogin lets the desktop redeem a browser-approved session using its private poll token.
// POST /api/v1/auth/desktop/poll
func (h *AuthHandler) PollDesktopLogin(c *gin.Context) {
	var req desktopLoginPollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	pendingSvc, err := h.pendingIdentityService()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	session, err := pendingSvc.GetBrowserSession(c.Request.Context(), req.SessionToken, req.PollToken)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := ensureDesktopLoginSession(sessionIntentProvider{
		intent:       session.Intent,
		providerType: session.ProviderType,
		providerKey:  session.ProviderKey,
	}); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if session.TargetUserID == nil || *session.TargetUserID <= 0 || desktopLoginFlowStep(session.LocalFlowState) != desktopLoginFlowStepCompleted {
		response.Success(c, desktopLoginPollResponse{Status: "pending"})
		return
	}

	user, err := h.userService.GetByID(c.Request.Context(), *session.TargetUserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := ensureLoginUserActive(user); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if _, err := pendingSvc.ConsumeBrowserSession(c.Request.Context(), session.SessionToken, session.BrowserSessionKey); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	tokenPair, err := h.authService.GenerateTokenPair(c.Request.Context(), user, "")
	if err != nil {
		response.InternalError(c, "Failed to generate token pair")
		return
	}

	h.authService.RecordSuccessfulLogin(c.Request.Context(), user.ID)
	userID := user.ID
	h.recordDesktopLoginEvent(c, "desktop_login_polled", &userID, desktopLoginDeviceID(session.LocalFlowState), map[string]any{
		"status":        "completed",
		"token_issued":  true,
		"redacted_auth": true,
	})
	response.Success(c, desktopLoginPollResponse{
		Status:       "completed",
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
		TokenType:    "Bearer",
		User:         dto.UserFromService(user),
	})
}

type sessionIntentProvider struct {
	intent       string
	providerType string
	providerKey  string
}

func ensureDesktopLoginSession(session sessionIntentProvider) error {
	if !strings.EqualFold(strings.TrimSpace(session.intent), oauthIntentLogin) ||
		!strings.EqualFold(strings.TrimSpace(session.providerType), desktopLoginProviderType) ||
		strings.TrimSpace(session.providerKey) != desktopLoginProviderKey {
		return infraerrors.BadRequest(desktopLoginSessionInvalid, "desktop login session is invalid")
	}
	return nil
}

func desktopLoginFlowStep(localFlowState map[string]any) string {
	value, ok := localFlowState["step"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func desktopLoginDeviceID(localFlowState map[string]any) string {
	value, ok := localFlowState["device_id"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func (h *AuthHandler) recordDesktopLoginEvent(c *gin.Context, eventType string, userID *int64, deviceID string, payload map[string]any) {
	if h == nil || h.codexPlusEventRepo == nil || c == nil || c.Request == nil {
		return
	}

	normalizedPayload := clonePendingMap(payload)
	normalizedPayload["redaction_applied"] = true
	input := service.CodexPlusEventCreate{
		EventType: strings.TrimSpace(eventType),
		Severity:  "info",
		Payload:   normalizedPayload,
	}
	if userID != nil && *userID > 0 {
		id := *userID
		input.UserID = &id
	}
	if trimmedDeviceID := strings.TrimSpace(deviceID); trimmedDeviceID != "" {
		input.DeviceID = &trimmedDeviceID
	}
	if requestID := desktopLoginRequestID(c); requestID != "" {
		input.RequestID = &requestID
	}
	_, _ = h.codexPlusEventRepo.Append(c.Request.Context(), input)
}

func desktopLoginRequestID(c *gin.Context) string {
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

func generateDesktopLoginVerificationCode() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(fmt.Sprintf("%06d", value.Int64())), nil
}

func (h *AuthHandler) buildDesktopLoginAuthorizeURL(c *gin.Context, sessionToken string, verificationCode string) string {
	base := ""
	if h != nil && h.settingSvc != nil {
		base = strings.TrimSpace(h.settingSvc.GetFrontendURL(c.Request.Context()))
	}
	if base == "" {
		scheme := "http"
		if isRequestHTTPS(c) {
			scheme = "https"
		}
		base = scheme + "://" + strings.TrimSpace(c.Request.Host)
	}

	query := url.Values{"session_token": []string{sessionToken}}
	if strings.TrimSpace(verificationCode) != "" {
		query.Set("verification_code", strings.TrimSpace(verificationCode))
	}

	u, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return desktopLoginAuthorizePath + "?" + query.Encode()
	}
	u.Path = desktopLoginAuthorizePath
	u.RawQuery = query.Encode()
	u.Fragment = ""
	return u.String()
}
