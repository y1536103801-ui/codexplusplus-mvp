package configregistry

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	UsagePolicyDraftStatusEditing        = "editing"
	UsagePolicyDraftStatusReadyForReview = "ready_for_review"
	UsagePolicyDraftStatusApproved       = "approved"
	UsagePolicyDraftStatusPublished      = "published"
	UsagePolicyDraftStatusArchived       = "archived"

	UsagePolicyExpiredBehaviorBlock            = "block"
	UsagePolicyExpiredBehaviorDegrade          = "degrade"
	UsagePolicyExpiredBehaviorAllowGracePeriod = "allow_grace_period"

	UsagePolicyOverageBehaviorBlock            = "block"
	UsagePolicyOverageBehaviorDegrade          = "degrade"
	UsagePolicyOverageBehaviorAllowPaidOverage = "allow_paid_overage"

	DeviceSupportUnlockDisabled       = "disabled"
	DeviceSupportUnlockSupportOnly    = "support_only"
	DeviceSupportUnlockAdminOrSupport = "admin_or_support"

	DeviceRevokedBehaviorBlockBootstrap = "block_bootstrap"
	DeviceRevokedBehaviorUsageViewOnly  = "allow_usage_view_only"
	DeviceRevokeReasonUserRequested     = "user_requested"
	DeviceRevokeReasonAdminRevoked      = "admin_revoked"
	DeviceRevokeReasonLimitExceeded     = "device_limit_exceeded"
	DeviceRevokeReasonRiskControl       = "risk_control"
	DeviceRevokeReasonInactive          = "inactive"
	DeviceRevokeReasonCompromised       = "compromised"
	DeviceRevokeReasonSupportUnlock     = "support_unlock"
	DeviceRevokeReasonUnknown           = "unknown"
	defaultUsagePolicyConfigVersion     = "codexplus-mvp-1"
	defaultUsagePolicyRateWindowSeconds = 60
	defaultUsagePolicyMonthlyQuota      = 3000000
)

var (
	usagePolicyIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,63}$`)
	usageCopyKeyPattern  = regexp.MustCompile(`^[a-z][a-z0-9_.-]{2,127}$`)
)

type UsagePolicyCatalog struct {
	DocumentMeta
	Policies []UsagePolicyRule `json:"policies"`
}

type UsagePolicyRule struct {
	PolicyID               string                  `json:"policy_id"`
	AppliesTo              UsagePolicyAppliesTo    `json:"applies_to"`
	LowBalanceThreshold    int                     `json:"low_balance_threshold"`
	DailyQuota             int                     `json:"daily_quota"`
	MonthlyQuota           *int                    `json:"monthly_quota"`
	ConcurrencyLimit       int                     `json:"concurrency_limit"`
	RPMLimit               int                     `json:"rpm_limit"`
	TPMLimit               int                     `json:"tpm_limit"`
	BurstLimit             int                     `json:"burst_limit"`
	RateLimitWindowSeconds int                     `json:"rate_limit_window_seconds"`
	ExpiredBehavior        string                  `json:"expired_behavior"`
	GracePeriodHours       int                     `json:"grace_period_hours"`
	OverageBehavior        string                  `json:"overage_behavior"`
	CopyKeys               UsagePolicyCopyKeys     `json:"copy_keys"`
	DevicePolicy           UsagePolicyDevicePolicy `json:"device_policy"`
}

type UsagePolicyAppliesTo struct {
	PlanIDs      []string `json:"plan_ids"`
	ModelGroups  []string `json:"model_groups"`
	UserSegments []string `json:"user_segments"`
}

type UsagePolicyCopyKeys struct {
	LowBalanceMessage          string `json:"low_balance_message"`
	InsufficientBalanceMessage string `json:"insufficient_balance_message"`
	RateLimitedMessage         string `json:"rate_limited_message"`
	ExpiredMessage             string `json:"expired_message"`
	RenewAction                string `json:"renew_action"`
	PurchaseAction             string `json:"purchase_action"`
	DeviceRevokedMessage       string `json:"device_revoked_message"`
}

type UsagePolicyDevicePolicy struct {
	RegistrationRequired        bool                         `json:"registration_required"`
	MaxDevicesPerUser           int                          `json:"max_devices_per_user"`
	AllowSelfServiceReplacement bool                         `json:"allow_self_service_replacement"`
	ReplacementCooldownHours    int                          `json:"replacement_cooldown_hours"`
	StrictEnforcementDefault    bool                         `json:"strict_enforcement_default"`
	RevokeReasonTaxonomy        []string                     `json:"revoke_reason_taxonomy"`
	SupportUnlockPolicy         string                       `json:"support_unlock_policy"`
	RevokedBehavior             string                       `json:"revoked_behavior"`
	MessageKeys                 UsagePolicyDeviceMessageKeys `json:"message_keys"`
}

type UsagePolicyDeviceMessageKeys struct {
	LimitReached          string `json:"limit_reached"`
	ReplacementCooldown   string `json:"replacement_cooldown"`
	Revoked               string `json:"revoked"`
	SupportUnlockRequired string `json:"support_unlock_required"`
}

func DefaultUsagePolicyCatalog(now time.Time) UsagePolicyCatalog {
	return UsagePolicyCatalog{
		DocumentMeta: DocumentMeta{
			ConfigVersion: defaultUsagePolicyConfigVersion,
			DraftStatus:   UsagePolicyDraftStatusPublished,
			PublishScope:  PublishScopeProduction,
			RollbackFrom:  nil,
			UpdatedBy:     "system",
			UpdatedAt:     now.UTC().Format(time.RFC3339),
			ChangeReason:  "initial Codex++ usage policy config",
		},
		Policies: []UsagePolicyRule{DefaultUsagePolicyRule()},
	}
}

func SampleUsagePolicyCatalog(now time.Time) UsagePolicyCatalog {
	catalog := DefaultUsagePolicyCatalog(now)
	catalog.ConfigVersion = "codexplus-usage-policy-sample-1"
	catalog.DraftStatus = UsagePolicyDraftStatusEditing
	catalog.PublishScope = PublishScopeDraft
	catalog.ChangeReason = "sample Codex++ usage policy draft for admin review"

	proMonthlyQuota := defaultUsagePolicyMonthlyQuota * 10
	catalog.Policies = append(catalog.Policies, UsagePolicyRule{
		PolicyID: "pro",
		AppliesTo: UsagePolicyAppliesTo{
			PlanIDs:      []string{"pro"},
			ModelGroups:  []string{"default", "premium"},
			UserSegments: []string{"paid"},
		},
		LowBalanceThreshold:    50000,
		DailyQuota:             500000,
		MonthlyQuota:           &proMonthlyQuota,
		ConcurrencyLimit:       4,
		RPMLimit:               120,
		TPMLimit:               200000,
		BurstLimit:             20,
		RateLimitWindowSeconds: defaultUsagePolicyRateWindowSeconds,
		ExpiredBehavior:        UsagePolicyExpiredBehaviorAllowGracePeriod,
		GracePeriodHours:       24,
		OverageBehavior:        UsagePolicyOverageBehaviorAllowPaidOverage,
		CopyKeys:               DefaultUsagePolicyCopyKeys(),
		DevicePolicy: UsagePolicyDevicePolicy{
			RegistrationRequired:        true,
			MaxDevicesPerUser:           3,
			AllowSelfServiceReplacement: true,
			ReplacementCooldownHours:    72,
			StrictEnforcementDefault:    true,
			RevokeReasonTaxonomy:        DefaultDeviceRevokeReasonTaxonomy(),
			SupportUnlockPolicy:         DeviceSupportUnlockAdminOrSupport,
			RevokedBehavior:             DeviceRevokedBehaviorBlockBootstrap,
			MessageKeys:                 DefaultUsagePolicyDeviceMessageKeys(),
		},
	})
	return catalog
}

func DefaultUsagePolicyRule() UsagePolicyRule {
	monthlyQuota := defaultUsagePolicyMonthlyQuota
	return UsagePolicyRule{
		PolicyID: "default",
		AppliesTo: UsagePolicyAppliesTo{
			PlanIDs:      []string{"starter"},
			ModelGroups:  []string{"default"},
			UserSegments: []string{"all"},
		},
		LowBalanceThreshold:    0,
		DailyQuota:             0,
		MonthlyQuota:           &monthlyQuota,
		ConcurrencyLimit:       1,
		RPMLimit:               1,
		TPMLimit:               1000,
		BurstLimit:             1,
		RateLimitWindowSeconds: defaultUsagePolicyRateWindowSeconds,
		ExpiredBehavior:        UsagePolicyExpiredBehaviorBlock,
		GracePeriodHours:       0,
		OverageBehavior:        UsagePolicyOverageBehaviorBlock,
		CopyKeys:               DefaultUsagePolicyCopyKeys(),
		DevicePolicy:           DefaultUsagePolicyDevicePolicy(),
	}
}

func DefaultUsagePolicyCopyKeys() UsagePolicyCopyKeys {
	return UsagePolicyCopyKeys{
		LowBalanceMessage:          "usage.low_balance",
		InsufficientBalanceMessage: "usage.insufficient_balance",
		RateLimitedMessage:         "usage.rate_limited",
		ExpiredMessage:             "usage.expired",
		RenewAction:                "usage.renew_action",
		PurchaseAction:             "usage.purchase_action",
		DeviceRevokedMessage:       "device.revoked",
	}
}

func DefaultUsagePolicyDevicePolicy() UsagePolicyDevicePolicy {
	return UsagePolicyDevicePolicy{
		RegistrationRequired:        true,
		MaxDevicesPerUser:           1,
		AllowSelfServiceReplacement: false,
		ReplacementCooldownHours:    0,
		StrictEnforcementDefault:    false,
		RevokeReasonTaxonomy:        DefaultDeviceRevokeReasonTaxonomy(),
		SupportUnlockPolicy:         DeviceSupportUnlockSupportOnly,
		RevokedBehavior:             DeviceRevokedBehaviorBlockBootstrap,
		MessageKeys:                 DefaultUsagePolicyDeviceMessageKeys(),
	}
}

func DefaultDeviceRevokeReasonTaxonomy() []string {
	return []string{
		DeviceRevokeReasonUserRequested,
		DeviceRevokeReasonAdminRevoked,
		DeviceRevokeReasonLimitExceeded,
		DeviceRevokeReasonRiskControl,
		DeviceRevokeReasonInactive,
		DeviceRevokeReasonCompromised,
		DeviceRevokeReasonSupportUnlock,
		DeviceRevokeReasonUnknown,
	}
}

func DefaultUsagePolicyDeviceMessageKeys() UsagePolicyDeviceMessageKeys {
	return UsagePolicyDeviceMessageKeys{
		LimitReached:          "device.limit_reached",
		ReplacementCooldown:   "device.replacement_cooldown",
		Revoked:               "device.revoked",
		SupportUnlockRequired: "device.support_unlock_required",
	}
}

func ValidateUsagePolicyCatalog(catalog UsagePolicyCatalog) error {
	ve := NewValidation("usage_policy")
	validateUsagePolicyMeta(ve, catalog.DocumentMeta)
	if len(catalog.Policies) == 0 {
		ve.Add("policies", "at least one usage policy is required")
		return ve.Err()
	}

	seen := make(map[string]int, len(catalog.Policies))
	for i, policy := range catalog.Policies {
		validateUsagePolicyRule(ve, i, policy, seen)
	}
	return ve.Err()
}

func validateUsagePolicyMeta(ve *ValidationError, meta DocumentMeta) {
	if strings.TrimSpace(meta.ConfigVersion) == "" {
		ve.Add("config_version", "config_version is required")
	}
	if !ValidUsagePolicyDraftStatus(meta.DraftStatus) {
		ve.Add("draft_status", "draft_status must be editing, ready_for_review, approved, published, or archived")
	}
	if !ValidPublishScope(meta.PublishScope) {
		ve.Add("publish_scope", "publish_scope must be draft, internal, canary, or production")
	}
	if strings.TrimSpace(meta.UpdatedBy) == "" {
		ve.Add("updated_by", "updated_by is required")
	}
	if strings.TrimSpace(meta.UpdatedAt) == "" {
		ve.Add("updated_at", "updated_at is required")
	} else if _, err := time.Parse(time.RFC3339, meta.UpdatedAt); err != nil {
		ve.Add("updated_at", "updated_at must be RFC3339")
	}
	if reason := strings.TrimSpace(meta.ChangeReason); reason == "" {
		ve.Add("change_reason", "change_reason is required")
	} else if len(reason) > 500 {
		ve.Add("change_reason", "change_reason must be at most 500 characters")
	}
}

func validateUsagePolicyRule(ve *ValidationError, index int, policy UsagePolicyRule, seen map[string]int) {
	path := func(field string) string {
		return fmt.Sprintf("policies[%d].%s", index, field)
	}

	id := strings.TrimSpace(policy.PolicyID)
	if id == "" {
		ve.Add(path("policy_id"), "policy_id is required")
	} else {
		if !usagePolicyIDPattern.MatchString(id) {
			ve.Add(path("policy_id"), "policy_id must match ^[a-z][a-z0-9_-]{1,63}$")
		}
		if previous, ok := seen[id]; ok {
			ve.Add(path("policy_id"), fmt.Sprintf("duplicate policy_id %q already used by policies[%d]", id, previous))
		} else {
			seen[id] = index
		}
	}

	validateUsagePolicyAppliesTo(ve, path("applies_to"), policy.AppliesTo)
	validateNonNegative(ve, path("low_balance_threshold"), policy.LowBalanceThreshold)
	validateNonNegative(ve, path("daily_quota"), policy.DailyQuota)
	if policy.MonthlyQuota != nil {
		validateNonNegative(ve, path("monthly_quota"), *policy.MonthlyQuota)
	}
	validateAtLeastOne(ve, path("concurrency_limit"), policy.ConcurrencyLimit)
	validateAtLeastOne(ve, path("rpm_limit"), policy.RPMLimit)
	validateAtLeastOne(ve, path("tpm_limit"), policy.TPMLimit)
	validateAtLeastOne(ve, path("burst_limit"), policy.BurstLimit)
	validateAtLeastOne(ve, path("rate_limit_window_seconds"), policy.RateLimitWindowSeconds)
	if !ValidUsagePolicyExpiredBehavior(policy.ExpiredBehavior) {
		ve.Add(path("expired_behavior"), "expired_behavior must be block, degrade, or allow_grace_period")
	}
	validateNonNegative(ve, path("grace_period_hours"), policy.GracePeriodHours)
	if !ValidUsagePolicyOverageBehavior(policy.OverageBehavior) {
		ve.Add(path("overage_behavior"), "overage_behavior must be block, degrade, or allow_paid_overage")
	}
	validateUsagePolicyCopyKeys(ve, path("copy_keys"), policy.CopyKeys)
	validateUsagePolicyDevicePolicy(ve, path("device_policy"), policy.DevicePolicy)
}

func validateUsagePolicyAppliesTo(ve *ValidationError, prefix string, appliesTo UsagePolicyAppliesTo) {
	validateUsagePolicyIDList(ve, prefix+".plan_ids", appliesTo.PlanIDs)
	validateUsagePolicyIDList(ve, prefix+".model_groups", appliesTo.ModelGroups)
	validateUsagePolicyIDList(ve, prefix+".user_segments", appliesTo.UserSegments)
}

func validateUsagePolicyIDList(ve *ValidationError, field string, values []string) {
	seen := make(map[string]int, len(values))
	for i, value := range values {
		trimmed := strings.TrimSpace(value)
		itemField := fmt.Sprintf("%s[%d]", field, i)
		if trimmed == "" {
			ve.Add(itemField, "value is required")
			continue
		}
		if !usagePolicyIDPattern.MatchString(trimmed) {
			ve.Add(itemField, "value must match ^[a-z][a-z0-9_-]{1,63}$")
		}
		if previous, ok := seen[trimmed]; ok {
			ve.Add(itemField, fmt.Sprintf("duplicate value %q already used at %s[%d]", trimmed, field, previous))
		} else {
			seen[trimmed] = i
		}
	}
}

func validateUsagePolicyCopyKeys(ve *ValidationError, prefix string, keys UsagePolicyCopyKeys) {
	validateCopyKey(ve, prefix+".low_balance_message", keys.LowBalanceMessage)
	validateCopyKey(ve, prefix+".insufficient_balance_message", keys.InsufficientBalanceMessage)
	validateCopyKey(ve, prefix+".rate_limited_message", keys.RateLimitedMessage)
	validateCopyKey(ve, prefix+".expired_message", keys.ExpiredMessage)
	validateCopyKey(ve, prefix+".renew_action", keys.RenewAction)
	validateCopyKey(ve, prefix+".purchase_action", keys.PurchaseAction)
	validateCopyKey(ve, prefix+".device_revoked_message", keys.DeviceRevokedMessage)
}

func validateUsagePolicyDevicePolicy(ve *ValidationError, prefix string, policy UsagePolicyDevicePolicy) {
	validateAtLeastOne(ve, prefix+".max_devices_per_user", policy.MaxDevicesPerUser)
	validateNonNegative(ve, prefix+".replacement_cooldown_hours", policy.ReplacementCooldownHours)

	if len(policy.RevokeReasonTaxonomy) == 0 {
		ve.Add(prefix+".revoke_reason_taxonomy", "at least one revoke reason is required")
	}
	seen := make(map[string]int, len(policy.RevokeReasonTaxonomy))
	for i, reason := range policy.RevokeReasonTaxonomy {
		trimmed := strings.TrimSpace(reason)
		field := fmt.Sprintf("%s.revoke_reason_taxonomy[%d]", prefix, i)
		if !ValidDeviceRevokeReason(trimmed) {
			ve.Add(field, "revoke reason is not allowed")
			continue
		}
		if previous, ok := seen[trimmed]; ok {
			ve.Add(field, fmt.Sprintf("duplicate revoke reason %q already used at %s.revoke_reason_taxonomy[%d]", trimmed, prefix, previous))
		} else {
			seen[trimmed] = i
		}
	}

	if !ValidDeviceSupportUnlockPolicy(policy.SupportUnlockPolicy) {
		ve.Add(prefix+".support_unlock_policy", "support_unlock_policy must be disabled, support_only, or admin_or_support")
	}
	if !ValidDeviceRevokedBehavior(policy.RevokedBehavior) {
		ve.Add(prefix+".revoked_behavior", "revoked_behavior must be block_bootstrap or allow_usage_view_only")
	}
	validateUsagePolicyDeviceMessageKeys(ve, prefix+".message_keys", policy.MessageKeys)
}

func validateUsagePolicyDeviceMessageKeys(ve *ValidationError, prefix string, keys UsagePolicyDeviceMessageKeys) {
	validateCopyKey(ve, prefix+".limit_reached", keys.LimitReached)
	validateCopyKey(ve, prefix+".replacement_cooldown", keys.ReplacementCooldown)
	validateCopyKey(ve, prefix+".revoked", keys.Revoked)
	validateCopyKey(ve, prefix+".support_unlock_required", keys.SupportUnlockRequired)
}

func validateNonNegative(ve *ValidationError, field string, value int) {
	if value < 0 {
		ve.Add(field, "must be greater than or equal to 0")
	}
}

func validateAtLeastOne(ve *ValidationError, field string, value int) {
	if value < 1 {
		ve.Add(field, "must be greater than or equal to 1")
	}
}

func validateCopyKey(ve *ValidationError, field, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		ve.Add(field, "copy key is required")
		return
	}
	if !usageCopyKeyPattern.MatchString(trimmed) {
		ve.Add(field, "copy key must match ^[a-z][a-z0-9_.-]{2,127}$")
	}
}

func ValidUsagePolicyDraftStatus(value string) bool {
	switch strings.TrimSpace(value) {
	case UsagePolicyDraftStatusEditing,
		UsagePolicyDraftStatusReadyForReview,
		UsagePolicyDraftStatusApproved,
		UsagePolicyDraftStatusPublished,
		UsagePolicyDraftStatusArchived:
		return true
	default:
		return false
	}
}

func ValidUsagePolicyExpiredBehavior(value string) bool {
	switch strings.TrimSpace(value) {
	case UsagePolicyExpiredBehaviorBlock,
		UsagePolicyExpiredBehaviorDegrade,
		UsagePolicyExpiredBehaviorAllowGracePeriod:
		return true
	default:
		return false
	}
}

func ValidUsagePolicyOverageBehavior(value string) bool {
	switch strings.TrimSpace(value) {
	case UsagePolicyOverageBehaviorBlock,
		UsagePolicyOverageBehaviorDegrade,
		UsagePolicyOverageBehaviorAllowPaidOverage:
		return true
	default:
		return false
	}
}

func ValidDeviceSupportUnlockPolicy(value string) bool {
	switch strings.TrimSpace(value) {
	case DeviceSupportUnlockDisabled,
		DeviceSupportUnlockSupportOnly,
		DeviceSupportUnlockAdminOrSupport:
		return true
	default:
		return false
	}
}

func ValidDeviceRevokedBehavior(value string) bool {
	switch strings.TrimSpace(value) {
	case DeviceRevokedBehaviorBlockBootstrap,
		DeviceRevokedBehaviorUsageViewOnly:
		return true
	default:
		return false
	}
}

func ValidDeviceRevokeReason(value string) bool {
	switch strings.TrimSpace(value) {
	case DeviceRevokeReasonUserRequested,
		DeviceRevokeReasonAdminRevoked,
		DeviceRevokeReasonLimitExceeded,
		DeviceRevokeReasonRiskControl,
		DeviceRevokeReasonInactive,
		DeviceRevokeReasonCompromised,
		DeviceRevokeReasonSupportUnlock,
		DeviceRevokeReasonUnknown:
		return true
	default:
		return false
	}
}
