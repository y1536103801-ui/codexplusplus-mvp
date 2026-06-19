//go:build unit

package service

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestCodexPlusCommerceResolveSubscriptionOrderMatchesPaymentPlanBinding(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 17, 8, 0, 0, 0, time.UTC)
	client := newCodexPlusCommerceTestClient(t)

	plan, err := client.SubscriptionPlan.Create().
		SetGroupID(20).
		SetName("Starter monthly").
		SetPrice(49).
		SetValidityDays(30).
		SetProductName("Codex++ Starter").
		Save(ctx)
	require.NoError(t, err)

	cfg := codexPlusClientTestConfig(now)
	groupRepo := &subscriptionGroupRepoStub{
		group: &Group{ID: 20, Name: "Codex++ Starter", Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription},
	}
	svc := NewCodexPlusCommerceEntitlementService(&codexPlusClientTestConfigReader{cfg: cfg}, groupRepo, nil, client)

	order := &dbent.PaymentOrder{
		ID:        9001,
		UserID:    7001,
		OrderType: payment.OrderTypeSubscription,
		PlanID:    &plan.ID,
	}
	entitlement, err := svc.ResolveSubscriptionOrder(ctx, order)
	require.NoError(t, err)
	require.NotNil(t, entitlement)
	require.Equal(t, "starter", entitlement.CodexPlanID)
	require.Equal(t, "default", entitlement.UsagePolicyID)
	require.Equal(t, int64(20), entitlement.GroupID)
	require.Equal(t, plan.ID, entitlement.PaymentPlanID)
}

func TestPaymentSubscriptionFulfillmentSkipsAlreadyGrantedCodexPlusOrder(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 17, 9, 0, 0, 0, time.UTC)
	client := newCodexPlusCommerceTestClient(t)
	cfg := codexPlusClientTestConfig(now)
	settingRepo := newCodexPlusCommerceSettingRepo(t, ctx, cfg)
	groupRepo := &subscriptionGroupRepoStub{
		group: &Group{ID: 20, Name: "Codex++ Starter", Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription},
	}
	subRepo := newSubscriptionUserSubRepoStub()
	subSvc := NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)
	paymentSvc := NewPaymentService(client, nil, nil, nil, subSvc, NewPaymentConfigService(client, settingRepo, nil), nil, groupRepo, nil)

	user := newCodexPlusCommerceUser(t, ctx, client, "already-granted@example.com")
	order := newCodexPlusCommerceSubscriptionOrder(t, ctx, client, user, OrderStatusPaid, now)
	subRepo.seed(&UserSubscription{
		ID:        1001,
		UserID:    user.ID,
		GroupID:   20,
		StartsAt:  now.Add(-time.Hour),
		ExpiresAt: now.AddDate(0, 0, 30),
		Status:    SubscriptionStatusActive,
		Notes:     "codexplus entitlement order " + strconv.FormatInt(order.ID, 10) + " plan starter",
	})

	err := paymentSvc.ExecuteSubscriptionFulfillment(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, 0, subRepo.createCalls)

	updated, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, updated.Status)
	require.Equal(t, 1, codexPlusCommerceAuditCount(t, ctx, client, order.ID, codexPlusCommerceEntitlementAuditAlreadyGrant))
	require.Equal(t, 1, codexPlusCommerceAuditCount(t, ctx, client, order.ID, "SUBSCRIPTION_SUCCESS"))
}

func TestPaymentExpiredGraceOrderGrantsCodexPlusEntitlementAndRefreshesClientState(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	client := newCodexPlusCommerceTestClient(t)
	cfg := codexPlusClientTestConfig(now.UTC())
	settingRepo := newCodexPlusCommerceSettingRepo(t, ctx, cfg)
	groupRepo := &subscriptionGroupRepoStub{
		group: &Group{ID: 20, Name: "Codex++ Starter", Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription},
	}
	subRepo := newSubscriptionUserSubRepoStub()
	subSvc := NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)
	paymentSvc := NewPaymentService(client, nil, nil, nil, subSvc, NewPaymentConfigService(client, settingRepo, nil), nil, groupRepo, nil)

	user := newCodexPlusCommerceUser(t, ctx, client, "expired-grace@example.com")
	order := newCodexPlusCommerceSubscriptionOrder(t, ctx, client, user, OrderStatusExpired, now)

	err := paymentSvc.toPaid(ctx, order, "trade-expired-grace", order.PayAmount, payment.TypeStripe)
	require.NoError(t, err)
	require.Equal(t, 1, subRepo.createCalls)

	updated, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, updated.Status)
	require.Equal(t, 1, codexPlusCommerceAuditCount(t, ctx, client, order.ID, codexPlusCommerceEntitlementAuditGranted))

	sub, err := subRepo.GetByUserIDAndGroupID(ctx, user.ID, 20)
	require.NoError(t, err)
	require.True(t, strings.Contains(sub.Notes, "codexplus entitlement order "+strconv.FormatInt(order.ID, 10)))

	clientSvc := NewCodexPlusClientService(
		&codexPlusClientTestConfigReader{cfg: cfg},
		&codexPlusClientTestUsers{user: &User{ID: user.ID, Email: user.Email, Username: user.Username, Status: StatusActive}},
		&codexPlusClientTestSubs{active: []UserSubscription{*sub}, all: []UserSubscription{*sub}},
		nil,
		nil,
	)
	clientSvc.SetNow(func() time.Time { return now.UTC() })
	bootstrap, err := clientSvc.Bootstrap(ctx, CodexPlusBootstrapInput{UserID: user.ID, DeviceID: "device-1234"})
	require.NoError(t, err)
	require.Equal(t, ClientServiceStatusAvailable, bootstrap.Service.Status)
	usage, err := clientSvc.ClientUsage(ctx, CodexPlusUsageInput{UserID: user.ID, DeviceID: "device-1234"})
	require.NoError(t, err)
	require.Equal(t, ClientServiceStatusAvailable, usage.ServiceStatus)

	err = paymentSvc.HandlePaymentNotification(ctx, &payment.PaymentNotification{
		OrderID: order.OutTradeNo,
		TradeNo: "trade-expired-grace",
		Status:  payment.NotificationStatusSuccess,
		Amount:  order.PayAmount,
	}, payment.TypeStripe)
	require.NoError(t, err)
	require.Equal(t, 1, subRepo.createCalls)
}

func newCodexPlusCommerceTestClient(t *testing.T) *dbent.Client {
	t.Helper()

	db, err := sql.Open("sqlite", "file:codexplus_commerce_entitlement?mode=memory&cache=shared&_fk=1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func newCodexPlusCommerceSettingRepo(t *testing.T, ctx context.Context, cfg *CodexPlusConfig) *fakeCodexPlusSettingRepo {
	t.Helper()

	repo := newFakeCodexPlusSettingRepo()
	raw, err := EncodeCodexPlusConfig(codexPlusCommercePublishableConfig(cfg))
	require.NoError(t, err)
	require.NoError(t, repo.Set(ctx, CodexPlusConfigSettingKey, raw))
	return repo
}

func codexPlusCommercePublishableConfig(cfg *CodexPlusConfig) *CodexPlusConfig {
	if cfg == nil {
		return nil
	}
	cp := *cfg
	cp.PlanCatalog.Plans = append([]CodexPlusPlan(nil), cfg.PlanCatalog.Plans...)
	cp.UsagePolicy.Policies = append([]CodexPlusUsageRule(nil), cfg.UsagePolicy.Policies...)
	for i := range cp.PlanCatalog.Plans {
		cp.PlanCatalog.Plans[i].CopyKeys = CodexPlusPlanCopyKeys{
			PurchaseAction:      "billing.action.purchase",
			RenewAction:         "billing.action.renew",
			UpgradeAction:       "billing.action.upgrade",
			NotPurchasedMessage: "billing.not_purchased",
			ExpiredMessage:      "billing.expired",
			LowBalanceMessage:   "billing.low_balance",
		}
	}
	for i := range cp.UsagePolicy.Policies {
		cp.UsagePolicy.Policies[i].InsufficientBalanceMessage = "usage.insufficient_balance"
		cp.UsagePolicy.Policies[i].RateLimitedMessage = "usage.rate_limited"
		cp.UsagePolicy.Policies[i].CopyKeys = CodexPlusUsagePolicyCopyKeys{
			LowBalanceMessage:          "usage.low_balance",
			InsufficientBalanceMessage: "usage.insufficient_balance",
			RateLimitedMessage:         "usage.rate_limited",
			ExpiredMessage:             "usage.expired",
			RenewAction:                "usage.renew_action",
			PurchaseAction:             "usage.purchase_action",
			DeviceRevokedMessage:       "device.revoked",
		}
	}
	return &cp
}

func newCodexPlusCommerceUser(t *testing.T, ctx context.Context, client *dbent.Client, email string) *dbent.User {
	t.Helper()

	user, err := client.User.Create().
		SetEmail(email).
		SetPasswordHash("hash").
		SetUsername(strings.TrimSuffix(email, "@example.com")).
		Save(ctx)
	require.NoError(t, err)
	return user
}

func newCodexPlusCommerceSubscriptionOrder(t *testing.T, ctx context.Context, client *dbent.Client, user *dbent.User, status string, now time.Time) *dbent.PaymentOrder {
	t.Helper()

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(49).
		SetPayAmount(49).
		SetFeeRate(0).
		SetRechargeCode("CODEXPLUS-" + status + "-" + strconv.FormatInt(now.UnixNano(), 10)).
		SetOutTradeNo("sub2_codexplus_" + status + "_" + strconv.FormatInt(now.UnixNano(), 10)).
		SetPaymentType(payment.TypeStripe).
		SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeSubscription).
		SetPlanID(501).
		SetSubscriptionGroupID(20).
		SetSubscriptionDays(30).
		SetStatus(status).
		SetExpiresAt(now.Add(-time.Minute)).
		SetUpdatedAt(now).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)
	return order
}

func codexPlusCommerceAuditCount(t *testing.T, ctx context.Context, client *dbent.Client, orderID int64, action string) int {
	t.Helper()

	count, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(orderID, 10)), paymentauditlog.ActionEQ(action)).
		Count(ctx)
	require.NoError(t, err)
	return count
}
