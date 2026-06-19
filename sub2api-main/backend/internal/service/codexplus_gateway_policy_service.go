package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	CodexPlusGatewayPolicyEventRejected = "gateway_policy_rejected"
	CodexPlusGatewayPolicyEventUsage    = "usage_recorded"

	CodexPlusServiceStatusAvailable        = "available"
	CodexPlusServiceStatusNotPurchased     = "not_purchased"
	CodexPlusServiceStatusExpired          = "expired"
	CodexPlusServiceStatusLowBalance       = "low_balance"
	CodexPlusServiceStatusDisabled         = "disabled"
	CodexPlusServiceStatusDeviceRevoked    = "device_revoked"
	CodexPlusServiceStatusModelUnavailable = "model_unavailable"
	CodexPlusServiceStatusRateLimited      = "rate_limited"
	CodexPlusServiceStatusGatewayUnhealthy = "gateway_unhealthy"

	CodexPlusGatewayErrorModelNotAllowed      = "GATEWAY_POLICY_MODEL_NOT_ALLOWED"
	CodexPlusGatewayErrorBalanceInsufficient  = "GATEWAY_POLICY_BALANCE_INSUFFICIENT"
	CodexPlusGatewayErrorEntitlementExpired   = "GATEWAY_POLICY_ENTITLEMENT_EXPIRED"
	CodexPlusGatewayErrorDeviceRevoked        = "GATEWAY_POLICY_DEVICE_REVOKED"
	CodexPlusGatewayErrorRateLimited          = "GATEWAY_POLICY_RATE_LIMITED"
	CodexPlusGatewayErrorConfigUnavailable    = "GATEWAY_POLICY_CONFIG_UNAVAILABLE"
	CodexPlusGatewayErrorEntitlementDisabled  = "GATEWAY_POLICY_ENTITLEMENT_DISABLED"
	CodexPlusGatewayErrorEntitlementNotBought = "GATEWAY_POLICY_ENTITLEMENT_NOT_PURCHASED"
	CodexPlusGatewayErrorDeviceBlocked        = "GATEWAY_POLICY_DEVICE_BLOCKED"
	CodexPlusGatewayErrorQuotaExceeded        = "GATEWAY_POLICY_QUOTA_EXCEEDED"
	CodexPlusGatewayErrorConcurrencyLimited   = "GATEWAY_POLICY_CONCURRENCY_LIMITED"
	CodexPlusGatewayErrorTPMLimited           = "GATEWAY_POLICY_TPM_LIMITED"
)

var ErrCodexPlusManagedProviderKeyNotFound = errors.New("codexplus managed provider key not found")
var ErrCodexPlusDeviceNotFound = errors.New("codexplus device not found")

type CodexPlusGatewayPolicyConfigReader interface {
	Get(ctx context.Context) (*CodexPlusConfig, error)
}

type CodexPlusManagedProviderKeyReader interface {
	GetByAPIKeyID(ctx context.Context, apiKeyID int64) (*CodexPlusManagedProviderKey, error)
}

type CodexPlusDeviceReader interface {
	GetByUserAndDevice(ctx context.Context, userID int64, deviceID string) (*CodexPlusDevice, error)
}

type CodexPlusGatewayPolicyEventRecorder interface {
	Append(ctx context.Context, input CodexPlusEventCreate) (*CodexPlusEvent, error)
}

type CodexPlusGatewayBillingChecker interface {
	CheckBillingEligibility(ctx context.Context, user *User, apiKey *APIKey, group *Group, subscription *UserSubscription, platform string) error
}

type CodexPlusGatewayEntitlementContext struct {
	Status      string
	PlanID      string
	ModelGroups []string
}

type CodexPlusGatewayPolicyInput struct {
	APIKey       *APIKey
	User         *User
	Group        *Group
	Subscription *UserSubscription

	RequestedModel  string
	Endpoint        string
	RequestID       string
	DeviceID        string
	Platform        string
	EstimatedTokens int

	Entitlement CodexPlusGatewayEntitlementContext

	StrictDeviceEnforcement bool
	CheckBilling            bool
	Metadata                map[string]any
}

type CodexPlusGatewayPolicyDecision struct {
	Allowed           bool
	Skipped           bool
	HTTPStatus        int
	ErrorCode         string
	ServiceStatus     string
	Reason            string
	Retryable         bool
	EventType         string
	ConfigVersion     string
	ManagedKey        *CodexPlusManagedProviderKey
	Model             *CodexPlusModel
	EventPayload      map[string]any
	EventError        error
	UsageEventPayload map[string]any
	UsageEventError   error
}

type CodexPlusGatewayPolicyService struct {
	configReader     CodexPlusGatewayPolicyConfigReader
	managedKeyReader CodexPlusManagedProviderKeyReader
	deviceReader     CodexPlusDeviceReader
	eventRecorder    CodexPlusGatewayPolicyEventRecorder
	billingChecker   CodexPlusGatewayBillingChecker
	now              func() time.Time
}

type codexPlusGatewayPolicyResolution struct {
	Entitlement CodexPlusGatewayEntitlementContext
	Plan        *CodexPlusPlan
	UsageRule   *CodexPlusUsageRule
}

type CodexPlusGatewayPolicyServiceOption func(*CodexPlusGatewayPolicyService)

func NewCodexPlusGatewayPolicyService(opts ...CodexPlusGatewayPolicyServiceOption) *CodexPlusGatewayPolicyService {
	s := &CodexPlusGatewayPolicyService{now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func WithCodexPlusGatewayPolicyConfigReader(reader CodexPlusGatewayPolicyConfigReader) CodexPlusGatewayPolicyServiceOption {
	return func(s *CodexPlusGatewayPolicyService) { s.configReader = reader }
}

func WithCodexPlusManagedProviderKeyReader(reader CodexPlusManagedProviderKeyReader) CodexPlusGatewayPolicyServiceOption {
	return func(s *CodexPlusGatewayPolicyService) { s.managedKeyReader = reader }
}

func WithCodexPlusGatewayDeviceReader(reader CodexPlusDeviceReader) CodexPlusGatewayPolicyServiceOption {
	return func(s *CodexPlusGatewayPolicyService) { s.deviceReader = reader }
}

func WithCodexPlusGatewayPolicyEventRecorder(recorder CodexPlusGatewayPolicyEventRecorder) CodexPlusGatewayPolicyServiceOption {
	return func(s *CodexPlusGatewayPolicyService) { s.eventRecorder = recorder }
}

func WithCodexPlusGatewayBillingChecker(checker CodexPlusGatewayBillingChecker) CodexPlusGatewayPolicyServiceOption {
	return func(s *CodexPlusGatewayPolicyService) { s.billingChecker = checker }
}

func WithCodexPlusGatewayPolicyClock(now func() time.Time) CodexPlusGatewayPolicyServiceOption {
	return func(s *CodexPlusGatewayPolicyService) {
		if now != nil {
			s.now = now
		}
	}
}

func (s *CodexPlusGatewayPolicyService) Evaluate(ctx context.Context, input CodexPlusGatewayPolicyInput) (*CodexPlusGatewayPolicyDecision, error) {
	if input.APIKey == nil {
		return s.reject(ctx, input, nil, nil, "", http.StatusServiceUnavailable, CodexPlusGatewayErrorConfigUnavailable, CodexPlusServiceStatusGatewayUnhealthy, "authenticated API key context is missing", true), nil
	}
	if input.User == nil {
		input.User = input.APIKey.User
	}
	if input.User == nil {
		return s.reject(ctx, input, nil, nil, "", http.StatusServiceUnavailable, CodexPlusGatewayErrorConfigUnavailable, CodexPlusServiceStatusGatewayUnhealthy, "authenticated user context is missing", true), nil
	}

	managedKey, err := s.lookupManagedKey(ctx, input.APIKey.ID)
	if err != nil {
		if errors.Is(err, ErrCodexPlusManagedProviderKeyNotFound) {
			return &CodexPlusGatewayPolicyDecision{
				Allowed:       true,
				Skipped:       true,
				ServiceStatus: CodexPlusServiceStatusAvailable,
				Reason:        "api key is not managed by Codex++",
			}, nil
		}
		return s.reject(ctx, input, nil, nil, "", http.StatusServiceUnavailable, CodexPlusGatewayErrorConfigUnavailable, CodexPlusServiceStatusGatewayUnhealthy, "managed provider key lookup failed", true), err
	}
	if strings.TrimSpace(managedKey.ManagedProviderID) != CodexPlusManagedProviderID {
		return &CodexPlusGatewayPolicyDecision{
			Allowed:       true,
			Skipped:       true,
			ServiceStatus: CodexPlusServiceStatusAvailable,
			Reason:        "api key belongs to another managed provider",
			ManagedKey:    managedKey,
		}, nil
	}
	if managedKey.APIKeyID != input.APIKey.ID || managedKey.UserID != input.User.ID || managedKey.UserID != input.APIKey.UserID {
		return s.reject(ctx, input, managedKey, nil, "", http.StatusForbidden, CodexPlusGatewayErrorEntitlementDisabled, CodexPlusServiceStatusDisabled, "managed provider key ownership mismatch", false), nil
	}
	if !isCodexPlusActiveStatus(managedKey.Status) {
		return s.reject(ctx, input, managedKey, nil, "", http.StatusForbidden, CodexPlusGatewayErrorEntitlementDisabled, CodexPlusServiceStatusDisabled, "managed provider key is not active", false), nil
	}

	cfg, err := s.getConfig(ctx)
	if err != nil {
		return s.reject(ctx, input, managedKey, nil, "", http.StatusServiceUnavailable, CodexPlusGatewayErrorConfigUnavailable, CodexPlusServiceStatusGatewayUnhealthy, "Codex++ gateway policy config is unavailable", true), err
	}
	input.StrictDeviceEnforcement = input.StrictDeviceEnforcement || codexPlusStrictDeviceEnforcementEnabled(cfg)
	resolved := resolveCodexPlusGatewayPolicy(cfg, input)
	input.Entitlement = resolved.Entitlement
	if resolved.UsageRule != nil && resolved.UsageRule.DevicePolicy.StrictEnforcementDefault {
		input.StrictDeviceEnforcement = true
	}

	if decision := s.checkEntitlement(ctx, input, managedKey, cfg); decision != nil {
		return decision, nil
	}

	model := findCodexPlusModel(cfg, input.RequestedModel)
	if model == nil {
		return s.reject(ctx, input, managedKey, nil, cfg.ConfigVersion, http.StatusForbidden, CodexPlusGatewayErrorModelNotAllowed, CodexPlusServiceStatusModelUnavailable, "requested model is not in Codex++ model catalog", false), nil
	}
	if !model.IsEnabled {
		return s.reject(ctx, input, managedKey, model, cfg.ConfigVersion, http.StatusForbidden, CodexPlusGatewayErrorModelNotAllowed, CodexPlusServiceStatusModelUnavailable, "requested model is disabled", false), nil
	}
	if !modelAllowedByEntitlement(cfg, model, input.Entitlement) {
		return s.reject(ctx, input, managedKey, model, cfg.ConfigVersion, http.StatusForbidden, CodexPlusGatewayErrorModelNotAllowed, CodexPlusServiceStatusModelUnavailable, "requested model is outside the Codex++ entitlement", false), nil
	}

	if decision := s.checkUsagePolicy(ctx, input, managedKey, model, cfg.ConfigVersion, resolved); decision != nil {
		return decision, nil
	}

	if decision := s.checkDevice(ctx, input, managedKey, model, cfg.ConfigVersion); decision != nil {
		return decision, nil
	}

	if input.CheckBilling && s.billingChecker != nil {
		if err := s.billingChecker.CheckBillingEligibility(ctx, input.User, input.APIKey, input.Group, input.Subscription, input.Platform); err != nil {
			return s.rejectForBillingError(ctx, input, managedKey, model, cfg.ConfigVersion, err), nil
		}
	}

	return s.allow(ctx, input, managedKey, model, cfg.ConfigVersion, resolved), nil
}

func (s *CodexPlusGatewayPolicyService) lookupManagedKey(ctx context.Context, apiKeyID int64) (*CodexPlusManagedProviderKey, error) {
	if s == nil || s.managedKeyReader == nil {
		return nil, ErrCodexPlusManagedProviderKeyNotFound
	}
	key, err := s.managedKeyReader.GetByAPIKeyID(ctx, apiKeyID)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, ErrCodexPlusManagedProviderKeyNotFound
	}
	return key, nil
}

func (s *CodexPlusGatewayPolicyService) getConfig(ctx context.Context) (*CodexPlusConfig, error) {
	if s == nil || s.configReader == nil {
		return nil, fmt.Errorf("codexplus gateway policy config reader is not initialized")
	}
	return s.configReader.Get(ctx)
}

func (s *CodexPlusGatewayPolicyService) checkEntitlement(ctx context.Context, input CodexPlusGatewayPolicyInput, managedKey *CodexPlusManagedProviderKey, cfg *CodexPlusConfig) *CodexPlusGatewayPolicyDecision {
	switch strings.TrimSpace(input.Entitlement.Status) {
	case "", CodexPlusServiceStatusAvailable:
	case CodexPlusServiceStatusNotPurchased:
		return s.reject(ctx, input, managedKey, nil, cfg.ConfigVersion, http.StatusForbidden, CodexPlusGatewayErrorEntitlementNotBought, CodexPlusServiceStatusNotPurchased, "Codex++ entitlement is missing", false)
	case CodexPlusServiceStatusExpired:
		return s.reject(ctx, input, managedKey, nil, cfg.ConfigVersion, http.StatusForbidden, CodexPlusGatewayErrorEntitlementExpired, CodexPlusServiceStatusExpired, "Codex++ entitlement is expired", false)
	case CodexPlusServiceStatusDisabled:
		return s.reject(ctx, input, managedKey, nil, cfg.ConfigVersion, http.StatusForbidden, CodexPlusGatewayErrorEntitlementDisabled, CodexPlusServiceStatusDisabled, "Codex++ entitlement is disabled", false)
	case CodexPlusServiceStatusLowBalance:
		return s.reject(ctx, input, managedKey, nil, cfg.ConfigVersion, http.StatusPaymentRequired, CodexPlusGatewayErrorBalanceInsufficient, CodexPlusServiceStatusLowBalance, "Codex++ balance or quota is exhausted", false)
	default:
		return s.reject(ctx, input, managedKey, nil, cfg.ConfigVersion, http.StatusForbidden, CodexPlusGatewayErrorEntitlementDisabled, CodexPlusServiceStatusDisabled, "Codex++ entitlement status is not allowed", false)
	}

	if input.Subscription != nil && input.Subscription.Status != "" && input.Subscription.Status != SubscriptionStatusActive {
		return s.reject(ctx, input, managedKey, nil, cfg.ConfigVersion, http.StatusForbidden, CodexPlusGatewayErrorEntitlementExpired, CodexPlusServiceStatusExpired, "subscription is not active", false)
	}
	if input.Subscription != nil && !input.Subscription.ExpiresAt.IsZero() && !s.now().Before(input.Subscription.ExpiresAt) {
		return s.reject(ctx, input, managedKey, nil, cfg.ConfigVersion, http.StatusForbidden, CodexPlusGatewayErrorEntitlementExpired, CodexPlusServiceStatusExpired, "subscription is expired", false)
	}
	return nil
}

func (s *CodexPlusGatewayPolicyService) checkDevice(ctx context.Context, input CodexPlusGatewayPolicyInput, managedKey *CodexPlusManagedProviderKey, model *CodexPlusModel, configVersion string) *CodexPlusGatewayPolicyDecision {
	deviceID := strings.TrimSpace(input.DeviceID)
	if deviceID == "" {
		if input.StrictDeviceEnforcement {
			return s.reject(ctx, input, managedKey, model, configVersion, http.StatusForbidden, CodexPlusGatewayErrorDeviceRevoked, CodexPlusServiceStatusDeviceRevoked, "Codex++ device context is required", false)
		}
		return nil
	}
	if s == nil || s.deviceReader == nil {
		if input.StrictDeviceEnforcement {
			return s.reject(ctx, input, managedKey, model, configVersion, http.StatusServiceUnavailable, CodexPlusGatewayErrorConfigUnavailable, CodexPlusServiceStatusGatewayUnhealthy, "Codex++ device policy reader is unavailable", true)
		}
		return nil
	}
	device, err := s.deviceReader.GetByUserAndDevice(ctx, input.User.ID, deviceID)
	if err != nil {
		if errors.Is(err, ErrCodexPlusDeviceNotFound) {
			if input.StrictDeviceEnforcement {
				return s.reject(ctx, input, managedKey, model, configVersion, http.StatusForbidden, CodexPlusGatewayErrorDeviceRevoked, CodexPlusServiceStatusDeviceRevoked, "Codex++ device is unknown", false)
			}
			return nil
		}
		return s.reject(ctx, input, managedKey, model, configVersion, http.StatusServiceUnavailable, CodexPlusGatewayErrorConfigUnavailable, CodexPlusServiceStatusGatewayUnhealthy, "Codex++ device policy lookup failed", true)
	}
	if device == nil {
		if input.StrictDeviceEnforcement {
			return s.reject(ctx, input, managedKey, model, configVersion, http.StatusForbidden, CodexPlusGatewayErrorDeviceRevoked, CodexPlusServiceStatusDeviceRevoked, "Codex++ device is unknown", false)
		}
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(device.Status)) {
	case "", StatusActive:
		return nil
	case "blocked":
		return s.reject(ctx, input, managedKey, model, configVersion, http.StatusForbidden, CodexPlusGatewayErrorDeviceBlocked, CodexPlusServiceStatusDeviceRevoked, "Codex++ device is blocked", false)
	case "revoked":
		return s.reject(ctx, input, managedKey, model, configVersion, http.StatusForbidden, CodexPlusGatewayErrorDeviceRevoked, CodexPlusServiceStatusDeviceRevoked, "Codex++ device is revoked", false)
	default:
		return s.reject(ctx, input, managedKey, model, configVersion, http.StatusForbidden, CodexPlusGatewayErrorDeviceRevoked, CodexPlusServiceStatusDeviceRevoked, "Codex++ device status is not allowed", false)
	}
}

func (s *CodexPlusGatewayPolicyService) checkUsagePolicy(ctx context.Context, input CodexPlusGatewayPolicyInput, managedKey *CodexPlusManagedProviderKey, model *CodexPlusModel, configVersion string, resolved codexPlusGatewayPolicyResolution) *CodexPlusGatewayPolicyDecision {
	rule := resolved.UsageRule
	if rule == nil {
		return s.reject(ctx, input, managedKey, model, configVersion, http.StatusServiceUnavailable, CodexPlusGatewayErrorConfigUnavailable, CodexPlusServiceStatusGatewayUnhealthy, "Codex++ usage policy is unavailable", true)
	}
	if rule.ConcurrencyLimit < 1 {
		return s.reject(ctx, input, managedKey, model, configVersion, http.StatusTooManyRequests, CodexPlusGatewayErrorConcurrencyLimited, CodexPlusServiceStatusRateLimited, "Codex++ concurrency limit is exhausted", true)
	}
	if rule.RPMLimit < 1 {
		return s.reject(ctx, input, managedKey, model, configVersion, http.StatusTooManyRequests, CodexPlusGatewayErrorRateLimited, CodexPlusServiceStatusRateLimited, "Codex++ RPM limit is exhausted", true)
	}
	if rule.TPMLimit < 1 {
		return s.reject(ctx, input, managedKey, model, configVersion, http.StatusTooManyRequests, CodexPlusGatewayErrorTPMLimited, CodexPlusServiceStatusRateLimited, "Codex++ TPM limit is exhausted", true)
	}
	if input.EstimatedTokens > 0 && input.EstimatedTokens > rule.TPMLimit {
		return s.reject(ctx, input, managedKey, model, configVersion, http.StatusTooManyRequests, CodexPlusGatewayErrorTPMLimited, CodexPlusServiceStatusRateLimited, "Codex++ estimated tokens exceed TPM limit", true)
	}
	return nil
}

func (s *CodexPlusGatewayPolicyService) rejectForBillingError(ctx context.Context, input CodexPlusGatewayPolicyInput, managedKey *CodexPlusManagedProviderKey, model *CodexPlusModel, configVersion string, err error) *CodexPlusGatewayPolicyDecision {
	switch {
	case errors.Is(err, ErrInsufficientBalance):
		return s.reject(ctx, input, managedKey, model, configVersion, http.StatusPaymentRequired, CodexPlusGatewayErrorBalanceInsufficient, CodexPlusServiceStatusLowBalance, "Codex++ balance is insufficient", false)
	case errors.Is(err, ErrSubscriptionInvalid), errors.Is(err, ErrSubscriptionExpired), errors.Is(err, ErrSubscriptionSuspended):
		return s.reject(ctx, input, managedKey, model, configVersion, http.StatusForbidden, CodexPlusGatewayErrorEntitlementExpired, CodexPlusServiceStatusExpired, "Codex++ subscription is not eligible", false)
	case errors.Is(err, ErrDailyLimitExceeded), errors.Is(err, ErrWeeklyLimitExceeded), errors.Is(err, ErrMonthlyLimitExceeded):
		return s.reject(ctx, input, managedKey, model, configVersion, http.StatusTooManyRequests, CodexPlusGatewayErrorQuotaExceeded, CodexPlusServiceStatusRateLimited, "Codex++ usage quota is exhausted", true)
	case errors.Is(err, ErrUserPlatformDailyQuotaExhausted), errors.Is(err, ErrUserPlatformWeeklyQuotaExhausted), errors.Is(err, ErrUserPlatformMonthlyQuotaExhausted):
		return s.reject(ctx, input, managedKey, model, configVersion, http.StatusTooManyRequests, CodexPlusGatewayErrorQuotaExceeded, CodexPlusServiceStatusRateLimited, "Codex++ platform usage quota is exhausted", true)
	case errors.Is(err, ErrAPIKeyRateLimit5hExceeded), errors.Is(err, ErrAPIKeyRateLimit1dExceeded), errors.Is(err, ErrAPIKeyRateLimit7dExceeded),
		errors.Is(err, ErrGroupRPMExceeded), errors.Is(err, ErrUserRPMExceeded):
		return s.reject(ctx, input, managedKey, model, configVersion, http.StatusTooManyRequests, CodexPlusGatewayErrorRateLimited, CodexPlusServiceStatusRateLimited, "Codex++ request rate or usage window is limited", true)
	case errors.Is(err, ErrBillingServiceUnavailable):
		return s.reject(ctx, input, managedKey, model, configVersion, http.StatusServiceUnavailable, CodexPlusGatewayErrorConfigUnavailable, CodexPlusServiceStatusGatewayUnhealthy, "Codex++ billing eligibility is unavailable", true)
	default:
		return s.reject(ctx, input, managedKey, model, configVersion, http.StatusServiceUnavailable, CodexPlusGatewayErrorConfigUnavailable, CodexPlusServiceStatusGatewayUnhealthy, "Codex++ billing eligibility failed", true)
	}
}

func (s *CodexPlusGatewayPolicyService) allow(ctx context.Context, input CodexPlusGatewayPolicyInput, managedKey *CodexPlusManagedProviderKey, model *CodexPlusModel, configVersion string, resolved codexPlusGatewayPolicyResolution) *CodexPlusGatewayPolicyDecision {
	payload := BuildCodexPlusGatewayPolicyUsagePayload(input, managedKey, model, configVersion, resolved, s.currentTime())
	decision := &CodexPlusGatewayPolicyDecision{
		Allowed:           true,
		HTTPStatus:        http.StatusOK,
		ServiceStatus:     CodexPlusServiceStatusAvailable,
		Reason:            "Codex++ gateway policy passed",
		EventType:         CodexPlusGatewayPolicyEventUsage,
		ConfigVersion:     configVersion,
		ManagedKey:        managedKey,
		Model:             model,
		UsageEventPayload: payload,
	}
	if s != nil && s.eventRecorder != nil {
		event := CodexPlusEventCreate{
			UserID:        int64PtrIfPositive(userIDFromGatewayPolicyInput(input)),
			DeviceID:      stringPtrIfNotBlank(input.DeviceID),
			EventType:     CodexPlusGatewayPolicyEventUsage,
			Severity:      "info",
			RequestID:     stringPtrIfNotBlank(input.RequestID),
			ConfigVersion: stringPtrIfNotBlank(configVersion),
			Payload:       payload,
		}
		_, decision.UsageEventError = s.eventRecorder.Append(ctx, event)
	}
	return decision
}

func (s *CodexPlusGatewayPolicyService) reject(ctx context.Context, input CodexPlusGatewayPolicyInput, managedKey *CodexPlusManagedProviderKey, model *CodexPlusModel, configVersion string, httpStatus int, errorCode, serviceStatus, reason string, retryable bool) *CodexPlusGatewayPolicyDecision {
	payload := BuildCodexPlusGatewayPolicyRejectionPayload(input, managedKey, model, configVersion, errorCode, serviceStatus, reason, s.currentTime())
	payload["http_status"] = httpStatus
	payload["retryable"] = retryable
	payload["service_status"] = nullableString(serviceStatus)
	payload["usage_event_id"] = codexPlusGatewayUsageEventID(input.RequestID, payload["event_id"])
	if metadata, ok := payload["metadata"].(map[string]any); ok {
		metadata["policy_decision_id"] = payload["event_id"]
		metadata["balance_state"] = codexPlusGatewayBalanceState(errorCode)
		metadata["rate_limit_kind"] = codexPlusGatewayRateLimitKind(errorCode)
		metadata["limit_scope"] = codexPlusGatewayLimitScope(errorCode)
	}
	payload = NormalizeCodexPlusAuditRiskPayload(CodexPlusAuditEventGatewayRejected, payload)
	decision := &CodexPlusGatewayPolicyDecision{
		Allowed:       false,
		HTTPStatus:    httpStatus,
		ErrorCode:     errorCode,
		ServiceStatus: serviceStatus,
		Reason:        reason,
		Retryable:     retryable,
		EventType:     CodexPlusGatewayPolicyEventRejected,
		ConfigVersion: configVersion,
		ManagedKey:    managedKey,
		Model:         model,
		EventPayload:  payload,
	}
	if s != nil && s.eventRecorder != nil {
		event := CodexPlusEventCreate{
			UserID:        int64PtrIfPositive(userIDFromGatewayPolicyInput(input)),
			DeviceID:      stringPtrIfNotBlank(input.DeviceID),
			EventType:     CodexPlusGatewayPolicyEventRejected,
			Severity:      "warning",
			RequestID:     stringPtrIfNotBlank(input.RequestID),
			ConfigVersion: stringPtrIfNotBlank(configVersion),
			Payload:       payload,
		}
		_, decision.EventError = s.eventRecorder.Append(ctx, event)
	}
	return decision
}

func (s *CodexPlusGatewayPolicyService) currentTime() time.Time {
	if s == nil || s.now == nil {
		return time.Now()
	}
	return s.now()
}

func BuildCodexPlusGatewayPolicyRejectionPayload(input CodexPlusGatewayPolicyInput, managedKey *CodexPlusManagedProviderKey, model *CodexPlusModel, configVersion, errorCode, serviceStatus, reason string, createdAt time.Time) map[string]any {
	modelID := strings.TrimSpace(input.RequestedModel)
	routeModel := ""
	modelGroup := ""
	if model != nil {
		if model.ModelID != "" {
			modelID = model.ModelID
		}
		routeModel = model.RouteModel
		modelGroup = model.ModelGroup
	}
	payload := map[string]any{
		"event_id":          codexPlusGatewayPolicyEventID(input.RequestID, createdAt),
		"event_type":        CodexPlusGatewayPolicyEventRejected,
		"user_id":           fmt.Sprintf("%d", userIDFromGatewayPolicyInput(input)),
		"device_id":         nullableString(input.DeviceID),
		"request_id":        nullableString(input.RequestID),
		"config_version":    nullableString(configVersion),
		"snapshot_version":  nil,
		"model_id":          nullableString(modelID),
		"error_code":        nullableString(errorCode),
		"provider_key_id":   providerKeyIDString(managedKey),
		"usage_event_id":    nil,
		"risk_tags":         []string{"gateway_policy_rejected"},
		"created_at":        createdAt.UTC().Format(time.RFC3339),
		"redaction_applied": true,
		"metadata": map[string]any{
			"endpoint":             strings.TrimSpace(input.Endpoint),
			"requested_model":      strings.TrimSpace(input.RequestedModel),
			"route_model":          routeModel,
			"model_group":          modelGroup,
			"managed_provider_id":  managedProviderID(managedKey),
			"api_key_id":           apiKeyIDString(input.APIKey),
			"redacted_key_summary": redactedGatewayAPIKeySummary(input.APIKey, managedKey),
			"reason":               reason,
			"service_status":       serviceStatus,
		},
	}
	metadata := payload["metadata"].(map[string]any)
	for k, v := range RedactCodexPlusGatewayEventMetadata(input.Metadata) {
		metadata[k] = v
	}
	return payload
}

func BuildCodexPlusGatewayPolicyUsagePayload(input CodexPlusGatewayPolicyInput, managedKey *CodexPlusManagedProviderKey, model *CodexPlusModel, configVersion string, resolved codexPlusGatewayPolicyResolution, createdAt time.Time) map[string]any {
	modelID := strings.TrimSpace(input.RequestedModel)
	routeModel := ""
	modelGroup := ""
	if model != nil {
		if strings.TrimSpace(model.ModelID) != "" {
			modelID = model.ModelID
		}
		routeModel = model.RouteModel
		modelGroup = model.ModelGroup
	}
	policyID := ""
	if resolved.UsageRule != nil {
		policyID = resolved.UsageRule.PolicyID
	}
	payload := map[string]any{
		"event_id":          codexPlusGatewayUsageEventID(input.RequestID, nil),
		"event_type":        CodexPlusGatewayPolicyEventUsage,
		"user_id":           fmt.Sprintf("%d", userIDFromGatewayPolicyInput(input)),
		"device_id":         nullableString(input.DeviceID),
		"request_id":        nullableString(input.RequestID),
		"config_version":    nullableString(configVersion),
		"snapshot_version":  nil,
		"model_id":          nullableString(modelID),
		"error_code":        nil,
		"service_status":    CodexPlusServiceStatusAvailable,
		"http_status":       http.StatusOK,
		"retryable":         false,
		"provider_key_id":   providerKeyIDString(managedKey),
		"usage_event_id":    codexPlusGatewayUsageEventID(input.RequestID, nil),
		"risk_tags":         []string{"usage_reconciliation"},
		"created_at":        createdAt.UTC().Format(time.RFC3339),
		"redaction_applied": true,
		"metadata": map[string]any{
			"route_model":          nullableString(routeModel),
			"model_group":          nullableString(modelGroup),
			"plan_id":              nullableString(resolved.Entitlement.PlanID),
			"policy_decision_id":   nullableString(codexPlusGatewayPolicyEventID(input.RequestID, createdAt)),
			"provider_route_id":    nullableString(input.Endpoint),
			"platform":             nullableString(input.Platform),
			"entitlement_state":    "active",
			"balance_state":        "ok",
			"rate_limit_kind":      "unknown",
			"limit_scope":          "user",
			"sampled":              false,
			"usage_policy_id":      nullableString(policyID),
			"estimated_tokens":     input.EstimatedTokens,
			"settlement_stage":     "preflight_authorized",
			"requires_settlement":  true,
			"managed_provider_id":  managedProviderID(managedKey),
			"redacted_key_summary": redactedGatewayAPIKeySummary(input.APIKey, managedKey),
		},
	}
	return payload
}

func RedactCodexPlusGatewayEventMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(metadata))
	for k, v := range metadata {
		key := strings.ToLower(strings.TrimSpace(k))
		if key == "" || isForbiddenCodexPlusGatewayEventField(key) {
			continue
		}
		switch value := v.(type) {
		case string:
			out[k] = redactCodexPlusSecretLikeString(value)
		case fmt.Stringer:
			out[k] = redactCodexPlusSecretLikeString(value.String())
		case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, nil:
			out[k] = value
		default:
			out[k] = fmt.Sprintf("%T", value)
		}
	}
	return out
}

func isForbiddenCodexPlusGatewayEventField(key string) bool {
	return strings.Contains(key, "authorization") ||
		strings.Contains(key, "api_key") ||
		strings.Contains(key, "apikey") ||
		strings.Contains(key, "jwt") ||
		strings.Contains(key, "token") ||
		strings.Contains(key, "credential") ||
		strings.Contains(key, "secret") ||
		strings.Contains(key, "prompt") ||
		strings.Contains(key, "response_body")
}

func redactCodexPlusSecretLikeString(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "sk-") || strings.Count(trimmed, ".") >= 2 || strings.HasPrefix(strings.ToLower(trimmed), "bearer ") {
		return "[redacted]"
	}
	return trimmed
}

func findCodexPlusModel(cfg *CodexPlusConfig, requestedModel string) *CodexPlusModel {
	if cfg == nil {
		return nil
	}
	requested := strings.TrimSpace(requestedModel)
	for i := range cfg.ModelCatalog.Models {
		model := &cfg.ModelCatalog.Models[i]
		if requested == "" && model.IsDefault {
			return model
		}
		if requested == model.ModelID || requested == model.RouteModel {
			return model
		}
	}
	return nil
}

func resolveCodexPlusGatewayPolicy(cfg *CodexPlusConfig, input CodexPlusGatewayPolicyInput) codexPlusGatewayPolicyResolution {
	entitlement := input.Entitlement
	if strings.TrimSpace(entitlement.Status) == "" {
		entitlement.Status = CodexPlusServiceStatusAvailable
	}
	if strings.TrimSpace(entitlement.Status) != CodexPlusServiceStatusAvailable {
		return codexPlusGatewayPolicyResolution{Entitlement: entitlement}
	}
	plan := codexPlusPlanForGatewayInput(cfg, input)
	if plan == nil {
		entitlement.Status = CodexPlusServiceStatusNotPurchased
		entitlement.PlanID = ""
		entitlement.ModelGroups = nil
		return codexPlusGatewayPolicyResolution{Entitlement: entitlement}
	}
	entitlement.PlanID = plan.PlanID
	entitlement.ModelGroups = append([]string(nil), plan.ModelGroups...)
	return codexPlusGatewayPolicyResolution{
		Entitlement: entitlement,
		Plan:        plan,
		UsageRule:   codexPlusUsageRuleForGatewayPlan(cfg, plan),
	}
}

func modelAllowedByEntitlement(cfg *CodexPlusConfig, model *CodexPlusModel, entitlement CodexPlusGatewayEntitlementContext) bool {
	if model == nil {
		return false
	}
	groups := make(map[string]struct{})
	for _, group := range entitlement.ModelGroups {
		if g := strings.TrimSpace(group); g != "" {
			groups[g] = struct{}{}
		}
	}
	if planID := strings.TrimSpace(entitlement.PlanID); planID != "" && cfg != nil {
		for _, plan := range cfg.PlanCatalog.Plans {
			if plan.PlanID == planID && isCodexPlusPlanEnabled(plan.Status) {
				for _, group := range plan.ModelGroups {
					if g := strings.TrimSpace(group); g != "" {
						groups[g] = struct{}{}
					}
				}
			}
		}
	}
	if len(groups) == 0 {
		return false
	}
	_, ok := groups[model.ModelGroup]
	return ok
}

func codexPlusUsageRuleForGatewayPlan(cfg *CodexPlusConfig, plan *CodexPlusPlan) *CodexPlusUsageRule {
	if cfg == nil || plan == nil {
		return nil
	}
	if policyID := strings.TrimSpace(plan.UsagePolicyID); policyID != "" {
		for i := range cfg.UsagePolicy.Policies {
			if strings.TrimSpace(cfg.UsagePolicy.Policies[i].PolicyID) == policyID {
				return &cfg.UsagePolicy.Policies[i]
			}
		}
		return nil
	}
	for i := range cfg.UsagePolicy.Policies {
		policy := &cfg.UsagePolicy.Policies[i]
		if containsCodexPlusStringFold(policy.AppliesTo.PlanIDs, plan.PlanID) {
			return policy
		}
		for _, group := range plan.ModelGroups {
			if containsCodexPlusStringFold(policy.AppliesTo.ModelGroups, group) {
				return policy
			}
		}
	}
	if len(cfg.UsagePolicy.Policies) > 0 {
		return &cfg.UsagePolicy.Policies[0]
	}
	return nil
}

func resolveCodexPlusGatewayEntitlement(cfg *CodexPlusConfig, input CodexPlusGatewayPolicyInput) CodexPlusGatewayEntitlementContext {
	entitlement := input.Entitlement
	if strings.TrimSpace(entitlement.Status) == "" {
		entitlement.Status = CodexPlusServiceStatusAvailable
	}
	if strings.TrimSpace(entitlement.Status) != CodexPlusServiceStatusAvailable {
		return entitlement
	}
	if strings.TrimSpace(entitlement.PlanID) != "" || len(entitlement.ModelGroups) > 0 || cfg == nil {
		return entitlement
	}
	if plan := codexPlusPlanForGatewayInput(cfg, input); plan != nil {
		entitlement.PlanID = plan.PlanID
		return entitlement
	}
	entitlement.Status = CodexPlusServiceStatusNotPurchased
	return entitlement
}

func codexPlusPlanForGatewayInput(cfg *CodexPlusConfig, input CodexPlusGatewayPolicyInput) *CodexPlusPlan {
	if cfg == nil {
		return nil
	}
	for i := range cfg.PlanCatalog.Plans {
		plan := &cfg.PlanCatalog.Plans[i]
		if strings.TrimSpace(plan.PlanID) != "" && isCodexPlusPlanEnabled(plan.Status) && codexPlusPlanMatchesGatewayInput(plan, input) {
			return plan
		}
	}
	return nil
}

func codexPlusPlanMatchesGatewayInput(plan *CodexPlusPlan, input CodexPlusGatewayPolicyInput) bool {
	if plan == nil {
		return false
	}
	sources := plan.EntitlementSources
	if input.Subscription != nil && containsCodexPlusInt64(sources.SubscriptionGroupIDs, input.Subscription.GroupID) {
		return true
	}
	if containsCodexPlusInt64(sources.APIKeyGroupIDs, codexPlusAPIKeyGroupID(input.APIKey)) {
		return true
	}
	if containsCodexPlusInt64(sources.APIKeyGroupIDs, codexPlusGroupID(input.Group)) {
		return true
	}
	if input.Subscription != nil && containsCodexPlusInt64(sources.SubscriptionGroupIDs, codexPlusGroupID(input.Subscription.Group)) {
		return true
	}
	for _, name := range []string{
		codexPlusGroupName(input.Group),
		codexPlusGroupName(codexPlusAPIKeyGroup(input.APIKey)),
		codexPlusGroupName(codexPlusSubscriptionGroup(input.Subscription)),
	} {
		if containsCodexPlusStringFold(sources.GroupNames, name) {
			return true
		}
	}
	return false
}

func codexPlusStrictDeviceEnforcementEnabled(cfg *CodexPlusConfig) bool {
	return cfg != nil && cfg.FeatureFlags.Flags.StrictDeviceEnforcement
}

func containsCodexPlusInt64(values []int64, target int64) bool {
	if target <= 0 {
		return false
	}
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsCodexPlusStringFold(values []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}

func codexPlusAPIKeyGroupID(apiKey *APIKey) int64 {
	if apiKey != nil && apiKey.GroupID != nil {
		return *apiKey.GroupID
	}
	return codexPlusGroupID(codexPlusAPIKeyGroup(apiKey))
}

func codexPlusGroupID(group *Group) int64 {
	if group == nil {
		return 0
	}
	return group.ID
}

func codexPlusGroupName(group *Group) string {
	if group == nil {
		return ""
	}
	return strings.TrimSpace(group.Name)
}

func codexPlusAPIKeyGroup(apiKey *APIKey) *Group {
	if apiKey == nil {
		return nil
	}
	return apiKey.Group
}

func codexPlusSubscriptionGroup(subscription *UserSubscription) *Group {
	if subscription == nil {
		return nil
	}
	return subscription.Group
}

func isCodexPlusPlanEnabled(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "active", "hidden", "internal":
		return true
	default:
		return false
	}
}

func isCodexPlusActiveStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", StatusActive:
		return true
	default:
		return false
	}
}

func nullableString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func providerKeyIDString(key *CodexPlusManagedProviderKey) any {
	if key == nil || key.ID <= 0 {
		return nil
	}
	return fmt.Sprintf("%d", key.ID)
}

func apiKeyIDString(key *APIKey) any {
	if key == nil || key.ID <= 0 {
		return nil
	}
	return fmt.Sprintf("%d", key.ID)
}

func managedProviderID(key *CodexPlusManagedProviderKey) string {
	if key == nil || strings.TrimSpace(key.ManagedProviderID) == "" {
		return CodexPlusManagedProviderID
	}
	return strings.TrimSpace(key.ManagedProviderID)
}

func redactedGatewayAPIKeySummary(apiKey *APIKey, managedKey *CodexPlusManagedProviderKey) string {
	if managedKey != nil && managedKey.KeyPrefix != nil && strings.TrimSpace(*managedKey.KeyPrefix) != "" {
		return strings.TrimSpace(*managedKey.KeyPrefix) + "...redacted"
	}
	if apiKey == nil || strings.TrimSpace(apiKey.Key) == "" {
		return ""
	}
	key := strings.TrimSpace(apiKey.Key)
	if len(key) <= 8 {
		return "[redacted]"
	}
	prefix := key[:minInt(8, len(key))]
	suffix := key[len(key)-minInt(4, len(key)):]
	return prefix + "..." + suffix
}

func codexPlusGatewayPolicyEventID(requestID string, createdAt time.Time) string {
	requestID = strings.TrimSpace(requestID)
	if requestID != "" {
		return "gateway-" + requestID
	}
	return fmt.Sprintf("gateway-%d", createdAt.UTC().UnixNano())
}

func codexPlusGatewayUsageEventID(requestID string, fallback any) string {
	requestID = strings.TrimSpace(requestID)
	if requestID != "" {
		return "usage-" + requestID
	}
	if value, ok := fallback.(string); ok && strings.TrimSpace(value) != "" {
		return "usage-" + strings.TrimPrefix(strings.TrimSpace(value), "gateway-")
	}
	return ""
}

func codexPlusGatewayBalanceState(errorCode string) string {
	switch strings.TrimSpace(errorCode) {
	case CodexPlusGatewayErrorBalanceInsufficient:
		return "insufficient"
	default:
		return "unknown"
	}
}

func codexPlusGatewayRateLimitKind(errorCode string) string {
	switch strings.TrimSpace(errorCode) {
	case CodexPlusGatewayErrorRateLimited:
		return "rpm"
	case CodexPlusGatewayErrorTPMLimited:
		return "tpm"
	case CodexPlusGatewayErrorConcurrencyLimited:
		return "concurrency"
	case CodexPlusGatewayErrorQuotaExceeded:
		return "daily_quota"
	default:
		return "unknown"
	}
}

func codexPlusGatewayLimitScope(errorCode string) string {
	switch strings.TrimSpace(errorCode) {
	case CodexPlusGatewayErrorModelNotAllowed:
		return "model"
	case CodexPlusGatewayErrorDeviceRevoked, CodexPlusGatewayErrorDeviceBlocked:
		return "device"
	default:
		return "user"
	}
}

func userIDFromGatewayPolicyInput(input CodexPlusGatewayPolicyInput) int64 {
	if input.User != nil {
		return input.User.ID
	}
	if input.APIKey != nil {
		return input.APIKey.UserID
	}
	return 0
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func stringPtrIfNotBlank(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func int64PtrIfPositive(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	return &value
}
