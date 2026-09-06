//go:build unit

package service

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/internal/membership"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestCreateOrderRejectsUnknownOrderTypeBeforeCheckout(t *testing.T) {
	_, err := (&PaymentService{}).CreateOrder(context.Background(), CreateOrderRequest{OrderType: "unknown"})
	require.Error(t, err)
	require.Equal(t, "INVALID_ORDER_TYPE", infraerrors.Reason(err))
}

func TestToPaidRejectsUnknownOrderTypeBeforeSettlement(t *testing.T) {
	err := (&PaymentService{}).toPaid(context.Background(), &dbent.PaymentOrder{OrderType: "unknown"}, "trade", 1, payment.TypeAlipay)
	require.Error(t, err)
	require.Equal(t, "INVALID_ORDER_TYPE", infraerrors.Reason(err))
}

func TestMembershipRefundPlanRejectsPartialForceAndDeductions(t *testing.T) {
	svc := &PaymentService{membership: &membership.Engine{}}
	base := func() *RefundPlan {
		return &RefundPlan{
			Order:         &dbent.PaymentOrder{OrderType: payment.OrderTypeMembership, Amount: 100},
			RefundAmount:  100,
			DeductionType: payment.DeductionTypeNone,
		}
	}

	tests := []struct {
		name   string
		mutate func(*RefundPlan)
	}{
		{name: "partial", mutate: func(p *RefundPlan) { p.RefundAmount = 99 }},
		{name: "force", mutate: func(p *RefundPlan) { p.Force = true }},
		{name: "balance flag", mutate: func(p *RefundPlan) { p.DeductBalance = true }},
		{name: "balance deduction", mutate: func(p *RefundPlan) { p.DeductionType = payment.DeductionTypeBalance; p.BalanceToDeduct = 1 }},
		{name: "subscription deduction", mutate: func(p *RefundPlan) {
			p.DeductionType = payment.DeductionTypeSubscription
			p.SubDaysToDeduct = 1
			p.SubscriptionID = 1
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := base()
			tt.mutate(p)
			err := svc.validateMembershipRefundPlan(p)
			require.Error(t, err)
			require.Equal(t, "MEMBERSHIP_REFUND_REVIEW", infraerrors.Reason(err))
		})
	}

	require.NoError(t, svc.validateMembershipRefundPlan(base()))
}

func TestRefundFinalizePlanDoesNotDefaultMembershipToBalanceDeduction(t *testing.T) {
	p := (&PaymentService{}).refundFinalizePlan(&dbent.PaymentOrder{
		ID:           42,
		OrderType:    payment.OrderTypeMembership,
		Amount:       100,
		PayAmount:    100,
		RefundAmount: 100,
	})

	require.False(t, p.DeductBalance)
	require.Equal(t, payment.DeductionTypeNone, p.DeductionType)
	require.Zero(t, p.BalanceToDeduct)
}

func TestMembershipRefundInProgressCannotBePreparedOrResubmitted(t *testing.T) {
	for _, status := range []string{OrderStatusRefundPending, OrderStatusRefunding} {
		t.Run(status, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			order := createInterruptedMembershipRefundOrderForTest(t, ctx, client, "membership-refund-in-progress-"+status)
			order, err := client.PaymentOrder.UpdateOneID(order.ID).SetStatus(status).Save(ctx)
			require.NoError(t, err)

			provider := &refundQueryProviderTestDouble{}
			restore := replacePaymentProviderFactoryForTest(t, provider)
			defer restore()
			svc := &PaymentService{entClient: client, membership: &membership.Engine{}}

			plan, early, err := svc.PrepareRefund(ctx, order.ID, 0, "retry", false, false)
			require.Nil(t, plan)
			require.Nil(t, early)
			require.Error(t, err)
			require.Equal(t, "REFUND_QUERY_REQUIRED", infraerrors.Reason(err))

			result, err := svc.ExecuteRefund(ctx, &RefundPlan{
				OrderID:       order.ID,
				Order:         order,
				RefundAmount:  order.Amount,
				GatewayAmount: order.Amount,
				Reason:        "retry",
				DeductionType: payment.DeductionTypeNone,
			})
			require.Nil(t, result)
			require.Error(t, err)
			require.Equal(t, "CONFLICT", infraerrors.Reason(err))
			require.Zero(t, provider.refundCalls)

			reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
			require.NoError(t, err)
			require.Equal(t, status, reloaded.Status)
		})
	}
}

func TestRestoreStatusPreservesMembershipPaid(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	user, err := client.User.Create().
		SetEmail("membership-refund-restore@example.com").
		SetPasswordHash("hash").
		SetUsername("membership-refund-restore").
		Save(ctx)
	require.NoError(t, err)
	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode("MEMBERSHIP-REFUND-RESTORE").
		SetOutTradeNo("sub2_membership_refund_restore").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("membership-refund-trade").
		SetOrderType(payment.OrderTypeMembership).
		SetStatus(OrderStatusRefunding).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)

	(&PaymentService{entClient: client}).restoreStatus(ctx, &RefundPlan{
		OrderID: order.ID,
		Order:   &dbent.PaymentOrder{ID: order.ID, OrderType: payment.OrderTypeMembership, Status: OrderStatusRefundPending},
	})

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPaid, reloaded.Status)
}

func TestRestoreStatusDoesNotOverwriteConcurrentMembershipRefund(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	user, err := client.User.Create().
		SetEmail("membership-refund-restore-race@example.com").
		SetPasswordHash("hash").
		SetUsername("membership-refund-restore-race").
		Save(ctx)
	require.NoError(t, err)
	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode("MEMBERSHIP-REFUND-RESTORE-RACE").
		SetOutTradeNo("sub2_membership_refund_restore_race").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("membership-refund-race-trade").
		SetOrderType(payment.OrderTypeMembership).
		SetStatus(OrderStatusRefunded).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)

	restored := (&PaymentService{entClient: client}).restoreStatus(ctx, &RefundPlan{
		OrderID: order.ID,
		Order:   &dbent.PaymentOrder{ID: order.ID, OrderType: payment.OrderTypeMembership},
	})
	require.False(t, restored)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefunded, reloaded.Status)
}

func TestRefundedMembershipFinalizationRetryIsReachableAndIdempotent(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	user, err := client.User.Create().
		SetEmail("membership-refund-retry@example.com").
		SetPasswordHash("hash").
		SetUsername("membership-refund-retry").
		Save(ctx)
	require.NoError(t, err)
	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode("MEMBERSHIP-REFUND-RETRY").
		SetOutTradeNo("sub2_membership_refund_retry").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("membership-refund-retry-trade").
		SetOrderType(payment.OrderTypeMembership).
		SetStatus(OrderStatusRefunded).
		SetRefundAmount(100).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)

	membershipDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer membershipDB.Close()
	membershipEngine := &membership.Engine{DB: membershipDB}
	svc := &PaymentService{entClient: client, membership: membershipEngine}

	// A committed payment refund can outlive a transient local finalization
	// failure. The persisted REFUNDED row must remain a retry source.
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT o.id,po.status,po.refund_amount FROM membership_payment_links").WithArgs(order.ID).
		WillReturnError(errors.New("transient membership database failure"))
	mock.ExpectRollback()
	result, err := svc.QueryAndFinalizeRefund(ctx, order.ID)
	require.Nil(t, result)
	require.Error(t, err)

	mock.ExpectQuery("SELECT l.payment_order_id FROM membership_payment_links").WithArgs(pendingPaymentReconcileLimit).
		WillReturnRows(sqlmock.NewRows([]string{"payment_order_id"}).AddRow(order.ID))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT o.id,po.status,po.refund_amount FROM membership_payment_links").WithArgs(order.ID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "refund_amount"}).AddRow("membership-order-1", "REFUNDED", 100.0))
	// A second finalization observes the membership side already done. It must
	// not insert another ledger entry or touch CDK allocation.
	mock.ExpectExec("UPDATE membership_orders SET payment_state='refunded'").WithArgs("membership-order-1").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	completed, err := svc.ReconcileMembershipRefundFinalizations(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, completed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMembershipRecoveryReconcilersRetryOnlyDurableRecoveryStates(t *testing.T) {
	tests := []struct {
		name               string
		reconcile          func(*PaymentService, context.Context) (int, error)
		query              string
		paymentStatus      string
		expectedOrderState string
		expectedAction     string
	}{
		{
			name:               "failed payment creation becomes manual review",
			reconcile:          (*PaymentService).ReconcileMembershipPaymentRecoveries,
			query:              "SELECT l.payment_order_id,po.status FROM membership_payment_links",
			paymentStatus:      OrderStatusFailed,
			expectedOrderState: "manual_review",
			expectedAction:     "payment_creation_unknown",
		},
		{
			name:               "failed refund restores paid review state",
			reconcile:          (*PaymentService).ReconcileMembershipRefundAborts,
			query:              "SELECT l.payment_order_id FROM membership_payment_links",
			paymentStatus:      OrderStatusRefundFailed,
			expectedOrderState: "paid",
			expectedAction:     "refund_aborted",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			svc := &PaymentService{membership: &membership.Engine{DB: db}}

			if tt.expectedOrderState == "manual_review" {
				mock.ExpectQuery(tt.query).WithArgs(pendingPaymentReconcileLimit).
					WillReturnRows(sqlmock.NewRows([]string{"payment_order_id", "status"}).AddRow(int64(9), tt.paymentStatus))
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT o.id,o.payment_state FROM membership_payment_links").WithArgs(int64(9)).
					WillReturnRows(sqlmock.NewRows([]string{"id", "payment_state"}).AddRow("order-1", "pending"))
				mock.ExpectExec("UPDATE membership_orders SET payment_state='manual_review'").WithArgs("order-1").WillReturnResult(sqlmock.NewResult(0, 1))
			} else {
				mock.ExpectQuery(tt.query).WithArgs(pendingPaymentReconcileLimit).
					WillReturnRows(sqlmock.NewRows([]string{"payment_order_id"}).AddRow(int64(9)))
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT o.id,o.payment_state FROM membership_payment_links").WithArgs(int64(9)).
					WillReturnRows(sqlmock.NewRows([]string{"id", "payment_state"}).AddRow("order-1", "refund_pending"))
				mock.ExpectExec("UPDATE membership_orders SET payment_state='paid'").WithArgs("order-1").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec("UPDATE membership_tasks SET state='review_required'").WithArgs("order-1").WillReturnResult(sqlmock.NewResult(0, 1))
			}
			mock.ExpectExec("INSERT INTO membership_events").WithArgs(sqlmock.AnyArg(), "order-1", tt.expectedAction, "payment_gateway", tt.expectedOrderState, sqlmock.AnyArg(), "").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("INSERT INTO membership_notifications").WithArgs(sqlmock.AnyArg(), "order-1").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()

			count, err := tt.reconcile(svc, context.Background())
			require.NoError(t, err)
			require.Equal(t, 1, count)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestInterruptedMembershipRefundReconcilesGatewayResultOnce(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createInterruptedMembershipRefundOrderForTest(t, ctx, client, "refund-reconcile-success")

	membershipDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer membershipDB.Close()
	provider := &refundQueryProviderTestDouble{refundResponse: &payment.RefundResponse{RefundID: "rf-recovered", Status: payment.ProviderStatusSuccess}}
	restore := replacePaymentProviderFactoryForTest(t, provider)
	defer restore()
	svc := &PaymentService{entClient: client, membership: &membership.Engine{DB: membershipDB}, loadBalancer: &captureLoadBalancer{}}

	mock.ExpectQuery("SELECT l.payment_order_id FROM membership_payment_links").WithArgs(pendingPaymentReconcileLimit).
		WillReturnRows(sqlmock.NewRows([]string{"payment_order_id"}).AddRow(order.ID))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT o.id,po.status,po.refund_amount FROM membership_payment_links").WithArgs(order.ID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "refund_amount"}).AddRow("membership-order-1", "REFUNDED", 100.0))
	mock.ExpectExec("UPDATE membership_orders SET payment_state='refunded'").WithArgs("membership-order-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE membership_cdks SET order_id=NULL,state='available'").WithArgs("membership-order-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO membership_ledger").WithArgs(sqlmock.AnyArg(), "membership-order-1", int64(10000), strconv.FormatInt(order.ID, 10)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO membership_events").WithArgs(sqlmock.AnyArg(), "membership-order-1", "refund_completed", "payment_webhook", "refunded", "", "").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO membership_notifications").WithArgs(sqlmock.AnyArg(), "membership-order-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT l.payment_order_id FROM membership_payment_links").WithArgs(pendingPaymentReconcileLimit).
		WillReturnRows(sqlmock.NewRows([]string{"payment_order_id"}))

	recovered, err := svc.ReconcileMembershipRefunding(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, recovered)
	recovered, err = svc.ReconcileMembershipRefunding(ctx)
	require.NoError(t, err)
	require.Zero(t, recovered)
	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefunded, reloaded.Status)
	require.Equal(t, 1, provider.queryCalls)
	require.Zero(t, provider.refundCalls)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestInterruptedMembershipRefundWithoutSafeQueryBecomesManualReview(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createInterruptedMembershipRefundOrderForTest(t, ctx, client, "refund-reconcile-manual")

	membershipDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer membershipDB.Close()
	restore := replacePaymentProviderFactoryForTest(t, refundProviderTestDouble{})
	defer restore()
	svc := &PaymentService{entClient: client, membership: &membership.Engine{DB: membershipDB}, loadBalancer: &captureLoadBalancer{}}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT o.id,o.payment_state FROM membership_payment_links").WithArgs(order.ID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "payment_state"}).AddRow("membership-order-1", "refund_pending"))
	mock.ExpectExec("UPDATE membership_orders SET payment_state='manual_review'").WithArgs("membership-order-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE membership_tasks SET state='review_required'").WithArgs("membership-order-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO membership_events").WithArgs(sqlmock.AnyArg(), "membership-order-1", "refund_manual_review", "payment_gateway", "manual_review", "REFUND_REVIEW", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO membership_notifications").WithArgs(sqlmock.AnyArg(), "membership-order-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, svc.reconcileMembershipRefundingOrder(ctx, order.ID))
	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefundPending, reloaded.Status)
	reviewAudit, err := client.PaymentAuditLog.Query().Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("REFUND_MANUAL_REVIEW")).Only(ctx)
	require.NoError(t, err)
	require.Contains(t, reviewAudit.Detail, "does not support status query")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestInterruptedMembershipRefundManualReviewCASDoesNotOverwriteConcurrentRefund(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createInterruptedMembershipRefundOrderForTest(t, ctx, client, "refund-reconcile-cas")
	_, err := client.PaymentOrder.UpdateOneID(order.ID).SetStatus(OrderStatusRefunded).Save(ctx)
	require.NoError(t, err)

	membershipDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer membershipDB.Close()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT o.id,o.payment_state FROM membership_payment_links").WithArgs(order.ID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "payment_state"}).AddRow("membership-order-1", "manual_review"))
	mock.ExpectCommit()
	svc := &PaymentService{entClient: client, membership: &membership.Engine{DB: membershipDB}}
	require.NoError(t, svc.markMembershipRefundManualReview(ctx, order, "stale query failure"))
	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefunded, reloaded.Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

func createInterruptedMembershipRefundOrderForTest(t *testing.T, ctx context.Context, client *dbent.Client, suffix string) *dbent.PaymentOrder {
	t.Helper()
	user, err := client.User.Create().SetEmail(suffix + "@example.com").SetPasswordHash("hash").SetUsername(suffix).Save(ctx)
	require.NoError(t, err)
	inst, err := client.PaymentProviderInstance.Create().SetProviderKey(payment.TypeStripe).SetName(suffix + "-provider").SetConfig("{}").SetSupportedTypes("stripe").SetEnabled(true).SetRefundEnabled(true).Save(ctx)
	require.NoError(t, err)
	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).SetUserEmail(user.Email).SetUserName(user.Username).
		SetAmount(100).SetPayAmount(100).SetFeeRate(0).
		SetRechargeCode("MEMBERSHIP-" + suffix).SetOutTradeNo("sub2_" + suffix).
		SetPaymentType(payment.TypeStripe).SetPaymentTradeNo("pi_" + suffix).
		SetOrderType(payment.OrderTypeMembership).SetStatus(OrderStatusRefunding).
		SetRefundAmount(100).SetRefundReason("interrupted refund").
		SetExpiresAt(time.Now().Add(time.Hour)).SetPaidAt(time.Now()).SetClientIP("127.0.0.1").SetSrcHost("api.example.com").
		SetProviderInstanceID(strconv.FormatInt(inst.ID, 10)).Save(ctx)
	require.NoError(t, err)
	return order
}

func TestMembershipPaymentRecoveryReconcilerReleasesCancelledAndExpiredReservations(t *testing.T) {
	for _, paymentStatus := range []string{OrderStatusCancelled, OrderStatusExpired} {
		t.Run(paymentStatus, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			svc := &PaymentService{membership: &membership.Engine{DB: db}}

			// These outcomes are known not to have created a provider charge, so a
			// retry must release the held checkout inventory instead of escalating it.
			mock.ExpectQuery("SELECT l.payment_order_id,po.status FROM membership_payment_links").
				WithArgs(pendingPaymentReconcileLimit).
				WillReturnRows(sqlmock.NewRows([]string{"payment_order_id", "status"}).AddRow(int64(9), paymentStatus))
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT o.id,o.payment_state FROM membership_payment_links").WithArgs(int64(9)).
				WillReturnRows(sqlmock.NewRows([]string{"id", "payment_state"}).AddRow("order-1", "pending"))
			mock.ExpectExec("UPDATE membership_orders SET payment_state='closed'").WithArgs("order-1").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("UPDATE membership_cdks SET state='available',order_id=NULL").WithArgs("order-1").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("UPDATE membership_coupons SET used=used-1").WithArgs("order-1").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("INSERT INTO membership_events").WithArgs(sqlmock.AnyArg(), "order-1", "payment_creation_failed", "payment_gateway", "closed", "", "").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("INSERT INTO membership_notifications").WithArgs(sqlmock.AnyArg(), "order-1").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()

			count, err := svc.ReconcileMembershipPaymentRecoveries(context.Background())
			require.NoError(t, err)
			require.Equal(t, 1, count)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
