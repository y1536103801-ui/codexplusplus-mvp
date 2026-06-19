package configregistry

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultUsagePolicyCatalogValid(t *testing.T) {
	catalog := DefaultUsagePolicyCatalog(time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC))

	if err := ValidateUsagePolicyCatalog(catalog); err != nil {
		t.Fatalf("default catalog should be valid: %v", err)
	}
	if got := catalog.Policies[0].RateLimitWindowSeconds; got != defaultUsagePolicyRateWindowSeconds {
		t.Fatalf("default rate window = %d, want %d", got, defaultUsagePolicyRateWindowSeconds)
	}
	if catalog.Policies[0].MonthlyQuota == nil {
		t.Fatal("default monthly quota should be explicit")
	}
}

func TestSampleUsagePolicyCatalogValid(t *testing.T) {
	catalog := SampleUsagePolicyCatalog(time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC))

	if err := ValidateUsagePolicyCatalog(catalog); err != nil {
		t.Fatalf("sample catalog should be valid: %v", err)
	}
	if len(catalog.Policies) != 2 {
		t.Fatalf("sample policy count = %d, want 2", len(catalog.Policies))
	}
}

func TestValidateUsagePolicyCatalogRejectsPolicyErrors(t *testing.T) {
	negativeMonthlyQuota := -1
	tests := []struct {
		name      string
		mutate    func(*UsagePolicyCatalog)
		wantField string
	}{
		{
			name: "duplicate policy_id",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.Policies = append(catalog.Policies, catalog.Policies[0])
			},
			wantField: "policies[1].policy_id",
		},
		{
			name: "negative low balance threshold",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.Policies[0].LowBalanceThreshold = -1
			},
			wantField: "policies[0].low_balance_threshold",
		},
		{
			name: "negative daily quota",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.Policies[0].DailyQuota = -1
			},
			wantField: "policies[0].daily_quota",
		},
		{
			name: "negative monthly quota",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.Policies[0].MonthlyQuota = &negativeMonthlyQuota
			},
			wantField: "policies[0].monthly_quota",
		},
		{
			name: "concurrency limit below one",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.Policies[0].ConcurrencyLimit = 0
			},
			wantField: "policies[0].concurrency_limit",
		},
		{
			name: "rpm limit below one",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.Policies[0].RPMLimit = 0
			},
			wantField: "policies[0].rpm_limit",
		},
		{
			name: "tpm limit below one",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.Policies[0].TPMLimit = 0
			},
			wantField: "policies[0].tpm_limit",
		},
		{
			name: "burst limit below one",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.Policies[0].BurstLimit = 0
			},
			wantField: "policies[0].burst_limit",
		},
		{
			name: "rate limit window below one",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.Policies[0].RateLimitWindowSeconds = 0
			},
			wantField: "policies[0].rate_limit_window_seconds",
		},
		{
			name: "invalid expired behavior",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.Policies[0].ExpiredBehavior = "pause"
			},
			wantField: "policies[0].expired_behavior",
		},
		{
			name: "invalid overage behavior",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.Policies[0].OverageBehavior = "warn"
			},
			wantField: "policies[0].overage_behavior",
		},
		{
			name: "missing copy key",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.Policies[0].CopyKeys.RateLimitedMessage = ""
			},
			wantField: "policies[0].copy_keys.rate_limited_message",
		},
		{
			name: "invalid copy key pattern",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.Policies[0].CopyKeys.PurchaseAction = "Purchase Now"
			},
			wantField: "policies[0].copy_keys.purchase_action",
		},
		{
			name: "invalid applies_to id",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.Policies[0].AppliesTo.PlanIDs = []string{"Starter"}
			},
			wantField: "policies[0].applies_to.plan_ids[0]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			catalog := validUsagePolicyCatalogForTest()
			tt.mutate(&catalog)

			assertValidationField(t, ValidateUsagePolicyCatalog(catalog), tt.wantField)
		})
	}
}

func TestValidateUsagePolicyCatalogRejectsInvalidDevicePolicy(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*UsagePolicyCatalog)
		wantField string
	}{
		{
			name: "max devices below one",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.Policies[0].DevicePolicy.MaxDevicesPerUser = 0
			},
			wantField: "policies[0].device_policy.max_devices_per_user",
		},
		{
			name: "negative replacement cooldown",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.Policies[0].DevicePolicy.ReplacementCooldownHours = -1
			},
			wantField: "policies[0].device_policy.replacement_cooldown_hours",
		},
		{
			name: "empty revoke reason taxonomy",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.Policies[0].DevicePolicy.RevokeReasonTaxonomy = nil
			},
			wantField: "policies[0].device_policy.revoke_reason_taxonomy",
		},
		{
			name: "invalid revoke reason",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.Policies[0].DevicePolicy.RevokeReasonTaxonomy = []string{"stolen"}
			},
			wantField: "policies[0].device_policy.revoke_reason_taxonomy[0]",
		},
		{
			name: "duplicate revoke reason",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.Policies[0].DevicePolicy.RevokeReasonTaxonomy = []string{
					DeviceRevokeReasonUnknown,
					DeviceRevokeReasonUnknown,
				}
			},
			wantField: "policies[0].device_policy.revoke_reason_taxonomy[1]",
		},
		{
			name: "invalid support unlock policy",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.Policies[0].DevicePolicy.SupportUnlockPolicy = "anyone"
			},
			wantField: "policies[0].device_policy.support_unlock_policy",
		},
		{
			name: "invalid revoked behavior",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.Policies[0].DevicePolicy.RevokedBehavior = "warn_only"
			},
			wantField: "policies[0].device_policy.revoked_behavior",
		},
		{
			name: "missing device message key",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.Policies[0].DevicePolicy.MessageKeys.SupportUnlockRequired = ""
			},
			wantField: "policies[0].device_policy.message_keys.support_unlock_required",
		},
		{
			name: "invalid device message key",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.Policies[0].DevicePolicy.MessageKeys.Revoked = "Device Revoked"
			},
			wantField: "policies[0].device_policy.message_keys.revoked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			catalog := validUsagePolicyCatalogForTest()
			tt.mutate(&catalog)

			assertValidationField(t, ValidateUsagePolicyCatalog(catalog), tt.wantField)
		})
	}
}

func TestValidateUsagePolicyCatalogRejectsInvalidMetadata(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*UsagePolicyCatalog)
		wantField string
	}{
		{
			name: "invalid draft status",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.DraftStatus = "draft"
			},
			wantField: "draft_status",
		},
		{
			name: "invalid updated_at",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.UpdatedAt = "not-time"
			},
			wantField: "updated_at",
		},
		{
			name: "missing change reason",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.ChangeReason = " "
			},
			wantField: "change_reason",
		},
		{
			name: "change reason too long",
			mutate: func(catalog *UsagePolicyCatalog) {
				catalog.ChangeReason = strings.Repeat("a", 501)
			},
			wantField: "change_reason",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			catalog := validUsagePolicyCatalogForTest()
			tt.mutate(&catalog)

			assertValidationField(t, ValidateUsagePolicyCatalog(catalog), tt.wantField)
		})
	}
}

func validUsagePolicyCatalogForTest() UsagePolicyCatalog {
	return DefaultUsagePolicyCatalog(time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC))
}

func assertValidationField(t *testing.T, err error, wantField string) {
	t.Helper()
	if err == nil {
		t.Fatalf("ValidateUsagePolicyCatalog returned nil, want field %s", wantField)
	}
	validation, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("error type = %T, want *ValidationError: %v", err, err)
	}
	for _, field := range validation.Fields {
		if field.Field == wantField {
			return
		}
	}
	t.Fatalf("missing validation field %s in %#v", wantField, validation.Fields)
}
