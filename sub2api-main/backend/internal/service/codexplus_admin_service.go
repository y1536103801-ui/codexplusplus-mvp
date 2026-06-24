package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const codexPlusConfigVersionsSettingKey = "codexplus_config_v1_versions"

type CodexPlusAdminService struct {
	configService       *CodexPlusConfigService
	settingRepo         SettingRepository
	adminService        AdminService
	subscriptionService *SubscriptionService
	paymentConfig       *PaymentConfigService
	deviceRepo          CodexPlusDeviceRepository
	eventRepo           CodexPlusEventRepository
	now                 func() time.Time
}

type CodexPlusConfigVersionEntry struct {
	ConfigVersion string           `json:"config_version"`
	PublishScope  string           `json:"publish_scope"`
	UpdatedBy     string           `json:"updated_by"`
	UpdatedAt     string           `json:"updated_at"`
	ChangeReason  string           `json:"change_reason"`
	RollbackFrom  *string          `json:"rollback_from,omitempty"`
	Config        *CodexPlusConfig `json:"config,omitempty"`
}

type CodexPlusOptions struct {
	Groups        []CodexPlusGroupOption       `json:"groups"`
	PaymentPlans  []CodexPlusPaymentPlanOption `json:"payment_plans"`
	Models        []CodexPlusModelOption       `json:"models"`
	FeatureFlags  []string                     `json:"feature_flags"`
	PolicyPresets []CodexPlusPolicyPreset      `json:"policy_presets"`
}

type CodexPlusGroupOption struct {
	ID               int64    `json:"id"`
	Name             string   `json:"name"`
	Platform         string   `json:"platform"`
	Status           string   `json:"status"`
	SubscriptionType string   `json:"subscription_type"`
	SupportedScopes  []string `json:"supported_model_scopes,omitempty"`
}

type CodexPlusPaymentPlanOption struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	GroupID      int64   `json:"group_id"`
	Price        float64 `json:"price"`
	ValidityDays int     `json:"validity_days"`
	ValidityUnit string  `json:"validity_unit"`
	ForSale      bool    `json:"for_sale"`
	ProductName  string  `json:"product_name"`
}

type CodexPlusModelOption struct {
	ModelID   string `json:"model_id"`
	GroupID   int64  `json:"group_id,omitempty"`
	GroupName string `json:"group_name,omitempty"`
	Platform  string `json:"platform,omitempty"`
}

type CodexPlusPolicyPreset struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type CodexPlusUserEntitlement struct {
	User                *CodexPlusUserSummary           `json:"user"`
	AllowedGroups       []CodexPlusGroupOption          `json:"allowed_groups"`
	ActiveSubscriptions []CodexPlusSubscriptionSummary  `json:"active_subscriptions"`
	Subscriptions       []CodexPlusSubscriptionSummary  `json:"subscriptions"`
	APIKeys             []CodexPlusAPIKeySummary        `json:"api_keys"`
	ManagedProviderKey  CodexPlusManagedProviderSummary `json:"managed_provider_key"`
	UsageSummary        any                             `json:"usage_summary,omitempty"`
	Devices             []CodexPlusDeviceSummary        `json:"devices"`
	RecentEvents        []CodexPlusEventSummary         `json:"recent_events"`
	AuditRiskSummary    *CodexPlusAuditRiskUserSummary  `json:"audit_risk_summary,omitempty"`
	IntegrationStatus   map[string]string               `json:"integration_status"`
}

type CodexPlusUserSummary struct {
	ID            int64   `json:"id"`
	Email         string  `json:"email"`
	Username      string  `json:"username"`
	Status        string  `json:"status"`
	Role          string  `json:"role"`
	Balance       float64 `json:"balance"`
	Concurrency   int     `json:"concurrency"`
	RPMLimit      int     `json:"rpm_limit"`
	AllowedGroups []int64 `json:"allowed_group_ids"`
}

type CodexPlusSubscriptionSummary struct {
	ID              int64   `json:"id"`
	UserID          int64   `json:"user_id"`
	GroupID         int64   `json:"group_id"`
	GroupName       string  `json:"group_name,omitempty"`
	GroupPlatform   string  `json:"group_platform,omitempty"`
	Status          string  `json:"status"`
	StartsAt        string  `json:"starts_at"`
	ExpiresAt       string  `json:"expires_at"`
	DailyUsageUSD   float64 `json:"daily_usage_usd"`
	WeeklyUsageUSD  float64 `json:"weekly_usage_usd"`
	MonthlyUsageUSD float64 `json:"monthly_usage_usd"`
	Notes           string  `json:"notes,omitempty"`
}

type CodexPlusAPIKeySummary struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	GroupID    *int64  `json:"group_id,omitempty"`
	Status     string  `json:"status"`
	MaskedKey  string  `json:"masked_key"`
	LastUsedAt *string `json:"last_used_at,omitempty"`
}

type CodexPlusManagedProviderSummary struct {
	Exists    bool   `json:"exists"`
	MaskedKey string `json:"masked_key,omitempty"`
	KeyID     int64  `json:"key_id,omitempty"`
}

type CodexPlusDeviceSummary struct {
	DeviceID string `json:"device_id"`
	Status   string `json:"status"`
	LastSeen string `json:"last_seen,omitempty"`
}

type CodexPlusEventSummary struct {
	EventType string `json:"event_type"`
	CreatedAt string `json:"created_at"`
	Summary   string `json:"summary"`
}

func NewCodexPlusAdminService(
	configService *CodexPlusConfigService,
	settingRepo SettingRepository,
	adminService AdminService,
	subscriptionService *SubscriptionService,
	paymentConfig *PaymentConfigService,
	deviceRepo CodexPlusDeviceRepository,
	eventRepo CodexPlusEventRepository,
) *CodexPlusAdminService {
	return &CodexPlusAdminService{
		configService:       configService,
		settingRepo:         settingRepo,
		adminService:        adminService,
		subscriptionService: subscriptionService,
		paymentConfig:       paymentConfig,
		deviceRepo:          deviceRepo,
		eventRepo:           eventRepo,
		now:                 time.Now,
	}
}

func (s *CodexPlusAdminService) GetCurrentConfig(ctx context.Context) (*CodexPlusConfig, error) {
	if s == nil || s.configService == nil {
		return nil, fmt.Errorf("codexplus admin service is not initialized")
	}
	return s.configService.Get(ctx)
}

func (s *CodexPlusAdminService) ValidateConfig(_ context.Context, draft *CodexPlusConfig) error {
	return ValidateCodexPlusConfig(draft)
}

func (s *CodexPlusAdminService) PublishConfig(ctx context.Context, draft CodexPlusConfig, actor, reason string) (*CodexPlusConfig, error) {
	if s == nil || s.configService == nil {
		return nil, fmt.Errorf("codexplus admin service is not initialized")
	}
	now := s.now()
	version := nextCodexPlusAdminConfigVersion(now)
	setCodexPlusConfigVersion(&draft, version)
	draft.RollbackFrom = nil
	published, err := s.configService.Publish(ctx, draft, actor, reason)
	if err != nil {
		return nil, err
	}
	if err := s.appendVersion(ctx, published); err != nil {
		return nil, err
	}
	return published, nil
}

func (s *CodexPlusAdminService) ListConfigVersions(ctx context.Context) ([]CodexPlusConfigVersionEntry, error) {
	entries, err := s.loadVersions(ctx)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		cfg, cfgErr := s.GetCurrentConfig(ctx)
		if cfgErr != nil {
			return nil, cfgErr
		}
		return []CodexPlusConfigVersionEntry{versionEntryFromConfig(cfg)}, nil
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].UpdatedAt > entries[j].UpdatedAt
	})
	return entries, nil
}

func (s *CodexPlusAdminService) RollbackConfig(ctx context.Context, version, actor, reason string) (*CodexPlusConfig, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return nil, infraerrors.BadRequest("CODEXPLUS_VERSION_REQUIRED", "config version is required")
	}
	entries, err := s.loadVersions(ctx)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.ConfigVersion == version && entry.Config != nil {
			draft := *entry.Config
			rollbackFrom := version
			draft.RollbackFrom = &rollbackFrom
			newVersion := nextCodexPlusAdminConfigVersion(s.now())
			setCodexPlusConfigVersion(&draft, newVersion)
			if strings.TrimSpace(reason) == "" {
				reason = "rollback to " + version
			}
			published, publishErr := s.configService.Publish(ctx, draft, actor, reason)
			if publishErr != nil {
				return nil, publishErr
			}
			if appendErr := s.appendVersion(ctx, published); appendErr != nil {
				return nil, appendErr
			}
			return published, nil
		}
	}
	return nil, infraerrors.NotFound("CODEXPLUS_VERSION_NOT_FOUND", "config version not found")
}

func (s *CodexPlusAdminService) GetOptions(ctx context.Context) (*CodexPlusOptions, error) {
	opts := &CodexPlusOptions{
		FeatureFlags: []string{
			"advanced_provider_config",
			"install_assistant",
			"new_user_tutorial",
			"model_selector",
			"diagnostic_export",
			"announcements",
			"force_update_prompt",
			"strict_device_enforcement",
		},
		PolicyPresets: []CodexPlusPolicyPreset{
			{ID: "default", Name: "Default", Description: "Balanced production-safe limits"},
			{ID: "strict", Name: "Strict", Description: "Lower RPM and no grace period"},
			{ID: "trial", Name: "Trial", Description: "Short-lived plan bootstrap policy"},
		},
	}
	if s == nil {
		return opts, nil
	}
	if s.adminService != nil {
		groups, err := s.adminService.GetAllGroupsIncludingInactive(ctx)
		if err != nil {
			return nil, err
		}
		opts.Groups = make([]CodexPlusGroupOption, 0, len(groups))
		modelsByID := map[string]CodexPlusModelOption{}
		for _, group := range groups {
			opts.Groups = append(opts.Groups, groupOptionFromService(group))
			candidates, err := s.adminService.GetGroupModelsListCandidates(ctx, group.ID, group.Platform)
			if err != nil {
				continue
			}
			for _, id := range candidates {
				id = strings.TrimSpace(id)
				if id == "" {
					continue
				}
				if _, exists := modelsByID[id]; !exists {
					modelsByID[id] = CodexPlusModelOption{
						ModelID:   id,
						GroupID:   group.ID,
						GroupName: group.Name,
						Platform:  group.Platform,
					}
				}
			}
		}
		for _, model := range modelsByID {
			opts.Models = append(opts.Models, model)
		}
		sort.Slice(opts.Models, func(i, j int) bool { return opts.Models[i].ModelID < opts.Models[j].ModelID })
	}
	if s.paymentConfig != nil {
		plans, err := s.paymentConfig.ListPlans(ctx)
		if err != nil {
			return nil, err
		}
		opts.PaymentPlans = make([]CodexPlusPaymentPlanOption, 0, len(plans))
		for _, plan := range plans {
			opts.PaymentPlans = append(opts.PaymentPlans, paymentPlanOptionFromEnt(plan))
		}
	}
	return opts, nil
}

func (s *CodexPlusAdminService) GetUserEntitlement(ctx context.Context, userID int64) (*CodexPlusUserEntitlement, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("CODEXPLUS_USER_ID_INVALID", "invalid user id")
	}
	if s == nil || s.adminService == nil {
		return nil, fmt.Errorf("codexplus admin service is not initialized")
	}
	user, err := s.adminService.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := &CodexPlusUserEntitlement{
		User:              userSummaryFromService(user),
		IntegrationStatus: map[string]string{},
		Devices:           []CodexPlusDeviceSummary{},
		RecentEvents:      []CodexPlusEventSummary{},
	}
	if len(user.AllowedGroups) > 0 {
		allGroups, err := s.adminService.GetAllGroupsIncludingInactive(ctx)
		if err == nil {
			byID := make(map[int64]Group, len(allGroups))
			for _, group := range allGroups {
				byID[group.ID] = group
			}
			for _, id := range user.AllowedGroups {
				if group, ok := byID[id]; ok {
					out.AllowedGroups = append(out.AllowedGroups, groupOptionFromService(group))
				}
			}
		}
	}
	if s.subscriptionService != nil {
		subs, err := s.subscriptionService.ListUserSubscriptions(ctx, userID)
		if err != nil {
			return nil, err
		}
		out.Subscriptions = make([]CodexPlusSubscriptionSummary, 0, len(subs))
		for _, sub := range subs {
			summary := subscriptionSummaryFromService(sub)
			out.Subscriptions = append(out.Subscriptions, summary)
			if sub.IsActive() {
				out.ActiveSubscriptions = append(out.ActiveSubscriptions, summary)
			}
		}
	}
	keys, _, err := s.adminService.GetUserAPIKeys(ctx, userID, 1, 100, "created_at", "desc")
	if err == nil {
		out.APIKeys = make([]CodexPlusAPIKeySummary, 0, len(keys))
		for _, key := range keys {
			summary := apiKeySummaryFromService(key)
			out.APIKeys = append(out.APIKeys, summary)
			if isCodexPlusManagedKey(key) && !out.ManagedProviderKey.Exists {
				out.ManagedProviderKey = CodexPlusManagedProviderSummary{
					Exists:    true,
					MaskedKey: summary.MaskedKey,
					KeyID:     key.ID,
				}
			}
		}
	}
	usage, err := s.adminService.GetUserUsageStats(ctx, userID, "30d")
	if err == nil {
		out.UsageSummary = usage
	}
	if devices, err := s.GetUserDevices(ctx, userID); err == nil {
		out.Devices = devices
		out.IntegrationStatus["devices"] = "loaded"
	} else {
		out.IntegrationStatus["devices"] = "unavailable"
	}
	if events, err := s.GetUserEvents(ctx, userID); err == nil {
		out.RecentEvents = events
		out.IntegrationStatus["events"] = "loaded"
	} else {
		out.IntegrationStatus["events"] = "unavailable"
	}
	if auditRisk, err := s.GetAuditRiskSummary(ctx, userID); err == nil {
		out.AuditRiskSummary = auditRisk
		out.IntegrationStatus["audit_risk"] = "loaded"
	} else {
		out.IntegrationStatus["audit_risk"] = "unavailable"
	}
	return out, nil
}

func (s *CodexPlusAdminService) GetUserDevices(ctx context.Context, userID int64) ([]CodexPlusDeviceSummary, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("CODEXPLUS_USER_ID_INVALID", "invalid user id")
	}
	if s == nil || s.deviceRepo == nil {
		return []CodexPlusDeviceSummary{}, nil
	}
	devices, err := s.deviceRepo.ListByUser(ctx, userID, 100)
	if err != nil {
		return nil, err
	}
	out := make([]CodexPlusDeviceSummary, 0, len(devices))
	for _, device := range devices {
		out = append(out, deviceSummaryFromService(device))
	}
	return out, nil
}

func (s *CodexPlusAdminService) GetUserEvents(ctx context.Context, userID int64) ([]CodexPlusEventSummary, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("CODEXPLUS_USER_ID_INVALID", "invalid user id")
	}
	if s == nil || s.eventRepo == nil {
		return []CodexPlusEventSummary{}, nil
	}
	events, err := s.eventRepo.ListByUser(ctx, userID, 100)
	if err != nil {
		return nil, err
	}
	out := make([]CodexPlusEventSummary, 0, len(events))
	for _, event := range events {
		out = append(out, eventSummaryFromService(event))
	}
	return out, nil
}

func (s *CodexPlusAdminService) GetAuditRiskSummary(ctx context.Context, userID int64) (*CodexPlusAuditRiskUserSummary, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("CODEXPLUS_USER_ID_INVALID", "invalid user id")
	}
	if s == nil || s.eventRepo == nil {
		return BuildCodexPlusAuditRiskUserSummary(nil), nil
	}
	return CodexPlusAuditRiskSummaryForUser(ctx, s.eventRepo, userID, 100)
}

func (s *CodexPlusAdminService) loadVersions(ctx context.Context) ([]CodexPlusConfigVersionEntry, error) {
	if s == nil || s.settingRepo == nil {
		return nil, nil
	}
	raw, err := s.settingRepo.GetValue(ctx, codexPlusConfigVersionsSettingKey)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return []CodexPlusConfigVersionEntry{}, nil
		}
		return nil, err
	}
	if strings.TrimSpace(raw) == "" {
		return []CodexPlusConfigVersionEntry{}, nil
	}
	var entries []CodexPlusConfigVersionEntry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return nil, fmt.Errorf("decode codexplus config versions: %w", err)
	}
	return entries, nil
}

func (s *CodexPlusAdminService) appendVersion(ctx context.Context, cfg *CodexPlusConfig) error {
	if s == nil || s.settingRepo == nil || cfg == nil {
		return nil
	}
	entries, err := s.loadVersions(ctx)
	if err != nil {
		return err
	}
	entry := versionEntryFromConfig(cfg)
	next := make([]CodexPlusConfigVersionEntry, 0, len(entries)+1)
	next = append(next, entry)
	for _, existing := range entries {
		if existing.ConfigVersion == entry.ConfigVersion {
			continue
		}
		next = append(next, existing)
		if len(next) >= 50 {
			break
		}
	}
	payload, err := json.Marshal(next)
	if err != nil {
		return err
	}
	return s.settingRepo.Set(ctx, codexPlusConfigVersionsSettingKey, string(payload))
}

func versionEntryFromConfig(cfg *CodexPlusConfig) CodexPlusConfigVersionEntry {
	if cfg == nil {
		return CodexPlusConfigVersionEntry{}
	}
	copyCfg := *cfg
	return CodexPlusConfigVersionEntry{
		ConfigVersion: cfg.ConfigVersion,
		PublishScope:  cfg.PublishScope,
		UpdatedBy:     cfg.UpdatedBy,
		UpdatedAt:     cfg.UpdatedAt,
		ChangeReason:  cfg.ChangeReason,
		RollbackFrom:  cfg.RollbackFrom,
		Config:        &copyCfg,
	}
}

func nextCodexPlusAdminConfigVersion(now time.Time) string {
	return "codexplus-mvp-" + strconv.FormatInt(now.UTC().UnixNano(), 10)
}

func setCodexPlusConfigVersion(cfg *CodexPlusConfig, version string) {
	if cfg == nil {
		return
	}
	cfg.ConfigVersion = version
	cfg.PlanCatalog.ConfigVersion = version
	cfg.ModelCatalog.ConfigVersion = version
	cfg.UsagePolicy.ConfigVersion = version
	cfg.FeatureFlags.ConfigVersion = version
}

func groupOptionFromService(group Group) CodexPlusGroupOption {
	return CodexPlusGroupOption{
		ID:               group.ID,
		Name:             group.Name,
		Platform:         group.Platform,
		Status:           group.Status,
		SubscriptionType: group.SubscriptionType,
		SupportedScopes:  group.SupportedModelScopes,
	}
}

func paymentPlanOptionFromEnt(plan *dbent.SubscriptionPlan) CodexPlusPaymentPlanOption {
	if plan == nil {
		return CodexPlusPaymentPlanOption{}
	}
	return CodexPlusPaymentPlanOption{
		ID:           plan.ID,
		Name:         plan.Name,
		GroupID:      plan.GroupID,
		Price:        plan.Price,
		ValidityDays: plan.ValidityDays,
		ValidityUnit: plan.ValidityUnit,
		ForSale:      plan.ForSale,
		ProductName:  plan.ProductName,
	}
}

func userSummaryFromService(user *User) *CodexPlusUserSummary {
	if user == nil {
		return nil
	}
	return &CodexPlusUserSummary{
		ID:            user.ID,
		Email:         user.Email,
		Username:      user.Username,
		Status:        user.Status,
		Role:          user.Role,
		Balance:       user.Balance,
		Concurrency:   user.Concurrency,
		RPMLimit:      user.RPMLimit,
		AllowedGroups: append([]int64(nil), user.AllowedGroups...),
	}
}

func subscriptionSummaryFromService(sub UserSubscription) CodexPlusSubscriptionSummary {
	groupName := ""
	groupPlatform := ""
	if sub.Group != nil {
		groupName = sub.Group.Name
		groupPlatform = sub.Group.Platform
	}
	return CodexPlusSubscriptionSummary{
		ID:              sub.ID,
		UserID:          sub.UserID,
		GroupID:         sub.GroupID,
		GroupName:       groupName,
		GroupPlatform:   groupPlatform,
		Status:          sub.Status,
		StartsAt:        sub.StartsAt.UTC().Format(time.RFC3339),
		ExpiresAt:       sub.ExpiresAt.UTC().Format(time.RFC3339),
		DailyUsageUSD:   sub.DailyUsageUSD,
		WeeklyUsageUSD:  sub.WeeklyUsageUSD,
		MonthlyUsageUSD: sub.MonthlyUsageUSD,
		Notes:           sub.Notes,
	}
}

func apiKeySummaryFromService(key APIKey) CodexPlusAPIKeySummary {
	var lastUsed *string
	if key.LastUsedAt != nil {
		formatted := key.LastUsedAt.UTC().Format(time.RFC3339)
		lastUsed = &formatted
	}
	return CodexPlusAPIKeySummary{
		ID:         key.ID,
		Name:       key.Name,
		GroupID:    key.GroupID,
		Status:     key.Status,
		MaskedKey:  maskCodexPlusKey(key.Key),
		LastUsedAt: lastUsed,
	}
}

func isCodexPlusManagedKey(key APIKey) bool {
	name := strings.ToLower(strings.TrimSpace(key.Name))
	return strings.Contains(name, "codex++") ||
		strings.Contains(name, "codex-plus") ||
		strings.Contains(name, CodexPlusManagedProviderID)
}

func maskCodexPlusKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

func deviceSummaryFromService(device CodexPlusDevice) CodexPlusDeviceSummary {
	lastSeen := ""
	if device.LastSeenAt != nil {
		lastSeen = device.LastSeenAt.UTC().Format(time.RFC3339)
	}
	return CodexPlusDeviceSummary{
		DeviceID: device.DeviceID,
		Status:   device.Status,
		LastSeen: lastSeen,
	}
}

func eventSummaryFromService(event CodexPlusEvent) CodexPlusEventSummary {
	summary := codexPlusPayloadString(event.Payload, "summary")
	if summary == "" {
		if metadata, ok := event.Payload["metadata"].(map[string]any); ok {
			summary = codexPlusPayloadString(metadata, "reason")
			if summary == "" {
				summary = codexPlusPayloadString(metadata, "service_status")
			}
		}
	}
	if summary == "" {
		summary = codexPlusPayloadString(event.Payload, "error_code")
	}
	if summary == "" {
		summary = event.EventType
	}
	return CodexPlusEventSummary{
		EventType: event.EventType,
		CreatedAt: event.CreatedAt.UTC().Format(time.RFC3339),
		Summary:   summary,
	}
}

func codexPlusPayloadString(payload map[string]any, key string) string {
	if len(payload) == 0 {
		return ""
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}
