package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	CodexPlusClientProviderName = "Codex++ Cloud"

	ClientServiceStatusAvailable        = "available"
	ClientServiceStatusNotPurchased     = "not_purchased"
	ClientServiceStatusExpired          = "expired"
	ClientServiceStatusLowBalance       = "low_balance"
	ClientServiceStatusDeviceRevoked    = "device_revoked"
	ClientServiceStatusModelUnavailable = "model_unavailable"
	ClientServiceStatusGatewayUnhealthy = "gateway_unhealthy"

	ClientDeviceStatusActive  = "active"
	ClientDeviceStatusRevoked = "revoked"
	ClientDeviceStatusBlocked = "blocked"
	ClientDeviceStatusUnknown = "unknown"
)

type CodexPlusClientConfigReader interface {
	Get(ctx context.Context) (*CodexPlusConfig, error)
}

type CodexPlusClientUserReader interface {
	GetByID(ctx context.Context, id int64) (*User, error)
}

type CodexPlusClientSubscriptionReader interface {
	ListUserSubscriptions(ctx context.Context, userID int64) ([]UserSubscription, error)
	ListActiveUserSubscriptions(ctx context.Context, userID int64) ([]UserSubscription, error)
}

type CodexPlusClientAPIKeyManager interface {
	Create(ctx context.Context, userID int64, req CreateAPIKeyRequest) (*APIKey, error)
	GetByID(ctx context.Context, id int64) (*APIKey, error)
	List(ctx context.Context, userID int64, params pagination.PaginationParams, filters APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error)
}

type CodexPlusClientDeviceStore interface {
	GetByUserAndDevice(ctx context.Context, userID int64, deviceID string) (*CodexPlusDevice, error)
	Upsert(ctx context.Context, input CodexPlusDeviceUpsert) (*CodexPlusDevice, error)
}

type CodexPlusClientManagedProviderKeyStore interface {
	GetByUserAndProvider(ctx context.Context, userID int64, providerID string) (*CodexPlusManagedProviderKey, error)
	Upsert(ctx context.Context, input CodexPlusManagedProviderKeyUpsert) (*CodexPlusManagedProviderKey, error)
}

type CodexPlusClientEventSink interface {
	Record(ctx context.Context, event CodexPlusClientEvent) error
}

type CodexPlusClientRedeemer interface {
	Redeem(ctx context.Context, userID int64, code string) (*RedeemCode, error)
}

type CodexPlusClientService struct {
	configReader     CodexPlusClientConfigReader
	userReader       CodexPlusClientUserReader
	subscriptions    CodexPlusClientSubscriptionReader
	apiKeys          CodexPlusClientAPIKeyManager
	redeemService    CodexPlusClientRedeemer
	devices          CodexPlusClientDeviceStore
	managedKeys      CodexPlusClientManagedProviderKeyStore
	events           CodexPlusClientEventSink
	gatewayBaseURL   string
	supportBaseURL   string
	minClientVersion string
	now              func() time.Time
}

type CodexPlusBootstrapInput struct {
	UserID        int64
	DeviceID      string
	ClientVersion string
	RequestID     string
}

type CodexPlusUsageInput struct {
	UserID    int64
	DeviceID  string
	RequestID string
}

type CodexPlusDeviceInput struct {
	DeviceID     string
	Platform     string
	AppVersion   string
	CodexVersion *string
	LastSeenAt   time.Time
	RequestID    string
}

type CodexPlusClientEvent struct {
	Type          string
	UserID        int64
	DeviceID      string
	Status        string
	RequestID     string
	ConfigVersion string
	OccurredAt    time.Time
	Metadata      map[string]string
}

type CodexPlusBootstrapSnapshot struct {
	Service       CodexPlusClientServiceState
	Provider      CodexPlusManagedProvider
	Plan          CodexPlusClientPlanSummary
	Models        []CodexPlusClientModel
	Usage         CodexPlusClientUsageSummary
	FeatureFlags  CodexPlusClientFeatureFlags
	Announcements []CodexPlusClientAnnouncement
	VersionPolicy CodexPlusClientVersionPolicy
	Device        CodexPlusDeviceSnapshot
}

type CodexPlusClientServiceState struct {
	Status     string
	Message    string
	MessageKey string
	ActionHint string
	Retryable  bool
	SupportURL *string
	ErrorCode  *string
}

type CodexPlusManagedProvider struct {
	ProviderID     string
	DisplayName    string
	GatewayBaseURL string
	AuthMode       string
	APIKey         *string
	KeySummary     CodexPlusManagedProviderKeySummary
	DefaultModel   string
}

type CodexPlusManagedProviderKeySummary struct {
	KeyID      string
	MaskedKey  string
	CreatedAt  time.Time
	LastUsedAt *time.Time
}

type CodexPlusClientPlanSummary struct {
	PlanID         string
	Name           string
	Status         string
	ExpiresAt      *time.Time
	RenewURL       *string
	CommerceAction CodexPlusClientAction
}

type CodexPlusClientModel struct {
	ModelID        string
	RouteModel     string
	Label          string
	IsDefault      bool
	IsAvailable    bool
	DisabledReason *string
}

type CodexPlusClientUsageSummary struct {
	BalanceDisplay     string
	LowBalance         bool
	PeriodUsageDisplay string
	RateLimitState     string
	RenewAction        CodexPlusClientAction
}

type CodexPlusClientAction struct {
	ActionType    string
	MessageKey    *string
	ActionCopyKey *string
	Label         string
	URL           *string
}

type CodexPlusClientFeatureFlags struct {
	AdvancedProviderConfig  bool
	InstallAssistant        bool
	NewUserTutorial         bool
	ModelSelector           bool
	DiagnosticExport        bool
	Announcements           bool
	ForceUpdatePrompt       bool
	StrictDeviceEnforcement bool
}

type CodexPlusClientAnnouncement struct {
	ID       string
	Severity string
	Message  string
	URL      *string
}

type CodexPlusClientVersionPolicy struct {
	ConfigVersion        string
	SnapshotVersion      string
	RefreshTTLSeconds    int
	ForceRefresh         bool
	MinimumClientVersion string
}

type CodexPlusDeviceSnapshot struct {
	DeviceID        string
	Status          string
	Message         string
	SnapshotVersion string
}

type CodexPlusUsageSnapshot struct {
	ServiceStatus   string
	BalanceSummary  CodexPlusClientBalanceSummary
	PeriodUsage     CodexPlusClientPeriodUsageSummary
	RateLimitState  string
	RenewAction     CodexPlusClientAction
	LastUpdatedAt   time.Time
	SnapshotVersion string
}

type CodexPlusClientBalanceSummary struct {
	BalanceDisplay string
	LowBalance     bool
}

type CodexPlusClientPeriodUsageSummary struct {
	PeriodID     string
	UsageDisplay string
	ResetAt      *time.Time
}

type CodexPlusRedeemResult struct {
	RedeemStatus            string
	EntitlementDeltaSummary string
	ServiceStatusAfter      string
	SnapshotVersion         string
	Message                 string
}

type codexPlusEntitlement struct {
	active       bool
	expired      bool
	lowBalance   bool
	activeSub    *UserSubscription
	expiredSub   *UserSubscription
	policy       *CodexPlusUsageRule
	plan         *CodexPlusPlan
	defaultModel *CodexPlusModel
}

func NewCodexPlusClientService(
	configReader CodexPlusClientConfigReader,
	userReader CodexPlusClientUserReader,
	subscriptions CodexPlusClientSubscriptionReader,
	apiKeys CodexPlusClientAPIKeyManager,
	redeemService CodexPlusClientRedeemer,
) *CodexPlusClientService {
	return &CodexPlusClientService{
		configReader:     configReader,
		userReader:       userReader,
		subscriptions:    subscriptions,
		apiKeys:          apiKeys,
		redeemService:    redeemService,
		gatewayBaseURL:   "https://api.codex-plus.example/v1",
		supportBaseURL:   "https://codex-plus.example",
		minClientVersion: "0.1.0",
		now:              time.Now,
	}
}

func (s *CodexPlusClientService) SetDeviceStore(store CodexPlusClientDeviceStore) {
	s.devices = store
}

func (s *CodexPlusClientService) SetManagedProviderKeyStore(store CodexPlusClientManagedProviderKeyStore) {
	s.managedKeys = store
}

func (s *CodexPlusClientService) SetEventSink(sink CodexPlusClientEventSink) {
	s.events = sink
}

func (s *CodexPlusClientService) SetGatewayBaseURL(value string) {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		s.gatewayBaseURL = trimmed
	}
}

func (s *CodexPlusClientService) SetSupportBaseURL(value string) {
	if trimmed := strings.TrimRight(strings.TrimSpace(value), "/"); trimmed != "" {
		s.supportBaseURL = trimmed
	}
}

func (s *CodexPlusClientService) SetNow(now func() time.Time) {
	if now != nil {
		s.now = now
	}
}

func (s *CodexPlusClientService) Bootstrap(ctx context.Context, input CodexPlusBootstrapInput) (*CodexPlusBootstrapSnapshot, error) {
	if err := s.validateReady(); err != nil {
		return nil, err
	}
	user, err := s.userReader.GetByID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("get codexplus client user: %w", err)
	}

	cfg, cfgErr := s.configReader.Get(ctx)
	if cfgErr != nil || cfg == nil {
		cfg = defaultClientConfig(s.now())
	}
	device := s.loadDevice(ctx, input.UserID, input.DeviceID)
	entitlement := s.resolveEntitlement(ctx, user, cfg)

	status := s.resolveServiceStatus(cfgErr, device, entitlement)
	provider, keyErr := s.providerForStatus(ctx, input.UserID, status, entitlement)
	if keyErr != nil && canReturnProviderKey(status) {
		status = ClientServiceStatusGatewayUnhealthy
		provider.APIKey = nil
		provider.DefaultModel = defaultModelID(entitlement.defaultModel)
	}

	snapshot := s.snapshotFor(user, cfg, device, entitlement, status, provider)
	s.emit(ctx, "bootstrap_requested", input.UserID, device.DeviceID, status, map[string]string{
		"client_version":   input.ClientVersion,
		"snapshot_version": snapshot.VersionPolicy.SnapshotVersion,
	}, input.RequestID, cfg.ConfigVersion)
	return snapshot, nil
}

func (s *CodexPlusClientService) Usage(ctx context.Context, userID int64) (*CodexPlusUsageSnapshot, error) {
	return s.usage(ctx, CodexPlusUsageInput{UserID: userID}, false)
}

func (s *CodexPlusClientService) ClientUsage(ctx context.Context, input CodexPlusUsageInput) (*CodexPlusUsageSnapshot, error) {
	return s.usage(ctx, input, true)
}

func (s *CodexPlusClientService) usage(ctx context.Context, input CodexPlusUsageInput, emitEvent bool) (*CodexPlusUsageSnapshot, error) {
	if err := s.validateReady(); err != nil {
		return nil, err
	}
	user, err := s.userReader.GetByID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("get codexplus client user: %w", err)
	}
	cfg, cfgErr := s.configReader.Get(ctx)
	if cfgErr != nil || cfg == nil {
		cfg = defaultClientConfig(s.now())
	}
	entitlement := s.resolveEntitlement(ctx, user, cfg)
	status := s.resolveServiceStatus(cfgErr, CodexPlusDeviceSnapshot{Status: ClientDeviceStatusActive}, entitlement)
	usage := s.usageFor(user, entitlement, status)
	now := s.now().UTC()
	snapshot := &CodexPlusUsageSnapshot{
		ServiceStatus: status,
		BalanceSummary: CodexPlusClientBalanceSummary{
			BalanceDisplay: usage.BalanceDisplay,
			LowBalance:     usage.LowBalance,
		},
		PeriodUsage:     periodUsageSummary(entitlement.activeSub, now),
		RateLimitState:  usage.RateLimitState,
		RenewAction:     usage.RenewAction,
		LastUpdatedAt:   now,
		SnapshotVersion: snapshotVersion(status, now),
	}
	if emitEvent {
		s.emit(ctx, "usage_requested", input.UserID, input.DeviceID, status, map[string]string{
			"snapshot_version": snapshot.SnapshotVersion,
		}, input.RequestID, cfg.ConfigVersion)
	}
	return snapshot, nil
}

func (s *CodexPlusClientService) UpsertDevice(ctx context.Context, userID int64, input CodexPlusDeviceInput) (*CodexPlusDeviceSnapshot, error) {
	if strings.TrimSpace(input.DeviceID) == "" {
		return nil, infraerrors.BadRequest("CLIENT_DEVICE_ID_REQUIRED", "device_id is required")
	}
	seenAt := input.LastSeenAt
	if seenAt.IsZero() {
		seenAt = s.now()
	}
	record := &CodexPlusDevice{
		UserID:     userID,
		DeviceID:   strings.TrimSpace(input.DeviceID),
		Status:     ClientDeviceStatusActive,
		LastSeenAt: &seenAt,
		CreatedAt:  seenAt,
		UpdatedAt:  seenAt,
	}
	var err error
	if s.devices != nil {
		record, err = s.devices.Upsert(ctx, CodexPlusDeviceUpsert{
			UserID:     userID,
			DeviceID:   record.DeviceID,
			Platform:   codexPlusClientStringPtrIfNotBlank(input.Platform),
			AppVersion: codexPlusClientStringPtrIfNotBlank(input.AppVersion),
			Status:     ClientDeviceStatusActive,
			LastSeenAt: &seenAt,
			Metadata: map[string]any{
				"codex_version": codexPlusClientNullableStringPtr(input.CodexVersion),
			},
		})
		if err != nil {
			return nil, fmt.Errorf("upsert codexplus device: %w", err)
		}
	}
	out := deviceSnapshotFromRecord(record, snapshotVersion(record.Status, s.now().UTC()))
	s.emit(ctx, "device_registered", userID, out.DeviceID, out.Status, map[string]string{
		"platform":    input.Platform,
		"app_version": input.AppVersion,
	}, input.RequestID, "")
	return &out, nil
}

func (s *CodexPlusClientService) Redeem(ctx context.Context, userID int64, code string, deviceID *string, requestID string) (*CodexPlusRedeemResult, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, infraerrors.BadRequest("CLIENT_REDEEM_CODE_REQUIRED", "code is required")
	}
	if s.redeemService == nil {
		return nil, infraerrors.ServiceUnavailable("CLIENT_REDEEM_UNAVAILABLE", "redeem service is unavailable")
	}
	redeemed, err := s.redeemService.Redeem(ctx, userID, code)
	if err != nil {
		result := s.redeemErrorResult(ctx, userID, err)
		s.emit(ctx, "redeem_attempted", userID, derefString(deviceID), result.RedeemStatus, nil, requestID, "")
		return result, nil
	}
	usage, usageErr := s.Usage(ctx, userID)
	statusAfter := ClientServiceStatusAvailable
	if usageErr == nil && usage != nil {
		statusAfter = usage.ServiceStatus
	}
	result := &CodexPlusRedeemResult{
		RedeemStatus:            "applied",
		EntitlementDeltaSummary: redeemDeltaSummary(redeemed),
		ServiceStatusAfter:      statusAfter,
		SnapshotVersion:         snapshotVersion(statusAfter, s.now().UTC()),
		Message:                 "Redeem code applied.",
	}
	s.emit(ctx, "redeem_attempted", userID, derefString(deviceID), result.RedeemStatus, nil, requestID, "")
	return result, nil
}

func (s *CodexPlusClientService) validateReady() error {
	if s == nil || s.configReader == nil || s.userReader == nil {
		return infraerrors.ServiceUnavailable("CLIENT_SERVICE_UNAVAILABLE", "codexplus client service is unavailable")
	}
	return nil
}

func (s *CodexPlusClientService) resolveServiceStatus(configErr error, device CodexPlusDeviceSnapshot, ent codexPlusEntitlement) string {
	if device.Status == ClientDeviceStatusRevoked || device.Status == ClientDeviceStatusBlocked {
		return ClientServiceStatusDeviceRevoked
	}
	if configErr != nil {
		return ClientServiceStatusGatewayUnhealthy
	}
	if !ent.active {
		if ent.expired {
			return ClientServiceStatusExpired
		}
		return ClientServiceStatusNotPurchased
	}
	if ent.defaultModel == nil || !ent.defaultModel.IsEnabled || ent.defaultModel.IsHidden {
		return ClientServiceStatusModelUnavailable
	}
	if ent.lowBalance {
		return ClientServiceStatusLowBalance
	}
	return ClientServiceStatusAvailable
}

func (s *CodexPlusClientService) resolveEntitlement(ctx context.Context, user *User, cfg *CodexPlusConfig) codexPlusEntitlement {
	ent := codexPlusEntitlement{}
	if user == nil {
		return ent
	}
	var activeSubs []UserSubscription
	var allSubs []UserSubscription
	if s.subscriptions != nil {
		if active, err := s.subscriptions.ListActiveUserSubscriptions(ctx, user.ID); err == nil && len(active) > 0 {
			activeSubs = active
		}
		if all, err := s.subscriptions.ListUserSubscriptions(ctx, user.ID); err == nil {
			allSubs = all
		}
	}

	for i := range activeSubs {
		if plan := codexPlusClientPlanForSubscription(cfg, &activeSubs[i]); plan != nil {
			sub := activeSubs[i]
			ent.activeSub = &sub
			ent.plan = plan
			ent.active = true
			break
		}
	}
	if !ent.active {
		for i := range allSubs {
			if !(allSubs[i].IsExpired() || allSubs[i].Status == SubscriptionStatusExpired) {
				continue
			}
			if plan := codexPlusClientPlanForSubscription(cfg, &allSubs[i]); plan != nil {
				sub := allSubs[i]
				ent.expiredSub = &sub
				ent.plan = plan
				ent.expired = true
				break
			}
		}
	}
	if !ent.active && !ent.expired && user.Balance > 0 {
		if plan := codexPlusClientPlanForUserGroups(cfg, user); plan != nil {
			ent.plan = plan
			ent.active = true
		}
	}
	if ent.plan != nil {
		ent.policy = codexPlusClientUsagePolicyForPlan(cfg, ent.plan)
		ent.defaultModel = codexPlusClientDefaultModelForPlan(cfg, ent.plan)
	} else {
		ent.defaultModel = findDefaultModel(cfg)
	}
	if ent.plan != nil && user.Balance > 0 && !ent.expired {
		ent.active = true
	}
	if ent.policy != nil && ent.active && user.Balance > 0 && user.Balance <= float64(ent.policy.LowBalanceThreshold) {
		ent.lowBalance = true
	}
	return ent
}

func (s *CodexPlusClientService) providerForStatus(ctx context.Context, userID int64, status string, ent codexPlusEntitlement) (CodexPlusManagedProvider, error) {
	provider := CodexPlusManagedProvider{
		ProviderID:     CodexPlusManagedProviderID,
		DisplayName:    CodexPlusClientProviderName,
		GatewayBaseURL: s.gatewayBaseURL,
		AuthMode:       "user_side_api_key",
		KeySummary: CodexPlusManagedProviderKeySummary{
			CreatedAt: s.now().UTC(),
		},
		DefaultModel: defaultModelID(ent.defaultModel),
	}
	key, err := s.ensureManagedAPIKey(ctx, userID, canReturnProviderKey(status))
	if err != nil {
		return provider, err
	}
	if key == nil {
		if !canReturnProviderKey(status) {
			provider.DefaultModel = ""
		}
		return provider, nil
	}
	provider.KeySummary = CodexPlusManagedProviderKeySummary{
		KeyID:      strconv.FormatInt(key.ID, 10),
		MaskedKey:  MaskAPIKeyForCodexPlusClient(key.Key),
		CreatedAt:  key.CreatedAt,
		LastUsedAt: key.LastUsedAt,
	}
	if canReturnProviderKey(status) {
		value := key.Key
		provider.APIKey = &value
	} else {
		provider.DefaultModel = ""
	}
	return provider, nil
}

func (s *CodexPlusClientService) ensureManagedAPIKey(ctx context.Context, userID int64, createIfMissing bool) (*APIKey, error) {
	if s.apiKeys == nil {
		return nil, nil
	}
	if s.managedKeys != nil {
		record, err := s.managedKeys.GetByUserAndProvider(ctx, userID, CodexPlusManagedProviderID)
		if err == nil && record != nil && record.APIKeyID > 0 {
			key, getErr := s.apiKeys.GetByID(ctx, record.APIKeyID)
			if getErr == nil && key != nil && key.UserID == userID {
				return key, nil
			}
			if getErr != nil && !infraerrors.IsNotFound(getErr) {
				return nil, getErr
			}
		} else if err != nil && !infraerrors.IsNotFound(err) {
			return nil, err
		}
		if !createIfMissing {
			return nil, nil
		}
		key, err := s.createManagedAPIKey(ctx, userID)
		if err != nil {
			return nil, err
		}
		_, err = s.managedKeys.Upsert(ctx, CodexPlusManagedProviderKeyUpsert{
			UserID:            userID,
			APIKeyID:          key.ID,
			ManagedProviderID: CodexPlusManagedProviderID,
			DisplayName:       CodexPlusClientProviderName,
			KeyPrefix:         codexPlusClientAPIKeyPrefixPtr(key.Key),
			Status:            StatusActive,
		})
		return key, err
	}

	key, err := s.findManagedAPIKeyByName(ctx, userID)
	if err != nil || key != nil || !createIfMissing {
		return key, err
	}
	return s.createManagedAPIKey(ctx, userID)
}

func (s *CodexPlusClientService) findManagedAPIKeyByName(ctx context.Context, userID int64) (*APIKey, error) {
	keys, _, err := s.apiKeys.List(ctx, userID, pagination.PaginationParams{Page: 1, PageSize: 100, SortBy: "created_at", SortOrder: "desc"}, APIKeyListFilters{})
	if err != nil {
		return nil, err
	}
	for i := range keys {
		if keys[i].UserID == userID && keys[i].Name == CodexPlusClientProviderName {
			key := keys[i]
			return &key, nil
		}
	}
	return nil, nil
}

func (s *CodexPlusClientService) createManagedAPIKey(ctx context.Context, userID int64) (*APIKey, error) {
	key, err := s.apiKeys.Create(ctx, userID, CreateAPIKeyRequest{
		Name: CodexPlusClientProviderName,
	})
	if err != nil {
		if errors.Is(err, ErrAPIKeyExists) {
			return s.findManagedAPIKeyByName(ctx, userID)
		}
		return nil, err
	}
	return key, nil
}

func (s *CodexPlusClientService) loadDevice(ctx context.Context, userID int64, deviceID string) CodexPlusDeviceSnapshot {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return CodexPlusDeviceSnapshot{
			Status:          ClientDeviceStatusUnknown,
			Message:         "Device not registered.",
			SnapshotVersion: snapshotVersion(ClientDeviceStatusUnknown, s.now().UTC()),
		}
	}
	if s.devices == nil {
		return CodexPlusDeviceSnapshot{
			DeviceID:        deviceID,
			Status:          ClientDeviceStatusActive,
			Message:         "Device active.",
			SnapshotVersion: snapshotVersion(ClientDeviceStatusActive, s.now().UTC()),
		}
	}
	record, err := s.devices.GetByUserAndDevice(ctx, userID, deviceID)
	if err != nil {
		if infraerrors.IsNotFound(err) {
			return CodexPlusDeviceSnapshot{
				DeviceID:        deviceID,
				Status:          ClientDeviceStatusUnknown,
				Message:         "Device not registered.",
				SnapshotVersion: snapshotVersion(ClientDeviceStatusUnknown, s.now().UTC()),
			}
		}
		return CodexPlusDeviceSnapshot{
			DeviceID:        deviceID,
			Status:          ClientDeviceStatusUnknown,
			Message:         "Device state temporarily unavailable.",
			SnapshotVersion: snapshotVersion(ClientServiceStatusGatewayUnhealthy, s.now().UTC()),
		}
	}
	out := deviceSnapshotFromRecord(record, snapshotVersion(record.Status, s.now().UTC()))
	return out
}

func (s *CodexPlusClientService) snapshotFor(user *User, cfg *CodexPlusConfig, device CodexPlusDeviceSnapshot, ent codexPlusEntitlement, status string, provider CodexPlusManagedProvider) *CodexPlusBootstrapSnapshot {
	now := s.now().UTC()
	serviceState := serviceStateForStatus(status, s.supportBaseURL, ent)
	version := CodexPlusClientVersionPolicy{
		ConfigVersion:        cfg.ConfigVersion,
		SnapshotVersion:      snapshotVersion(status, now),
		RefreshTTLSeconds:    refreshTTLForStatus(status),
		ForceRefresh:         status == ClientServiceStatusModelUnavailable,
		MinimumClientVersion: s.minClientVersion,
	}
	device.SnapshotVersion = version.SnapshotVersion
	return &CodexPlusBootstrapSnapshot{
		Service:       serviceState,
		Provider:      provider,
		Plan:          s.planFor(ent, status),
		Models:        s.modelsFor(cfg, ent, status),
		Usage:         s.usageFor(user, ent, status),
		FeatureFlags:  featureFlagsForStatus(cfg, status),
		Announcements: announcementsForStatus(status, s.supportBaseURL),
		VersionPolicy: version,
		Device:        device,
	}
}

func (s *CodexPlusClientService) planFor(ent codexPlusEntitlement, status string) CodexPlusClientPlanSummary {
	billingURL := s.url("/billing")
	if status == ClientServiceStatusNotPurchased {
		return CodexPlusClientPlanSummary{
			Status:         "none",
			RenewURL:       &billingURL,
			CommerceAction: clientAction("purchase", messageKeyForStatus(status, ent), planActionCopyKey(ent.plan, "purchase"), "Choose a plan", &billingURL),
		}
	}
	if ent.plan == nil {
		return CodexPlusClientPlanSummary{
			Status:         "none",
			RenewURL:       &billingURL,
			CommerceAction: clientAction("purchase", messageKeyForStatus(ClientServiceStatusNotPurchased, ent), "billing.action.purchase", "Choose a plan", &billingURL),
		}
	}
	out := CodexPlusClientPlanSummary{
		PlanID:   ent.plan.PlanID,
		Name:     ent.plan.Name,
		Status:   "active",
		RenewURL: ent.plan.RenewURL,
	}
	if out.RenewURL == nil {
		out.RenewURL = &billingURL
	}
	out.CommerceAction = clientAction("manage_plan", planMessageKey(ent.plan, "active"), "action.manage_plan", "Manage plan", out.RenewURL)
	if ent.activeSub != nil {
		expiresAt := ent.activeSub.ExpiresAt.UTC()
		out.ExpiresAt = &expiresAt
	}
	if status == ClientServiceStatusExpired {
		out.Status = "expired"
		out.CommerceAction = clientAction("renew", messageKeyForStatus(status, ent), planActionCopyKey(ent.plan, "renew"), "Renew plan", out.RenewURL)
		if ent.expiredSub != nil {
			expiresAt := ent.expiredSub.ExpiresAt.UTC()
			out.ExpiresAt = &expiresAt
		}
	} else if status == ClientServiceStatusLowBalance {
		out.CommerceAction = clientAction("recharge", messageKeyForStatus(status, ent), usageActionCopyKey(ent.policy, "renew"), "Recharge", out.RenewURL)
	} else if status == ClientServiceStatusDeviceRevoked {
		supportURL := s.url("/support")
		out.CommerceAction = clientAction("contact_support", messageKeyForStatus(status, ent), "action.contact_support", "Contact support", &supportURL)
	} else if status == ClientServiceStatusGatewayUnhealthy || status == ClientServiceStatusModelUnavailable {
		statusURL := s.url("/status")
		out.CommerceAction = clientAction("check_status", messageKeyForStatus(status, ent), "action.check_status", "Check status", &statusURL)
	}
	return out
}

func (s *CodexPlusClientService) modelsFor(cfg *CodexPlusConfig, ent codexPlusEntitlement, status string) []CodexPlusClientModel {
	if status == ClientServiceStatusNotPurchased || status == ClientServiceStatusExpired || status == ClientServiceStatusDeviceRevoked || status == ClientServiceStatusGatewayUnhealthy {
		return []CodexPlusClientModel{}
	}
	if cfg == nil {
		return []CodexPlusClientModel{}
	}
	models := make([]CodexPlusClientModel, 0, len(cfg.ModelCatalog.Models))
	for _, model := range cfg.ModelCatalog.Models {
		if model.IsHidden {
			continue
		}
		if ent.plan != nil && !codexPlusClientModelAllowedByPlan(ent.plan, model) {
			continue
		}
		disabledReason := model.DisabledReason
		isAvailable := model.IsEnabled
		if !isAvailable && disabledReason == nil {
			reason := "disabled_by_admin"
			disabledReason = &reason
		}
		models = append(models, CodexPlusClientModel{
			ModelID:        model.ModelID,
			RouteModel:     model.RouteModel,
			Label:          model.DisplayName,
			IsDefault:      model.IsDefault,
			IsAvailable:    isAvailable,
			DisabledReason: disabledReason,
		})
	}
	return models
}

func (s *CodexPlusClientService) usageFor(user *User, ent codexPlusEntitlement, status string) CodexPlusClientUsageSummary {
	billingURL := s.url("/billing")
	supportURL := s.url("/support")
	statusURL := s.url("/status")
	switch status {
	case ClientServiceStatusNotPurchased:
		return CodexPlusClientUsageSummary{
			BalanceDisplay:     "No active plan",
			PeriodUsageDisplay: "No usage",
			RateLimitState:     "blocked",
			RenewAction:        clientAction("purchase", messageKeyForStatus(status, ent), usageActionCopyKey(ent.policy, "purchase"), "Choose a plan", &billingURL),
		}
	case ClientServiceStatusExpired:
		return CodexPlusClientUsageSummary{
			BalanceDisplay:     "Plan expired",
			PeriodUsageDisplay: "No current usage",
			RateLimitState:     "blocked",
			RenewAction:        clientAction("renew", messageKeyForStatus(status, ent), usageActionCopyKey(ent.policy, "renew"), "Renew plan", &billingURL),
		}
	case ClientServiceStatusDeviceRevoked:
		return CodexPlusClientUsageSummary{
			BalanceDisplay:     "Hidden while device is revoked",
			PeriodUsageDisplay: "Hidden while device is revoked",
			RateLimitState:     "blocked",
			RenewAction:        clientAction("contact_support", messageKeyForStatus(status, ent), "action.contact_support", "Contact support", &supportURL),
		}
	case ClientServiceStatusGatewayUnhealthy:
		return CodexPlusClientUsageSummary{
			BalanceDisplay:     "Temporarily unavailable",
			PeriodUsageDisplay: "Temporarily unavailable",
			RateLimitState:     "blocked",
			RenewAction:        clientAction("check_status", messageKeyForStatus(status, ent), "action.check_status", "Check status", &statusURL),
		}
	}
	balance := 0.0
	if user != nil {
		balance = user.Balance
	}
	label := "Manage plan"
	actionType := "manage_plan"
	copyKey := "action.manage_plan"
	messageKey := "usage.manage_plan"
	if status == ClientServiceStatusLowBalance {
		label = "Recharge"
		actionType = "recharge"
		copyKey = usageActionCopyKey(ent.policy, "renew")
		messageKey = messageKeyForStatus(status, ent)
	}
	return CodexPlusClientUsageSummary{
		BalanceDisplay:     formatCredits(balance) + " credits remaining",
		LowBalance:         status == ClientServiceStatusLowBalance,
		PeriodUsageDisplay: periodUsageDisplay(ent.activeSub),
		RateLimitState:     "normal",
		RenewAction:        clientAction(actionType, messageKey, copyKey, label, &billingURL),
	}
}

func (s *CodexPlusClientService) redeemErrorResult(ctx context.Context, userID int64, err error) *CodexPlusRedeemResult {
	status := "not_allowed"
	message := "Redeem code could not be applied."
	switch {
	case errors.Is(err, ErrRedeemCodeNotFound):
		status = "invalid"
		message = "Redeem code is invalid."
	case errors.Is(err, ErrRedeemCodeUsed):
		status = "already_used"
		message = "Redeem code has already been used."
	case errors.Is(err, ErrRedeemCodeExpired):
		status = "expired"
		message = "Redeem code has expired."
	}
	serviceStatus := ClientServiceStatusNotPurchased
	if usage, usageErr := s.Usage(ctx, userID); usageErr == nil && usage != nil {
		serviceStatus = usage.ServiceStatus
	}
	return &CodexPlusRedeemResult{
		RedeemStatus:            status,
		EntitlementDeltaSummary: "",
		ServiceStatusAfter:      serviceStatus,
		SnapshotVersion:         snapshotVersion(serviceStatus, s.now().UTC()),
		Message:                 message,
	}
}

func (s *CodexPlusClientService) emit(ctx context.Context, eventType string, userID int64, deviceID, status string, metadata map[string]string, requestID, configVersion string) {
	if s.events == nil {
		return
	}
	_ = s.events.Record(ctx, CodexPlusClientEvent{
		Type:          eventType,
		UserID:        userID,
		DeviceID:      deviceID,
		Status:        status,
		RequestID:     requestID,
		ConfigVersion: configVersion,
		OccurredAt:    s.now().UTC(),
		Metadata:      metadata,
	})
}

func (s *CodexPlusClientService) url(path string) string {
	base := strings.TrimRight(s.supportBaseURL, "/")
	if base == "" {
		base = "https://codex-plus.example"
	}
	return base + path
}

func defaultClientConfig(now time.Time) *CodexPlusConfig {
	cfg := DefaultCodexPlusConfig(now)
	return &cfg
}

func findDefaultModel(cfg *CodexPlusConfig) *CodexPlusModel {
	if cfg == nil {
		return nil
	}
	for i := range cfg.ModelCatalog.Models {
		if cfg.ModelCatalog.Models[i].IsDefault {
			return &cfg.ModelCatalog.Models[i]
		}
	}
	return nil
}

func codexPlusClientPlanForSubscription(cfg *CodexPlusConfig, sub *UserSubscription) *CodexPlusPlan {
	if cfg == nil || sub == nil {
		return nil
	}
	for i := range cfg.PlanCatalog.Plans {
		plan := &cfg.PlanCatalog.Plans[i]
		if !isCodexPlusPlanEnabled(plan.Status) {
			continue
		}
		sources := plan.EntitlementSources
		if containsCodexPlusInt64(sources.SubscriptionGroupIDs, sub.GroupID) {
			return plan
		}
		if sub.Group != nil {
			if containsCodexPlusInt64(sources.SubscriptionGroupIDs, sub.Group.ID) {
				return plan
			}
			if containsCodexPlusStringFold(sources.GroupNames, sub.Group.Name) {
				return plan
			}
		}
	}
	return nil
}

func codexPlusClientPlanForUserGroups(cfg *CodexPlusConfig, user *User) *CodexPlusPlan {
	if cfg == nil || user == nil || len(user.AllowedGroups) == 0 {
		return nil
	}
	for i := range cfg.PlanCatalog.Plans {
		plan := &cfg.PlanCatalog.Plans[i]
		if !isCodexPlusPlanEnabled(plan.Status) {
			continue
		}
		sources := plan.EntitlementSources
		for _, groupID := range user.AllowedGroups {
			if containsCodexPlusInt64(sources.SubscriptionGroupIDs, groupID) || containsCodexPlusInt64(sources.APIKeyGroupIDs, groupID) {
				return plan
			}
		}
	}
	return nil
}

func codexPlusClientUsagePolicyForPlan(cfg *CodexPlusConfig, plan *CodexPlusPlan) *CodexPlusUsageRule {
	if cfg == nil || plan == nil {
		return nil
	}
	policyID := strings.TrimSpace(plan.UsagePolicyID)
	if policyID == "" {
		return nil
	}
	for i := range cfg.UsagePolicy.Policies {
		if strings.TrimSpace(cfg.UsagePolicy.Policies[i].PolicyID) == policyID {
			return &cfg.UsagePolicy.Policies[i]
		}
	}
	return nil
}

func codexPlusClientDefaultModelForPlan(cfg *CodexPlusConfig, plan *CodexPlusPlan) *CodexPlusModel {
	if cfg == nil || plan == nil {
		return nil
	}
	for i := range cfg.ModelCatalog.Models {
		model := &cfg.ModelCatalog.Models[i]
		if model.IsDefault && model.IsEnabled && !model.IsHidden && codexPlusClientModelAllowedByPlan(plan, *model) {
			return model
		}
	}
	for i := range cfg.ModelCatalog.Models {
		model := &cfg.ModelCatalog.Models[i]
		if model.IsEnabled && !model.IsHidden && codexPlusClientModelAllowedByPlan(plan, *model) {
			return model
		}
	}
	return nil
}

func codexPlusClientModelAllowedByPlan(plan *CodexPlusPlan, model CodexPlusModel) bool {
	if plan == nil {
		return true
	}
	modelGroup := strings.TrimSpace(model.ModelGroup)
	if modelGroup == "" {
		return false
	}
	for _, allowed := range plan.ModelGroups {
		if strings.EqualFold(strings.TrimSpace(allowed), modelGroup) {
			return true
		}
	}
	return false
}

func defaultModelID(model *CodexPlusModel) string {
	if model == nil {
		return ""
	}
	return model.ModelID
}

func canReturnProviderKey(status string) bool {
	return status == ClientServiceStatusAvailable ||
		status == ClientServiceStatusLowBalance ||
		status == ClientServiceStatusModelUnavailable
}

func featureFlagsForStatus(cfg *CodexPlusConfig, status string) CodexPlusClientFeatureFlags {
	flags := CodexPlusClientFeatureFlags{}
	if cfg != nil {
		flags = CodexPlusClientFeatureFlags{
			AdvancedProviderConfig:  cfg.FeatureFlags.Flags.AdvancedProviderConfig,
			InstallAssistant:        cfg.FeatureFlags.Flags.InstallAssistant,
			NewUserTutorial:         cfg.FeatureFlags.Flags.NewUserTutorial,
			ModelSelector:           cfg.FeatureFlags.Flags.ModelSelector,
			DiagnosticExport:        cfg.FeatureFlags.Flags.DiagnosticExport,
			Announcements:           cfg.FeatureFlags.Flags.Announcements,
			ForceUpdatePrompt:       cfg.FeatureFlags.Flags.ForceUpdatePrompt,
			StrictDeviceEnforcement: cfg.FeatureFlags.Flags.StrictDeviceEnforcement,
		}
	}
	if status == ClientServiceStatusDeviceRevoked {
		flags.NewUserTutorial = false
		flags.ModelSelector = false
	}
	if status == ClientServiceStatusExpired || status == ClientServiceStatusNotPurchased || status == ClientServiceStatusGatewayUnhealthy {
		flags.ModelSelector = false
	}
	return flags
}

func announcementsForStatus(status, supportBaseURL string) []CodexPlusClientAnnouncement {
	if status != ClientServiceStatusGatewayUnhealthy {
		return []CodexPlusClientAnnouncement{}
	}
	statusURL := strings.TrimRight(supportBaseURL, "/") + "/status"
	return []CodexPlusClientAnnouncement{{
		ID:       "gateway-maintenance",
		Severity: "warning",
		Message:  "Gateway maintenance is in progress.",
		URL:      &statusURL,
	}}
}

func serviceStateForStatus(status, supportBaseURL string, ent codexPlusEntitlement) CodexPlusClientServiceState {
	billing := strings.TrimRight(supportBaseURL, "/") + "/billing"
	support := strings.TrimRight(supportBaseURL, "/") + "/support"
	statusURL := strings.TrimRight(supportBaseURL, "/") + "/status"
	switch status {
	case ClientServiceStatusAvailable:
		return CodexPlusClientServiceState{Status: status, Message: "Ready to use Codex++ Cloud.", MessageKey: "service.available", ActionHint: "none", Retryable: true}
	case ClientServiceStatusLowBalance:
		code := "CLIENT_ENTITLEMENT_LOW_BALANCE"
		return CodexPlusClientServiceState{Status: status, Message: "Your remaining Codex++ balance is low.", MessageKey: messageKeyForStatus(status, ent), ActionHint: "recharge", Retryable: true, SupportURL: &billing, ErrorCode: &code}
	case ClientServiceStatusExpired:
		code := "CLIENT_ENTITLEMENT_EXPIRED"
		return CodexPlusClientServiceState{Status: status, Message: "Your Codex++ plan has expired.", MessageKey: messageKeyForStatus(status, ent), ActionHint: "renew", SupportURL: &billing, ErrorCode: &code}
	case ClientServiceStatusNotPurchased:
		code := "CLIENT_ENTITLEMENT_NOT_PURCHASED"
		return CodexPlusClientServiceState{Status: status, Message: "Purchase or redeem a Codex++ plan to continue.", MessageKey: messageKeyForStatus(status, ent), ActionHint: "purchase", SupportURL: &billing, ErrorCode: &code}
	case ClientServiceStatusDeviceRevoked:
		code := "CLIENT_DEVICE_REVOKED"
		return CodexPlusClientServiceState{Status: status, Message: "This device cannot use Codex++ Cloud.", MessageKey: messageKeyForStatus(status, ent), ActionHint: "contact_support", SupportURL: &support, ErrorCode: &code}
	case ClientServiceStatusModelUnavailable:
		code := "GATEWAY_POLICY_MODEL_NOT_ALLOWED"
		return CodexPlusClientServiceState{Status: status, Message: "The selected model is currently unavailable.", MessageKey: messageKeyForStatus(status, ent), ActionHint: "retry", Retryable: true, ErrorCode: &code}
	case ClientServiceStatusGatewayUnhealthy:
		code := "GATEWAY_POLICY_CONFIG_UNAVAILABLE"
		return CodexPlusClientServiceState{Status: status, Message: "Codex++ Cloud is temporarily unavailable. Please retry later.", MessageKey: messageKeyForStatus(status, ent), ActionHint: "retry", Retryable: true, SupportURL: &statusURL, ErrorCode: &code}
	default:
		return CodexPlusClientServiceState{Status: status, Message: "Codex++ Cloud is temporarily unavailable. Please retry later.", MessageKey: messageKeyForStatus(status, ent), ActionHint: "retry", Retryable: true}
	}
}

func refreshTTLForStatus(status string) int {
	switch status {
	case ClientServiceStatusGatewayUnhealthy:
		return 60
	case ClientServiceStatusModelUnavailable:
		return 120
	case ClientServiceStatusLowBalance:
		return 180
	default:
		return 300
	}
}

func snapshotVersion(status string, now time.Time) string {
	if status == "" {
		status = "unknown"
	}
	return "snap_" + now.UTC().Format("20060102_150405") + "_" + strings.ReplaceAll(status, "-", "_")
}

func formatCredits(value float64) string {
	if math.Abs(value-math.Round(value)) < 0.000001 {
		return strconv.FormatInt(int64(math.Round(value)), 10)
	}
	return strconv.FormatFloat(value, 'f', 2, 64)
}

func periodUsageDisplay(sub *UserSubscription) string {
	if sub == nil {
		return "0 credits used today"
	}
	return formatCredits(sub.DailyUsageUSD) + " credits used today"
}

func periodUsageSummary(sub *UserSubscription, now time.Time) CodexPlusClientPeriodUsageSummary {
	now = now.UTC()
	resetAt := nextUTCDateBoundary(now)
	out := CodexPlusClientPeriodUsageSummary{
		PeriodID:     now.Format("2006-01-02"),
		UsageDisplay: periodUsageDisplay(sub),
		ResetAt:      &resetAt,
	}
	if sub == nil {
		return out
	}
	if sub.DailyWindowStart != nil {
		out.PeriodID = sub.DailyWindowStart.UTC().Format("2006-01-02")
		if reset := sub.DailyResetTime(); reset != nil {
			resetUTC := reset.UTC()
			out.ResetAt = &resetUTC
		}
	}
	return out
}

func nextUTCDateBoundary(now time.Time) time.Time {
	now = now.UTC()
	return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
}

func clientAction(actionType, messageKey, actionCopyKey, label string, url *string) CodexPlusClientAction {
	return CodexPlusClientAction{
		ActionType:    codexPlusFallbackString(actionType, "none"),
		MessageKey:    codexPlusClientStringPtrIfNotBlank(messageKey),
		ActionCopyKey: codexPlusClientStringPtrIfNotBlank(actionCopyKey),
		Label:         codexPlusFallbackString(label, "Continue"),
		URL:           url,
	}
}

func messageKeyForStatus(status string, ent codexPlusEntitlement) string {
	switch status {
	case ClientServiceStatusAvailable:
		return "service.available"
	case ClientServiceStatusLowBalance:
		if ent.policy != nil && strings.TrimSpace(ent.policy.CopyKeys.LowBalanceMessage) != "" {
			return ent.policy.CopyKeys.LowBalanceMessage
		}
		if ent.plan != nil && strings.TrimSpace(ent.plan.CopyKeys.LowBalanceMessage) != "" {
			return ent.plan.CopyKeys.LowBalanceMessage
		}
		return "usage.low_balance"
	case ClientServiceStatusExpired:
		if ent.policy != nil && strings.TrimSpace(ent.policy.CopyKeys.ExpiredMessage) != "" {
			return ent.policy.CopyKeys.ExpiredMessage
		}
		if ent.plan != nil && strings.TrimSpace(ent.plan.CopyKeys.ExpiredMessage) != "" {
			return ent.plan.CopyKeys.ExpiredMessage
		}
		return "usage.expired"
	case ClientServiceStatusNotPurchased:
		if ent.plan != nil && strings.TrimSpace(ent.plan.CopyKeys.NotPurchasedMessage) != "" {
			return ent.plan.CopyKeys.NotPurchasedMessage
		}
		return "billing.message.not_purchased"
	case ClientServiceStatusDeviceRevoked:
		if ent.policy != nil && strings.TrimSpace(ent.policy.CopyKeys.DeviceRevokedMessage) != "" {
			return ent.policy.CopyKeys.DeviceRevokedMessage
		}
		if ent.policy != nil && strings.TrimSpace(ent.policy.DevicePolicy.MessageKeys.Revoked) != "" {
			return ent.policy.DevicePolicy.MessageKeys.Revoked
		}
		return "device.revoked"
	case ClientServiceStatusModelUnavailable:
		return "model.unavailable"
	case ClientServiceStatusGatewayUnhealthy:
		return "gateway.unhealthy"
	default:
		return "service.unavailable"
	}
}

func planMessageKey(plan *CodexPlusPlan, status string) string {
	if plan == nil || strings.TrimSpace(plan.PlanID) == "" {
		return "plan.none." + codexPlusFallbackString(status, "unknown")
	}
	return "plan." + strings.TrimSpace(plan.PlanID) + "." + codexPlusFallbackString(status, "unknown")
}

func planActionCopyKey(plan *CodexPlusPlan, action string) string {
	if plan == nil {
		switch action {
		case "renew":
			return "billing.action.renew"
		case "upgrade":
			return "billing.action.upgrade"
		default:
			return "billing.action.purchase"
		}
	}
	switch action {
	case "renew":
		return codexPlusFallbackString(plan.CopyKeys.RenewAction, "billing.action.renew")
	case "upgrade":
		return codexPlusFallbackString(plan.CopyKeys.UpgradeAction, "billing.action.upgrade")
	default:
		return codexPlusFallbackString(plan.CopyKeys.PurchaseAction, "billing.action.purchase")
	}
}

func usageActionCopyKey(policy *CodexPlusUsageRule, action string) string {
	if policy == nil {
		switch action {
		case "renew":
			return "usage.renew_action"
		default:
			return "usage.purchase_action"
		}
	}
	switch action {
	case "renew":
		return codexPlusFallbackString(policy.CopyKeys.RenewAction, "usage.renew_action")
	default:
		return codexPlusFallbackString(policy.CopyKeys.PurchaseAction, "usage.purchase_action")
	}
}

func redeemDeltaSummary(code *RedeemCode) string {
	if code == nil {
		return "Redeem applied."
	}
	switch code.Type {
	case RedeemTypeBalance:
		return formatCredits(code.Value) + " credits added."
	case RedeemTypeConcurrency:
		return formatCredits(code.Value) + " concurrency added."
	case RedeemTypeSubscription:
		return "Subscription updated."
	default:
		return "Redeem applied."
	}
}

func deviceSnapshotFromRecord(record *CodexPlusDevice, version string) CodexPlusDeviceSnapshot {
	if record == nil {
		return CodexPlusDeviceSnapshot{Status: ClientDeviceStatusUnknown, Message: "Device not registered.", SnapshotVersion: version}
	}
	status := strings.TrimSpace(record.Status)
	if status == "" {
		status = ClientDeviceStatusActive
	}
	message := "Device active."
	switch status {
	case ClientDeviceStatusRevoked:
		message = "Device revoked by administrator."
	case ClientDeviceStatusBlocked:
		message = "Device blocked."
	}
	return CodexPlusDeviceSnapshot{
		DeviceID:        record.DeviceID,
		Status:          status,
		Message:         message,
		SnapshotVersion: version,
	}
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func codexPlusClientStringPtrIfNotBlank(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func codexPlusClientNullableStringPtr(value *string) any {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func codexPlusClientAPIKeyPrefixPtr(key string) *string {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	prefixLen := 8
	if len(key) < prefixLen {
		prefixLen = len(key)
	}
	prefix := key[:prefixLen]
	return &prefix
}

func MaskAPIKeyForCodexPlusClient(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "..." + key
	}
	prefixLen := 7
	if len(key) < prefixLen {
		prefixLen = len(key)
	}
	return key[:prefixLen] + "..." + key[len(key)-4:]
}
