package configregistry

import (
	"errors"
	"strings"
	"testing"
)

func TestDefaultPlanCatalogValidAndCoversContractFields(t *testing.T) {
	catalog := DefaultPlanCatalog()
	if err := ValidatePlanCatalog(catalog); err != nil {
		t.Fatalf("default plan catalog should be valid: %v", err)
	}

	if len(catalog.Plans) != 1 {
		t.Fatalf("expected one default plan, got %d", len(catalog.Plans))
	}
	plan := catalog.Plans[0]
	if plan.PriceAmountMinor != 1999 || plan.DisplayPrice == "" {
		t.Fatalf("default plan must carry backend price and display_price, got %d %q", plan.PriceAmountMinor, plan.DisplayPrice)
	}
	if plan.BillingPeriod != BillingPeriodMonthly {
		t.Fatalf("expected monthly billing period, got %q", plan.BillingPeriod)
	}
	if plan.EntitlementGrant.BalanceCredit <= 0 || plan.EntitlementGrant.DurationDays <= 0 || plan.EntitlementGrant.DailyQuota == nil || plan.EntitlementGrant.PeriodQuota == nil {
		t.Fatalf("default plan must include balance, duration, daily quota, and period quota: %+v", plan.EntitlementGrant)
	}
	if len(plan.EntitlementSources.SubscriptionGroupIDs) == 0 || len(plan.EntitlementSources.APIKeyGroupIDs) == 0 || len(plan.EntitlementSources.GroupNames) == 0 {
		t.Fatalf("default plan must include all entitlement source families: %+v", plan.EntitlementSources)
	}
	if len(plan.ModelGroups) == 0 || plan.UsagePolicyID == "" {
		t.Fatalf("default plan must link model groups and usage policy: %+v", plan)
	}
	if plan.PurchaseURL == nil || plan.RenewURL == nil {
		t.Fatalf("default plan must include commerce URLs")
	}
	if plan.CopyKeys.PurchaseAction == "" || plan.CopyKeys.RenewAction == "" || plan.CopyKeys.UpgradeAction == "" ||
		plan.CopyKeys.NotPurchasedMessage == "" || plan.CopyKeys.ExpiredMessage == "" || plan.CopyKeys.LowBalanceMessage == "" {
		t.Fatalf("default plan must include every client copy key: %+v", plan.CopyKeys)
	}
	if !plan.IsListed || plan.Status != PlanStatusActive {
		t.Fatalf("default plan should be listed and active, got listed=%v status=%q", plan.IsListed, plan.Status)
	}
}

func TestValidatePlanCatalogAcceptsFrozenDraftStatuses(t *testing.T) {
	for _, status := range []string{
		PlanCatalogDraftStatusEditing,
		PlanCatalogDraftStatusReadyForReview,
		PlanCatalogDraftStatusApproved,
		PlanCatalogDraftStatusPublished,
		PlanCatalogDraftStatusArchived,
	} {
		t.Run(status, func(t *testing.T) {
			catalog := DefaultPlanCatalog()
			catalog.DraftStatus = status
			if err := ValidatePlanCatalog(catalog); err != nil {
				t.Fatalf("draft status %q should be valid: %v", status, err)
			}
		})
	}
}

func TestValidatePlanCatalogRejectsContractViolations(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*PlanCatalog)
		field  string
	}{
		{
			name: "duplicate plan_id",
			mutate: func(catalog *PlanCatalog) {
				catalog.Plans = append(catalog.Plans, catalog.Plans[0])
			},
			field: "plans[1].plan_id",
		},
		{
			name: "negative price",
			mutate: func(catalog *PlanCatalog) {
				catalog.Plans[0].PriceAmountMinor = -1
			},
			field: "price_amount_minor",
		},
		{
			name: "missing display_price",
			mutate: func(catalog *PlanCatalog) {
				catalog.Plans[0].DisplayPrice = " "
			},
			field: "display_price",
		},
		{
			name: "invalid billing_period",
			mutate: func(catalog *PlanCatalog) {
				catalog.Plans[0].BillingPeriod = "weekly"
			},
			field: "billing_period",
		},
		{
			name: "negative entitlement grant",
			mutate: func(catalog *PlanCatalog) {
				catalog.Plans[0].EntitlementGrant.BalanceCredit = -1
			},
			field: "entitlement_grant.balance_credit",
		},
		{
			name: "invalid entitlement source",
			mutate: func(catalog *PlanCatalog) {
				catalog.Plans[0].EntitlementSources.APIKeyGroupIDs = []int{0}
			},
			field: "api_key_group_ids[0]",
		},
		{
			name: "empty model_groups",
			mutate: func(catalog *PlanCatalog) {
				catalog.Plans[0].ModelGroups = nil
			},
			field: "model_groups",
		},
		{
			name: "missing usage_policy_id",
			mutate: func(catalog *PlanCatalog) {
				catalog.Plans[0].UsagePolicyID = ""
			},
			field: "usage_policy_id",
		},
		{
			name: "invalid purchase url",
			mutate: func(catalog *PlanCatalog) {
				badURL := "not-a-url"
				catalog.Plans[0].PurchaseURL = &badURL
			},
			field: "purchase_url",
		},
		{
			name: "missing copy key",
			mutate: func(catalog *PlanCatalog) {
				catalog.Plans[0].CopyKeys.PurchaseAction = ""
			},
			field: "copy_keys.purchase_action",
		},
		{
			name: "invalid status",
			mutate: func(catalog *PlanCatalog) {
				catalog.Plans[0].Status = "retired"
			},
			field: "status",
		},
		{
			name: "hidden plan cannot be listed",
			mutate: func(catalog *PlanCatalog) {
				catalog.Plans[0].Status = PlanStatusHidden
				catalog.Plans[0].PurchaseURL = nil
			},
			field: "is_listed",
		},
		{
			name: "unlisted plan cannot be purchased",
			mutate: func(catalog *PlanCatalog) {
				catalog.Plans[0].IsListed = false
			},
			field: "purchase_url",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			catalog := DefaultPlanCatalog()
			test.mutate(&catalog)
			assertPlanValidationField(t, ValidatePlanCatalog(catalog), test.field)
		})
	}
}

func assertPlanValidationField(t *testing.T, err error, expectedField string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected validation error for %s", expectedField)
	}
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	for _, field := range validation.Fields {
		if strings.Contains(field.Field, expectedField) {
			return
		}
	}
	t.Fatalf("expected validation field containing %q, got %+v", expectedField, validation.Fields)
}
