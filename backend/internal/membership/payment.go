package membership

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

const membershipPaymentRedirectTicketTTL = 5 * time.Minute

type SQLTransaction interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

// CreatePaymentRedirectTicket stores only a digest of a short-lived random
// ticket. The raw ticket is returned once to build a same-site browser URL.
func (e *Engine) CreatePaymentRedirectTicket(ctx context.Context, userID int64, membershipOrderID string, paymentOrderID int64) (string, error) {
	if e == nil || e.DB == nil || userID <= 0 || strings.TrimSpace(membershipOrderID) == "" || paymentOrderID <= 0 {
		return "", ErrInvalid
	}
	return e.issuePaymentRedirectTicket(ctx, userID, membershipOrderID, paymentOrderID)
}

// ReissuePaymentRedirectTicket recovers a short-lived local payment link for
// the authenticated membership-order owner. The payment order is resolved
// server-side so a caller cannot bind a ticket to another payment.
func (e *Engine) ReissuePaymentRedirectTicket(ctx context.Context, userID int64, membershipOrderID string) (string, int64, error) {
	if e == nil || e.DB == nil || userID <= 0 || strings.TrimSpace(membershipOrderID) == "" {
		return "", 0, ErrInvalid
	}

	var paymentOrderID int64
	err := e.DB.QueryRowContext(ctx, `SELECT l.payment_order_id
        FROM membership_orders o
        JOIN membership_payment_links l ON l.order_id=o.id
        JOIN payment_orders po ON po.id=l.payment_order_id
        WHERE o.id=$1 AND o.user_id=$2 AND o.kind='customer'
          AND o.payment_state='pending' AND po.user_id=$2 AND po.order_type='membership'
          AND po.status='PENDING' AND po.expires_at>now()`, membershipOrderID, userID).Scan(&paymentOrderID)
	if err != nil {
		return "", 0, ErrUnavailable
	}
	ticket, err := e.issuePaymentRedirectTicket(ctx, userID, membershipOrderID, paymentOrderID)
	if err != nil {
		return "", 0, err
	}
	return ticket, paymentOrderID, nil
}

func (e *Engine) issuePaymentRedirectTicket(ctx context.Context, userID int64, membershipOrderID string, paymentOrderID int64) (string, error) {
	raw := make([]byte, 32)
	read := rand.Read
	if e.redirectTicketRandom != nil {
		read = e.redirectTicketRandom
	}
	if n, err := read(raw); err != nil || n != len(raw) {
		return "", ErrUnavailable
	}
	digest := sha256.Sum256(raw)
	// Cleanup is deliberately best-effort. A stale-ticket cleanup outage must
	// never strand a customer whose pending payment can still be resumed.
	_, _ = e.DB.ExecContext(ctx, `DELETE FROM membership_payment_redirect_tickets
        WHERE consumed_at IS NOT NULL OR expires_at < now()-interval '1 day'`)
	result, err := e.DB.ExecContext(ctx, `INSERT INTO membership_payment_redirect_tickets(ticket_hash,order_id,payment_order_id,user_id,expires_at)
        SELECT $1,o.id,l.payment_order_id,o.user_id,now()+($5 * interval '1 second')
        FROM membership_orders o
        JOIN membership_payment_links l ON l.order_id=o.id
        JOIN payment_orders po ON po.id=l.payment_order_id
        WHERE o.id=$2 AND o.user_id=$3 AND o.kind='customer'
          AND l.payment_order_id=$4 AND po.user_id=$3 AND po.order_type='membership'
          AND o.payment_state='pending' AND po.status='PENDING' AND po.expires_at>now()`, digest[:], membershipOrderID, userID, paymentOrderID, int(membershipPaymentRedirectTicketTTL.Seconds()))
	if err != nil {
		return "", err
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		return "", ErrUnavailable
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// ConsumePaymentRedirectTicket consumes a ticket before reading its provider
// target. A failed validation still commits consumption, so a token cannot be
// replayed if order state changes after the first navigation attempt.
func (e *Engine) ConsumePaymentRedirectTicket(ctx context.Context, ticket string) (string, error) {
	if e == nil || e.DB == nil {
		return "", ErrInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(ticket))
	if err != nil || len(raw) != 32 {
		return "", ErrUnavailable
	}
	digest := sha256.Sum256(raw)
	tx, err := e.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var membershipOrderID string
	var paymentOrderID, userID int64
	if err = tx.QueryRowContext(ctx, `UPDATE membership_payment_redirect_tickets
        SET consumed_at=now()
        WHERE ticket_hash=$1 AND consumed_at IS NULL AND expires_at>now()
        RETURNING order_id,payment_order_id,user_id`, digest[:]).Scan(&membershipOrderID, &paymentOrderID, &userID); err != nil {
		return "", publicError(err)
	}
	var payURL, qrCode sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT po.pay_url,po.qr_code
        FROM membership_orders o
        JOIN membership_payment_links l ON l.order_id=o.id
        JOIN payment_orders po ON po.id=l.payment_order_id
        WHERE o.id=$1 AND o.user_id=$2 AND o.kind='customer'
          AND l.payment_order_id=$3 AND po.user_id=$2 AND po.order_type='membership'
		  AND o.payment_state='pending' AND po.status='PENDING' AND po.expires_at>now()
        FOR UPDATE OF o,po`, membershipOrderID, userID, paymentOrderID).Scan(&payURL, &qrCode)
	if err == nil {
		var targetErr error
		payURL.String, targetErr = approvedPaymentRedirectTarget(payURL.String, qrCode.String)
		if targetErr != nil {
			err = targetErr
		}
	}
	if commitErr := tx.Commit(); commitErr != nil {
		return "", commitErr
	}
	if err != nil {
		return "", publicError(err)
	}
	return payURL.String, nil
}

func approvedPaymentRedirectTarget(payURL, qrCode string) (string, error) {
	for _, candidate := range []string{payURL, qrCode} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		parsed, err := url.ParseRequestURI(candidate)
		if err != nil || parsed.Host == "" || parsed.User != nil {
			return "", ErrUnavailable
		}
		switch strings.ToLower(parsed.Scheme) {
		case "http", "https":
			return parsed.String(), nil
		default:
			return "", ErrUnavailable
		}
	}
	return "", ErrUnavailable
}

func (e *Engine) PaymentQuote(ctx context.Context, userID int64, id string) (int64, error) {
	if err := e.Ready(ctx); err != nil {
		return 0, err
	}
	if !e.PaymentsEnabled {
		return 0, ErrUnavailable
	}
	var amount int64
	var allowed bool
	err := e.DB.QueryRowContext(ctx, `SELECT o.price_minor,p.for_sale AND NOT p.paused AND p.verified_at IS NOT NULL AND p.inventory_checked_at>now()-interval '2 minutes' AND p.upstream_available>0 AND o.credential_expires_at>now() AND o.consent_version=$3
        FROM membership_orders o JOIN membership_products p ON p.sku=o.sku WHERE o.id=$1 AND o.user_id=$2 AND o.kind='customer' AND o.payment_state='created'`, id, userID, ConsentVersion).Scan(&amount, &allowed)
	if err != nil {
		return 0, publicError(err)
	}
	if !allowed {
		return 0, ErrUnavailable
	}
	return amount, nil
}

func (e *Engine) AttachPayment(ctx context.Context, tx SQLTransaction, userID, paymentID int64, id string, amount float64) error {
	// The link and payment row commit together, before the gateway is invoked.
	res, err := tx.ExecContext(ctx, `UPDATE membership_orders o SET payment_state='pending',updated_at=now() FROM membership_products p
        WHERE o.id=$1 AND o.user_id=$2 AND o.sku=p.sku AND o.kind='customer' AND o.payment_state='created' AND o.price_minor=$3
        AND o.credential_expires_at>now() AND o.consent_version=$4 AND p.for_sale AND NOT p.paused AND p.verified_at IS NOT NULL
        AND p.inventory_checked_at>now()-interval '2 minutes' AND p.upstream_available>0
        AND EXISTS(SELECT 1 FROM membership_cdks c WHERE c.order_id=o.id AND c.state='reserved')`, id, userID, int64(math.Round(amount*100)), ConsentVersion)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return ErrConflict
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO membership_payment_links(payment_order_id,order_id) VALUES($1,$2)`, paymentID, id); err != nil {
		return err
	}
	return event(ctx, tx, id, "payment_pending", actorID(userID), "pending", "", "", false)
}

// RecoverPaymentCreation restores the membership reservation after checkout
// creation fails. A provider call can fail after the provider accepted it, so
// only failures known to have happened before that call release inventory.
func (e *Engine) RecoverPaymentCreation(ctx context.Context, paymentID int64, uncertain bool) error {
	return e.transaction(ctx, func(tx *sql.Tx) error {
		var id, state string
		if err := tx.QueryRowContext(ctx, `SELECT o.id,o.payment_state FROM membership_payment_links l JOIN membership_orders o ON o.id=l.order_id WHERE l.payment_order_id=$1 FOR UPDATE OF o`, paymentID).Scan(&id, &state); err != nil {
			return err
		}
		if state != "pending" {
			return nil
		}
		if uncertain {
			if _, err := tx.ExecContext(ctx, `UPDATE membership_orders SET payment_state='manual_review',updated_at=now() WHERE id=$1 AND payment_state='pending'`, id); err != nil {
				return err
			}
			return event(ctx, tx, id, "payment_creation_unknown", "payment_gateway", "manual_review", "", "", true)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE membership_orders SET payment_state='closed',credential_ref=NULL,credential_expires_at=NULL,input_required=true,updated_at=now() WHERE id=$1 AND payment_state='pending'`, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE membership_cdks SET state='available',order_id=NULL WHERE order_id=$1 AND state='reserved'`, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE membership_coupons SET used=used-1 WHERE code=(SELECT coupon_code FROM membership_orders WHERE id=$1) AND used>0`, id); err != nil {
			return err
		}
		return event(ctx, tx, id, "payment_creation_failed", "payment_gateway", "closed", "", "", true)
	})
}

// ConfirmPayment is only called after the payment provider's signature, merchant,
// currency and amount have been verified by the existing payment service.
func (e *Engine) ConfirmPayment(ctx context.Context, paymentID int64, trade string) error {
	return e.transaction(ctx, func(tx *sql.Tx) error {
		var id, state, channel, orderState string
		var coupon sql.NullString
		var inputRequired bool
		var amount float64
		var price int64
		var target sql.NullString
		if err := tx.QueryRowContext(ctx, `SELECT o.id,o.payment_state,o.channel,po.status,o.coupon_code,o.input_required,po.pay_amount,o.price_minor,o.target_hash
            FROM membership_payment_links l JOIN membership_orders o ON o.id=l.order_id JOIN payment_orders po ON po.id=l.payment_order_id
			WHERE l.payment_order_id=$1 AND po.order_type='membership' FOR UPDATE OF o,po`, paymentID).Scan(&id, &state, &channel, &orderState, &coupon, &inputRequired, &amount, &price, &target); err != nil {
			return err
		}
		if state == "refund_pending" || state == "refunded" {
			return nil
		}
		if state == "paid" && orderState == "PAID" {
			return nil
		}
		if state != "pending" && state != "closed" && state != "manual_review" && state != "paid" {
			return ErrConflict
		}
		res, err := tx.ExecContext(ctx, `UPDATE payment_orders SET status='PAID',paid_at=COALESCE(paid_at,now()),payment_trade_no=$2,updated_at=now() WHERE id=$1 AND status IN ('PENDING','CANCELLED','EXPIRED','FAILED')`, paymentID, trade)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n != 1 && orderState != "PAID" {
			return ErrConflict
		}
		if n == 0 || state == "paid" {
			return nil
		}
		next := "manual_review"
		var cdk sql.NullString
		if state == "pending" {
			err = tx.QueryRowContext(ctx, `SELECT id FROM membership_cdks WHERE order_id=$1 AND state='reserved' FOR UPDATE`, id).Scan(&cdk)
			if err != nil && err != sql.ErrNoRows {
				return err
			}
			if cdk.Valid {
				next = "paid"
			}
		}
		if state == "closed" && coupon.Valid {
			res, err := tx.ExecContext(ctx, `UPDATE membership_coupons SET used=used+1 WHERE code=$1`, coupon.String)
			if err != nil {
				return err
			}
			n, err := res.RowsAffected()
			if err != nil {
				return err
			}
			if n != 1 {
				return ErrConflict
			}
		}
		if _, err = tx.ExecContext(ctx, `UPDATE membership_orders SET payment_state=$2,paid_at=COALESCE(paid_at,now()),updated_at=now() WHERE id=$1`, id, next); err != nil {
			return err
		}
		if next == "paid" {
			taskState := "queued"
			taskTarget := target
			if inputRequired || !target.Valid {
				taskState = "awaiting_input"
			}
			// A different active order for this target requires review; payment remains paid.
			if target.Valid {
				if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "membership:target:"+target.String); err != nil {
					return err
				}
				var count int
				if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM membership_tasks WHERE target_hash=$1 AND state NOT IN ('succeeded','failed','canceled')`, target.String).Scan(&count); err != nil {
					return err
				}
				if count > 0 {
					taskState = "review_required"
					taskTarget = sql.NullString{}
				}
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO membership_tasks(id,order_id,state,channel,cdk_id,target_hash) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(order_id) DO NOTHING`, uuid.NewString(), id, taskState, channel, cdk.String, taskTarget); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO membership_ledger(id,order_id,kind,amount_minor,reference) VALUES($1,$2,'payment',$3,$4) ON CONFLICT DO NOTHING`, uuid.NewString(), id, int64(math.Round(amount*100)), actorID(paymentID)); err != nil {
			return err
		}
		return event(ctx, tx, id, "payment_confirmed", "payment_webhook", next, "", "", true)
	})
}

// AbortRefund returns a prepared membership refund to a paid, manually
// reviewable state after the payment gateway did not complete the refund.
// It never queues the original fulfillment task again.
func (e *Engine) AbortRefund(ctx context.Context, paymentID int64) error {
	return e.transaction(ctx, func(tx *sql.Tx) error {
		var id, state string
		if err := tx.QueryRowContext(ctx, `SELECT o.id,o.payment_state FROM membership_payment_links l JOIN membership_orders o ON o.id=l.order_id WHERE l.payment_order_id=$1 FOR UPDATE OF o`, paymentID).Scan(&id, &state); err != nil {
			return err
		}
		if state != "refund_pending" {
			return nil
		}
		if _, err := tx.ExecContext(ctx, `UPDATE membership_orders SET payment_state='paid',updated_at=now() WHERE id=$1 AND payment_state='refund_pending'`, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE membership_tasks SET state='review_required',lease_token=NULL,lease_until=NULL,updated_at=now() WHERE order_id=$1 AND state='canceled'`, id); err != nil {
			return err
		}
		return event(ctx, tx, id, "refund_aborted", "payment_gateway", "paid", "REFUND_FAILED", "", true)
	})
}

// MarkRefundManualReview makes an interrupted refund visible without claiming
// that the gateway refund failed or restoring fulfillment automatically.
func (e *Engine) MarkRefundManualReview(ctx context.Context, paymentID int64, evidence string) error {
	return e.transaction(ctx, func(tx *sql.Tx) error {
		var id, state string
		if err := tx.QueryRowContext(ctx, `SELECT o.id,o.payment_state FROM membership_payment_links l JOIN membership_orders o ON o.id=l.order_id WHERE l.payment_order_id=$1 FOR UPDATE OF o`, paymentID).Scan(&id, &state); err != nil {
			return err
		}
		if state != "refund_pending" {
			return nil
		}
		if _, err := tx.ExecContext(ctx, `UPDATE membership_orders SET payment_state='manual_review',updated_at=now() WHERE id=$1 AND payment_state='refund_pending'`, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE membership_tasks SET state='review_required',lease_token=NULL,lease_until=NULL,updated_at=now() WHERE order_id=$1 AND state='canceled'`, id); err != nil {
			return err
		}
		return event(ctx, tx, id, "refund_manual_review", "payment_gateway", "manual_review", "REFUND_REVIEW", evidence, true)
	})
}

func (e *Engine) PrepareRefund(ctx context.Context, paymentID int64) error {
	return e.transaction(ctx, func(tx *sql.Tx) error {
		var id, payment string
		var task, state sql.NullString
		var lease sql.NullTime
		if err := tx.QueryRowContext(ctx, `SELECT o.id,o.payment_state,t.id,t.state,t.lease_until FROM membership_payment_links l JOIN membership_orders o ON o.id=l.order_id LEFT JOIN membership_tasks t ON t.order_id=o.id WHERE l.payment_order_id=$1 FOR UPDATE OF o`, paymentID).Scan(&id, &payment, &task, &state, &lease); err != nil {
			return err
		}
		if payment != "paid" && payment != "manual_review" && payment != "refund_pending" {
			return ErrConflict
		}
		if lease.Valid && lease.Time.After(time.Now()) {
			return ErrConflict
		}
		var uncertain int
		if task.Valid {
			if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM membership_attempts WHERE task_id=$1 AND state<>'not_submitted'`, task.String).Scan(&uncertain); err != nil {
				return err
			}
		}
		if uncertain > 0 || state.String == "succeeded" {
			return ErrReview
		}
		if _, err := tx.ExecContext(ctx, `UPDATE membership_orders SET payment_state='refund_pending',refund_requested_at=COALESCE(refund_requested_at,now()),updated_at=now() WHERE id=$1`, id); err != nil {
			return err
		}
		if task.Valid {
			if _, err := tx.ExecContext(ctx, `UPDATE membership_tasks SET state='canceled',updated_at=now() WHERE id=$1`, task.String); err != nil {
				return err
			}
		}
		return event(ctx, tx, id, "refund_prepared", "admin", "refund_pending", "", "", false)
	})
}

func (e *Engine) FinalizeRefund(ctx context.Context, paymentID int64) error {
	return e.transaction(ctx, func(tx *sql.Tx) error {
		var id, state string
		var amount float64
		if err := tx.QueryRowContext(ctx, `SELECT o.id,po.status,po.refund_amount FROM membership_payment_links l JOIN membership_orders o ON o.id=l.order_id JOIN payment_orders po ON po.id=l.payment_order_id WHERE l.payment_order_id=$1 FOR UPDATE OF o`, paymentID).Scan(&id, &state, &amount); err != nil {
			return err
		}
		if state != "REFUNDED" {
			return ErrReview
		}
		res, err := tx.ExecContext(ctx, `UPDATE membership_orders SET payment_state='refunded',updated_at=now() WHERE id=$1 AND payment_state<>'refunded'`, id)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return nil
		}
		if _, err = tx.ExecContext(ctx, `UPDATE membership_cdks SET order_id=NULL,state='available' WHERE order_id=$1 AND state='reserved'`, id); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO membership_ledger(id,order_id,kind,amount_minor,reference) VALUES($1,$2,'refund',$3,$4) ON CONFLICT DO NOTHING`, uuid.NewString(), id, int64(math.Round(amount*100)), actorID(paymentID)); err != nil {
			return err
		}
		return event(ctx, tx, id, "refund_completed", "payment_webhook", "refunded", "", "", true)
	})
}
