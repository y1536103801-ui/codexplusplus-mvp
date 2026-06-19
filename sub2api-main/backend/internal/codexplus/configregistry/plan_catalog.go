package configregistry

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	PlanCatalogDraftStatusEditing        = "editing"
	PlanCatalogDraftStatusReadyForReview = "ready_for_review"
	PlanCatalogDraftStatusApproved       = "approved"
	PlanCatalogDraftStatusPublished      = "published"
	PlanCatalogDraftStatusArchived       = "archived"

	BillingPeriodNone      = "none"
	BillingPeriodTrial     = "trial"
	BillingPeriodMonthly   = "monthly"
	BillingPeriodQuarterly = "quarterly"
	BillingPeriodYearly    = "yearly"
	BillingPeriodOneTime   = "one_time"

	PlanStatusActive   = "active"
	PlanStatusHidden   = "hidden"
	PlanStatusDisabled = "disabled"
)

var (
	planIDPattern            = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,63}$`)
	modelGroupPattern        = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,63}$`)
	planUsagePolicyIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,63}$`)
	currencyPattern          = regexp.MustCompile(`^[A-Z]{3}$`)
	copyKeyPattern           = regexp.MustCompile(`^[a-z][a-z0-9_.-]{2,127}$`)
)

type PlanCatalog struct {
	DocumentMeta
	Plans []PlanCatalogPlan `json:"plans"`
}

type PlanCatalogPlan struct {
	PlanID              string                  `json:"plan_id"`
	Name                string                  `json:"name"`
	Description         string                  `json:"description"`
	BillingPeriod       string                  `json:"billing_period"`
	Currency            string                  `json:"currency"`
	PriceAmountMinor    int                     `json:"price_amount_minor"`
	DisplayPrice        string                  `json:"display_price"`
	EntitlementGrant    PlanEntitlementGrant    `json:"entitlement_grant"`
	EntitlementSources  PlanEntitlementSources  `json:"entitlement_sources"`
	ModelGroups         []string                `json:"model_groups"`
	UsagePolicyID       string                  `json:"usage_policy_id"`
	PurchaseURL         *string                 `json:"purchase_url"`
	RenewURL            *string                 `json:"renew_url"`
	CopyKeys            PlanCopyKeys            `json:"copy_keys"`
	IsListed            bool                    `json:"is_listed"`
	Status              string                  `json:"status"`
	SortOrder           int                     `json:"sort_order"`
	ExternalBillingRefs PlanExternalBillingRefs `json:"external_billing_refs"`
}

type PlanEntitlementGrant struct {
	BalanceCredit int  `json:"balance_credit"`
	DurationDays  int  `json:"duration_days"`
	DailyQuota    *int `json:"daily_quota"`
	PeriodQuota   *int `json:"period_quota,omitempty"`
}

type PlanEntitlementSources struct {
	SubscriptionGroupIDs []int    `json:"subscription_group_ids"`
	APIKeyGroupIDs       []int    `json:"api_key_group_ids"`
	GroupNames           []string `json:"group_names"`
}

type PlanCopyKeys struct {
	PurchaseAction      string `json:"purchase_action"`
	RenewAction         string `json:"renew_action"`
	UpgradeAction       string `json:"upgrade_action"`
	NotPurchasedMessage string `json:"not_purchased_message"`
	ExpiredMessage      string `json:"expired_message"`
	LowBalanceMessage   string `json:"low_balance_message"`
}

type PlanExternalBillingRefs struct {
	ProductID *string `json:"product_id"`
	SKUID     *string `json:"sku_id"`
}

func DefaultPlanCatalog() PlanCatalog {
	dailyQuota := 500
	periodQuota := 15000
	purchaseURL := "https://codex-plus.local/billing/plans/pro-monthly/purchase"
	renewURL := "https://codex-plus.local/billing/plans/pro-monthly/renew"
	productID := "codex-plus-pro"
	skuID := "pro-monthly"

	return PlanCatalog{
		DocumentMeta: DocumentMeta{
			ConfigVersion: "plan-catalog.default.v1",
			DraftStatus:   PlanCatalogDraftStatusPublished,
			PublishScope:  PublishScopeProduction,
			RollbackFrom:  nil,
			UpdatedBy:     "system",
			UpdatedAt:     "2026-01-01T00:00:00Z",
			ChangeReason:  "default plan catalog seed",
		},
		Plans: []PlanCatalogPlan{
			{
				PlanID:           "pro_monthly",
				Name:             "Codex++ Pro Monthly",
				Description:      "Monthly Codex++ managed access with configured model groups and quota policy.",
				BillingPeriod:    BillingPeriodMonthly,
				Currency:         "USD",
				PriceAmountMinor: 1999,
				DisplayPrice:     "$19.99/month",
				EntitlementGrant: PlanEntitlementGrant{
					BalanceCredit: 2000,
					DurationDays:  30,
					DailyQuota:    &dailyQuota,
					PeriodQuota:   &periodQuota,
				},
				EntitlementSources: PlanEntitlementSources{
					SubscriptionGroupIDs: []int{101},
					APIKeyGroupIDs:       []int{201},
					GroupNames:           []string{"codex-plus-pro"},
				},
				ModelGroups:   []string{"codex_standard", "codex_premium"},
				UsagePolicyID: "pro_monthly_policy",
				PurchaseURL:   &purchaseURL,
				RenewURL:      &renewURL,
				CopyKeys: PlanCopyKeys{
					PurchaseAction:      "billing.action.purchase",
					RenewAction:         "billing.action.renew",
					UpgradeAction:       "billing.action.upgrade",
					NotPurchasedMessage: "billing.message.not_purchased",
					ExpiredMessage:      "billing.message.expired",
					LowBalanceMessage:   "billing.message.low_balance",
				},
				IsListed:  true,
				Status:    PlanStatusActive,
				SortOrder: 10,
				ExternalBillingRefs: PlanExternalBillingRefs{
					ProductID: &productID,
					SKUID:     &skuID,
				},
			},
		},
	}
}

func ValidatePlanCatalog(catalog PlanCatalog) error {
	ve := NewValidation("plan_catalog")
	validatePlanCatalogMeta(ve, catalog.DocumentMeta)

	if len(catalog.Plans) == 0 {
		ve.Add("plans", "plans must contain at least one plan")
		return ve.Err()
	}

	seenPlanIDs := make(map[string]int, len(catalog.Plans))
	for index, plan := range catalog.Plans {
		field := func(name string) string {
			return "plans[" + strconv.Itoa(index) + "]." + name
		}

		planID := strings.TrimSpace(plan.PlanID)
		if !planIDPattern.MatchString(planID) {
			ve.Add(field("plan_id"), "plan_id must match ^[a-z][a-z0-9_-]{1,63}$")
		} else if firstIndex, ok := seenPlanIDs[planID]; ok {
			ve.Add(field("plan_id"), "plan_id duplicates plans["+strconv.Itoa(firstIndex)+"].plan_id")
		} else {
			seenPlanIDs[planID] = index
		}

		if trimmedLen(plan.Name) == 0 || trimmedLen(plan.Name) > 80 {
			ve.Add(field("name"), "name is required and must be at most 80 characters")
		}
		if trimmedLen(plan.Description) == 0 || trimmedLen(plan.Description) > 400 {
			ve.Add(field("description"), "description is required and must be at most 400 characters")
		}
		if !ValidBillingPeriod(plan.BillingPeriod) {
			ve.Add(field("billing_period"), "billing_period must be none, trial, monthly, quarterly, yearly, or one_time")
		}
		if !currencyPattern.MatchString(strings.TrimSpace(plan.Currency)) {
			ve.Add(field("currency"), "currency must be an ISO 4217 uppercase code")
		}
		if plan.PriceAmountMinor < 0 {
			ve.Add(field("price_amount_minor"), "price_amount_minor must be >= 0")
		}
		if strings.TrimSpace(plan.DisplayPrice) == "" {
			ve.Add(field("display_price"), "display_price is required and must be backend supplied")
		}

		validateEntitlementGrant(ve, field("entitlement_grant"), plan.EntitlementGrant)
		validateEntitlementSources(ve, field("entitlement_sources"), plan.EntitlementSources)
		validateModelGroups(ve, field("model_groups"), plan.ModelGroups)

		if !planUsagePolicyIDPattern.MatchString(strings.TrimSpace(plan.UsagePolicyID)) {
			ve.Add(field("usage_policy_id"), "usage_policy_id is required and must match ^[a-z][a-z0-9_-]{1,63}$")
		}
		validateCommerceURL(ve, field("purchase_url"), plan.PurchaseURL)
		validateCommerceURL(ve, field("renew_url"), plan.RenewURL)
		validateCopyKeys(ve, field("copy_keys"), plan.CopyKeys)

		if !ValidPlanStatus(plan.Status) {
			ve.Add(field("status"), "status must be active, hidden, or disabled")
		}
		if plan.SortOrder < 0 {
			ve.Add(field("sort_order"), "sort_order must be >= 0")
		}
		if plan.Status != PlanStatusActive && plan.IsListed {
			ve.Add(field("is_listed"), "only active plans can be listed")
		}
		if plan.PurchaseURL != nil && (!plan.IsListed || plan.Status != PlanStatusActive) {
			ve.Add(field("purchase_url"), "unlisted, hidden, or disabled plans cannot be purchased")
		}
		if plan.Status == PlanStatusDisabled && plan.RenewURL != nil {
			ve.Add(field("renew_url"), "disabled plans cannot be renewed")
		}
	}

	return ve.Err()
}

func ValidBillingPeriod(value string) bool {
	switch strings.TrimSpace(value) {
	case BillingPeriodNone, BillingPeriodTrial, BillingPeriodMonthly, BillingPeriodQuarterly, BillingPeriodYearly, BillingPeriodOneTime:
		return true
	default:
		return false
	}
}

func ValidPlanStatus(value string) bool {
	switch strings.TrimSpace(value) {
	case PlanStatusActive, PlanStatusHidden, PlanStatusDisabled:
		return true
	default:
		return false
	}
}

func validatePlanCatalogMeta(ve *ValidationError, meta DocumentMeta) {
	if strings.TrimSpace(meta.ConfigVersion) == "" {
		ve.Add("config_version", "config_version is required")
	}
	if !validPlanCatalogDraftStatus(meta.DraftStatus) {
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
	changeReasonLen := trimmedLen(meta.ChangeReason)
	if changeReasonLen == 0 || changeReasonLen > 500 {
		ve.Add("change_reason", "change_reason is required and must be at most 500 characters")
	}
}

func validPlanCatalogDraftStatus(value string) bool {
	switch strings.TrimSpace(value) {
	case PlanCatalogDraftStatusEditing, PlanCatalogDraftStatusReadyForReview, PlanCatalogDraftStatusApproved, PlanCatalogDraftStatusPublished, PlanCatalogDraftStatusArchived:
		return true
	default:
		return false
	}
}

func validateEntitlementGrant(ve *ValidationError, field string, grant PlanEntitlementGrant) {
	if grant.BalanceCredit < 0 {
		ve.Add(field+".balance_credit", "balance_credit must be >= 0")
	}
	if grant.DurationDays < 0 {
		ve.Add(field+".duration_days", "duration_days must be >= 0")
	}
	if grant.DailyQuota != nil && *grant.DailyQuota < 0 {
		ve.Add(field+".daily_quota", "daily_quota must be null or >= 0")
	}
	if grant.PeriodQuota != nil && *grant.PeriodQuota < 0 {
		ve.Add(field+".period_quota", "period_quota must be null or >= 0")
	}
}

func validateEntitlementSources(ve *ValidationError, field string, sources PlanEntitlementSources) {
	validatePositiveUniqueInts(ve, field+".subscription_group_ids", sources.SubscriptionGroupIDs)
	validatePositiveUniqueInts(ve, field+".api_key_group_ids", sources.APIKeyGroupIDs)

	seen := make(map[string]int, len(sources.GroupNames))
	for index, name := range sources.GroupNames {
		trimmed := strings.TrimSpace(name)
		itemField := field + ".group_names[" + strconv.Itoa(index) + "]"
		if trimmed == "" {
			ve.Add(itemField, "group_names entries must be non-empty")
			continue
		}
		if firstIndex, ok := seen[trimmed]; ok {
			ve.Add(itemField, "group_names entry duplicates index "+strconv.Itoa(firstIndex))
			continue
		}
		seen[trimmed] = index
	}
}

func validateModelGroups(ve *ValidationError, field string, groups []string) {
	if len(groups) == 0 {
		ve.Add(field, "model_groups must contain at least one group")
		return
	}

	seen := make(map[string]int, len(groups))
	for index, group := range groups {
		trimmed := strings.TrimSpace(group)
		itemField := field + "[" + strconv.Itoa(index) + "]"
		if !modelGroupPattern.MatchString(trimmed) {
			ve.Add(itemField, "model_groups entries must match ^[a-z][a-z0-9_-]{1,63}$")
			continue
		}
		if firstIndex, ok := seen[trimmed]; ok {
			ve.Add(itemField, "model_groups entry duplicates index "+strconv.Itoa(firstIndex))
			continue
		}
		seen[trimmed] = index
	}
}

func validatePositiveUniqueInts(ve *ValidationError, field string, values []int) {
	seen := make(map[int]int, len(values))
	for index, value := range values {
		itemField := field + "[" + strconv.Itoa(index) + "]"
		if value < 1 {
			ve.Add(itemField, "entitlement source ids must be >= 1")
			continue
		}
		if firstIndex, ok := seen[value]; ok {
			ve.Add(itemField, "entitlement source id duplicates index "+strconv.Itoa(firstIndex))
			continue
		}
		seen[value] = index
	}
}

func validateCommerceURL(ve *ValidationError, field string, raw *string) {
	if raw == nil {
		return
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		ve.Add(field, "commerce URL must be null or an absolute URI")
		return
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		ve.Add(field, "commerce URL must be null or an absolute URI")
	}
}

func validateCopyKeys(ve *ValidationError, field string, keys PlanCopyKeys) {
	validatePlanCopyKey(ve, field+".purchase_action", keys.PurchaseAction)
	validatePlanCopyKey(ve, field+".renew_action", keys.RenewAction)
	validatePlanCopyKey(ve, field+".upgrade_action", keys.UpgradeAction)
	validatePlanCopyKey(ve, field+".not_purchased_message", keys.NotPurchasedMessage)
	validatePlanCopyKey(ve, field+".expired_message", keys.ExpiredMessage)
	validatePlanCopyKey(ve, field+".low_balance_message", keys.LowBalanceMessage)
}

func validatePlanCopyKey(ve *ValidationError, field string, value string) {
	if !copyKeyPattern.MatchString(strings.TrimSpace(value)) {
		ve.Add(field, "copy key is required and must match ^[a-z][a-z0-9_.-]{2,127}$")
	}
}

func trimmedLen(value string) int {
	return len(strings.TrimSpace(value))
}
