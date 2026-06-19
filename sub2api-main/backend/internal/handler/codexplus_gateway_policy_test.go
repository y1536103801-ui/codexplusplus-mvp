//go:build unit

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type handlerCodexPlusConfigReader struct {
	cfg   *service.CodexPlusConfig
	calls int
}

func (r *handlerCodexPlusConfigReader) Get(context.Context) (*service.CodexPlusConfig, error) {
	r.calls++
	return r.cfg, nil
}

type handlerCodexPlusManagedKeyReader struct {
	key   *service.CodexPlusManagedProviderKey
	calls int
}

func (r *handlerCodexPlusManagedKeyReader) GetByAPIKeyID(context.Context, int64) (*service.CodexPlusManagedProviderKey, error) {
	r.calls++
	if r.key == nil {
		return nil, service.ErrCodexPlusManagedProviderKeyNotFound
	}
	return r.key, nil
}

type handlerCodexPlusDeviceReader struct {
	device       *service.CodexPlusDevice
	lastDeviceID string
	calls        int
}

func (r *handlerCodexPlusDeviceReader) GetByUserAndDevice(_ context.Context, _ int64, deviceID string) (*service.CodexPlusDevice, error) {
	r.calls++
	r.lastDeviceID = deviceID
	if r.device == nil {
		return nil, service.ErrCodexPlusDeviceNotFound
	}
	return r.device, nil
}

type handlerCodexPlusBillingChecker struct {
	calls int
}

func (b *handlerCodexPlusBillingChecker) CheckBillingEligibility(context.Context, *service.User, *service.APIKey, *service.Group, *service.UserSubscription, string) error {
	b.calls++
	return nil
}

func TestCodexPlusGatewayPolicyHelperSkipsUnmanagedAPIKey(t *testing.T) {
	ctx, _ := handlerCodexPlusTestContext("/v1/responses", "")
	configReader := &handlerCodexPlusConfigReader{cfg: handlerCodexPlusPolicyConfig()}
	managedReader := &handlerCodexPlusManagedKeyReader{}
	policy := service.NewCodexPlusGatewayPolicyService(
		service.WithCodexPlusGatewayPolicyConfigReader(configReader),
		service.WithCodexPlusManagedProviderKeyReader(managedReader),
	)
	apiKey := handlerCodexPlusAPIKey()
	wroteError := false

	ok := enforceCodexPlusGatewayPolicy(ctx, policy, apiKey, nil, "codex-default", "/v1/responses", func(int, string, string) {
		wroteError = true
	})

	if !ok {
		t.Fatalf("enforceCodexPlusGatewayPolicy() = false, want true for unmanaged API key")
	}
	if wroteError {
		t.Fatalf("unmanaged API key should not write an error")
	}
	if managedReader.calls != 1 {
		t.Fatalf("managed key lookups = %d, want 1", managedReader.calls)
	}
	if configReader.calls != 0 {
		t.Fatalf("config reads = %d, want 0 for unmanaged API key", configReader.calls)
	}
}

func TestCodexPlusGatewayPolicyHelperRejectsManagedUnauthorizedModel(t *testing.T) {
	ctx, recorder := handlerCodexPlusTestContext("/v1/responses", "")
	policy := handlerCodexPlusPolicy(&handlerCodexPlusDeviceReader{})
	apiKey := handlerCodexPlusAPIKey()
	var gotStatus int
	var gotCode string
	var gotMessage string

	ok := enforceCodexPlusGatewayPolicy(ctx, policy, apiKey, nil, "codex-pro", "/v1/responses", func(status int, code, message string) {
		gotStatus = status
		gotCode = code
		gotMessage = message
	})

	if ok {
		t.Fatalf("enforceCodexPlusGatewayPolicy() = true, want false for unauthorized model")
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("response recorder was written unexpectedly, code=%d", recorder.Code)
	}
	if gotStatus != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", gotStatus, http.StatusForbidden)
	}
	if gotCode != service.CodexPlusGatewayErrorModelNotAllowed {
		t.Fatalf("code = %q, want %q", gotCode, service.CodexPlusGatewayErrorModelNotAllowed)
	}
	if gotMessage == "" {
		t.Fatalf("message should describe rejection")
	}
}

func TestCodexPlusGatewayPolicyHelperPassesDeviceHeaderToPolicy(t *testing.T) {
	deviceReader := &handlerCodexPlusDeviceReader{
		device: &service.CodexPlusDevice{UserID: 10, DeviceID: "dev-revoked", Status: "revoked"},
	}
	ctx, _ := handlerCodexPlusTestContext("/v1/messages", "dev-revoked")
	policy := handlerCodexPlusPolicy(deviceReader)
	apiKey := handlerCodexPlusAPIKey()
	var gotCode string

	ok := enforceCodexPlusGatewayPolicy(ctx, policy, apiKey, nil, "codex-default", "/v1/messages", func(_ int, code, _ string) {
		gotCode = code
	})

	if ok {
		t.Fatalf("enforceCodexPlusGatewayPolicy() = true, want false for revoked device")
	}
	if deviceReader.calls != 1 {
		t.Fatalf("device lookups = %d, want 1", deviceReader.calls)
	}
	if deviceReader.lastDeviceID != "dev-revoked" {
		t.Fatalf("device id = %q, want dev-revoked", deviceReader.lastDeviceID)
	}
	if gotCode != service.CodexPlusGatewayErrorDeviceRevoked {
		t.Fatalf("code = %q, want %q", gotCode, service.CodexPlusGatewayErrorDeviceRevoked)
	}
}

func handlerCodexPlusPolicy(deviceReader *handlerCodexPlusDeviceReader) *service.CodexPlusGatewayPolicyService {
	return service.NewCodexPlusGatewayPolicyService(
		service.WithCodexPlusGatewayPolicyConfigReader(&handlerCodexPlusConfigReader{cfg: handlerCodexPlusPolicyConfig()}),
		service.WithCodexPlusManagedProviderKeyReader(&handlerCodexPlusManagedKeyReader{key: handlerCodexPlusManagedKey()}),
		service.WithCodexPlusGatewayDeviceReader(deviceReader),
		service.WithCodexPlusGatewayBillingChecker(&handlerCodexPlusBillingChecker{}),
	)
}

func handlerCodexPlusTestContext(path, deviceID string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, path, nil)
	if deviceID != "" {
		c.Request.Header.Set("X-CodexPlus-Device-Id", deviceID)
	}
	return c, w
}

func handlerCodexPlusAPIKey() *service.APIKey {
	user := &service.User{ID: 10, Status: service.StatusActive, Balance: 10, Concurrency: 1}
	groupID := int64(501)
	group := &service.Group{ID: groupID, Name: "Starter Access", Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true}
	return &service.APIKey{
		ID:      99,
		UserID:  user.ID,
		Key:     "test-managed-key",
		GroupID: &groupID,
		Status:  service.StatusActive,
		User:    user,
		Group:   group,
	}
}

func handlerCodexPlusManagedKey() *service.CodexPlusManagedProviderKey {
	prefix := "test-key"
	return &service.CodexPlusManagedProviderKey{
		ID:                7,
		UserID:            10,
		APIKeyID:          99,
		ManagedProviderID: service.CodexPlusManagedProviderID,
		DisplayName:       "Codex++ Cloud",
		KeyPrefix:         &prefix,
		Status:            service.StatusActive,
	}
}

func handlerCodexPlusPolicyConfig() *service.CodexPlusConfig {
	cfg := service.DefaultCodexPlusConfig(time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC))
	cfg.PlanCatalog.Plans = []service.CodexPlusPlan{{
		PlanID:        "starter",
		Name:          "Starter",
		BillingPeriod: "month",
		Currency:      "USD",
		DisplayPrice:  "$10",
		EntitlementSources: service.CodexPlusEntitlementSources{
			APIKeyGroupIDs: []int64{501},
		},
		ModelGroups: []string{"default"},
		Status:      "active",
		IsListed:    true,
	}}
	cfg.ModelCatalog.Models = []service.CodexPlusModel{
		{
			ModelID:           "codex-default",
			DisplayName:       "Codex Default",
			RouteModel:        "codex-default",
			ModelGroup:        "default",
			ContextWindow:     128000,
			BillingMultiplier: 1,
			IsDefault:         true,
			IsEnabled:         true,
		},
		{
			ModelID:           "codex-pro",
			DisplayName:       "Codex Pro",
			RouteModel:        "codex-pro",
			ModelGroup:        "pro",
			ContextWindow:     200000,
			BillingMultiplier: 2,
			IsEnabled:         true,
		},
	}
	return &cfg
}
