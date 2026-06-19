package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/internal/payment"
)

const (
	codexPlusCommerceEntitlementAuditGranted      = "CODEXPLUS_ENTITLEMENT_GRANTED"
	codexPlusCommerceEntitlementAuditAlreadyGrant = "CODEXPLUS_ENTITLEMENT_ALREADY_GRANTED"
	codexPlusCommerceEntitlementAuditUnmapped     = "CODEXPLUS_ENTITLEMENT_UNMAPPED"
	codexPlusCommerceEntitlementAuditConfigError  = "CODEXPLUS_ENTITLEMENT_CONFIG_ERROR"
)

type CodexPlusCommerceEntitlementService struct {
	configReader   CodexPlusClientConfigReader
	groupRepo      GroupRepository
	subscription   *SubscriptionService
	entClient      *dbent.Client
	audit          func(context.Context, int64, string, string, map[string]any)
	unmappedLogged map[int64]struct{}
}

type CodexPlusCommerceEntitlement struct {
	ConfigVersion string
	CodexPlanID   string
	UsagePolicyID string
	ModelGroups   []string

	OrderID       int64
	UserID        int64
	PaymentPlanID int64
	GroupID       int64
	GroupName     string
	ValidityDays  int
}

func NewCodexPlusCommerceEntitlementService(configReader CodexPlusClientConfigReader, groupRepo GroupRepository, subscription *SubscriptionService, entClient *dbent.Client) *CodexPlusCommerceEntitlementService {
	return &CodexPlusCommerceEntitlementService{
		configReader:   configReader,
		groupRepo:      groupRepo,
		subscription:   subscription,
		entClient:      entClient,
		unmappedLogged: make(map[int64]struct{}),
	}
}

func (s *CodexPlusCommerceEntitlementService) SetAuditSink(audit func(context.Context, int64, string, string, map[string]any)) {
	if s != nil {
		s.audit = audit
	}
}

func (s *CodexPlusCommerceEntitlementService) ResolveSubscriptionOrder(ctx context.Context, order *dbent.PaymentOrder) (*CodexPlusCommerceEntitlement, error) {
	if s == nil || order == nil || order.OrderType != payment.OrderTypeSubscription {
		return nil, nil
	}
	groupID, paymentPlan, err := s.paymentPlanBinding(ctx, order)
	if err != nil {
		return nil, err
	}
	if groupID <= 0 {
		return nil, nil
	}

	group, _ := s.loadGroup(ctx, groupID)
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		s.writeAudit(ctx, order.ID, codexPlusCommerceEntitlementAuditConfigError, "system", map[string]any{
			"error": err.Error(),
		})
		return nil, nil
	}
	if cfg == nil {
		return nil, nil
	}
	plan := codexPlusCommercePlanForBinding(cfg, groupID, group, paymentPlan)
	if plan == nil {
		s.writeUnmappedAudit(ctx, order, groupID, group, paymentPlan)
		return nil, nil
	}

	return &CodexPlusCommerceEntitlement{
		ConfigVersion: cfg.ConfigVersion,
		CodexPlanID:   strings.TrimSpace(plan.PlanID),
		UsagePolicyID: strings.TrimSpace(plan.UsagePolicyID),
		ModelGroups:   append([]string(nil), plan.ModelGroups...),
		OrderID:       order.ID,
		UserID:        order.UserID,
		PaymentPlanID: codexPlusCommercePaymentPlanID(order, paymentPlan),
		GroupID:       groupID,
		GroupName:     codexPlusGroupName(group),
		ValidityDays:  codexPlusCommerceOrderValidityDays(order),
	}, nil
}

func (s *CodexPlusCommerceEntitlementService) AlreadyGranted(ctx context.Context, order *dbent.PaymentOrder, entitlement *CodexPlusCommerceEntitlement) bool {
	if order == nil {
		return false
	}
	if s.hasAudit(ctx, order.ID, codexPlusCommerceEntitlementAuditGranted) {
		return true
	}
	if s.hasAudit(ctx, order.ID, "SUBSCRIPTION_SUCCESS") {
		return true
	}
	groupID := codexPlusCommerceOrderGroupID(order)
	if entitlement != nil && entitlement.GroupID > 0 {
		groupID = entitlement.GroupID
	}
	if groupID <= 0 || s.subscription == nil || s.subscription.userSubRepo == nil {
		return false
	}
	sub, err := s.subscription.userSubRepo.GetByUserIDAndGroupID(ctx, order.UserID, groupID)
	if err != nil || sub == nil {
		return false
	}
	return codexPlusCommerceSubscriptionHasOrderMarker(sub, order.ID)
}

func (s *CodexPlusCommerceEntitlementService) GrantNote(order *dbent.PaymentOrder, entitlement *CodexPlusCommerceEntitlement) string {
	if entitlement == nil || strings.TrimSpace(entitlement.CodexPlanID) == "" {
		return codexPlusCommerceLegacyOrderNote(order)
	}
	return fmt.Sprintf("codexplus entitlement order %d plan %s", order.ID, entitlement.CodexPlanID)
}

func (s *CodexPlusCommerceEntitlementService) RecordGrant(ctx context.Context, order *dbent.PaymentOrder, entitlement *CodexPlusCommerceEntitlement, sub *UserSubscription, reused bool) {
	if entitlement == nil || order == nil {
		return
	}
	detail := entitlement.auditDetail()
	detail["reused"] = reused
	if sub != nil {
		detail["subscription_id"] = sub.ID
		detail["subscription_expires_at"] = sub.ExpiresAt.UTC().Format(time.RFC3339)
	}
	s.writeAudit(ctx, order.ID, codexPlusCommerceEntitlementAuditGranted, "system", detail)
}

func (s *CodexPlusCommerceEntitlementService) RecordAlreadyGranted(ctx context.Context, order *dbent.PaymentOrder, entitlement *CodexPlusCommerceEntitlement) {
	if order == nil {
		return
	}
	detail := map[string]any{"reason": "order entitlement already applied"}
	if entitlement != nil {
		for key, value := range entitlement.auditDetail() {
			detail[key] = value
		}
	}
	s.writeAudit(ctx, order.ID, codexPlusCommerceEntitlementAuditAlreadyGrant, "system", detail)
}

func (e *CodexPlusCommerceEntitlement) auditDetail() map[string]any {
	if e == nil {
		return map[string]any{}
	}
	return map[string]any{
		"config_version":  e.ConfigVersion,
		"codex_plan_id":   e.CodexPlanID,
		"usage_policy_id": e.UsagePolicyID,
		"model_groups":    append([]string(nil), e.ModelGroups...),
		"payment_plan_id": e.PaymentPlanID,
		"group_id":        e.GroupID,
		"group_name":      e.GroupName,
		"validity_days":   e.ValidityDays,
	}
}

func (s *CodexPlusCommerceEntitlementService) paymentPlanBinding(ctx context.Context, order *dbent.PaymentOrder) (int64, *dbent.SubscriptionPlan, error) {
	groupID := codexPlusCommerceOrderGroupID(order)
	if groupID > 0 || order == nil || order.PlanID == nil || *order.PlanID <= 0 || s == nil || s.entClient == nil {
		return groupID, nil, nil
	}
	plan, err := s.entClient.SubscriptionPlan.Get(ctx, *order.PlanID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return 0, nil, nil
		}
		return 0, nil, err
	}
	return plan.GroupID, plan, nil
}

func (s *CodexPlusCommerceEntitlementService) loadGroup(ctx context.Context, groupID int64) (*Group, error) {
	if s == nil || s.groupRepo == nil || groupID <= 0 {
		return nil, nil
	}
	return s.groupRepo.GetByID(ctx, groupID)
}

func (s *CodexPlusCommerceEntitlementService) loadConfig(ctx context.Context) (*CodexPlusConfig, error) {
	if s == nil || s.configReader == nil {
		return nil, nil
	}
	return s.configReader.Get(ctx)
}

func (s *CodexPlusCommerceEntitlementService) writeUnmappedAudit(ctx context.Context, order *dbent.PaymentOrder, groupID int64, group *Group, paymentPlan *dbent.SubscriptionPlan) {
	if s == nil || order == nil {
		return
	}
	if _, ok := s.unmappedLogged[order.ID]; ok {
		return
	}
	s.unmappedLogged[order.ID] = struct{}{}
	s.writeAudit(ctx, order.ID, codexPlusCommerceEntitlementAuditUnmapped, "system", map[string]any{
		"payment_plan_id": codexPlusCommercePaymentPlanID(order, paymentPlan),
		"group_id":        groupID,
		"group_name":      codexPlusGroupName(group),
		"reason":          "no active Codex++ plan matches payment subscription group",
	})
}

func (s *CodexPlusCommerceEntitlementService) hasAudit(ctx context.Context, orderID int64, action string) bool {
	if s == nil || s.entClient == nil || orderID <= 0 || strings.TrimSpace(action) == "" {
		return false
	}
	c, err := s.entClient.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(orderID, 10)), paymentauditlog.ActionEQ(action)).
		Limit(1).
		Count(ctx)
	return err == nil && c > 0
}

func (s *CodexPlusCommerceEntitlementService) writeAudit(ctx context.Context, orderID int64, action, operator string, detail map[string]any) {
	if s == nil || orderID <= 0 || strings.TrimSpace(action) == "" {
		return
	}
	if s.audit != nil {
		s.audit(ctx, orderID, action, operator, detail)
		return
	}
	if s.entClient == nil {
		return
	}
	payload, _ := json.Marshal(detail)
	if operator == "" {
		operator = "system"
	}
	if _, err := s.entClient.PaymentAuditLog.Create().
		SetOrderID(strconv.FormatInt(orderID, 10)).
		SetAction(action).
		SetOperator(operator).
		SetDetail(string(payload)).
		Save(ctx); err != nil {
		slog.Error("codexplus commerce entitlement audit failed", "orderID", orderID, "action", action, "error", err)
	}
}

func codexPlusCommercePlanForBinding(cfg *CodexPlusConfig, groupID int64, group *Group, paymentPlan *dbent.SubscriptionPlan) *CodexPlusPlan {
	if cfg == nil || groupID <= 0 {
		return nil
	}
	for i := range cfg.PlanCatalog.Plans {
		plan := &cfg.PlanCatalog.Plans[i]
		if strings.TrimSpace(plan.PlanID) == "" || !isCodexPlusPlanEnabled(plan.Status) {
			continue
		}
		if codexPlusCommercePlanMatchesBinding(plan, groupID, group, paymentPlan) {
			return plan
		}
	}
	return nil
}

func codexPlusCommercePlanMatchesBinding(plan *CodexPlusPlan, groupID int64, group *Group, paymentPlan *dbent.SubscriptionPlan) bool {
	if plan == nil {
		return false
	}
	sources := plan.EntitlementSources
	if containsCodexPlusInt64(sources.SubscriptionGroupIDs, groupID) {
		return true
	}
	if group != nil {
		if containsCodexPlusInt64(sources.SubscriptionGroupIDs, group.ID) {
			return true
		}
		if containsCodexPlusStringFold(sources.GroupNames, group.Name) {
			return true
		}
	}
	if paymentPlan != nil {
		if containsCodexPlusStringFold(sources.GroupNames, paymentPlan.Name) {
			return true
		}
		if codexPlusCommerceExternalRefMatches(plan, paymentPlan) {
			return true
		}
	}
	return false
}

func codexPlusCommerceExternalRefMatches(plan *CodexPlusPlan, paymentPlan *dbent.SubscriptionPlan) bool {
	if plan == nil || paymentPlan == nil {
		return false
	}
	for _, ref := range []string{derefString(plan.ExternalBillingRefs.ProductID), derefString(plan.ExternalBillingRefs.SKUID)} {
		ref = strings.TrimSpace(ref)
		switch ref {
		case "":
			continue
		case strconv.FormatInt(paymentPlan.ID, 10), "subscription_plan:" + strconv.FormatInt(paymentPlan.ID, 10), paymentPlan.Name, paymentPlan.ProductName:
			return true
		}
	}
	return false
}

func codexPlusCommerceOrderGroupID(order *dbent.PaymentOrder) int64 {
	if order == nil || order.SubscriptionGroupID == nil {
		return 0
	}
	return *order.SubscriptionGroupID
}

func codexPlusCommercePaymentPlanID(order *dbent.PaymentOrder, paymentPlan *dbent.SubscriptionPlan) int64 {
	if order != nil && order.PlanID != nil {
		return *order.PlanID
	}
	if paymentPlan != nil {
		return paymentPlan.ID
	}
	return 0
}

func codexPlusCommerceOrderValidityDays(order *dbent.PaymentOrder) int {
	if order == nil || order.SubscriptionDays == nil {
		return 0
	}
	return *order.SubscriptionDays
}

func codexPlusCommerceLegacyOrderNote(order *dbent.PaymentOrder) string {
	if order == nil {
		return ""
	}
	return fmt.Sprintf("payment order %d", order.ID)
}

func codexPlusCommerceSubscriptionHasOrderMarker(sub *UserSubscription, orderID int64) bool {
	if sub == nil || orderID <= 0 {
		return false
	}
	notes := strings.ToLower(sub.Notes)
	if notes == "" {
		return false
	}
	legacy := strings.ToLower(fmt.Sprintf("payment order %d", orderID))
	codex := strings.ToLower(fmt.Sprintf("codexplus entitlement order %d", orderID))
	return strings.Contains(notes, legacy) || strings.Contains(notes, codex)
}

func (s *PaymentService) codexPlusCommerceEntitlementService() *CodexPlusCommerceEntitlementService {
	if s == nil {
		return nil
	}
	var reader CodexPlusClientConfigReader
	if s.configService != nil && s.configService.settingRepo != nil {
		reader = NewCodexPlusConfigService(s.configService.settingRepo)
	}
	svc := NewCodexPlusCommerceEntitlementService(reader, s.groupRepo, s.subscriptionSvc, s.entClient)
	svc.SetAuditSink(s.writeAuditLog)
	return svc
}
