package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/paymentproviderinstance"
	"github.com/Wei-Shaw/sub2api/ent/predicate"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/payment/provider"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/servertiming"
)

// --- Refund Flow ---

var createPaymentProviderFromInstance = provider.CreateProvider

// getOrderProviderInstance looks up the provider instance that processed this order.
// For legacy orders without provider_instance_id, it resolves only when the
// historical instance is uniquely identifiable from the stored order fields.
func (s *PaymentService) getOrderProviderInstance(ctx context.Context, o *dbent.PaymentOrder) (*dbent.PaymentProviderInstance, error) {
	if s == nil || s.entClient == nil || o == nil {
		return nil, nil
	}

	if snapshot := psOrderProviderSnapshot(o); snapshot != nil {
		return s.resolveSnapshotOrderProviderInstance(ctx, o, snapshot)
	}

	instIDStr := strings.TrimSpace(psStringValue(o.ProviderInstanceID))
	if instIDStr == "" {
		return s.resolveUniqueLegacyOrderProviderInstance(ctx, o)
	}

	instID, err := strconv.ParseInt(instIDStr, 10, 64)
	if err != nil {
		return nil, nil
	}
	return s.entClient.PaymentProviderInstance.Get(ctx, instID)
}

// getRefundOrderProviderInstance resolves the provider instance for refund paths.
// Refunds must be pinned to an explicit historical binding, so legacy
// "best-effort" provider guessing is intentionally not allowed here.
func (s *PaymentService) getRefundOrderProviderInstance(ctx context.Context, o *dbent.PaymentOrder) (*dbent.PaymentProviderInstance, error) {
	if s == nil || s.entClient == nil || o == nil {
		return nil, nil
	}

	if snapshot := psOrderProviderSnapshot(o); snapshot != nil {
		return s.resolveSnapshotOrderProviderInstance(ctx, o, snapshot)
	}

	instIDStr := strings.TrimSpace(psStringValue(o.ProviderInstanceID))
	if instIDStr == "" {
		return nil, nil
	}

	instID, err := strconv.ParseInt(instIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("order %d refund provider instance id is invalid: %s", o.ID, instIDStr)
	}
	inst, err := s.entClient.PaymentProviderInstance.Get(ctx, instID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, fmt.Errorf("order %d refund provider instance %s is missing", o.ID, instIDStr)
		}
		return nil, err
	}
	return inst, nil
}

func (s *PaymentService) resolveUniqueLegacyOrderProviderInstance(ctx context.Context, o *dbent.PaymentOrder) (*dbent.PaymentProviderInstance, error) {
	paymentType := payment.GetBasePaymentType(strings.TrimSpace(o.PaymentType))
	providerKey := strings.TrimSpace(psStringValue(o.ProviderKey))
	if providerKey != "" {
		instances, err := s.entClient.PaymentProviderInstance.Query().
			Where(paymentproviderinstance.ProviderKeyEQ(providerKey)).
			All(ctx)
		if err != nil {
			return nil, err
		}
		matched := psFilterLegacyOrderProviderInstances(paymentType, instances)
		if len(matched) == 1 {
			return matched[0], nil
		}
		return nil, nil
	}

	if paymentType == "" {
		return nil, nil
	}

	instances, err := s.entClient.PaymentProviderInstance.Query().
		All(ctx)
	if err != nil {
		return nil, err
	}

	matched := psFilterLegacyOrderProviderInstances(paymentType, instances)
	if len(matched) == 1 {
		return matched[0], nil
	}
	return nil, nil
}

func psFilterLegacyOrderProviderInstances(orderPaymentType string, instances []*dbent.PaymentProviderInstance) []*dbent.PaymentProviderInstance {
	if len(instances) == 0 {
		return nil
	}
	if strings.TrimSpace(orderPaymentType) == "" {
		return instances
	}
	var matched []*dbent.PaymentProviderInstance
	for _, inst := range instances {
		if psLegacyOrderMatchesInstance(orderPaymentType, inst) {
			matched = append(matched, inst)
		}
	}
	return matched
}

func psLegacyOrderMatchesInstance(orderPaymentType string, inst *dbent.PaymentProviderInstance) bool {
	if inst == nil {
		return false
	}

	baseType := payment.GetBasePaymentType(strings.TrimSpace(orderPaymentType))
	instanceProviderKey := strings.TrimSpace(inst.ProviderKey)
	if baseType == "" {
		return false
	}

	if baseType == payment.TypeStripe {
		return instanceProviderKey == payment.TypeStripe
	}
	if instanceProviderKey == payment.TypeStripe {
		return false
	}
	if instanceProviderKey == baseType {
		return true
	}
	return payment.InstanceSupportsType(inst.SupportedTypes, baseType)
}

func (s *PaymentService) RequestRefund(ctx context.Context, oid, uid int64, reason string) error {
	o, err := s.validateRefundRequest(ctx, oid, uid)
	if err != nil {
		return err
	}
	u, err := s.userRepo.GetByID(ctx, o.UserID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	if u.Balance < o.Amount {
		return infraerrors.BadRequest("BALANCE_NOT_ENOUGH", "refund amount exceeds balance")
	}
	nr := strings.TrimSpace(reason)
	now := time.Now()
	by := fmt.Sprintf("%d", uid)
	c, err := s.entClient.PaymentOrder.Update().Where(paymentorder.IDEQ(oid), paymentorder.UserIDEQ(uid), paymentorder.StatusEQ(OrderStatusCompleted), paymentorder.OrderTypeEQ(payment.OrderTypeBalance)).SetStatus(OrderStatusRefundRequested).SetRefundRequestedAt(now).SetRefundRequestReason(nr).SetRefundRequestedBy(by).SetRefundAmount(o.Amount).Save(ctx)
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}
	if c == 0 {
		return infraerrors.Conflict("CONFLICT", "order status changed")
	}
	s.writeAuditLog(ctx, oid, "REFUND_REQUESTED", fmt.Sprintf("user:%d", uid), map[string]any{"amount": o.Amount, "reason": nr})
	return nil
}

func (s *PaymentService) validateRefundRequest(ctx context.Context, oid, uid int64) (*dbent.PaymentOrder, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.UserID != uid {
		return nil, infraerrors.Forbidden("FORBIDDEN", "no permission")
	}
	if o.OrderType != payment.OrderTypeBalance {
		return nil, infraerrors.BadRequest("INVALID_ORDER_TYPE", "only balance orders can request refund")
	}
	if o.Status != OrderStatusCompleted {
		return nil, infraerrors.BadRequest("INVALID_STATUS", "only completed orders can request refund")
	}
	// Check provider instance allows user refund
	inst, err := s.getRefundOrderProviderInstance(ctx, o)
	if err != nil || inst == nil {
		return nil, infraerrors.Forbidden("USER_REFUND_DISABLED", "refund is not available for this order")
	}
	if !inst.AllowUserRefund {
		return nil, infraerrors.Forbidden("USER_REFUND_DISABLED", "user refund is not enabled for this provider")
	}
	return o, nil
}

func (s *PaymentService) PrepareRefund(ctx context.Context, oid int64, amt float64, reason string, force, deduct bool) (*RefundPlan, *RefundResult, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return nil, nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if !isSupportedPaymentOrderType(o.OrderType) {
		return nil, nil, infraerrors.BadRequest("INVALID_ORDER_TYPE", "unsupported order type")
	}
	ok := []string{OrderStatusCompleted, OrderStatusRefundRequested, OrderStatusRefundPending, OrderStatusRefundFailed}
	if o.OrderType == payment.OrderTypeMembership {
		if o.Status == OrderStatusRefundPending || o.Status == OrderStatusRefunding {
			return nil, nil, infraerrors.Conflict("REFUND_QUERY_REQUIRED", "membership refund is already in progress; query or review the existing refund")
		}
		ok = []string{OrderStatusPaid, OrderStatusRefundFailed}
		if s.membership == nil || force || deduct || (amt > 0 && math.Abs(amt-o.Amount) > paymentAmountToleranceForCurrency(PaymentOrderCurrency(o))) {
			return nil, nil, infraerrors.BadRequest("MEMBERSHIP_REFUND_REVIEW", "membership refunds require confirmed non-delivery and a full refund")
		}
	}
	if !psSliceContains(ok, o.Status) {
		return nil, nil, infraerrors.BadRequest("INVALID_STATUS", "order status does not allow refund")
	}
	if o.OrderType == payment.OrderTypeMembership && s.hasAuditLog(ctx, o.ID, "REFUND_MANUAL_REVIEW") {
		return nil, nil, infraerrors.Conflict("MEMBERSHIP_REFUND_REVIEW", "refund outcome requires manual review before another refund can be submitted")
	}
	// Check provider instance allows admin refund
	inst, instErr := s.getRefundOrderProviderInstance(ctx, o)
	if instErr != nil {
		slog.Warn("refund: provider instance lookup failed", "orderID", oid, "error", instErr)
		return nil, nil, infraerrors.InternalServer("PROVIDER_LOOKUP_FAILED", "failed to look up payment provider for this order")
	}
	if inst == nil {
		// Legacy order without provider_instance_id — block refund
		return nil, nil, infraerrors.Forbidden("REFUND_DISABLED", "refund is not available for this order")
	}
	if !inst.RefundEnabled {
		return nil, nil, infraerrors.Forbidden("REFUND_DISABLED", "refund is not enabled for this provider")
	}
	if math.IsNaN(amt) || math.IsInf(amt, 0) {
		return nil, nil, infraerrors.BadRequest("INVALID_AMOUNT", "invalid refund amount")
	}
	if amt <= 0 {
		amt = o.Amount
	}
	orderCurrency := PaymentOrderCurrency(o)
	if amt-o.Amount > paymentAmountToleranceForCurrency(orderCurrency) {
		return nil, nil, infraerrors.BadRequest("REFUND_AMOUNT_EXCEEDED", "refund amount exceeds recharge")
	}
	ga := calculateGatewayRefundAmount(o.Amount, o.PayAmount, amt, orderCurrency)
	rr := strings.TrimSpace(reason)
	if rr == "" && o.RefundRequestReason != nil {
		rr = *o.RefundRequestReason
	}
	if rr == "" {
		rr = fmt.Sprintf("refund order:%d", o.ID)
	}
	p := &RefundPlan{OrderID: oid, Order: o, RefundAmount: amt, GatewayAmount: ga, Reason: rr, Force: force, DeductBalance: deduct, DeductionType: payment.DeductionTypeNone}
	if o.OrderType == payment.OrderTypeMembership {
		if err := s.validateMembershipRefundPlan(p); err != nil {
			return nil, nil, err
		}
		if err := s.membership.PrepareRefund(ctx, oid); err != nil {
			return nil, nil, infraerrors.Conflict("MEMBERSHIP_REFUND_REVIEW", "resolve fulfillment before refunding")
		}
	}
	if deduct {
		if er := s.prepDeduct(ctx, o, p, force); er != nil {
			return nil, er, nil
		}
	}
	return p, nil, nil
}

func (s *PaymentService) prepDeduct(ctx context.Context, o *dbent.PaymentOrder, p *RefundPlan, force bool) *RefundResult {
	if o.OrderType == payment.OrderTypeSubscription {
		p.DeductionType = payment.DeductionTypeSubscription
		if o.SubscriptionGroupID != nil && o.SubscriptionDays != nil {
			p.SubDaysToDeduct = *o.SubscriptionDays
			sub, err := s.subscriptionSvc.GetActiveSubscription(ctx, o.UserID, *o.SubscriptionGroupID)
			if err == nil && sub != nil {
				p.SubscriptionID = sub.ID
			} else if !force {
				return &RefundResult{Success: false, Warning: "cannot find active subscription for deduction, use force", RequireForce: true}
			}
		}
		return nil
	}
	u, err := s.userRepo.GetByID(ctx, o.UserID)
	if err != nil {
		if !force {
			return &RefundResult{Success: false, Warning: "cannot fetch user balance, use force", RequireForce: true}
		}
		return nil
	}
	p.DeductionType = payment.DeductionTypeBalance
	if u.Balance < p.RefundAmount && !force {
		return &RefundResult{Success: false, Warning: "user balance is insufficient for deduction, use force", RequireForce: true}
	}
	p.BalanceToDeduct = math.Max(0, math.Min(p.RefundAmount, u.Balance))
	return nil
}

type availableBalanceDeductor interface {
	DeductAvailableBalance(ctx context.Context, id int64, amount float64) (float64, error)
}

func (s *PaymentService) deductAvailableBalance(ctx context.Context, userID int64, amount float64) (float64, error) {
	repo, ok := s.userRepo.(availableBalanceDeductor)
	if !ok {
		return 0, errors.New("user repository does not support available balance deduction")
	}
	return repo.DeductAvailableBalance(ctx, userID, amount)
}

func (s *PaymentService) ExecuteRefund(ctx context.Context, p *RefundPlan) (*RefundResult, error) {
	if err := s.validateMembershipRefundPlan(p); err != nil {
		return nil, err
	}
	where := []predicate.PaymentOrder{paymentorder.IDEQ(p.OrderID)}
	if p.Order.OrderType == payment.OrderTypeMembership {
		// A pending or in-flight membership refund may already have reached the
		// gateway. It is recoverable only by querying its status, never by a
		// second Refund submission.
		where = append(where,
			paymentorder.OrderTypeEQ(payment.OrderTypeMembership),
			paymentorder.StatusIn(OrderStatusPaid, OrderStatusRefundFailed),
		)
	} else {
		where = append(where, paymentorder.StatusIn(OrderStatusCompleted, OrderStatusRefundRequested, OrderStatusRefundPending, OrderStatusRefundFailed))
	}
	c, err := s.entClient.PaymentOrder.Update().Where(where...).
		SetStatus(OrderStatusRefunding).
		SetRefundAmount(p.RefundAmount).
		SetRefundReason(p.Reason).
		SetForceRefund(p.Force).
		ClearFailedAt().
		ClearFailedReason().
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("lock: %w", err)
	}
	if c == 0 {
		return nil, infraerrors.Conflict("CONFLICT", "order status changed")
	}
	// Persist the intent before the provider call so a post-CAS crash can be
	// reconciled by a status query without issuing another refund.
	s.writeAuditLog(ctx, p.OrderID, "REFUND_INTENT", "admin", map[string]any{
		"refundAmount": p.RefundAmount,
		"reason":       p.Reason,
		"force":        p.Force,
	})
	if p.DeductionType == payment.DeductionTypeBalance && p.BalanceToDeduct > 0 {
		// Skip balance deduction on retry if previous attempt already deducted
		// but failed to roll back (REFUND_ROLLBACK_FAILED in audit log).
		if !s.hasAuditLog(ctx, p.OrderID, "REFUND_ROLLBACK_FAILED") {
			deducted, err := s.deductAvailableBalance(ctx, p.Order.UserID, p.BalanceToDeduct)
			if err != nil {
				s.restoreStatus(ctx, p)
				return nil, fmt.Errorf("deduction: %w", err)
			}
			p.BalanceToDeduct = deducted
		} else {
			slog.Warn("skipping balance deduction on retry (previous rollback failed)", "orderID", p.OrderID)
			p.BalanceToDeduct = 0
		}
	}
	if p.DeductionType == payment.DeductionTypeSubscription && p.SubDaysToDeduct > 0 && p.SubscriptionID > 0 {
		if !s.hasAuditLog(ctx, p.OrderID, "REFUND_ROLLBACK_FAILED") {
			_, err := s.subscriptionSvc.ExtendSubscription(ctx, p.SubscriptionID, -p.SubDaysToDeduct)
			if err != nil {
				if errors.Is(err, ErrAdjustWouldExpire) {
					// Deduction would expire the subscription — revoke it entirely
					slog.Info("subscription deduction would expire, revoking", "orderID", p.OrderID, "subID", p.SubscriptionID, "days", p.SubDaysToDeduct)
					if revokeErr := s.subscriptionSvc.RevokeSubscription(ctx, p.SubscriptionID); revokeErr != nil {
						s.restoreStatus(ctx, p)
						return nil, fmt.Errorf("revoke subscription: %w", revokeErr)
					}
				} else {
					// Other errors (DB failure, not found) — abort refund
					s.restoreStatus(ctx, p)
					return nil, fmt.Errorf("deduct subscription days: %w", err)
				}
			}
		} else {
			slog.Warn("skipping subscription deduction on retry (previous rollback failed)", "orderID", p.OrderID)
			p.SubDaysToDeduct = 0
		}
	}
	resp, err := s.gwRefund(ctx, p)
	if err != nil {
		return s.handleGwFail(ctx, p, err)
	}
	return s.finishRefund(ctx, p, resp)
}

func (s *PaymentService) gwRefund(ctx context.Context, p *RefundPlan) (*payment.RefundResponse, error) {
	if p.Order.PaymentTradeNo == "" {
		s.writeAuditLog(ctx, p.Order.ID, "REFUND_NO_TRADE_NO", "admin", map[string]any{"detail": "skipped"})
		return &payment.RefundResponse{Status: payment.ProviderStatusSuccess}, nil
	}

	// Use the exact provider instance that created this order, not a random one
	// from the registry. Each instance has its own merchant credentials.
	prov, err := s.getRefundProvider(ctx, p.Order)
	if err != nil {
		return nil, fmt.Errorf("get refund provider: %w", err)
	}
	if err := validateProviderSnapshotMetadata(p.Order, prov.ProviderKey(), providerMerchantIdentityMetadata(prov)); err != nil {
		s.writeAuditLog(ctx, p.Order.ID, "REFUND_PROVIDER_METADATA_MISMATCH", "admin", map[string]any{
			"detail": err.Error(),
		})
		return nil, err
	}
	finishProviderCall := servertiming.ObserveDependency(ctx, "payment")
	resp, err := prov.Refund(ctx, payment.RefundRequest{
		TradeNo: p.Order.PaymentTradeNo,
		OrderID: p.Order.OutTradeNo,
		Amount:  formatGatewayRefundAmount(p.GatewayAmount, p.Order),
		Reason:  p.Reason,
	})
	finishProviderCall()
	if err != nil {
		if resp != nil && strings.TrimSpace(resp.Status) == payment.ProviderStatusPending {
			return resp, nil
		}
		return nil, err
	}
	if err := validateRefundProviderResponse(resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func formatGatewayRefundAmount(amount float64, order *dbent.PaymentOrder) string {
	return payment.FormatAmountForCurrency(amount, PaymentOrderCurrency(order))
}

func validateRefundProviderResponse(resp *payment.RefundResponse) error {
	if resp == nil {
		return fmt.Errorf("payment refund response missing")
	}
	status := strings.TrimSpace(resp.Status)
	switch status {
	case payment.ProviderStatusSuccess, payment.ProviderStatusRefunded, payment.ProviderStatusPending:
		return nil
	case payment.ProviderStatusFailed:
		return fmt.Errorf("payment refund failed: status %s", status)
	default:
		return fmt.Errorf("payment refund returned unknown status: %s", status)
	}
}

func (s *PaymentService) finishRefund(ctx context.Context, p *RefundPlan, resp *payment.RefundResponse) (*RefundResult, error) {
	if err := validateRefundProviderResponse(resp); err != nil {
		return s.handleGwFail(ctx, p, err)
	}
	switch strings.TrimSpace(resp.Status) {
	case payment.ProviderStatusSuccess, payment.ProviderStatusRefunded:
		return s.markRefundOk(ctx, p)
	case payment.ProviderStatusPending:
		return s.markRefundPending(ctx, p, resp)
	default:
		return s.handleGwFail(ctx, p, fmt.Errorf("payment refund returned unknown status: %s", strings.TrimSpace(resp.Status)))
	}
}

func (s *PaymentService) QueryAndFinalizeRefund(ctx context.Context, oid int64) (*RefundResult, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.OrderType == payment.OrderTypeMembership && o.Status == OrderStatusRefunded {
		if err := s.finalizeMembershipRefund(ctx, s.refundFinalizePlan(o)); err != nil {
			return nil, err
		}
		return &RefundResult{Success: true}, nil
	}
	if o.Status != OrderStatusRefundPending {
		return nil, infraerrors.BadRequest("INVALID_STATUS", "only refund pending orders can be finalized")
	}

	prov, err := s.getRefundProvider(ctx, o)
	if err != nil {
		return nil, fmt.Errorf("get refund provider: %w", err)
	}
	queryProvider, ok := prov.(payment.RefundQueryProvider)
	if !ok {
		return nil, infraerrors.BadRequest("REFUND_QUERY_UNSUPPORTED", "this payment provider does not support refund status query; please verify manually")
	}

	pendingDetail := s.latestRefundPendingDetail(ctx, oid)
	finishProviderCall := servertiming.ObserveDependency(ctx, "payment")
	resp, err := queryProvider.QueryRefund(ctx, payment.RefundQueryRequest{
		TradeNo:  o.PaymentTradeNo,
		OrderID:  o.OutTradeNo,
		RefundID: pendingDetail.RefundID,
		Amount:   formatGatewayRefundAmount(o.RefundAmount, o),
	})
	finishProviderCall()
	if err != nil {
		return nil, fmt.Errorf("query refund: %w", err)
	}
	if err := validateRefundProviderResponse(resp); err != nil {
		return s.finalizeRefundFailed(ctx, o, err)
	}

	plan := s.refundFinalizePlan(o)
	if !pendingDetail.DeductionRollbackOK {
		plan.BalanceToDeduct = 0
		plan.SubDaysToDeduct = 0
	} else if o.OrderType == payment.OrderTypeSubscription {
		if early := s.prepDeduct(ctx, o, plan, true); early != nil {
			return early, nil
		}
	}
	switch strings.TrimSpace(resp.Status) {
	case payment.ProviderStatusSuccess, payment.ProviderStatusRefunded:
		return s.finalizePendingRefundSuccess(ctx, plan)
	case payment.ProviderStatusPending:
		s.writeAuditLog(ctx, oid, "REFUND_QUERY_PENDING", "admin", map[string]any{"refundID": resp.RefundID})
		return &RefundResult{Success: false, Warning: "gateway refund is still pending confirmation"}, nil
	default:
		return s.finalizeRefundFailed(ctx, o, fmt.Errorf("payment refund returned unknown status: %s", strings.TrimSpace(resp.Status)))
	}
}

func (s *PaymentService) finalizePendingRefundSuccess(ctx context.Context, p *RefundPlan) (_ *RefundResult, err error) {
	if err := s.validateMembershipRefundPlan(p); err != nil {
		return nil, err
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin refund finalization: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	txCtx := dbent.NewTxContext(ctx, tx)

	claimed, err := tx.PaymentOrder.Update().
		Where(paymentorder.IDEQ(p.OrderID), paymentorder.StatusEQ(OrderStatusRefundPending)).
		SetStatus(OrderStatusRefunding).
		Save(txCtx)
	if err != nil {
		return nil, fmt.Errorf("claim pending refund: %w", err)
	}
	if claimed == 0 {
		return nil, infraerrors.Conflict("CONFLICT", "order status changed")
	}

	if err := s.applyRefundFinalDeduction(txCtx, p); err != nil {
		return nil, err
	}
	result, err := s.markRefundOkTx(txCtx, tx.Client(), p)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit refund finalization: %w", err)
	}
	if err := s.finalizeMembershipRefund(ctx, p); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *PaymentService) refundFinalizePlan(o *dbent.PaymentOrder) *RefundPlan {
	refundAmount := o.RefundAmount
	reason := strings.TrimSpace(psStringValue(o.RefundReason))
	if reason == "" {
		reason = fmt.Sprintf("refund order:%d", o.ID)
	}
	plan := &RefundPlan{
		OrderID:       o.ID,
		Order:         o,
		RefundAmount:  refundAmount,
		GatewayAmount: calculateGatewayRefundAmount(o.Amount, o.PayAmount, refundAmount, PaymentOrderCurrency(o)),
		Reason:        reason,
		Force:         o.ForceRefund,
		DeductBalance: true,
		DeductionType: payment.DeductionTypeBalance,
		BalanceToDeduct: func() float64 {
			if o.OrderType == payment.OrderTypeBalance {
				return refundAmount
			}
			return 0
		}(),
	}
	if o.OrderType == payment.OrderTypeMembership {
		plan.DeductBalance = false
		plan.DeductionType = payment.DeductionTypeNone
	}
	return plan
}

func (s *PaymentService) applyRefundFinalDeduction(ctx context.Context, p *RefundPlan) error {
	if p.DeductionType == payment.DeductionTypeBalance && p.BalanceToDeduct > 0 {
		deducted, err := s.deductAvailableBalance(ctx, p.Order.UserID, p.BalanceToDeduct)
		if err != nil {
			return fmt.Errorf("deduction: %w", err)
		}
		p.BalanceToDeduct = deducted
	}
	if p.DeductionType == payment.DeductionTypeSubscription && p.SubDaysToDeduct > 0 && p.SubscriptionID > 0 {
		if _, err := s.subscriptionSvc.ExtendSubscription(ctx, p.SubscriptionID, -p.SubDaysToDeduct); err != nil {
			if errors.Is(err, ErrAdjustWouldExpire) {
				if revokeErr := s.subscriptionSvc.RevokeSubscription(ctx, p.SubscriptionID); revokeErr != nil {
					return fmt.Errorf("revoke subscription: %w", revokeErr)
				}
			} else {
				return fmt.Errorf("deduct subscription days: %w", err)
			}
		}
	}
	return nil
}

func (s *PaymentService) finalizeRefundFailed(ctx context.Context, o *dbent.PaymentOrder, gErr error) (*RefundResult, error) {
	now := time.Now()
	updated, err := s.entClient.PaymentOrder.Update().Where(
		paymentorder.IDEQ(o.ID),
		paymentorder.StatusEQ(OrderStatusRefundPending),
	).SetStatus(OrderStatusRefundFailed).SetFailedAt(now).SetFailedReason(psErrMsg(gErr)).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("mark refund failed: %w", err)
	}
	if updated == 0 {
		return &RefundResult{Success: false, Warning: "refund status changed concurrently"}, nil
	}
	if o.OrderType == payment.OrderTypeMembership && s.membership != nil {
		if err := s.membership.AbortRefund(ctx, o.ID); err != nil {
			slog.Error("restore membership refund state failed", "orderID", o.ID, "error", err)
		}
	}
	s.writeAuditLog(ctx, o.ID, "REFUND_FAILED", "admin", map[string]any{"detail": psErrMsg(gErr)})
	return &RefundResult{Success: false, Warning: "gateway refund failed: " + psErrMsg(gErr)}, nil
}

type refundPendingAuditDetail struct {
	RefundID            string `json:"refundID"`
	DeductionRollbackOK bool   `json:"deductionRollbackOK"`
}

func (s *PaymentService) latestRefundPendingDetail(ctx context.Context, oid int64) refundPendingAuditDetail {
	logEntry, err := s.entClient.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(oid, 10)), paymentauditlog.ActionEQ("REFUND_PENDING")).
		Order(paymentauditlog.ByCreatedAt(sql.OrderDesc())).
		First(ctx)
	if err != nil || logEntry == nil {
		return refundPendingAuditDetail{DeductionRollbackOK: true}
	}
	detail := refundPendingAuditDetail{DeductionRollbackOK: true}
	_ = json.Unmarshal([]byte(logEntry.Detail), &detail)
	detail.RefundID = strings.TrimSpace(detail.RefundID)
	return detail
}

// getRefundProvider creates a provider using the order's original instance config.
// Delegates to getOrderProvider which handles instance lookup and fallback.
func (s *PaymentService) getRefundProvider(ctx context.Context, o *dbent.PaymentOrder) (payment.Provider, error) {
	inst, err := s.getRefundOrderProviderInstance(ctx, o)
	if err != nil {
		return nil, err
	}
	if inst == nil {
		return nil, fmt.Errorf("refund provider instance is unavailable for order %d", o.ID)
	}
	return s.createProviderFromInstance(ctx, inst)
}

func (s *PaymentService) handleGwFail(ctx context.Context, p *RefundPlan, gErr error) (*RefundResult, error) {
	if s.RollbackRefund(ctx, p, gErr) {
		if s.restoreStatus(ctx, p) {
			s.abortMembershipRefund(ctx, p)
		}
		s.writeAuditLog(ctx, p.OrderID, "REFUND_GATEWAY_FAILED", "admin", map[string]any{"detail": psErrMsg(gErr)})
		return &RefundResult{Success: false, Warning: "gateway failed: " + psErrMsg(gErr) + ", rolled back"}, nil
	}
	now := time.Now()
	updated, updateErr := s.entClient.PaymentOrder.Update().Where(
		paymentorder.IDEQ(p.OrderID),
		paymentorder.StatusEQ(OrderStatusRefunding),
	).SetStatus(OrderStatusRefundFailed).SetFailedAt(now).SetFailedReason(psErrMsg(gErr)).Save(ctx)
	if updateErr != nil {
		return nil, fmt.Errorf("mark refund failed: %w", updateErr)
	}
	if updated == 1 {
		s.abortMembershipRefund(ctx, p)
	}
	s.writeAuditLog(ctx, p.OrderID, "REFUND_FAILED", "admin", map[string]any{"detail": psErrMsg(gErr)})
	return nil, infraerrors.InternalServer("REFUND_FAILED", psErrMsg(gErr))
}

func (s *PaymentService) abortMembershipRefund(ctx context.Context, p *RefundPlan) {
	if p == nil || p.Order == nil || p.Order.OrderType != payment.OrderTypeMembership || s.membership == nil {
		return
	}
	if err := s.membership.AbortRefund(ctx, p.OrderID); err != nil {
		slog.Error("restore membership refund state failed", "orderID", p.OrderID, "error", err)
	}
}

// ReconcileMembershipRefundFinalizations retries the local membership commit
// after a gateway refund was durably recorded. FinalizeRefund is idempotent,
// so completed rows are harmless and rows left in refund_pending converge.
func (s *PaymentService) ReconcileMembershipRefundFinalizations(ctx context.Context) (int, error) {
	if s == nil || s.membership == nil || s.membership.DB == nil {
		return 0, nil
	}
	rows, err := s.membership.DB.QueryContext(ctx, `SELECT l.payment_order_id
        FROM membership_payment_links l
        JOIN membership_orders o ON o.id=l.order_id
        JOIN payment_orders po ON po.id=l.payment_order_id
        WHERE po.order_type='membership' AND po.status='REFUNDED'
          AND o.payment_state<>'refunded'
        ORDER BY po.updated_at,po.id
        LIMIT $1`, pendingPaymentReconcileLimit)
	if err != nil {
		return 0, fmt.Errorf("query refunded membership orders: %w", err)
	}
	defer rows.Close()
	var paymentIDs []int64
	for rows.Next() {
		var paymentID int64
		if err := rows.Scan(&paymentID); err != nil {
			return 0, fmt.Errorf("scan refunded membership order: %w", err)
		}
		paymentIDs = append(paymentIDs, paymentID)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate refunded membership orders: %w", err)
	}
	completed := 0
	for _, paymentID := range paymentIDs {
		if err := s.membership.FinalizeRefund(ctx, paymentID); err != nil {
			slog.Warn("membership refund finalization retry failed", "orderID", paymentID, "error", err)
			continue
		}
		completed++
	}
	return completed, nil
}

// ReconcileMembershipRefundAborts retries the local refund abort after a
// provider refund failed but the membership transaction was unavailable.
func (s *PaymentService) ReconcileMembershipRefundAborts(ctx context.Context) (int, error) {
	if s == nil || s.membership == nil || s.membership.DB == nil {
		return 0, nil
	}
	rows, err := s.membership.DB.QueryContext(ctx, `SELECT l.payment_order_id
        FROM membership_payment_links l
        JOIN membership_orders o ON o.id=l.order_id
        JOIN payment_orders po ON po.id=l.payment_order_id
        WHERE o.payment_state='refund_pending' AND po.order_type='membership'
	          AND po.status IN ('PAID','REFUND_FAILED')
        ORDER BY po.updated_at,po.id
        LIMIT $1`, pendingPaymentReconcileLimit)
	if err != nil {
		return 0, fmt.Errorf("query membership refund aborts: %w", err)
	}
	defer rows.Close()
	var paymentIDs []int64
	for rows.Next() {
		var paymentID int64
		if err := rows.Scan(&paymentID); err != nil {
			return 0, fmt.Errorf("scan membership refund abort: %w", err)
		}
		paymentIDs = append(paymentIDs, paymentID)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate membership refund aborts: %w", err)
	}
	recovered := 0
	for _, paymentID := range paymentIDs {
		if err := s.membership.AbortRefund(ctx, paymentID); err != nil {
			slog.Warn("membership refund abort retry failed", "orderID", paymentID, "error", err)
			continue
		}
		recovered++
	}
	return recovered, nil
}

// ReconcileMembershipRefunding recovers the crash window after a membership
// refund was claimed locally but before its provider result was recorded. It
// only queries the provider; it must never submit another refund.
func (s *PaymentService) ReconcileMembershipRefunding(ctx context.Context) (int, error) {
	if s == nil || s.entClient == nil || s.membership == nil || s.membership.DB == nil {
		return 0, nil
	}
	rows, err := s.membership.DB.QueryContext(ctx, `SELECT l.payment_order_id
        FROM membership_payment_links l
        JOIN membership_orders o ON o.id=l.order_id
        JOIN payment_orders po ON po.id=l.payment_order_id
        WHERE o.payment_state IN ('refund_pending','manual_review') AND po.order_type='membership'
	          AND po.status='REFUNDING'
        ORDER BY po.updated_at,po.id
        LIMIT $1`, pendingPaymentReconcileLimit)
	if err != nil {
		return 0, fmt.Errorf("query interrupted membership refunds: %w", err)
	}
	defer rows.Close()
	var paymentIDs []int64
	for rows.Next() {
		var paymentID int64
		if err := rows.Scan(&paymentID); err != nil {
			return 0, fmt.Errorf("scan interrupted membership refund: %w", err)
		}
		paymentIDs = append(paymentIDs, paymentID)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate interrupted membership refunds: %w", err)
	}

	recovered := 0
	for _, paymentID := range paymentIDs {
		if err := s.reconcileMembershipRefundingOrder(ctx, paymentID); err != nil {
			slog.Warn("membership interrupted refund reconciliation failed", "orderID", paymentID, "error", err)
			continue
		}
		recovered++
	}
	return recovered, nil
}

func (s *PaymentService) reconcileMembershipRefundingOrder(ctx context.Context, paymentID int64) error {
	o, err := s.entClient.PaymentOrder.Get(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("load interrupted refund order: %w", err)
	}
	if o.OrderType != payment.OrderTypeMembership || o.Status != OrderStatusRefunding {
		return nil
	}
	p := s.refundFinalizePlan(o)
	if strings.TrimSpace(o.PaymentTradeNo) == "" {
		_, err := s.markRefundOk(ctx, p)
		return err
	}

	prov, err := s.getRefundProvider(ctx, o)
	if err != nil {
		return s.markMembershipRefundManualReview(ctx, o, "refund provider unavailable: "+psErrMsg(err))
	}
	queryProvider, ok := prov.(payment.RefundQueryProvider)
	if !ok {
		return s.markMembershipRefundManualReview(ctx, o, "refund provider does not support status query")
	}
	detail := s.latestRefundPendingDetail(ctx, paymentID)
	finishProviderCall := servertiming.ObserveDependency(ctx, "payment")
	resp, err := queryProvider.QueryRefund(ctx, payment.RefundQueryRequest{
		TradeNo:  o.PaymentTradeNo,
		OrderID:  o.OutTradeNo,
		RefundID: detail.RefundID,
		Amount:   formatGatewayRefundAmount(o.RefundAmount, o),
	})
	finishProviderCall()
	if err != nil {
		return s.markMembershipRefundManualReview(ctx, o, "refund status query failed: "+psErrMsg(err))
	}
	if resp == nil {
		return s.markMembershipRefundManualReview(ctx, o, "refund status query returned no result")
	}

	switch strings.TrimSpace(resp.Status) {
	case payment.ProviderStatusSuccess, payment.ProviderStatusRefunded:
		_, err := s.markRefundOk(ctx, p)
		return err
	case payment.ProviderStatusPending:
		_, err := s.markRefundPending(ctx, p, resp)
		return err
	case payment.ProviderStatusFailed:
		_, err := s.finalizeRefundingFailed(ctx, o, fmt.Errorf("payment refund failed: provider status %s", resp.Status))
		return err
	default:
		return s.markMembershipRefundManualReview(ctx, o, "refund status query returned unknown status: "+strings.TrimSpace(resp.Status))
	}
}

func (s *PaymentService) finalizeRefundingFailed(ctx context.Context, o *dbent.PaymentOrder, gErr error) (*RefundResult, error) {
	updated, err := s.entClient.PaymentOrder.Update().Where(
		paymentorder.IDEQ(o.ID),
		paymentorder.StatusEQ(OrderStatusRefunding),
	).SetStatus(OrderStatusRefundFailed).SetFailedAt(time.Now()).SetFailedReason(psErrMsg(gErr)).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("mark interrupted refund failed: %w", err)
	}
	if updated == 0 {
		return &RefundResult{Success: false, Warning: "refund status changed concurrently"}, nil
	}
	s.abortMembershipRefund(ctx, s.refundFinalizePlan(o))
	s.writeAuditLog(ctx, o.ID, "REFUND_FAILED", "reconciler", map[string]any{"detail": psErrMsg(gErr)})
	return &RefundResult{Success: false, Warning: "gateway refund failed: " + psErrMsg(gErr)}, nil
}

// markMembershipRefundManualReview preserves the unsettled payment fact while
// moving the membership order to its visible review state. The membership side
// is committed first: a crash before the payment-order CAS remains selectable
// as manual_review + REFUNDING on the next periodic run. Keeping the payment
// order REFUND_PENDING prevents a later automatic retry from submitting a new
// upstream refund; PrepareRefund also checks the durable audit marker.
func (s *PaymentService) markMembershipRefundManualReview(ctx context.Context, o *dbent.PaymentOrder, detail string) error {
	if err := s.membership.MarkRefundManualReview(ctx, o.ID, detail); err != nil {
		return fmt.Errorf("mark membership refund for manual review: %w", err)
	}
	updated, err := s.entClient.PaymentOrder.Update().Where(
		paymentorder.IDEQ(o.ID),
		paymentorder.StatusEQ(OrderStatusRefunding),
	).SetStatus(OrderStatusRefundPending).SetFailedReason(detail).Save(ctx)
	if err != nil {
		return fmt.Errorf("mark interrupted refund for manual review: %w", err)
	}
	if updated == 0 {
		return nil
	}
	s.writeAuditLog(ctx, o.ID, "REFUND_MANUAL_REVIEW", "reconciler", map[string]any{"detail": detail})
	return nil
}

func (s *PaymentService) markRefundOk(ctx context.Context, p *RefundPlan) (*RefundResult, error) {
	if err := s.validateMembershipRefundPlan(p); err != nil {
		return nil, err
	}
	fs := OrderStatusRefunded
	if p.RefundAmount < p.Order.Amount {
		fs = OrderStatusPartiallyRefunded
	}
	now := time.Now()
	updated, err := s.entClient.PaymentOrder.Update().Where(
		paymentorder.IDEQ(p.OrderID),
		paymentorder.StatusEQ(OrderStatusRefunding),
	).SetStatus(fs).SetRefundAmount(p.RefundAmount).SetRefundReason(p.Reason).SetRefundAt(now).SetForceRefund(p.Force).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("mark refund: %w", err)
	}
	if updated == 0 {
		return nil, infraerrors.Conflict("CONFLICT", "order status changed")
	}
	s.writeAuditLog(ctx, p.OrderID, "REFUND_SUCCESS", "admin", map[string]any{"refundAmount": p.RefundAmount, "reason": p.Reason, "balanceDeducted": p.BalanceToDeduct, "force": p.Force})
	if err := s.finalizeMembershipRefund(ctx, p); err != nil {
		return nil, err
	}
	return &RefundResult{Success: true, BalanceDeducted: p.BalanceToDeduct, SubDaysDeducted: p.SubDaysToDeduct}, nil
}

func (s *PaymentService) markRefundOkTx(ctx context.Context, client *dbent.Client, p *RefundPlan) (*RefundResult, error) {
	if err := s.validateMembershipRefundPlan(p); err != nil {
		return nil, err
	}
	fs := OrderStatusRefunded
	if p.RefundAmount < p.Order.Amount {
		fs = OrderStatusPartiallyRefunded
	}
	now := time.Now()
	_, err := client.PaymentOrder.UpdateOneID(p.OrderID).SetStatus(fs).SetRefundAmount(p.RefundAmount).SetRefundReason(p.Reason).SetRefundAt(now).SetForceRefund(p.Force).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("mark refund: %w", err)
	}
	detail, err := json.Marshal(map[string]any{"refundAmount": p.RefundAmount, "reason": p.Reason, "balanceDeducted": p.BalanceToDeduct, "force": p.Force})
	if err != nil {
		return nil, fmt.Errorf("marshal refund audit: %w", err)
	}
	if _, err := client.PaymentAuditLog.Create().
		SetOrderID(strconv.FormatInt(p.OrderID, 10)).
		SetAction("REFUND_SUCCESS").
		SetDetail(string(detail)).
		SetOperator("admin").
		Save(ctx); err != nil {
		return nil, fmt.Errorf("write refund audit: %w", err)
	}
	return &RefundResult{Success: true, BalanceDeducted: p.BalanceToDeduct, SubDaysDeducted: p.SubDaysToDeduct}, nil
}

func (s *PaymentService) validateMembershipRefundPlan(p *RefundPlan) error {
	if p == nil || p.Order == nil {
		return infraerrors.BadRequest("INVALID_REFUND_PLAN", "refund plan is required")
	}
	if p.Order.OrderType != payment.OrderTypeMembership {
		return nil
	}
	if s.membership == nil {
		return infraerrors.Conflict("MEMBERSHIP_UNAVAILABLE", "membership refund awaiting reconciliation")
	}
	if p.Force || p.DeductBalance || p.DeductionType != payment.DeductionTypeNone || p.BalanceToDeduct != 0 || p.SubDaysToDeduct != 0 || p.SubscriptionID != 0 || math.Abs(p.RefundAmount-p.Order.Amount) > paymentAmountToleranceForCurrency(PaymentOrderCurrency(p.Order)) {
		return infraerrors.BadRequest("MEMBERSHIP_REFUND_REVIEW", "membership refunds require confirmed non-delivery and a full refund")
	}
	return nil
}

func (s *PaymentService) finalizeMembershipRefund(ctx context.Context, p *RefundPlan) error {
	if p == nil || p.Order == nil || p.Order.OrderType != payment.OrderTypeMembership {
		return nil
	}
	if s.membership == nil {
		return infraerrors.Conflict("MEMBERSHIP_UNAVAILABLE", "membership refund awaiting reconciliation")
	}
	if err := s.membership.FinalizeRefund(ctx, p.OrderID); err != nil {
		return fmt.Errorf("finalize membership refund: %w", err)
	}
	return nil
}

func (s *PaymentService) markRefundPending(ctx context.Context, p *RefundPlan, resp *payment.RefundResponse) (*RefundResult, error) {
	balanceDeducted := p.BalanceToDeduct
	subDaysDeducted := p.SubDaysToDeduct
	rollbackOK := s.RollbackRefund(ctx, p, nil)
	if rollbackOK {
		p.BalanceToDeduct = 0
		p.SubDaysToDeduct = 0
	}

	updated, err := s.entClient.PaymentOrder.Update().Where(
		paymentorder.IDEQ(p.OrderID),
		paymentorder.StatusEQ(OrderStatusRefunding),
	).
		SetStatus(OrderStatusRefundPending).
		SetRefundAmount(p.RefundAmount).
		SetRefundReason(p.Reason).
		ClearRefundAt().
		SetForceRefund(p.Force).
		ClearFailedAt().
		ClearFailedReason().
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("mark refund pending: %w", err)
	}
	if updated == 0 {
		return nil, infraerrors.Conflict("CONFLICT", "order status changed")
	}

	detail := map[string]any{
		"refundID":            refundResponseID(resp),
		"refundAmount":        p.RefundAmount,
		"reason":              p.Reason,
		"force":               p.Force,
		"balanceDeducted":     p.BalanceToDeduct,
		"subDaysDeducted":     p.SubDaysToDeduct,
		"balanceRolledBack":   balanceDeducted,
		"subDaysRolledBack":   subDaysDeducted,
		"deductionRollbackOK": rollbackOK,
	}
	s.writeAuditLog(ctx, p.OrderID, "REFUND_PENDING", "admin", detail)

	warning := "gateway refund is pending confirmation"
	if !rollbackOK {
		warning += "; refund deduction rollback failed"
	}
	return &RefundResult{Success: false, Warning: warning}, nil
}

func refundResponseID(resp *payment.RefundResponse) string {
	if resp == nil {
		return ""
	}
	return strings.TrimSpace(resp.RefundID)
}

func (s *PaymentService) RollbackRefund(ctx context.Context, p *RefundPlan, gErr error) bool {
	if p.DeductionType == payment.DeductionTypeBalance && p.BalanceToDeduct > 0 {
		if err := s.userRepo.UpdateBalance(ctx, p.Order.UserID, p.BalanceToDeduct); err != nil {
			slog.Error("[CRITICAL] rollback failed", "orderID", p.OrderID, "amount", p.BalanceToDeduct, "error", err)
			s.writeAuditLog(ctx, p.OrderID, "REFUND_ROLLBACK_FAILED", "admin", map[string]any{"gatewayError": psErrMsg(gErr), "rollbackError": psErrMsg(err), "balanceDeducted": p.BalanceToDeduct})
			return false
		}
	}
	if p.DeductionType == payment.DeductionTypeSubscription && p.SubDaysToDeduct > 0 && p.SubscriptionID > 0 {
		if _, err := s.subscriptionSvc.ExtendSubscription(ctx, p.SubscriptionID, p.SubDaysToDeduct); err != nil {
			slog.Error("[CRITICAL] subscription rollback failed", "orderID", p.OrderID, "subID", p.SubscriptionID, "days", p.SubDaysToDeduct, "error", err)
			s.writeAuditLog(ctx, p.OrderID, "REFUND_ROLLBACK_FAILED", "admin", map[string]any{"gatewayError": psErrMsg(gErr), "rollbackError": psErrMsg(err), "subDaysDeducted": p.SubDaysToDeduct})
			return false
		}
	}
	return true
}

func (s *PaymentService) restoreStatus(ctx context.Context, p *RefundPlan) bool {
	rs := OrderStatusCompleted
	if p.Order.OrderType == payment.OrderTypeMembership {
		rs = OrderStatusPaid
	} else if p.Order.Status == OrderStatusRefundRequested {
		rs = OrderStatusRefundRequested
	}
	updated, err := s.entClient.PaymentOrder.Update().Where(
		paymentorder.IDEQ(p.OrderID),
		paymentorder.StatusEQ(OrderStatusRefunding),
	).SetStatus(rs).Save(ctx)
	if err != nil {
		slog.Error("restore refund status failed", "orderID", p.OrderID, "error", err)
		return false
	}
	return updated == 1
}
