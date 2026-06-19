package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	configregistry "github.com/Wei-Shaw/sub2api/internal/codexplus/configregistry"
)

const (
	CodexPlusConfigSettingKey  = "codexplus_config_v1"
	CodexPlusConfigVersionMVP  = "codexplus-mvp-1"
	CodexPlusManagedProviderID = "codex-plus-cloud"
)

var (
	ErrCodexPlusConfigInvalid = errors.New("codexplus config invalid")
)

type CodexPlusConfigService struct {
	settingRepo SettingRepository
	now         func() time.Time
}

type CodexPlusConfig struct {
	ConfigVersion string                   `json:"config_version"`
	DraftStatus   string                   `json:"draft_status,omitempty"`
	PublishScope  string                   `json:"publish_scope"`
	UpdatedBy     string                   `json:"updated_by"`
	UpdatedAt     string                   `json:"updated_at"`
	ChangeReason  string                   `json:"change_reason"`
	RollbackFrom  *string                  `json:"rollback_from"`
	PlanCatalog   CodexPlusPlanCatalog     `json:"plan_catalog"`
	ModelCatalog  CodexPlusModelCatalog    `json:"model_catalog"`
	UsagePolicy   CodexPlusUsagePolicy     `json:"usage_policy"`
	FeatureFlags  CodexPlusFeatureFlagsDoc `json:"feature_flags"`
}

type CodexPlusPlanCatalog struct {
	ConfigVersion string          `json:"config_version"`
	DraftStatus   string          `json:"draft_status,omitempty"`
	PublishScope  string          `json:"publish_scope,omitempty"`
	RollbackFrom  *string         `json:"rollback_from,omitempty"`
	UpdatedBy     string          `json:"updated_by,omitempty"`
	UpdatedAt     string          `json:"updated_at,omitempty"`
	ChangeReason  string          `json:"change_reason,omitempty"`
	Plans         []CodexPlusPlan `json:"plans"`
}

type CodexPlusPlan struct {
	PlanID              string                           `json:"plan_id"`
	Name                string                           `json:"name"`
	Description         string                           `json:"description,omitempty"`
	BillingPeriod       string                           `json:"billing_period"`
	Currency            string                           `json:"currency"`
	PriceAmountMinor    int                              `json:"price_amount_minor"`
	DisplayPrice        string                           `json:"display_price"`
	EntitlementGrant    CodexPlusEntitlementGrant        `json:"entitlement_grant"`
	EntitlementSources  CodexPlusEntitlementSources      `json:"entitlement_sources,omitempty"`
	ModelGroups         []string                         `json:"model_groups"`
	UsagePolicyID       string                           `json:"usage_policy_id,omitempty"`
	PurchaseURL         *string                          `json:"purchase_url,omitempty"`
	RenewURL            *string                          `json:"renew_url"`
	CopyKeys            CodexPlusPlanCopyKeys            `json:"copy_keys,omitempty"`
	IsListed            bool                             `json:"is_listed"`
	Status              string                           `json:"status"`
	SortOrder           int                              `json:"sort_order"`
	ExternalBillingRefs CodexPlusPlanExternalBillingRefs `json:"external_billing_refs,omitempty"`
}

type CodexPlusEntitlementSources struct {
	SubscriptionGroupIDs []int64  `json:"subscription_group_ids,omitempty"`
	APIKeyGroupIDs       []int64  `json:"api_key_group_ids,omitempty"`
	GroupNames           []string `json:"group_names,omitempty"`
}

type CodexPlusEntitlementGrant struct {
	BalanceCredit int  `json:"balance_credit"`
	DurationDays  int  `json:"duration_days"`
	DailyQuota    *int `json:"daily_quota"`
	PeriodQuota   *int `json:"period_quota,omitempty"`
}

type CodexPlusPlanCopyKeys struct {
	PurchaseAction      string `json:"purchase_action,omitempty"`
	RenewAction         string `json:"renew_action,omitempty"`
	UpgradeAction       string `json:"upgrade_action,omitempty"`
	NotPurchasedMessage string `json:"not_purchased_message,omitempty"`
	ExpiredMessage      string `json:"expired_message,omitempty"`
	LowBalanceMessage   string `json:"low_balance_message,omitempty"`
}

type CodexPlusPlanExternalBillingRefs struct {
	ProductID *string `json:"product_id,omitempty"`
	SKUID     *string `json:"sku_id,omitempty"`
}

type CodexPlusModelCatalog struct {
	ConfigVersion string           `json:"config_version"`
	DraftStatus   string           `json:"draft_status,omitempty"`
	PublishScope  string           `json:"publish_scope,omitempty"`
	RollbackFrom  *string          `json:"rollback_from,omitempty"`
	UpdatedBy     string           `json:"updated_by,omitempty"`
	UpdatedAt     string           `json:"updated_at,omitempty"`
	ChangeReason  string           `json:"change_reason,omitempty"`
	Models        []CodexPlusModel `json:"models"`
}

type CodexPlusModel struct {
	ModelID                    string   `json:"model_id"`
	DisplayName                string   `json:"display_name"`
	RouteModel                 string   `json:"route_model"`
	ModelGroup                 string   `json:"model_group"`
	ContextWindow              int      `json:"context_window"`
	BillingMultiplier          float64  `json:"billing_multiplier"`
	IsDefault                  bool     `json:"is_default"`
	IsEnabled                  bool     `json:"is_enabled"`
	IsHidden                   bool     `json:"is_hidden"`
	DisabledReason             *string  `json:"disabled_reason"`
	RolloutChannel             string   `json:"rollout_channel,omitempty"`
	QualityTier                string   `json:"quality_tier,omitempty"`
	FallbackModelID            *string  `json:"fallback_model_id,omitempty"`
	DeprecationAt              *string  `json:"deprecation_at,omitempty"`
	DisabledReplacementModelID *string  `json:"disabled_replacement_model_id,omitempty"`
	DisabledMessageKey         *string  `json:"disabled_message_key,omitempty"`
	SortOrder                  int      `json:"sort_order"`
	OperatorTags               []string `json:"operator_tags,omitempty"`
}

type CodexPlusUsagePolicy struct {
	ConfigVersion string               `json:"config_version"`
	DraftStatus   string               `json:"draft_status,omitempty"`
	PublishScope  string               `json:"publish_scope,omitempty"`
	RollbackFrom  *string              `json:"rollback_from,omitempty"`
	UpdatedBy     string               `json:"updated_by,omitempty"`
	UpdatedAt     string               `json:"updated_at,omitempty"`
	ChangeReason  string               `json:"change_reason,omitempty"`
	Policies      []CodexPlusUsageRule `json:"policies"`
}

type CodexPlusUsageRule struct {
	PolicyID                   string                           `json:"policy_id"`
	AppliesTo                  CodexPlusUsagePolicyAppliesTo    `json:"applies_to,omitempty"`
	LowBalanceThreshold        int                              `json:"low_balance_threshold"`
	DailyQuota                 int                              `json:"daily_quota"`
	MonthlyQuota               *int                             `json:"monthly_quota,omitempty"`
	ConcurrencyLimit           int                              `json:"concurrency_limit"`
	RPMLimit                   int                              `json:"rpm_limit"`
	TPMLimit                   int                              `json:"tpm_limit"`
	BurstLimit                 int                              `json:"burst_limit,omitempty"`
	RateLimitWindowSeconds     int                              `json:"rate_limit_window_seconds,omitempty"`
	ExpiredBehavior            string                           `json:"expired_behavior"`
	GracePeriodHours           int                              `json:"grace_period_hours"`
	OverageBehavior            string                           `json:"overage_behavior,omitempty"`
	CopyKeys                   CodexPlusUsagePolicyCopyKeys     `json:"copy_keys,omitempty"`
	DevicePolicy               CodexPlusUsagePolicyDevicePolicy `json:"device_policy,omitempty"`
	InsufficientBalanceMessage string                           `json:"insufficient_balance_message"`
	RateLimitedMessage         string                           `json:"rate_limited_message,omitempty"`
}

type CodexPlusUsagePolicyAppliesTo struct {
	PlanIDs      []string `json:"plan_ids,omitempty"`
	ModelGroups  []string `json:"model_groups,omitempty"`
	UserSegments []string `json:"user_segments,omitempty"`
}

type CodexPlusUsagePolicyCopyKeys struct {
	LowBalanceMessage          string `json:"low_balance_message,omitempty"`
	InsufficientBalanceMessage string `json:"insufficient_balance_message,omitempty"`
	RateLimitedMessage         string `json:"rate_limited_message,omitempty"`
	ExpiredMessage             string `json:"expired_message,omitempty"`
	RenewAction                string `json:"renew_action,omitempty"`
	PurchaseAction             string `json:"purchase_action,omitempty"`
	DeviceRevokedMessage       string `json:"device_revoked_message,omitempty"`
}

type CodexPlusUsagePolicyDevicePolicy struct {
	RegistrationRequired        bool                                  `json:"registration_required,omitempty"`
	MaxDevicesPerUser           int                                   `json:"max_devices_per_user,omitempty"`
	AllowSelfServiceReplacement bool                                  `json:"allow_self_service_replacement,omitempty"`
	ReplacementCooldownHours    int                                   `json:"replacement_cooldown_hours,omitempty"`
	StrictEnforcementDefault    bool                                  `json:"strict_enforcement_default,omitempty"`
	RevokeReasonTaxonomy        []string                              `json:"revoke_reason_taxonomy,omitempty"`
	SupportUnlockPolicy         string                                `json:"support_unlock_policy,omitempty"`
	RevokedBehavior             string                                `json:"revoked_behavior,omitempty"`
	MessageKeys                 CodexPlusUsagePolicyDeviceMessageKeys `json:"message_keys,omitempty"`
}

type CodexPlusUsagePolicyDeviceMessageKeys struct {
	LimitReached          string `json:"limit_reached,omitempty"`
	ReplacementCooldown   string `json:"replacement_cooldown,omitempty"`
	Revoked               string `json:"revoked,omitempty"`
	SupportUnlockRequired string `json:"support_unlock_required,omitempty"`
}

type CodexPlusFeatureFlagsDoc struct {
	ConfigVersion string                       `json:"config_version"`
	DraftStatus   string                       `json:"draft_status,omitempty"`
	PublishScope  string                       `json:"publish_scope,omitempty"`
	RollbackFrom  *string                      `json:"rollback_from,omitempty"`
	UpdatedBy     string                       `json:"updated_by,omitempty"`
	UpdatedAt     string                       `json:"updated_at,omitempty"`
	ChangeReason  string                       `json:"change_reason,omitempty"`
	Flags         CodexPlusFeatureFlags        `json:"flags"`
	Exposure      CodexPlusFeatureFlagExposure `json:"exposure,omitempty"`
	CopyKeys      CodexPlusFeatureFlagCopyKeys `json:"copy_keys,omitempty"`
}

type CodexPlusFeatureFlags struct {
	AdvancedProviderConfig  bool `json:"advanced_provider_config"`
	InstallAssistant        bool `json:"install_assistant"`
	NewUserTutorial         bool `json:"new_user_tutorial"`
	ModelSelector           bool `json:"model_selector"`
	DiagnosticExport        bool `json:"diagnostic_export"`
	Announcements           bool `json:"announcements"`
	ForceUpdatePrompt       bool `json:"force_update_prompt"`
	StrictDeviceEnforcement bool `json:"strict_device_enforcement"`
}

type CodexPlusFeatureFlagExposure struct {
	ClientVisible []configregistry.FeatureFlagName `json:"client_visible,omitempty"`
	ServerOnly    []configregistry.FeatureFlagName `json:"server_only,omitempty"`
}

type CodexPlusFeatureFlagCopyKeys struct {
	ForceUpdatePrompt     string `json:"force_update_prompt,omitempty"`
	InstallAssistantEntry string `json:"install_assistant_entry,omitempty"`
	NewUserTutorialEntry  string `json:"new_user_tutorial_entry,omitempty"`
	DiagnosticExportEntry string `json:"diagnostic_export_entry,omitempty"`
	AnnouncementEntry     string `json:"announcement_entry,omitempty"`
}

func NewCodexPlusConfigService(settingRepo SettingRepository) *CodexPlusConfigService {
	return &CodexPlusConfigService{
		settingRepo: settingRepo,
		now:         time.Now,
	}
}

func (s *CodexPlusConfigService) Get(ctx context.Context) (*CodexPlusConfig, error) {
	if s == nil || s.settingRepo == nil {
		return nil, fmt.Errorf("codexplus config service is not initialized")
	}
	raw, err := s.settingRepo.GetValue(ctx, CodexPlusConfigSettingKey)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			cfg := DefaultCodexPlusConfig(s.now())
			return &cfg, nil
		}
		return nil, fmt.Errorf("get codexplus config: %w", err)
	}
	cfg, err := DecodeCodexPlusConfig(raw)
	if err != nil {
		return nil, err
	}
	if err := ValidateCodexPlusConfig(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (s *CodexPlusConfigService) EnsureDefault(ctx context.Context) (*CodexPlusConfig, error) {
	if s == nil || s.settingRepo == nil {
		return nil, fmt.Errorf("codexplus config service is not initialized")
	}
	cfg, err := s.Get(ctx)
	if err != nil {
		return nil, err
	}
	raw, err := EncodeCodexPlusConfig(cfg)
	if err != nil {
		return nil, err
	}
	if err := s.settingRepo.Set(ctx, CodexPlusConfigSettingKey, raw); err != nil {
		return nil, fmt.Errorf("store default codexplus config: %w", err)
	}
	return cfg, nil
}

func (s *CodexPlusConfigService) Publish(ctx context.Context, draft CodexPlusConfig, actor, reason string) (*CodexPlusConfig, error) {
	if s == nil || s.settingRepo == nil {
		return nil, fmt.Errorf("codexplus config service is not initialized")
	}
	NormalizeCodexPlusConfig(&draft, s.now(), strings.TrimSpace(actor), strings.TrimSpace(reason))
	if err := ValidateCodexPlusConfig(&draft); err != nil {
		return nil, err
	}
	raw, err := EncodeCodexPlusConfig(&draft)
	if err != nil {
		return nil, err
	}
	if err := s.settingRepo.Set(ctx, CodexPlusConfigSettingKey, raw); err != nil {
		return nil, fmt.Errorf("publish codexplus config: %w", err)
	}
	return &draft, nil
}

func DecodeCodexPlusConfig(raw string) (*CodexPlusConfig, error) {
	var cfg CodexPlusConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("%w: decode: %v", ErrCodexPlusConfigInvalid, err)
	}
	return &cfg, nil
}

func EncodeCodexPlusConfig(cfg *CodexPlusConfig) (string, error) {
	if err := ValidateCodexPlusConfig(cfg); err != nil {
		return "", err
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal codexplus config: %w", err)
	}
	return string(b), nil
}

func DefaultCodexPlusConfig(now time.Time) CodexPlusConfig {
	planCatalog := configregistry.DefaultPlanCatalog()
	modelCatalog := configregistry.DefaultModelCatalog()
	usagePolicy := configregistry.DefaultUsagePolicyCatalog(now)
	featureFlags := configregistry.DefaultFeatureFlags(now)

	cfg := CodexPlusConfig{
		ConfigVersion: CodexPlusConfigVersionMVP,
		DraftStatus:   "published",
		PublishScope:  "production",
		UpdatedBy:     "system",
		UpdatedAt:     now.UTC().Format(time.RFC3339),
		ChangeReason:  "initial Codex++ config center defaults",
		PlanCatalog:   codexPlusPlanCatalogFromRegistry(planCatalog),
		ModelCatalog:  codexPlusModelCatalogFromRegistry(modelCatalog),
		UsagePolicy:   codexPlusUsagePolicyFromRegistry(usagePolicy),
		FeatureFlags:  codexPlusFeatureFlagsFromRegistry(featureFlags),
	}
	codexPlusAlignDefaultConfigReferences(&cfg)
	return cfg
}

func NormalizeCodexPlusConfig(cfg *CodexPlusConfig, now time.Time, actor, reason string) {
	if cfg == nil {
		return
	}
	if strings.TrimSpace(cfg.ConfigVersion) == "" {
		cfg.ConfigVersion = CodexPlusConfigVersionMVP
	}
	cfg.DraftStatus = "published"
	if strings.TrimSpace(cfg.PublishScope) == "" {
		cfg.PublishScope = "production"
	}
	if actor == "" {
		actor = "system"
	}
	if reason == "" {
		reason = cfg.ChangeReason
	}
	cfg.UpdatedBy = actor
	cfg.UpdatedAt = now.UTC().Format(time.RFC3339)
	cfg.ChangeReason = strings.TrimSpace(reason)
	normalizeCodexPlusPlanCatalogMeta(&cfg.PlanCatalog, *cfg)
	normalizeCodexPlusModelCatalogMeta(&cfg.ModelCatalog, *cfg)
	normalizeCodexPlusUsagePolicyMeta(&cfg.UsagePolicy, *cfg)
	normalizeCodexPlusFeatureFlagsMeta(&cfg.FeatureFlags, *cfg)
}

func ValidateCodexPlusConfig(cfg *CodexPlusConfig) error {
	if cfg == nil {
		return fmt.Errorf("%w: config is nil", ErrCodexPlusConfigInvalid)
	}
	if strings.TrimSpace(cfg.ConfigVersion) == "" {
		return fmt.Errorf("%w: config_version is required", ErrCodexPlusConfigInvalid)
	}
	if strings.TrimSpace(cfg.DraftStatus) != "" && !validCodexPlusDraftStatus(cfg.DraftStatus) {
		return fmt.Errorf("%w: invalid draft_status %q", ErrCodexPlusConfigInvalid, cfg.DraftStatus)
	}
	if !validCodexPlusPublishScope(cfg.PublishScope) {
		return fmt.Errorf("%w: invalid publish_scope %q", ErrCodexPlusConfigInvalid, cfg.PublishScope)
	}
	if _, err := time.Parse(time.RFC3339, cfg.UpdatedAt); strings.TrimSpace(cfg.UpdatedAt) != "" && err != nil {
		return fmt.Errorf("%w: updated_at must be RFC3339", ErrCodexPlusConfigInvalid)
	}
	if err := validateCodexPlusPlans(cfg.PlanCatalog.Plans); err != nil {
		return err
	}
	if err := validateCodexPlusModels(cfg.ModelCatalog.Models); err != nil {
		return err
	}
	if err := validateCodexPlusUsagePolicies(cfg.UsagePolicy.Policies); err != nil {
		return err
	}
	if err := configregistry.ValidatePlanCatalog(codexPlusPlanCatalogToRegistry(cfg)); err != nil {
		return fmt.Errorf("%w: %v", ErrCodexPlusConfigInvalid, err)
	}
	if err := configregistry.ValidateModelCatalog(codexPlusModelCatalogToRegistry(cfg)); err != nil {
		return fmt.Errorf("%w: %v", ErrCodexPlusConfigInvalid, err)
	}
	if err := configregistry.ValidateUsagePolicyCatalog(codexPlusUsagePolicyToRegistry(cfg)); err != nil {
		return fmt.Errorf("%w: %v", ErrCodexPlusConfigInvalid, err)
	}
	if err := configregistry.ValidateFeatureFlags(codexPlusFeatureFlagsToRegistry(cfg)); err != nil {
		return fmt.Errorf("%w: %v", ErrCodexPlusConfigInvalid, err)
	}
	return nil
}

func validCodexPlusDraftStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "draft", "editing", "ready_for_review", "approved", "published", "archived":
		return true
	default:
		return false
	}
}

func validCodexPlusPublishScope(scope string) bool {
	switch strings.TrimSpace(scope) {
	case "draft", "internal", "canary", "production":
		return true
	default:
		return false
	}
}

func normalizeCodexPlusPlanCatalogMeta(catalog *CodexPlusPlanCatalog, cfg CodexPlusConfig) {
	if catalog == nil {
		return
	}
	if strings.TrimSpace(catalog.ConfigVersion) == "" {
		catalog.ConfigVersion = cfg.ConfigVersion
	}
	catalog.DraftStatus = codexPlusCatalogDraftStatus("", cfg.DraftStatus, cfg.PublishScope)
	catalog.PublishScope = codexPlusCatalogPublishScope(catalog.PublishScope, cfg.PublishScope)
	catalog.UpdatedBy = codexPlusCatalogUpdatedBy(catalog.UpdatedBy, cfg.UpdatedBy)
	catalog.UpdatedAt = codexPlusCatalogUpdatedAt(catalog.UpdatedAt, cfg.UpdatedAt)
	catalog.ChangeReason = codexPlusCatalogChangeReason(catalog.ChangeReason, cfg.ChangeReason)
}

func normalizeCodexPlusModelCatalogMeta(catalog *CodexPlusModelCatalog, cfg CodexPlusConfig) {
	if catalog == nil {
		return
	}
	if strings.TrimSpace(catalog.ConfigVersion) == "" {
		catalog.ConfigVersion = cfg.ConfigVersion
	}
	catalog.DraftStatus = codexPlusCatalogDraftStatus("", cfg.DraftStatus, cfg.PublishScope)
	catalog.PublishScope = codexPlusCatalogPublishScope(catalog.PublishScope, cfg.PublishScope)
	catalog.UpdatedBy = codexPlusCatalogUpdatedBy(catalog.UpdatedBy, cfg.UpdatedBy)
	catalog.UpdatedAt = codexPlusCatalogUpdatedAt(catalog.UpdatedAt, cfg.UpdatedAt)
	catalog.ChangeReason = codexPlusCatalogChangeReason(catalog.ChangeReason, cfg.ChangeReason)
}

func normalizeCodexPlusUsagePolicyMeta(policy *CodexPlusUsagePolicy, cfg CodexPlusConfig) {
	if policy == nil {
		return
	}
	if strings.TrimSpace(policy.ConfigVersion) == "" {
		policy.ConfigVersion = cfg.ConfigVersion
	}
	policy.DraftStatus = codexPlusCatalogDraftStatus("", cfg.DraftStatus, cfg.PublishScope)
	policy.PublishScope = codexPlusCatalogPublishScope(policy.PublishScope, cfg.PublishScope)
	policy.UpdatedBy = codexPlusCatalogUpdatedBy(policy.UpdatedBy, cfg.UpdatedBy)
	policy.UpdatedAt = codexPlusCatalogUpdatedAt(policy.UpdatedAt, cfg.UpdatedAt)
	policy.ChangeReason = codexPlusCatalogChangeReason(policy.ChangeReason, cfg.ChangeReason)
}

func normalizeCodexPlusFeatureFlagsMeta(flags *CodexPlusFeatureFlagsDoc, cfg CodexPlusConfig) {
	if flags == nil {
		return
	}
	if strings.TrimSpace(flags.ConfigVersion) == "" {
		flags.ConfigVersion = cfg.ConfigVersion
	}
	flags.DraftStatus = codexPlusCatalogDraftStatus("", cfg.DraftStatus, cfg.PublishScope)
	flags.PublishScope = codexPlusCatalogPublishScope(flags.PublishScope, cfg.PublishScope)
	flags.UpdatedBy = codexPlusCatalogUpdatedBy(flags.UpdatedBy, cfg.UpdatedBy)
	flags.UpdatedAt = codexPlusCatalogUpdatedAt(flags.UpdatedAt, cfg.UpdatedAt)
	flags.ChangeReason = codexPlusCatalogChangeReason(flags.ChangeReason, cfg.ChangeReason)
}

func codexPlusPlanCatalogFromRegistry(in configregistry.PlanCatalog) CodexPlusPlanCatalog {
	out := CodexPlusPlanCatalog{
		ConfigVersion: in.ConfigVersion,
		DraftStatus:   in.DraftStatus,
		PublishScope:  in.PublishScope,
		RollbackFrom:  in.RollbackFrom,
		UpdatedBy:     in.UpdatedBy,
		UpdatedAt:     in.UpdatedAt,
		ChangeReason:  in.ChangeReason,
		Plans:         make([]CodexPlusPlan, 0, len(in.Plans)),
	}
	for _, plan := range in.Plans {
		out.Plans = append(out.Plans, CodexPlusPlan{
			PlanID:           plan.PlanID,
			Name:             plan.Name,
			Description:      plan.Description,
			BillingPeriod:    plan.BillingPeriod,
			Currency:         plan.Currency,
			PriceAmountMinor: plan.PriceAmountMinor,
			DisplayPrice:     plan.DisplayPrice,
			EntitlementGrant: CodexPlusEntitlementGrant{
				BalanceCredit: plan.EntitlementGrant.BalanceCredit,
				DurationDays:  plan.EntitlementGrant.DurationDays,
				DailyQuota:    plan.EntitlementGrant.DailyQuota,
				PeriodQuota:   plan.EntitlementGrant.PeriodQuota,
			},
			EntitlementSources: CodexPlusEntitlementSources{
				SubscriptionGroupIDs: codexPlusIntToInt64Slice(plan.EntitlementSources.SubscriptionGroupIDs),
				APIKeyGroupIDs:       codexPlusIntToInt64Slice(plan.EntitlementSources.APIKeyGroupIDs),
				GroupNames:           append([]string(nil), plan.EntitlementSources.GroupNames...),
			},
			ModelGroups:   append([]string(nil), plan.ModelGroups...),
			UsagePolicyID: plan.UsagePolicyID,
			PurchaseURL:   plan.PurchaseURL,
			RenewURL:      plan.RenewURL,
			CopyKeys: CodexPlusPlanCopyKeys{
				PurchaseAction:      plan.CopyKeys.PurchaseAction,
				RenewAction:         plan.CopyKeys.RenewAction,
				UpgradeAction:       plan.CopyKeys.UpgradeAction,
				NotPurchasedMessage: plan.CopyKeys.NotPurchasedMessage,
				ExpiredMessage:      plan.CopyKeys.ExpiredMessage,
				LowBalanceMessage:   plan.CopyKeys.LowBalanceMessage,
			},
			IsListed:  plan.IsListed,
			Status:    plan.Status,
			SortOrder: plan.SortOrder,
			ExternalBillingRefs: CodexPlusPlanExternalBillingRefs{
				ProductID: plan.ExternalBillingRefs.ProductID,
				SKUID:     plan.ExternalBillingRefs.SKUID,
			},
		})
	}
	return out
}

func codexPlusModelCatalogFromRegistry(in configregistry.ModelCatalog) CodexPlusModelCatalog {
	out := CodexPlusModelCatalog{
		ConfigVersion: in.ConfigVersion,
		DraftStatus:   in.DraftStatus,
		PublishScope:  in.PublishScope,
		RollbackFrom:  in.RollbackFrom,
		UpdatedBy:     in.UpdatedBy,
		UpdatedAt:     in.UpdatedAt,
		ChangeReason:  in.ChangeReason,
		Models:        make([]CodexPlusModel, 0, len(in.Models)),
	}
	for _, model := range in.Models {
		out.Models = append(out.Models, CodexPlusModel{
			ModelID:                    model.ModelID,
			DisplayName:                model.DisplayName,
			RouteModel:                 model.RouteModel,
			ModelGroup:                 model.ModelGroup,
			ContextWindow:              model.ContextWindow,
			BillingMultiplier:          model.BillingMultiplier,
			IsDefault:                  model.IsDefault,
			IsEnabled:                  model.IsEnabled,
			IsHidden:                   model.IsHidden,
			DisabledReason:             model.DisabledReason,
			RolloutChannel:             model.RolloutChannel,
			QualityTier:                model.QualityTier,
			FallbackModelID:            model.FallbackModelID,
			DeprecationAt:              model.DeprecationAt,
			DisabledReplacementModelID: model.DisabledReplacementModelID,
			DisabledMessageKey:         model.DisabledMessageKey,
			SortOrder:                  model.SortOrder,
			OperatorTags:               append([]string(nil), model.OperatorTags...),
		})
	}
	return out
}

func codexPlusUsagePolicyFromRegistry(in configregistry.UsagePolicyCatalog) CodexPlusUsagePolicy {
	out := CodexPlusUsagePolicy{
		ConfigVersion: in.ConfigVersion,
		DraftStatus:   in.DraftStatus,
		PublishScope:  in.PublishScope,
		RollbackFrom:  in.RollbackFrom,
		UpdatedBy:     in.UpdatedBy,
		UpdatedAt:     in.UpdatedAt,
		ChangeReason:  in.ChangeReason,
		Policies:      make([]CodexPlusUsageRule, 0, len(in.Policies)),
	}
	for _, policy := range in.Policies {
		out.Policies = append(out.Policies, CodexPlusUsageRule{
			PolicyID: policy.PolicyID,
			AppliesTo: CodexPlusUsagePolicyAppliesTo{
				PlanIDs:      append([]string(nil), policy.AppliesTo.PlanIDs...),
				ModelGroups:  append([]string(nil), policy.AppliesTo.ModelGroups...),
				UserSegments: append([]string(nil), policy.AppliesTo.UserSegments...),
			},
			LowBalanceThreshold:    policy.LowBalanceThreshold,
			DailyQuota:             policy.DailyQuota,
			MonthlyQuota:           policy.MonthlyQuota,
			ConcurrencyLimit:       policy.ConcurrencyLimit,
			RPMLimit:               policy.RPMLimit,
			TPMLimit:               policy.TPMLimit,
			BurstLimit:             policy.BurstLimit,
			RateLimitWindowSeconds: policy.RateLimitWindowSeconds,
			ExpiredBehavior:        policy.ExpiredBehavior,
			GracePeriodHours:       policy.GracePeriodHours,
			OverageBehavior:        policy.OverageBehavior,
			CopyKeys: CodexPlusUsagePolicyCopyKeys{
				LowBalanceMessage:          policy.CopyKeys.LowBalanceMessage,
				InsufficientBalanceMessage: policy.CopyKeys.InsufficientBalanceMessage,
				RateLimitedMessage:         policy.CopyKeys.RateLimitedMessage,
				ExpiredMessage:             policy.CopyKeys.ExpiredMessage,
				RenewAction:                policy.CopyKeys.RenewAction,
				PurchaseAction:             policy.CopyKeys.PurchaseAction,
				DeviceRevokedMessage:       policy.CopyKeys.DeviceRevokedMessage,
			},
			DevicePolicy: CodexPlusUsagePolicyDevicePolicy{
				RegistrationRequired:        policy.DevicePolicy.RegistrationRequired,
				MaxDevicesPerUser:           policy.DevicePolicy.MaxDevicesPerUser,
				AllowSelfServiceReplacement: policy.DevicePolicy.AllowSelfServiceReplacement,
				ReplacementCooldownHours:    policy.DevicePolicy.ReplacementCooldownHours,
				StrictEnforcementDefault:    policy.DevicePolicy.StrictEnforcementDefault,
				RevokeReasonTaxonomy:        append([]string(nil), policy.DevicePolicy.RevokeReasonTaxonomy...),
				SupportUnlockPolicy:         policy.DevicePolicy.SupportUnlockPolicy,
				RevokedBehavior:             policy.DevicePolicy.RevokedBehavior,
				MessageKeys: CodexPlusUsagePolicyDeviceMessageKeys{
					LimitReached:          policy.DevicePolicy.MessageKeys.LimitReached,
					ReplacementCooldown:   policy.DevicePolicy.MessageKeys.ReplacementCooldown,
					Revoked:               policy.DevicePolicy.MessageKeys.Revoked,
					SupportUnlockRequired: policy.DevicePolicy.MessageKeys.SupportUnlockRequired,
				},
			},
			InsufficientBalanceMessage: policy.CopyKeys.InsufficientBalanceMessage,
			RateLimitedMessage:         policy.CopyKeys.RateLimitedMessage,
		})
	}
	return out
}

func codexPlusFeatureFlagsFromRegistry(in configregistry.FeatureFlags) CodexPlusFeatureFlagsDoc {
	return CodexPlusFeatureFlagsDoc{
		ConfigVersion: in.ConfigVersion,
		DraftStatus:   in.DraftStatus,
		PublishScope:  in.PublishScope,
		RollbackFrom:  in.RollbackFrom,
		UpdatedBy:     in.UpdatedBy,
		UpdatedAt:     in.UpdatedAt,
		ChangeReason:  in.ChangeReason,
		Flags: CodexPlusFeatureFlags{
			AdvancedProviderConfig:  in.Flags.AdvancedProviderConfig,
			InstallAssistant:        in.Flags.InstallAssistant,
			NewUserTutorial:         in.Flags.NewUserTutorial,
			ModelSelector:           in.Flags.ModelSelector,
			DiagnosticExport:        in.Flags.DiagnosticExport,
			Announcements:           in.Flags.Announcements,
			ForceUpdatePrompt:       in.Flags.ForceUpdatePrompt,
			StrictDeviceEnforcement: in.Flags.StrictDeviceEnforcement,
		},
		Exposure: CodexPlusFeatureFlagExposure{
			ClientVisible: append([]configregistry.FeatureFlagName(nil), in.Exposure.ClientVisible...),
			ServerOnly:    append([]configregistry.FeatureFlagName(nil), in.Exposure.ServerOnly...),
		},
		CopyKeys: CodexPlusFeatureFlagCopyKeys{
			ForceUpdatePrompt:     in.CopyKeys.ForceUpdatePrompt,
			InstallAssistantEntry: in.CopyKeys.InstallAssistantEntry,
			NewUserTutorialEntry:  in.CopyKeys.NewUserTutorialEntry,
			DiagnosticExportEntry: in.CopyKeys.DiagnosticExportEntry,
			AnnouncementEntry:     in.CopyKeys.AnnouncementEntry,
		},
	}
}

func codexPlusAlignDefaultConfigReferences(cfg *CodexPlusConfig) {
	if cfg == nil {
		return
	}
	modelGroups := make([]string, 0, len(cfg.ModelCatalog.Models))
	seenModelGroups := map[string]struct{}{}
	for _, model := range cfg.ModelCatalog.Models {
		group := strings.TrimSpace(model.ModelGroup)
		if group == "" {
			continue
		}
		if _, seen := seenModelGroups[group]; seen {
			continue
		}
		seenModelGroups[group] = struct{}{}
		modelGroups = append(modelGroups, group)
	}
	policyID := ""
	if len(cfg.UsagePolicy.Policies) > 0 {
		policyID = strings.TrimSpace(cfg.UsagePolicy.Policies[0].PolicyID)
	}
	for i := range cfg.PlanCatalog.Plans {
		if policyID != "" && !codexPlusUsagePolicyExists(cfg, cfg.PlanCatalog.Plans[i].UsagePolicyID) {
			cfg.PlanCatalog.Plans[i].UsagePolicyID = policyID
		}
		if len(modelGroups) > 0 && !codexPlusPlanHasKnownModelGroup(cfg.PlanCatalog.Plans[i], seenModelGroups) {
			cfg.PlanCatalog.Plans[i].ModelGroups = append([]string(nil), modelGroups...)
		}
	}
	planIDs := make([]string, 0, len(cfg.PlanCatalog.Plans))
	seenPlanIDs := map[string]struct{}{}
	for _, plan := range cfg.PlanCatalog.Plans {
		id := strings.TrimSpace(plan.PlanID)
		if id == "" {
			continue
		}
		seenPlanIDs[id] = struct{}{}
		planIDs = append(planIDs, id)
	}
	for i := range cfg.UsagePolicy.Policies {
		if len(planIDs) > 0 && !codexPlusSliceIntersectsSet(cfg.UsagePolicy.Policies[i].AppliesTo.PlanIDs, seenPlanIDs) {
			cfg.UsagePolicy.Policies[i].AppliesTo.PlanIDs = append([]string(nil), planIDs...)
		}
		if len(modelGroups) > 0 && !codexPlusSliceIntersectsSet(cfg.UsagePolicy.Policies[i].AppliesTo.ModelGroups, seenModelGroups) {
			cfg.UsagePolicy.Policies[i].AppliesTo.ModelGroups = append([]string(nil), modelGroups...)
		}
	}
}

func codexPlusUsagePolicyExists(cfg *CodexPlusConfig, policyID string) bool {
	policyID = strings.TrimSpace(policyID)
	if cfg == nil || policyID == "" {
		return false
	}
	for _, policy := range cfg.UsagePolicy.Policies {
		if strings.TrimSpace(policy.PolicyID) == policyID {
			return true
		}
	}
	return false
}

func codexPlusPlanHasKnownModelGroup(plan CodexPlusPlan, modelGroups map[string]struct{}) bool {
	for _, group := range plan.ModelGroups {
		if _, ok := modelGroups[strings.TrimSpace(group)]; ok {
			return true
		}
	}
	return false
}

func codexPlusSliceIntersectsSet(values []string, allowed map[string]struct{}) bool {
	for _, value := range values {
		if _, ok := allowed[strings.TrimSpace(value)]; ok {
			return true
		}
	}
	return false
}

func codexPlusPlanCatalogToRegistry(cfg *CodexPlusConfig) configregistry.PlanCatalog {
	catalog := cfg.PlanCatalog
	out := configregistry.PlanCatalog{
		DocumentMeta: codexPlusRegistryMeta(cfg, catalog.ConfigVersion, catalog.DraftStatus, catalog.PublishScope, catalog.RollbackFrom, catalog.UpdatedBy, catalog.UpdatedAt, catalog.ChangeReason),
		Plans:        make([]configregistry.PlanCatalogPlan, 0, len(catalog.Plans)),
	}
	for _, plan := range catalog.Plans {
		copyKeys := codexPlusPlanCopyKeysForRegistry(plan.CopyKeys)
		out.Plans = append(out.Plans, configregistry.PlanCatalogPlan{
			PlanID:           strings.TrimSpace(plan.PlanID),
			Name:             codexPlusFallbackString(plan.Name, plan.PlanID),
			Description:      codexPlusFallbackString(plan.Description, codexPlusFallbackString(plan.Name, plan.PlanID)+" plan"),
			BillingPeriod:    codexPlusBillingPeriodForRegistry(plan.BillingPeriod),
			Currency:         codexPlusFallbackString(plan.Currency, "USD"),
			PriceAmountMinor: plan.PriceAmountMinor,
			DisplayPrice:     strings.TrimSpace(plan.DisplayPrice),
			EntitlementGrant: configregistry.PlanEntitlementGrant{
				BalanceCredit: plan.EntitlementGrant.BalanceCredit,
				DurationDays:  plan.EntitlementGrant.DurationDays,
				DailyQuota:    plan.EntitlementGrant.DailyQuota,
				PeriodQuota:   plan.EntitlementGrant.PeriodQuota,
			},
			EntitlementSources: configregistry.PlanEntitlementSources{
				SubscriptionGroupIDs: codexPlusInt64ToIntSlice(plan.EntitlementSources.SubscriptionGroupIDs),
				APIKeyGroupIDs:       codexPlusInt64ToIntSlice(plan.EntitlementSources.APIKeyGroupIDs),
				GroupNames:           append([]string(nil), plan.EntitlementSources.GroupNames...),
			},
			ModelGroups:   append([]string(nil), plan.ModelGroups...),
			UsagePolicyID: codexPlusFallbackString(plan.UsagePolicyID, "default"),
			PurchaseURL:   plan.PurchaseURL,
			RenewURL:      plan.RenewURL,
			CopyKeys:      copyKeys,
			IsListed:      plan.IsListed,
			Status:        codexPlusPlanStatusForRegistry(plan.Status),
			SortOrder:     plan.SortOrder,
			ExternalBillingRefs: configregistry.PlanExternalBillingRefs{
				ProductID: plan.ExternalBillingRefs.ProductID,
				SKUID:     plan.ExternalBillingRefs.SKUID,
			},
		})
	}
	return out
}

func codexPlusModelCatalogToRegistry(cfg *CodexPlusConfig) configregistry.ModelCatalog {
	catalog := cfg.ModelCatalog
	out := configregistry.ModelCatalog{
		DocumentMeta: codexPlusRegistryMeta(cfg, catalog.ConfigVersion, catalog.DraftStatus, catalog.PublishScope, catalog.RollbackFrom, catalog.UpdatedBy, catalog.UpdatedAt, catalog.ChangeReason),
		Models:       make([]configregistry.ModelCatalogModel, 0, len(catalog.Models)),
	}
	for _, model := range catalog.Models {
		disabledReason := model.DisabledReason
		if !model.IsEnabled && (disabledReason == nil || strings.TrimSpace(*disabledReason) == "") {
			disabledReason = codexPlusStringPtr("disabled by admin")
		}
		disabledMessageKey := model.DisabledMessageKey
		if !model.IsEnabled && (disabledMessageKey == nil || strings.TrimSpace(*disabledMessageKey) == "") {
			disabledMessageKey = codexPlusStringPtr("model.disabled")
		}
		out.Models = append(out.Models, configregistry.ModelCatalogModel{
			ModelID:                    strings.TrimSpace(model.ModelID),
			DisplayName:                codexPlusFallbackString(model.DisplayName, model.ModelID),
			RouteModel:                 strings.TrimSpace(model.RouteModel),
			ModelGroup:                 strings.TrimSpace(model.ModelGroup),
			ContextWindow:              model.ContextWindow,
			BillingMultiplier:          model.BillingMultiplier,
			IsDefault:                  model.IsDefault,
			IsEnabled:                  model.IsEnabled,
			IsHidden:                   model.IsHidden,
			DisabledReason:             disabledReason,
			RolloutChannel:             codexPlusFallbackString(model.RolloutChannel, configregistry.RolloutChannelStable),
			QualityTier:                codexPlusFallbackString(model.QualityTier, configregistry.QualityTierStandard),
			FallbackModelID:            model.FallbackModelID,
			DeprecationAt:              model.DeprecationAt,
			DisabledReplacementModelID: model.DisabledReplacementModelID,
			DisabledMessageKey:         disabledMessageKey,
			SortOrder:                  model.SortOrder,
			OperatorTags:               append([]string(nil), model.OperatorTags...),
		})
	}
	return out
}

func codexPlusUsagePolicyToRegistry(cfg *CodexPlusConfig) configregistry.UsagePolicyCatalog {
	policyDoc := cfg.UsagePolicy
	out := configregistry.UsagePolicyCatalog{
		DocumentMeta: codexPlusRegistryMeta(cfg, policyDoc.ConfigVersion, policyDoc.DraftStatus, policyDoc.PublishScope, policyDoc.RollbackFrom, policyDoc.UpdatedBy, policyDoc.UpdatedAt, policyDoc.ChangeReason),
		Policies:     make([]configregistry.UsagePolicyRule, 0, len(policyDoc.Policies)),
	}
	for _, policy := range policyDoc.Policies {
		copyKeys := codexPlusUsageCopyKeysForRegistry(policy)
		out.Policies = append(out.Policies, configregistry.UsagePolicyRule{
			PolicyID: strings.TrimSpace(policy.PolicyID),
			AppliesTo: configregistry.UsagePolicyAppliesTo{
				PlanIDs:      append([]string(nil), policy.AppliesTo.PlanIDs...),
				ModelGroups:  append([]string(nil), policy.AppliesTo.ModelGroups...),
				UserSegments: append([]string(nil), policy.AppliesTo.UserSegments...),
			},
			LowBalanceThreshold:    policy.LowBalanceThreshold,
			DailyQuota:             policy.DailyQuota,
			MonthlyQuota:           policy.MonthlyQuota,
			ConcurrencyLimit:       policy.ConcurrencyLimit,
			RPMLimit:               policy.RPMLimit,
			TPMLimit:               policy.TPMLimit,
			BurstLimit:             codexPlusPositiveOrDefault(policy.BurstLimit, policy.RPMLimit),
			RateLimitWindowSeconds: codexPlusPositiveOrDefault(policy.RateLimitWindowSeconds, 60),
			ExpiredBehavior:        codexPlusFallbackString(policy.ExpiredBehavior, configregistry.UsagePolicyExpiredBehaviorBlock),
			GracePeriodHours:       policy.GracePeriodHours,
			OverageBehavior:        codexPlusFallbackString(policy.OverageBehavior, configregistry.UsagePolicyOverageBehaviorBlock),
			CopyKeys:               copyKeys,
			DevicePolicy:           codexPlusDevicePolicyForRegistry(policy.DevicePolicy),
		})
	}
	return out
}

func codexPlusFeatureFlagsToRegistry(cfg *CodexPlusConfig) configregistry.FeatureFlags {
	doc := cfg.FeatureFlags
	defaults := configregistry.DefaultFeatureFlags(time.Unix(0, 0).UTC())
	out := configregistry.FeatureFlags{
		DocumentMeta: codexPlusRegistryMeta(cfg, doc.ConfigVersion, doc.DraftStatus, doc.PublishScope, doc.RollbackFrom, doc.UpdatedBy, doc.UpdatedAt, doc.ChangeReason),
		Flags: configregistry.FeatureFlagValues{
			AdvancedProviderConfig:  doc.Flags.AdvancedProviderConfig,
			InstallAssistant:        doc.Flags.InstallAssistant,
			NewUserTutorial:         doc.Flags.NewUserTutorial,
			ModelSelector:           doc.Flags.ModelSelector,
			DiagnosticExport:        doc.Flags.DiagnosticExport,
			Announcements:           doc.Flags.Announcements,
			ForceUpdatePrompt:       doc.Flags.ForceUpdatePrompt,
			StrictDeviceEnforcement: doc.Flags.StrictDeviceEnforcement,
		},
		Exposure: configregistry.FeatureFlagExposure{
			ClientVisible: append([]configregistry.FeatureFlagName(nil), doc.Exposure.ClientVisible...),
			ServerOnly:    append([]configregistry.FeatureFlagName(nil), doc.Exposure.ServerOnly...),
		},
		CopyKeys: configregistry.FeatureFlagCopyKeys{
			ForceUpdatePrompt:     codexPlusFallbackString(doc.CopyKeys.ForceUpdatePrompt, defaults.CopyKeys.ForceUpdatePrompt),
			InstallAssistantEntry: codexPlusFallbackString(doc.CopyKeys.InstallAssistantEntry, defaults.CopyKeys.InstallAssistantEntry),
			NewUserTutorialEntry:  codexPlusFallbackString(doc.CopyKeys.NewUserTutorialEntry, defaults.CopyKeys.NewUserTutorialEntry),
			DiagnosticExportEntry: codexPlusFallbackString(doc.CopyKeys.DiagnosticExportEntry, defaults.CopyKeys.DiagnosticExportEntry),
			AnnouncementEntry:     codexPlusFallbackString(doc.CopyKeys.AnnouncementEntry, defaults.CopyKeys.AnnouncementEntry),
		},
		Semantics: configregistry.FeatureFlagsRuntimeSemantics{
			DiagnosticExportRedactionReady: true,
		},
	}
	if len(out.Exposure.ClientVisible) == 0 && len(out.Exposure.ServerOnly) == 0 {
		out.Exposure = defaults.Exposure
	}
	return out
}

func codexPlusRegistryMeta(cfg *CodexPlusConfig, version, draftStatus, publishScope string, rollbackFrom *string, updatedBy, updatedAt, changeReason string) configregistry.DocumentMeta {
	if cfg != nil {
		version = codexPlusFallbackString(version, cfg.ConfigVersion)
		draftStatus = codexPlusCatalogDraftStatus(draftStatus, cfg.DraftStatus, cfg.PublishScope)
		publishScope = codexPlusCatalogPublishScope(publishScope, cfg.PublishScope)
		updatedBy = codexPlusCatalogUpdatedBy(updatedBy, cfg.UpdatedBy)
		updatedAt = codexPlusCatalogUpdatedAt(updatedAt, cfg.UpdatedAt)
		changeReason = codexPlusCatalogChangeReason(changeReason, cfg.ChangeReason)
	} else {
		draftStatus = codexPlusCatalogDraftStatus(draftStatus, "", publishScope)
		publishScope = codexPlusCatalogPublishScope(publishScope, "")
		updatedBy = codexPlusCatalogUpdatedBy(updatedBy, "")
		updatedAt = codexPlusCatalogUpdatedAt(updatedAt, "")
		changeReason = codexPlusCatalogChangeReason(changeReason, "")
	}
	return configregistry.DocumentMeta{
		ConfigVersion: codexPlusFallbackString(version, CodexPlusConfigVersionMVP),
		DraftStatus:   draftStatus,
		PublishScope:  publishScope,
		RollbackFrom:  rollbackFrom,
		UpdatedBy:     updatedBy,
		UpdatedAt:     updatedAt,
		ChangeReason:  changeReason,
	}
}

func codexPlusCatalogDraftStatus(value, topLevelStatus, publishScope string) string {
	status := strings.TrimSpace(value)
	if status == "" {
		status = strings.TrimSpace(topLevelStatus)
	}
	switch status {
	case "editing", "ready_for_review", "approved", "published", "archived":
		return status
	case "draft":
		return "editing"
	}
	if strings.TrimSpace(publishScope) == "production" {
		return "published"
	}
	return "editing"
}

func codexPlusCatalogPublishScope(value, topLevelScope string) string {
	scope := strings.TrimSpace(value)
	if scope == "" {
		scope = strings.TrimSpace(topLevelScope)
	}
	if scope == "" {
		return "production"
	}
	return scope
}

func codexPlusCatalogUpdatedBy(value, topLevelValue string) string {
	return codexPlusFallbackString(value, codexPlusFallbackString(topLevelValue, "system"))
}

func codexPlusCatalogUpdatedAt(value, topLevelValue string) string {
	return codexPlusFallbackString(value, codexPlusFallbackString(topLevelValue, time.Unix(0, 0).UTC().Format(time.RFC3339)))
}

func codexPlusCatalogChangeReason(value, topLevelValue string) string {
	return codexPlusFallbackString(value, codexPlusFallbackString(topLevelValue, "Codex++ config center update"))
}

func codexPlusBillingPeriodForRegistry(value string) string {
	switch strings.TrimSpace(value) {
	case "month":
		return configregistry.BillingPeriodMonthly
	case "quarter":
		return configregistry.BillingPeriodQuarterly
	case "year", "annual":
		return configregistry.BillingPeriodYearly
	case "one-time", "one_time":
		return configregistry.BillingPeriodOneTime
	case "":
		return configregistry.BillingPeriodNone
	default:
		return strings.TrimSpace(value)
	}
}

func codexPlusPlanStatusForRegistry(value string) string {
	switch strings.TrimSpace(value) {
	case "":
		return configregistry.PlanStatusHidden
	default:
		return strings.TrimSpace(value)
	}
}

func codexPlusPlanCopyKeysForRegistry(keys CodexPlusPlanCopyKeys) configregistry.PlanCopyKeys {
	return configregistry.PlanCopyKeys{
		PurchaseAction:      codexPlusFallbackString(keys.PurchaseAction, "billing.action.purchase"),
		RenewAction:         codexPlusFallbackString(keys.RenewAction, "billing.action.renew"),
		UpgradeAction:       codexPlusFallbackString(keys.UpgradeAction, "billing.action.upgrade"),
		NotPurchasedMessage: codexPlusFallbackString(keys.NotPurchasedMessage, "billing.message.not_purchased"),
		ExpiredMessage:      codexPlusFallbackString(keys.ExpiredMessage, "billing.message.expired"),
		LowBalanceMessage:   codexPlusFallbackString(keys.LowBalanceMessage, "billing.message.low_balance"),
	}
}

func codexPlusUsageCopyKeysForRegistry(policy CodexPlusUsageRule) configregistry.UsagePolicyCopyKeys {
	return configregistry.UsagePolicyCopyKeys{
		LowBalanceMessage:          codexPlusFallbackString(policy.CopyKeys.LowBalanceMessage, "usage.low_balance"),
		InsufficientBalanceMessage: codexPlusFallbackString(policy.CopyKeys.InsufficientBalanceMessage, codexPlusFallbackString(policy.InsufficientBalanceMessage, "usage.insufficient_balance")),
		RateLimitedMessage:         codexPlusFallbackString(policy.CopyKeys.RateLimitedMessage, codexPlusFallbackString(policy.RateLimitedMessage, "usage.rate_limited")),
		ExpiredMessage:             codexPlusFallbackString(policy.CopyKeys.ExpiredMessage, "usage.expired"),
		RenewAction:                codexPlusFallbackString(policy.CopyKeys.RenewAction, "usage.renew_action"),
		PurchaseAction:             codexPlusFallbackString(policy.CopyKeys.PurchaseAction, "usage.purchase_action"),
		DeviceRevokedMessage:       codexPlusFallbackString(policy.CopyKeys.DeviceRevokedMessage, "device.revoked"),
	}
}

func codexPlusDevicePolicyForRegistry(policy CodexPlusUsagePolicyDevicePolicy) configregistry.UsagePolicyDevicePolicy {
	defaults := configregistry.DefaultUsagePolicyDevicePolicy()
	if policy.MaxDevicesPerUser < 1 {
		policy.MaxDevicesPerUser = defaults.MaxDevicesPerUser
	}
	if len(policy.RevokeReasonTaxonomy) == 0 {
		policy.RevokeReasonTaxonomy = defaults.RevokeReasonTaxonomy
	}
	if strings.TrimSpace(policy.SupportUnlockPolicy) == "" {
		policy.SupportUnlockPolicy = defaults.SupportUnlockPolicy
	}
	if strings.TrimSpace(policy.RevokedBehavior) == "" {
		policy.RevokedBehavior = defaults.RevokedBehavior
	}
	if strings.TrimSpace(policy.MessageKeys.LimitReached) == "" {
		policy.MessageKeys.LimitReached = defaults.MessageKeys.LimitReached
	}
	if strings.TrimSpace(policy.MessageKeys.ReplacementCooldown) == "" {
		policy.MessageKeys.ReplacementCooldown = defaults.MessageKeys.ReplacementCooldown
	}
	if strings.TrimSpace(policy.MessageKeys.Revoked) == "" {
		policy.MessageKeys.Revoked = defaults.MessageKeys.Revoked
	}
	if strings.TrimSpace(policy.MessageKeys.SupportUnlockRequired) == "" {
		policy.MessageKeys.SupportUnlockRequired = defaults.MessageKeys.SupportUnlockRequired
	}
	return configregistry.UsagePolicyDevicePolicy{
		RegistrationRequired:        policy.RegistrationRequired,
		MaxDevicesPerUser:           policy.MaxDevicesPerUser,
		AllowSelfServiceReplacement: policy.AllowSelfServiceReplacement,
		ReplacementCooldownHours:    policy.ReplacementCooldownHours,
		StrictEnforcementDefault:    policy.StrictEnforcementDefault,
		RevokeReasonTaxonomy:        append([]string(nil), policy.RevokeReasonTaxonomy...),
		SupportUnlockPolicy:         policy.SupportUnlockPolicy,
		RevokedBehavior:             policy.RevokedBehavior,
		MessageKeys: configregistry.UsagePolicyDeviceMessageKeys{
			LimitReached:          policy.MessageKeys.LimitReached,
			ReplacementCooldown:   policy.MessageKeys.ReplacementCooldown,
			Revoked:               policy.MessageKeys.Revoked,
			SupportUnlockRequired: policy.MessageKeys.SupportUnlockRequired,
		},
	}
}

func codexPlusIntToInt64Slice(values []int) []int64 {
	out := make([]int64, 0, len(values))
	for _, value := range values {
		out = append(out, int64(value))
	}
	return out
}

func codexPlusInt64ToIntSlice(values []int64) []int {
	out := make([]int, 0, len(values))
	for _, value := range values {
		out = append(out, int(value))
	}
	return out
}

func codexPlusPositiveOrDefault(value, fallback int) int {
	if value > 0 {
		return value
	}
	if fallback > 0 {
		return fallback
	}
	return 1
}

func codexPlusFallbackString(value, fallback string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return fallback
}

func codexPlusStringPtr(value string) *string {
	return &value
}

func validateCodexPlusPlans(plans []CodexPlusPlan) error {
	if len(plans) == 0 {
		return fmt.Errorf("%w: at least one plan is required", ErrCodexPlusConfigInvalid)
	}
	seen := make(map[string]struct{}, len(plans))
	for i, plan := range plans {
		id := strings.TrimSpace(plan.PlanID)
		if id == "" {
			return fmt.Errorf("%w: plan[%d].plan_id is required", ErrCodexPlusConfigInvalid, i)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("%w: duplicate plan_id %q", ErrCodexPlusConfigInvalid, id)
		}
		seen[id] = struct{}{}
		if strings.TrimSpace(plan.Name) == "" {
			return fmt.Errorf("%w: plan[%d].name is required", ErrCodexPlusConfigInvalid, i)
		}
		if len(plan.ModelGroups) == 0 {
			return fmt.Errorf("%w: plan[%d] must reference at least one model group", ErrCodexPlusConfigInvalid, i)
		}
		if plan.EntitlementGrant.BalanceCredit < 0 || plan.EntitlementGrant.DurationDays < 0 {
			return fmt.Errorf("%w: plan[%d] entitlement values cannot be negative", ErrCodexPlusConfigInvalid, i)
		}
		if err := validateCodexPlusEntitlementSources(i, plan.EntitlementSources); err != nil {
			return err
		}
	}
	return nil
}

func validateCodexPlusEntitlementSources(index int, sources CodexPlusEntitlementSources) error {
	for _, id := range sources.SubscriptionGroupIDs {
		if id <= 0 {
			return fmt.Errorf("%w: plan[%d].entitlement_sources.subscription_group_ids must be positive", ErrCodexPlusConfigInvalid, index)
		}
	}
	for _, id := range sources.APIKeyGroupIDs {
		if id <= 0 {
			return fmt.Errorf("%w: plan[%d].entitlement_sources.api_key_group_ids must be positive", ErrCodexPlusConfigInvalid, index)
		}
	}
	for _, name := range sources.GroupNames {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("%w: plan[%d].entitlement_sources.group_names cannot contain empty values", ErrCodexPlusConfigInvalid, index)
		}
	}
	return nil
}

func validateCodexPlusModels(models []CodexPlusModel) error {
	if len(models) == 0 {
		return fmt.Errorf("%w: at least one model is required", ErrCodexPlusConfigInvalid)
	}
	seen := make(map[string]struct{}, len(models))
	defaultCount := 0
	for i, model := range models {
		id := strings.TrimSpace(model.ModelID)
		if id == "" {
			return fmt.Errorf("%w: model[%d].model_id is required", ErrCodexPlusConfigInvalid, i)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("%w: duplicate model_id %q", ErrCodexPlusConfigInvalid, id)
		}
		seen[id] = struct{}{}
		if strings.TrimSpace(model.RouteModel) == "" {
			return fmt.Errorf("%w: model[%d].route_model is required", ErrCodexPlusConfigInvalid, i)
		}
		if strings.TrimSpace(model.ModelGroup) == "" {
			return fmt.Errorf("%w: model[%d].model_group is required", ErrCodexPlusConfigInvalid, i)
		}
		if model.ContextWindow < 1024 {
			return fmt.Errorf("%w: model[%d].context_window is too small", ErrCodexPlusConfigInvalid, i)
		}
		if model.BillingMultiplier <= 0 {
			return fmt.Errorf("%w: model[%d].billing_multiplier must be positive", ErrCodexPlusConfigInvalid, i)
		}
		if model.IsDefault {
			defaultCount++
			if !model.IsEnabled {
				return fmt.Errorf("%w: default model must be enabled", ErrCodexPlusConfigInvalid)
			}
		}
	}
	if defaultCount != 1 {
		return fmt.Errorf("%w: exactly one default model is required", ErrCodexPlusConfigInvalid)
	}
	return nil
}

func validateCodexPlusUsagePolicies(policies []CodexPlusUsageRule) error {
	if len(policies) == 0 {
		return fmt.Errorf("%w: at least one usage policy is required", ErrCodexPlusConfigInvalid)
	}
	seen := make(map[string]struct{}, len(policies))
	for i, policy := range policies {
		id := strings.TrimSpace(policy.PolicyID)
		if id == "" {
			return fmt.Errorf("%w: policy[%d].policy_id is required", ErrCodexPlusConfigInvalid, i)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("%w: duplicate policy_id %q", ErrCodexPlusConfigInvalid, id)
		}
		seen[id] = struct{}{}
		if policy.LowBalanceThreshold < 0 || policy.DailyQuota < 0 || policy.GracePeriodHours < 0 {
			return fmt.Errorf("%w: policy[%d] numeric thresholds cannot be negative", ErrCodexPlusConfigInvalid, i)
		}
		if policy.ConcurrencyLimit < 1 || policy.RPMLimit < 1 || policy.TPMLimit < 1 {
			return fmt.Errorf("%w: policy[%d] limits must be positive", ErrCodexPlusConfigInvalid, i)
		}
		if !validCodexPlusExpiredBehavior(policy.ExpiredBehavior) {
			return fmt.Errorf("%w: invalid expired_behavior %q", ErrCodexPlusConfigInvalid, policy.ExpiredBehavior)
		}
		if strings.TrimSpace(policy.InsufficientBalanceMessage) == "" && strings.TrimSpace(policy.CopyKeys.InsufficientBalanceMessage) == "" {
			return fmt.Errorf("%w: policy[%d].insufficient_balance_message is required", ErrCodexPlusConfigInvalid, i)
		}
	}
	return nil
}

func validCodexPlusExpiredBehavior(value string) bool {
	switch strings.TrimSpace(value) {
	case "block", "degrade", "allow_grace_period":
		return true
	default:
		return false
	}
}
