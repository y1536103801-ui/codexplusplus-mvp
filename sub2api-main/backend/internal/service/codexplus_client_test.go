package service

import (
	"context"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

func TestCodexPlusClientBootstrapCreatesAndReusesManagedKey(t *testing.T) {
	now := time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC)
	keys := &codexPlusClientTestKeys{now: now, nextID: 100}
	svc := newCodexPlusClientTestService(now, &User{ID: 1, Balance: 1520, Status: StatusActive}, keys)

	first, err := svc.Bootstrap(context.Background(), CodexPlusBootstrapInput{UserID: 1, DeviceID: "device-1234"})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	second, err := svc.Bootstrap(context.Background(), CodexPlusBootstrapInput{UserID: 1, DeviceID: "device-1234"})
	if err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}

	if first.Service.Status != ClientServiceStatusAvailable {
		t.Fatalf("status = %q, want available", first.Service.Status)
	}
	if first.Provider.APIKey == nil || *first.Provider.APIKey == "" {
		t.Fatal("available bootstrap did not include user-side gateway key")
	}
	if second.Provider.KeySummary.KeyID != first.Provider.KeySummary.KeyID {
		t.Fatalf("managed key was not reused: first=%s second=%s", first.Provider.KeySummary.KeyID, second.Provider.KeySummary.KeyID)
	}
	if keys.createCount != 1 {
		t.Fatalf("create count = %d, want 1", keys.createCount)
	}
}

func TestCodexPlusClientBootstrapIncludesContractFieldsAndEventContext(t *testing.T) {
	now := time.Date(2026, 6, 16, 1, 0, 0, 0, time.UTC)
	events := &codexPlusClientTestEvents{}
	keys := &codexPlusClientTestKeys{now: now, nextID: 100}
	svc := newCodexPlusClientTestService(now, &User{ID: 1, Balance: 1520, Status: StatusActive}, keys)
	svc.SetEventSink(events)

	snapshot, err := svc.Bootstrap(context.Background(), CodexPlusBootstrapInput{
		UserID:        1,
		DeviceID:      "device-1234",
		ClientVersion: "0.2.0",
		RequestID:     "req-bootstrap-1",
	})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	if snapshot.Service.MessageKey != "service.available" {
		t.Fatalf("message_key = %q, want service.available", snapshot.Service.MessageKey)
	}
	if snapshot.Plan.CommerceAction.ActionType != "manage_plan" {
		t.Fatalf("commerce action = %q, want manage_plan", snapshot.Plan.CommerceAction.ActionType)
	}
	if snapshot.Plan.CommerceAction.ActionCopyKey == nil || *snapshot.Plan.CommerceAction.ActionCopyKey == "" {
		t.Fatal("commerce action missing action_copy_key")
	}
	if !snapshot.FeatureFlags.Announcements || snapshot.FeatureFlags.ForceUpdatePrompt || snapshot.FeatureFlags.StrictDeviceEnforcement {
		t.Fatalf("feature flags = %+v, want contract feature flag snapshot", snapshot.FeatureFlags)
	}
	if len(events.events) != 1 {
		t.Fatalf("events len = %d, want 1", len(events.events))
	}
	event := events.events[0]
	if event.Type != "bootstrap_requested" || event.RequestID != "req-bootstrap-1" || event.ConfigVersion != "cfg_test" {
		t.Fatalf("event = %+v, want bootstrap request with request/config version", event)
	}
	if event.Metadata["snapshot_version"] != snapshot.VersionPolicy.SnapshotVersion {
		t.Fatalf("event snapshot_version = %q, want %q", event.Metadata["snapshot_version"], snapshot.VersionPolicy.SnapshotVersion)
	}
}

func TestCodexPlusClientUsageReturnsContractShapeAndEvent(t *testing.T) {
	now := time.Date(2026, 6, 16, 1, 0, 0, 0, time.UTC)
	events := &codexPlusClientTestEvents{}
	keys := &codexPlusClientTestKeys{now: now, nextID: 100}
	svc := newCodexPlusClientTestService(now, &User{ID: 1, Balance: 1520, Status: StatusActive}, keys)
	svc.SetEventSink(events)

	usage, err := svc.ClientUsage(context.Background(), CodexPlusUsageInput{
		UserID:    1,
		DeviceID:  "device-1234",
		RequestID: "req-usage-1",
	})
	if err != nil {
		t.Fatalf("usage: %v", err)
	}

	if usage.BalanceSummary.BalanceDisplay != "1520 credits remaining" || usage.BalanceSummary.LowBalance {
		t.Fatalf("balance summary = %+v, want available balance", usage.BalanceSummary)
	}
	if usage.PeriodUsage.PeriodID != "2026-06-16" || usage.PeriodUsage.UsageDisplay != "118 credits used today" || usage.PeriodUsage.ResetAt == nil {
		t.Fatalf("period usage = %+v, want contract period usage summary", usage.PeriodUsage)
	}
	if usage.RenewAction.ActionType != "manage_plan" || usage.RenewAction.ActionCopyKey == nil || usage.RenewAction.MessageKey == nil {
		t.Fatalf("renew action = %+v, want contract client action", usage.RenewAction)
	}
	if len(events.events) != 1 {
		t.Fatalf("events len = %d, want 1", len(events.events))
	}
	event := events.events[0]
	if event.Type != "usage_requested" || event.RequestID != "req-usage-1" || event.ConfigVersion != "cfg_test" || event.DeviceID != "device-1234" {
		t.Fatalf("event = %+v, want usage request with request/config/device context", event)
	}
}

func TestCodexPlusClientBootstrapSelectsPlanPolicyAndModelsFromEntitlement(t *testing.T) {
	now := time.Date(2026, 6, 16, 2, 0, 0, 0, time.UTC)
	cfg := codexPlusClientTestConfig(now)
	cfg.PlanCatalog.Plans = append([]CodexPlusPlan{
		{
			PlanID:        "unused",
			Name:          "Unused",
			UsagePolicyID: "unused-policy",
			EntitlementSources: CodexPlusEntitlementSources{
				SubscriptionGroupIDs: []int64{99},
			},
			ModelGroups: []string{"unused"},
			RenewURL:    cfg.PlanCatalog.Plans[0].RenewURL,
			IsListed:    true,
			Status:      "active",
		},
	}, cfg.PlanCatalog.Plans...)
	cfg.ModelCatalog.Models = append([]CodexPlusModel{
		{
			ModelID:           "hidden-from-starter",
			DisplayName:       "Hidden From Starter",
			RouteModel:        "openai/hidden-from-starter",
			ModelGroup:        "unused",
			ContextWindow:     8192,
			BillingMultiplier: 1,
			IsDefault:         true,
			IsEnabled:         true,
		},
	}, cfg.ModelCatalog.Models...)
	cfg.UsagePolicy.Policies = append([]CodexPlusUsageRule{
		{
			PolicyID:            "unused-policy",
			LowBalanceThreshold: 5000,
		},
	}, cfg.UsagePolicy.Policies...)

	user := &User{ID: 1, Balance: 1520, Status: StatusActive}
	activeSub := UserSubscription{
		ID:        10,
		UserID:    user.ID,
		GroupID:   20,
		StartsAt:  now.Add(-time.Hour),
		ExpiresAt: now.AddDate(0, 1, 0),
		Status:    SubscriptionStatusActive,
	}
	keys := &codexPlusClientTestKeys{now: now, nextID: 100}
	svc := NewCodexPlusClientService(
		&codexPlusClientTestConfigReader{cfg: cfg},
		&codexPlusClientTestUsers{user: user},
		&codexPlusClientTestSubs{active: []UserSubscription{activeSub}, all: []UserSubscription{activeSub}},
		keys,
		nil,
	)
	svc.SetNow(func() time.Time { return now })

	snapshot, err := svc.Bootstrap(context.Background(), CodexPlusBootstrapInput{UserID: 1, DeviceID: "device-1234"})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	if snapshot.Plan.PlanID != "starter" {
		t.Fatalf("plan id = %q, want starter from subscription entitlement", snapshot.Plan.PlanID)
	}
	if len(snapshot.Models) != 1 || snapshot.Models[0].ModelID != "gpt-5-codex" {
		t.Fatalf("models = %+v, want only starter model group", snapshot.Models)
	}
	if snapshot.Service.Status != ClientServiceStatusAvailable {
		t.Fatalf("status = %q, want available using starter policy", snapshot.Service.Status)
	}
}

func TestCodexPlusClientUpsertDeviceIsIdempotentAndEmitsEvent(t *testing.T) {
	now := time.Date(2026, 6, 16, 3, 0, 0, 0, time.UTC)
	events := &codexPlusClientTestEvents{}
	devices := &codexPlusClientTestDevices{}
	svc := newCodexPlusClientTestService(now, &User{ID: 1, Balance: 1520, Status: StatusActive}, nil)
	svc.SetDeviceStore(devices)
	svc.SetEventSink(events)

	codexVersion := "0.2.0"
	input := CodexPlusDeviceInput{
		DeviceID:     " device-1234 ",
		Platform:     "windows",
		AppVersion:   "0.2.0",
		CodexVersion: &codexVersion,
		LastSeenAt:   now,
		RequestID:    "req-device-1",
	}
	first, err := svc.UpsertDevice(context.Background(), 1, input)
	if err != nil {
		t.Fatalf("first upsert device: %v", err)
	}
	second, err := svc.UpsertDevice(context.Background(), 1, input)
	if err != nil {
		t.Fatalf("second upsert device: %v", err)
	}

	if first.DeviceID != "device-1234" || second.DeviceID != "device-1234" || second.Status != ClientDeviceStatusActive {
		t.Fatalf("device snapshots = %+v then %+v, want idempotent active device", first, second)
	}
	if devices.upsertCount != 2 || devices.lastInput.UserID != 1 || devices.lastInput.DeviceID != "device-1234" {
		t.Fatalf("device store = count %d input %+v, want two scoped upserts", devices.upsertCount, devices.lastInput)
	}
	if devices.lastInput.Metadata["codex_version"] != codexVersion {
		t.Fatalf("metadata codex_version = %#v, want provided value", devices.lastInput.Metadata["codex_version"])
	}
	if len(events.events) != 2 {
		t.Fatalf("events len = %d, want 2", len(events.events))
	}
	event := events.events[1]
	if event.Type != "device_registered" || event.UserID != 1 || event.DeviceID != "device-1234" || event.RequestID != "req-device-1" {
		t.Fatalf("event = %+v, want device event with user/device/request context", event)
	}
	if event.Metadata["platform"] != "windows" || event.Metadata["app_version"] != "0.2.0" {
		t.Fatalf("event metadata = %+v, want client device metadata", event.Metadata)
	}
}

func TestCodexPlusClientRedeemMapsStatusesAndEmitsEvent(t *testing.T) {
	now := time.Date(2026, 6, 16, 4, 0, 0, 0, time.UTC)
	deviceID := "device-1234"

	successEvents := &codexPlusClientTestEvents{}
	successRedeemer := &codexPlusClientTestRedeemer{
		code: &RedeemCode{ID: 7, Code: "VALID", Type: RedeemTypeBalance, Value: 50, Status: StatusUsed},
	}
	svc := newCodexPlusClientTestService(now, &User{ID: 1, Balance: 1520, Status: StatusActive}, nil)
	svc.redeemService = successRedeemer
	svc.SetEventSink(successEvents)

	result, err := svc.Redeem(context.Background(), 1, " VALID ", &deviceID, "req-redeem-1")
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}
	if successRedeemer.calls != 1 || successRedeemer.codeValue != "VALID" {
		t.Fatalf("redeemer calls = %d code %q, want trimmed redeem", successRedeemer.calls, successRedeemer.codeValue)
	}
	if result.RedeemStatus != "applied" || result.EntitlementDeltaSummary != "50 credits added." || result.ServiceStatusAfter != ClientServiceStatusAvailable {
		t.Fatalf("result = %+v, want applied redeem with refreshed status", result)
	}
	if len(successEvents.events) != 1 || successEvents.events[0].Type != "redeem_attempted" || successEvents.events[0].Status != "applied" || successEvents.events[0].RequestID != "req-redeem-1" {
		t.Fatalf("events = %+v, want applied redeem event", successEvents.events)
	}

	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{name: "not-found", err: ErrRedeemCodeNotFound, want: "invalid"},
		{name: "used", err: ErrRedeemCodeUsed, want: "already_used"},
		{name: "expired", err: ErrRedeemCodeExpired, want: "expired"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := &codexPlusClientTestEvents{}
			redeemer := &codexPlusClientTestRedeemer{err: tc.err}
			svc := newCodexPlusClientTestService(now, &User{ID: 1, Balance: 1520, Status: StatusActive}, nil)
			svc.redeemService = redeemer
			svc.SetEventSink(events)

			result, err := svc.Redeem(context.Background(), 1, "BAD", &deviceID, "req-redeem-failed")
			if err != nil {
				t.Fatalf("redeem returned error: %v", err)
			}
			if result.RedeemStatus != tc.want {
				t.Fatalf("status = %q, want %q", result.RedeemStatus, tc.want)
			}
			if len(events.events) != 1 || events.events[0].Status != tc.want || events.events[0].RequestID != "req-redeem-failed" {
				t.Fatalf("events = %+v, want failed redeem event", events.events)
			}
		})
	}
}

func TestCodexPlusClientBootstrapNotPurchasedDoesNotCreateKey(t *testing.T) {
	now := time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC)
	keys := &codexPlusClientTestKeys{now: now, nextID: 100}
	svc := newCodexPlusClientTestService(now, &User{ID: 1, Balance: 0, Status: StatusActive}, keys)
	svc.subscriptions = &codexPlusClientTestSubs{}

	snapshot, err := svc.Bootstrap(context.Background(), CodexPlusBootstrapInput{UserID: 1, DeviceID: "device-1234"})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if snapshot.Service.Status != ClientServiceStatusNotPurchased {
		t.Fatalf("status = %q, want not_purchased", snapshot.Service.Status)
	}
	if snapshot.Provider.APIKey != nil {
		t.Fatal("not_purchased bootstrap leaked provider api_key")
	}
	if keys.createCount != 0 {
		t.Fatalf("create count = %d, want 0", keys.createCount)
	}
}

func TestCodexPlusClientBootstrapRevokedDeviceSuppressesKey(t *testing.T) {
	now := time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC)
	keys := &codexPlusClientTestKeys{now: now, nextID: 100}
	svc := newCodexPlusClientTestService(now, &User{ID: 1, Balance: 1520, Status: StatusActive}, keys)
	svc.SetDeviceStore(&codexPlusClientTestDevices{
		record: &CodexPlusDevice{UserID: 1, DeviceID: "device-1234", Status: ClientDeviceStatusRevoked},
	})

	snapshot, err := svc.Bootstrap(context.Background(), CodexPlusBootstrapInput{UserID: 1, DeviceID: "device-1234"})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if snapshot.Service.Status != ClientServiceStatusDeviceRevoked {
		t.Fatalf("status = %q, want device_revoked", snapshot.Service.Status)
	}
	if snapshot.Provider.APIKey != nil {
		t.Fatal("device_revoked bootstrap leaked provider api_key")
	}
	if keys.createCount != 0 {
		t.Fatalf("create count = %d, want 0", keys.createCount)
	}
}

func newCodexPlusClientTestService(now time.Time, user *User, keys *codexPlusClientTestKeys) *CodexPlusClientService {
	cfg := codexPlusClientTestConfig(now)
	activeSub := UserSubscription{
		ID:            10,
		UserID:        user.ID,
		GroupID:       20,
		StartsAt:      now.Add(-time.Hour),
		ExpiresAt:     now.AddDate(0, 1, 0),
		Status:        SubscriptionStatusActive,
		DailyUsageUSD: 118,
	}
	subReader := &codexPlusClientTestSubs{active: []UserSubscription{activeSub}, all: []UserSubscription{activeSub}}
	svc := NewCodexPlusClientService(&codexPlusClientTestConfigReader{cfg: cfg}, &codexPlusClientTestUsers{user: user}, subReader, keys, nil)
	svc.SetNow(func() time.Time { return now })
	return svc
}

func codexPlusClientTestConfig(now time.Time) *CodexPlusConfig {
	billingURL := "https://codex-plus.example/billing"
	return &CodexPlusConfig{
		ConfigVersion: "cfg_test",
		PublishScope:  "production",
		UpdatedBy:     "test",
		UpdatedAt:     now.UTC().Format(time.RFC3339),
		PlanCatalog: CodexPlusPlanCatalog{
			ConfigVersion: "cfg_test",
			Plans: []CodexPlusPlan{{
				PlanID:        "starter",
				Name:          "Starter",
				BillingPeriod: "month",
				Currency:      "USD",
				DisplayPrice:  "$0",
				EntitlementGrant: CodexPlusEntitlementGrant{
					BalanceCredit: 1000,
					DurationDays:  30,
				},
				EntitlementSources: CodexPlusEntitlementSources{
					SubscriptionGroupIDs: []int64{20},
				},
				ModelGroups:   []string{"default"},
				UsagePolicyID: "default",
				RenewURL:      &billingURL,
				IsListed:      true,
				Status:        "active",
			}},
		},
		ModelCatalog: CodexPlusModelCatalog{
			ConfigVersion: "cfg_test",
			Models: []CodexPlusModel{{
				ModelID:           "gpt-5-codex",
				DisplayName:       "GPT-5 Codex",
				RouteModel:        "openai/gpt-5-codex",
				ModelGroup:        "default",
				ContextWindow:     8192,
				BillingMultiplier: 1,
				IsDefault:         true,
				IsEnabled:         true,
			}},
		},
		UsagePolicy: CodexPlusUsagePolicy{
			ConfigVersion: "cfg_test",
			Policies: []CodexPlusUsageRule{{
				PolicyID:                   "default",
				LowBalanceThreshold:        100,
				DailyQuota:                 1000,
				ConcurrencyLimit:           1,
				RPMLimit:                   60,
				TPMLimit:                   1000,
				ExpiredBehavior:            "block",
				InsufficientBalanceMessage: "Codex++ entitlement is not active.",
			}},
		},
		FeatureFlags: CodexPlusFeatureFlagsDoc{
			ConfigVersion: "cfg_test",
			Flags: CodexPlusFeatureFlags{
				InstallAssistant: true,
				NewUserTutorial:  true,
				ModelSelector:    true,
				DiagnosticExport: true,
				Announcements:    true,
			},
		},
	}
}

type codexPlusClientTestConfigReader struct {
	cfg *CodexPlusConfig
	err error
}

func (r *codexPlusClientTestConfigReader) Get(ctx context.Context) (*CodexPlusConfig, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.cfg, nil
}

type codexPlusClientTestUsers struct {
	user *User
}

func (r *codexPlusClientTestUsers) GetByID(ctx context.Context, id int64) (*User, error) {
	if r.user == nil || r.user.ID != id {
		return nil, ErrUserNotFound
	}
	cp := *r.user
	return &cp, nil
}

type codexPlusClientTestSubs struct {
	active []UserSubscription
	all    []UserSubscription
}

func (r *codexPlusClientTestSubs) ListUserSubscriptions(ctx context.Context, userID int64) ([]UserSubscription, error) {
	return append([]UserSubscription(nil), r.all...), nil
}

func (r *codexPlusClientTestSubs) ListActiveUserSubscriptions(ctx context.Context, userID int64) ([]UserSubscription, error) {
	return append([]UserSubscription(nil), r.active...), nil
}

type codexPlusClientTestKeys struct {
	now         time.Time
	nextID      int64
	keys        []APIKey
	createCount int
}

func (r *codexPlusClientTestKeys) Create(ctx context.Context, userID int64, req CreateAPIKeyRequest) (*APIKey, error) {
	r.createCount++
	r.nextID++
	key := APIKey{
		ID:        r.nextID,
		UserID:    userID,
		Key:       "sk-user-test-" + strconvFormatIntCodexPlusClient(r.nextID),
		Name:      req.Name,
		Status:    StatusActive,
		CreatedAt: r.now,
		UpdatedAt: r.now,
	}
	r.keys = append(r.keys, key)
	return &key, nil
}

func (r *codexPlusClientTestKeys) GetByID(ctx context.Context, id int64) (*APIKey, error) {
	for i := range r.keys {
		if r.keys[i].ID == id {
			key := r.keys[i]
			return &key, nil
		}
	}
	return nil, ErrAPIKeyNotFound
}

func (r *codexPlusClientTestKeys) List(ctx context.Context, userID int64, params pagination.PaginationParams, filters APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error) {
	out := make([]APIKey, 0, len(r.keys))
	for i := range r.keys {
		if r.keys[i].UserID == userID {
			out = append(out, r.keys[i])
		}
	}
	return out, &pagination.PaginationResult{Total: int64(len(out)), Page: 1, PageSize: len(out), Pages: 1}, nil
}

type codexPlusClientTestDevices struct {
	record      *CodexPlusDevice
	upsertCount int
	lastInput   CodexPlusDeviceUpsert
}

func (r *codexPlusClientTestDevices) GetByUserAndDevice(ctx context.Context, userID int64, deviceID string) (*CodexPlusDevice, error) {
	if r.record == nil || r.record.UserID != userID || r.record.DeviceID != deviceID {
		return nil, infraerrors.NotFound("CLIENT_DEVICE_NOT_FOUND", "device not found")
	}
	cp := *r.record
	return &cp, nil
}

func (r *codexPlusClientTestDevices) Upsert(ctx context.Context, input CodexPlusDeviceUpsert) (*CodexPlusDevice, error) {
	r.upsertCount++
	r.lastInput = input
	if r.record != nil {
		cp := *r.record
		return &cp, nil
	}
	lastSeenAt := time.Now().UTC()
	if input.LastSeenAt != nil {
		lastSeenAt = input.LastSeenAt.UTC()
	}
	r.record = &CodexPlusDevice{
		UserID:     input.UserID,
		DeviceID:   input.DeviceID,
		Status:     ClientDeviceStatusActive,
		LastSeenAt: &lastSeenAt,
		CreatedAt:  lastSeenAt,
		UpdatedAt:  lastSeenAt,
	}
	cp := *r.record
	return &cp, nil
}

type codexPlusClientTestEvents struct {
	events []CodexPlusClientEvent
}

func (r *codexPlusClientTestEvents) Record(ctx context.Context, event CodexPlusClientEvent) error {
	cp := event
	if event.Metadata != nil {
		cp.Metadata = make(map[string]string, len(event.Metadata))
		for key, value := range event.Metadata {
			cp.Metadata[key] = value
		}
	}
	r.events = append(r.events, cp)
	return nil
}

type codexPlusClientTestRedeemer struct {
	code      *RedeemCode
	err       error
	calls     int
	userID    int64
	codeValue string
}

func (r *codexPlusClientTestRedeemer) Redeem(ctx context.Context, userID int64, code string) (*RedeemCode, error) {
	r.calls++
	r.userID = userID
	r.codeValue = code
	if r.err != nil {
		return nil, r.err
	}
	if r.code == nil {
		return nil, ErrRedeemCodeNotFound
	}
	cp := *r.code
	return &cp, nil
}

func strconvFormatIntCodexPlusClient(v int64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := v < 0
	if neg {
		v = -v
	}
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
