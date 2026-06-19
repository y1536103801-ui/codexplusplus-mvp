package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

type fakeCodexPlusGatewayConfigReader struct {
	cfg   *CodexPlusConfig
	err   error
	calls int
}

func (f *fakeCodexPlusGatewayConfigReader) Get(context.Context) (*CodexPlusConfig, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.cfg, nil
}

type fakeCodexPlusManagedKeyReader struct {
	key   *CodexPlusManagedProviderKey
	err   error
	calls int
}

func (f *fakeCodexPlusManagedKeyReader) GetByAPIKeyID(context.Context, int64) (*CodexPlusManagedProviderKey, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if f.key == nil {
		return nil, ErrCodexPlusManagedProviderKeyNotFound
	}
	return f.key, nil
}

type fakeCodexPlusDeviceReader struct {
	device *CodexPlusDevice
	err    error
	calls  int
}

func (f *fakeCodexPlusDeviceReader) GetByUserAndDevice(context.Context, int64, string) (*CodexPlusDevice, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if f.device == nil {
		return nil, ErrCodexPlusDeviceNotFound
	}
	return f.device, nil
}

type fakeCodexPlusPolicyEventRecorder struct {
	events []CodexPlusEventCreate
	err    error
}

func (f *fakeCodexPlusPolicyEventRecorder) Append(_ context.Context, input CodexPlusEventCreate) (*CodexPlusEvent, error) {
	f.events = append(f.events, input)
	if f.err != nil {
		return nil, f.err
	}
	return &CodexPlusEvent{EventType: input.EventType, Payload: input.Payload}, nil
}

type fakeCodexPlusGatewayBillingChecker struct {
	err   error
	calls int
}

func (f *fakeCodexPlusGatewayBillingChecker) CheckBillingEligibility(context.Context, *User, *APIKey, *Group, *UserSubscription, string) error {
	f.calls++
	return f.err
}

func TestCodexPlusGatewayPolicyEvaluateUnmanagedAPIKeySkipsPolicy(t *testing.T) {
	cfg := &fakeCodexPlusGatewayConfigReader{cfg: codexPlusGatewayPolicyTestConfig()}
	managed := &fakeCodexPlusManagedKeyReader{}
	svc := NewCodexPlusGatewayPolicyService(
		WithCodexPlusGatewayPolicyConfigReader(cfg),
		WithCodexPlusManagedProviderKeyReader(managed),
	)

	decision, err := svc.Evaluate(context.Background(), codexPlusGatewayPolicyBaseInput())
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if !decision.Allowed || !decision.Skipped {
		t.Fatalf("decision = %+v, want allowed skipped", decision)
	}
	if cfg.calls != 0 {
		t.Fatalf("config reader calls = %d, want 0 for unmanaged key", cfg.calls)
	}
}

func TestCodexPlusGatewayPolicyEvaluateManagedAllowedModelPasses(t *testing.T) {
	billing := &fakeCodexPlusGatewayBillingChecker{}
	svc := codexPlusGatewayPolicyTestService(
		codexPlusGatewayPolicyTestConfig(),
		codexPlusGatewayManagedKey(10, 99),
		nil,
		nil,
		billing,
	)
	input := codexPlusGatewayPolicyBaseInput()
	input.CheckBilling = true

	decision, err := svc.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if !decision.Allowed || decision.Skipped {
		t.Fatalf("decision = %+v, want allowed managed request", decision)
	}
	if decision.ConfigVersion != CodexPlusConfigVersionMVP {
		t.Fatalf("ConfigVersion = %q", decision.ConfigVersion)
	}
	if billing.calls != 1 {
		t.Fatalf("billing calls = %d, want 1", billing.calls)
	}
}

func TestCodexPlusGatewayPolicyEvaluateDisabledModelRejectsAndRedactsEvent(t *testing.T) {
	recorder := &fakeCodexPlusPolicyEventRecorder{}
	input := codexPlusGatewayPolicyBaseInput()
	input.RequestedModel = "codex-disabled"
	input.Metadata = map[string]any{
		"authorization": "Bearer sk-should-not-appear",
		"note":          "safe-note",
		"candidate":     "sk-secret-value",
	}
	svc := codexPlusGatewayPolicyTestService(
		codexPlusGatewayPolicyTestConfig(),
		codexPlusGatewayManagedKey(10, 99),
		nil,
		recorder,
		nil,
	)

	decision, err := svc.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	assertCodexPlusRejected(t, decision, http.StatusForbidden, CodexPlusGatewayErrorModelNotAllowed, CodexPlusServiceStatusModelUnavailable)
	if len(recorder.events) != 1 {
		t.Fatalf("recorded events = %d, want 1", len(recorder.events))
	}
	if containsCodexPlusPayloadString(recorder.events[0].Payload, "sk-should-not-appear") ||
		containsCodexPlusPayloadString(recorder.events[0].Payload, "sk-secret-value") {
		t.Fatalf("event payload leaked a secret: %+v", recorder.events[0].Payload)
	}
	metadata := recorder.events[0].Payload["metadata"].(map[string]any)
	if metadata["note"] != "safe-note" {
		t.Fatalf("metadata note = %v, want safe-note", metadata["note"])
	}
	if metadata["candidate"] != "[redacted]" {
		t.Fatalf("metadata candidate = %v, want [redacted]", metadata["candidate"])
	}
	if _, ok := metadata["authorization"]; ok {
		t.Fatalf("metadata authorization should be omitted: %+v", metadata)
	}
}

func TestCodexPlusGatewayPolicyEvaluateModelOutsidePlanRejects(t *testing.T) {
	input := codexPlusGatewayPolicyBaseInput()
	input.RequestedModel = "codex-pro"
	input.Entitlement.PlanID = "starter"
	svc := codexPlusGatewayPolicyTestService(
		codexPlusGatewayPolicyTestConfig(),
		codexPlusGatewayManagedKey(10, 99),
		nil,
		nil,
		nil,
	)

	decision, err := svc.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	assertCodexPlusRejected(t, decision, http.StatusForbidden, CodexPlusGatewayErrorModelNotAllowed, CodexPlusServiceStatusModelUnavailable)
}

func TestCodexPlusGatewayPolicyEvaluateRequiresConfiguredEntitlementSource(t *testing.T) {
	input := codexPlusGatewayPolicyBaseInput()
	input.Entitlement = CodexPlusGatewayEntitlementContext{
		Status:      CodexPlusServiceStatusAvailable,
		PlanID:      "starter",
		ModelGroups: []string{"default"},
	}
	cfg := codexPlusGatewayPolicyTestConfig()
	cfg.PlanCatalog.Plans[0].EntitlementSources = CodexPlusEntitlementSources{}
	svc := codexPlusGatewayPolicyTestService(
		cfg,
		codexPlusGatewayManagedKey(10, 99),
		nil,
		nil,
		nil,
	)

	decision, err := svc.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	assertCodexPlusRejected(t, decision, http.StatusForbidden, CodexPlusGatewayErrorEntitlementNotBought, CodexPlusServiceStatusNotPurchased)
}

func TestCodexPlusGatewayPolicyEvaluateMapsAPIKeyGroupFromConfig(t *testing.T) {
	cfg := codexPlusGatewayPolicyTestConfig()
	cfg.PlanCatalog.Plans[0].EntitlementSources.APIKeyGroupIDs = []int64{501}
	input := codexPlusGatewayPolicyBaseInput()
	input.Entitlement = CodexPlusGatewayEntitlementContext{Status: CodexPlusServiceStatusAvailable}
	groupID := int64(501)
	input.APIKey.GroupID = &groupID
	input.Group = &Group{ID: groupID, Name: "Starter Access", Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true}
	input.APIKey.Group = input.Group
	svc := codexPlusGatewayPolicyTestService(
		cfg,
		codexPlusGatewayManagedKey(10, 99),
		nil,
		nil,
		nil,
	)

	decision, err := svc.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if !decision.Allowed || decision.Model == nil || decision.Model.ModelID != "codex-default" {
		t.Fatalf("decision = %+v, want allowed default model through mapped group", decision)
	}

	input.RequestedModel = "codex-pro"
	decision, err = svc.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("Evaluate() pro error = %v", err)
	}
	assertCodexPlusRejected(t, decision, http.StatusForbidden, CodexPlusGatewayErrorModelNotAllowed, CodexPlusServiceStatusModelUnavailable)
}

func TestCodexPlusGatewayPolicyEvaluateMapsSubscriptionGroupFromConfig(t *testing.T) {
	cfg := codexPlusGatewayPolicyTestConfig()
	cfg.PlanCatalog.Plans[1].EntitlementSources.SubscriptionGroupIDs = []int64{601}
	input := codexPlusGatewayPolicyBaseInput()
	input.Entitlement = CodexPlusGatewayEntitlementContext{Status: CodexPlusServiceStatusAvailable}
	input.APIKey.GroupID = nil
	input.APIKey.Group = nil
	input.Group = nil
	input.Subscription = &UserSubscription{
		UserID:    input.User.ID,
		GroupID:   601,
		Status:    SubscriptionStatusActive,
		ExpiresAt: time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC),
	}
	input.RequestedModel = "codex-pro"
	svc := codexPlusGatewayPolicyTestService(
		cfg,
		codexPlusGatewayManagedKey(10, 99),
		nil,
		nil,
		nil,
	)

	decision, err := svc.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if !decision.Allowed || decision.Model == nil || decision.Model.ModelID != "codex-pro" {
		t.Fatalf("decision = %+v, want allowed pro model through mapped subscription group", decision)
	}
}

func TestCodexPlusGatewayPolicyEvaluateMapsSubscriptionGroupObjectFromConfig(t *testing.T) {
	cfg := codexPlusGatewayPolicyTestConfig()
	cfg.PlanCatalog.Plans[1].EntitlementSources.SubscriptionGroupIDs = []int64{602}
	input := codexPlusGatewayPolicyBaseInput()
	input.Entitlement = CodexPlusGatewayEntitlementContext{Status: CodexPlusServiceStatusAvailable}
	input.APIKey.GroupID = nil
	input.APIKey.Group = nil
	input.Group = nil
	input.Subscription = &UserSubscription{
		UserID:    input.User.ID,
		Status:    SubscriptionStatusActive,
		ExpiresAt: time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC),
		Group:     &Group{ID: 602, Name: "Pro Subscription", Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true},
	}
	input.RequestedModel = "codex-pro"
	svc := codexPlusGatewayPolicyTestService(
		cfg,
		codexPlusGatewayManagedKey(10, 99),
		nil,
		nil,
		nil,
	)

	decision, err := svc.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if !decision.Allowed || decision.Model == nil || decision.Model.ModelID != "codex-pro" {
		t.Fatalf("decision = %+v, want allowed pro model through mapped subscription group object", decision)
	}
}

func TestCodexPlusGatewayPolicyEvaluateDeviceRejection(t *testing.T) {
	tests := []struct {
		name      string
		status    string
		errorCode string
	}{
		{name: "revoked", status: "revoked", errorCode: CodexPlusGatewayErrorDeviceRevoked},
		{name: "blocked", status: "blocked", errorCode: CodexPlusGatewayErrorDeviceBlocked},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := codexPlusGatewayPolicyBaseInput()
			input.DeviceID = "dev-1"
			input.StrictDeviceEnforcement = true
			svc := codexPlusGatewayPolicyTestService(
				codexPlusGatewayPolicyTestConfig(),
				codexPlusGatewayManagedKey(10, 99),
				&CodexPlusDevice{UserID: 10, DeviceID: "dev-1", Status: tt.status},
				nil,
				nil,
			)

			decision, err := svc.Evaluate(context.Background(), input)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			assertCodexPlusRejected(t, decision, http.StatusForbidden, tt.errorCode, CodexPlusServiceStatusDeviceRevoked)
		})
	}
}

func TestCodexPlusGatewayPolicyEvaluateMissingDeviceOnlyRejectsInStrictMode(t *testing.T) {
	input := codexPlusGatewayPolicyBaseInput()
	input.StrictDeviceEnforcement = false
	svc := codexPlusGatewayPolicyTestService(
		codexPlusGatewayPolicyTestConfig(),
		codexPlusGatewayManagedKey(10, 99),
		nil,
		nil,
		nil,
	)
	decision, err := svc.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("decision = %+v, want allowed without strict device enforcement", decision)
	}

	input.StrictDeviceEnforcement = true
	decision, err = svc.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("Evaluate() strict error = %v", err)
	}
	assertCodexPlusRejected(t, decision, http.StatusForbidden, CodexPlusGatewayErrorDeviceRevoked, CodexPlusServiceStatusDeviceRevoked)
}

func TestCodexPlusGatewayPolicyEvaluateStrictUnknownDeviceRejects(t *testing.T) {
	input := codexPlusGatewayPolicyBaseInput()
	input.DeviceID = "unknown-device"
	input.StrictDeviceEnforcement = true
	svc := codexPlusGatewayPolicyTestService(
		codexPlusGatewayPolicyTestConfig(),
		codexPlusGatewayManagedKey(10, 99),
		nil,
		nil,
		nil,
	)

	decision, err := svc.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	assertCodexPlusRejected(t, decision, http.StatusForbidden, CodexPlusGatewayErrorDeviceRevoked, CodexPlusServiceStatusDeviceRevoked)
}

func TestCodexPlusGatewayPolicyEvaluateStrictDeviceFromConfig(t *testing.T) {
	cfg := codexPlusGatewayPolicyTestConfig()
	cfg.FeatureFlags.Flags.StrictDeviceEnforcement = true
	input := codexPlusGatewayPolicyBaseInput()
	input.DeviceID = ""
	input.StrictDeviceEnforcement = false
	svc := codexPlusGatewayPolicyTestService(
		cfg,
		codexPlusGatewayManagedKey(10, 99),
		nil,
		nil,
		nil,
	)

	decision, err := svc.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	assertCodexPlusRejected(t, decision, http.StatusForbidden, CodexPlusGatewayErrorDeviceRevoked, CodexPlusServiceStatusDeviceRevoked)
}

func TestCodexPlusGatewayPolicyEvaluateBillingErrorsMapToProtocolStatus(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		httpStatus    int
		errorCode     string
		serviceStatus string
		retryable     bool
	}{
		{name: "low balance", err: ErrInsufficientBalance, httpStatus: http.StatusPaymentRequired, errorCode: CodexPlusGatewayErrorBalanceInsufficient, serviceStatus: CodexPlusServiceStatusLowBalance},
		{name: "rate limited", err: ErrGroupRPMExceeded, httpStatus: http.StatusTooManyRequests, errorCode: CodexPlusGatewayErrorRateLimited, serviceStatus: CodexPlusServiceStatusRateLimited, retryable: true},
		{name: "billing unavailable", err: ErrBillingServiceUnavailable, httpStatus: http.StatusServiceUnavailable, errorCode: CodexPlusGatewayErrorConfigUnavailable, serviceStatus: CodexPlusServiceStatusGatewayUnhealthy, retryable: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := codexPlusGatewayPolicyBaseInput()
			input.CheckBilling = true
			svc := codexPlusGatewayPolicyTestService(
				codexPlusGatewayPolicyTestConfig(),
				codexPlusGatewayManagedKey(10, 99),
				nil,
				nil,
				&fakeCodexPlusGatewayBillingChecker{err: tt.err},
			)

			decision, err := svc.Evaluate(context.Background(), input)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			assertCodexPlusRejected(t, decision, tt.httpStatus, tt.errorCode, tt.serviceStatus)
			if decision.Retryable != tt.retryable {
				t.Fatalf("Retryable = %v, want %v", decision.Retryable, tt.retryable)
			}
		})
	}
}

func TestCodexPlusGatewayPolicyEvaluateConfigFailureFailsClosed(t *testing.T) {
	configErr := errors.New("config store down")
	svc := NewCodexPlusGatewayPolicyService(
		WithCodexPlusGatewayPolicyConfigReader(&fakeCodexPlusGatewayConfigReader{err: configErr}),
		WithCodexPlusManagedProviderKeyReader(&fakeCodexPlusManagedKeyReader{key: codexPlusGatewayManagedKey(10, 99)}),
	)

	decision, err := svc.Evaluate(context.Background(), codexPlusGatewayPolicyBaseInput())
	if !errors.Is(err, configErr) {
		t.Fatalf("Evaluate() error = %v, want %v", err, configErr)
	}
	assertCodexPlusRejected(t, decision, http.StatusServiceUnavailable, CodexPlusGatewayErrorConfigUnavailable, CodexPlusServiceStatusGatewayUnhealthy)
}

func codexPlusGatewayPolicyTestService(cfg *CodexPlusConfig, managedKey *CodexPlusManagedProviderKey, device *CodexPlusDevice, recorder *fakeCodexPlusPolicyEventRecorder, billing *fakeCodexPlusGatewayBillingChecker) *CodexPlusGatewayPolicyService {
	opts := []CodexPlusGatewayPolicyServiceOption{
		WithCodexPlusGatewayPolicyConfigReader(&fakeCodexPlusGatewayConfigReader{cfg: cfg}),
		WithCodexPlusManagedProviderKeyReader(&fakeCodexPlusManagedKeyReader{key: managedKey}),
		WithCodexPlusGatewayDeviceReader(&fakeCodexPlusDeviceReader{device: device}),
		WithCodexPlusGatewayPolicyClock(func() time.Time {
			return time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
		}),
	}
	if recorder != nil {
		opts = append(opts, WithCodexPlusGatewayPolicyEventRecorder(recorder))
	}
	if billing != nil {
		opts = append(opts, WithCodexPlusGatewayBillingChecker(billing))
	}
	return NewCodexPlusGatewayPolicyService(opts...)
}

func codexPlusGatewayPolicyBaseInput() CodexPlusGatewayPolicyInput {
	user := &User{ID: 10, Status: StatusActive, Balance: 10}
	groupID := int64(501)
	group := &Group{ID: groupID, Name: "Starter Access", Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true}
	apiKey := &APIKey{
		ID:      99,
		UserID:  user.ID,
		Key:     "test-managed-provider-key",
		Status:  StatusActive,
		User:    user,
		GroupID: &groupID,
		Group:   group,
	}
	return CodexPlusGatewayPolicyInput{
		APIKey:         apiKey,
		User:           user,
		Group:          group,
		RequestedModel: "codex-default",
		Endpoint:       "/v1/responses",
		RequestID:      "req-123",
		Platform:       PlatformOpenAI,
		Entitlement: CodexPlusGatewayEntitlementContext{
			Status: CodexPlusServiceStatusAvailable,
			PlanID: "starter",
		},
	}
}

func codexPlusGatewayManagedKey(userID, apiKeyID int64) *CodexPlusManagedProviderKey {
	prefix := "test-key"
	return &CodexPlusManagedProviderKey{
		ID:                7,
		UserID:            userID,
		APIKeyID:          apiKeyID,
		ManagedProviderID: CodexPlusManagedProviderID,
		DisplayName:       "Codex++ Cloud",
		KeyPrefix:         &prefix,
		Status:            StatusActive,
	}
}

func codexPlusGatewayPolicyTestConfig() *CodexPlusConfig {
	cfg := DefaultCodexPlusConfig(time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC))
	cfg.PlanCatalog.Plans = []CodexPlusPlan{{
		PlanID:        "starter",
		Name:          "Starter",
		BillingPeriod: "month",
		Currency:      "USD",
		DisplayPrice:  "$10",
		EntitlementSources: CodexPlusEntitlementSources{
			APIKeyGroupIDs: []int64{501},
		},
		ModelGroups:   []string{"default"},
		UsagePolicyID: "starter_policy",
		Status:        "active",
	}, {
		PlanID:        "pro",
		Name:          "Pro",
		BillingPeriod: "month",
		Currency:      "USD",
		DisplayPrice:  "$30",
		ModelGroups:   []string{"default", "pro"},
		UsagePolicyID: "pro_policy",
		Status:        "active",
	}}
	cfg.UsagePolicy.Policies = []CodexPlusUsageRule{{
		PolicyID:                   "starter_policy",
		AppliesTo:                  CodexPlusUsagePolicyAppliesTo{PlanIDs: []string{"starter"}, ModelGroups: []string{"default"}},
		LowBalanceThreshold:        1,
		DailyQuota:                 100,
		ConcurrencyLimit:           2,
		RPMLimit:                   60,
		TPMLimit:                   100000,
		ExpiredBehavior:            "block",
		GracePeriodHours:           0,
		InsufficientBalanceMessage: "usage.insufficient_balance",
		RateLimitedMessage:         "usage.rate_limited",
	}, {
		PolicyID:                   "pro_policy",
		AppliesTo:                  CodexPlusUsagePolicyAppliesTo{PlanIDs: []string{"pro"}, ModelGroups: []string{"default", "pro"}},
		LowBalanceThreshold:        1,
		DailyQuota:                 500,
		ConcurrencyLimit:           4,
		RPMLimit:                   120,
		TPMLimit:                   200000,
		ExpiredBehavior:            "block",
		GracePeriodHours:           0,
		InsufficientBalanceMessage: "usage.insufficient_balance",
		RateLimitedMessage:         "usage.rate_limited",
	}}
	cfg.ModelCatalog.Models = []CodexPlusModel{{
		ModelID:           "codex-default",
		DisplayName:       "Default",
		RouteModel:        "codex-default",
		ModelGroup:        "default",
		ContextWindow:     8192,
		BillingMultiplier: 1,
		IsDefault:         true,
		IsEnabled:         true,
	}, {
		ModelID:           "codex-pro",
		DisplayName:       "Pro",
		RouteModel:        "codex-pro",
		ModelGroup:        "pro",
		ContextWindow:     8192,
		BillingMultiplier: 2,
		IsEnabled:         true,
	}, {
		ModelID:           "codex-disabled",
		DisplayName:       "Disabled",
		RouteModel:        "codex-disabled",
		ModelGroup:        "default",
		ContextWindow:     8192,
		BillingMultiplier: 1,
		IsEnabled:         false,
	}}
	return &cfg
}

func assertCodexPlusRejected(t *testing.T, decision *CodexPlusGatewayPolicyDecision, httpStatus int, errorCode, serviceStatus string) {
	t.Helper()
	if decision == nil {
		t.Fatal("decision is nil")
	}
	if decision.Allowed {
		t.Fatalf("decision = %+v, want rejection", decision)
	}
	if decision.HTTPStatus != httpStatus || decision.ErrorCode != errorCode || decision.ServiceStatus != serviceStatus {
		t.Fatalf("decision = %+v, want status=%d code=%s service_status=%s", decision, httpStatus, errorCode, serviceStatus)
	}
	if decision.EventType != CodexPlusGatewayPolicyEventRejected {
		t.Fatalf("EventType = %q, want %q", decision.EventType, CodexPlusGatewayPolicyEventRejected)
	}
	if decision.EventPayload == nil {
		t.Fatalf("EventPayload is nil")
	}
}

func containsCodexPlusPayloadString(value any, needle string) bool {
	switch v := value.(type) {
	case map[string]any:
		for _, child := range v {
			if containsCodexPlusPayloadString(child, needle) {
				return true
			}
		}
	case []string:
		for _, child := range v {
			if strings.Contains(child, needle) {
				return true
			}
		}
	case []any:
		for _, child := range v {
			if containsCodexPlusPayloadString(child, needle) {
				return true
			}
		}
	case string:
		return strings.Contains(v, needle)
	}
	return false
}
