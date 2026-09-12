package membership

import (
	"context"
	"database/sql"
	"regexp"
	"time"

	"github.com/google/uuid"
)

var (
	evidencePattern              = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_:/.-]{7,199}$`)
	controlledEvidenceRefPattern = regexp.MustCompile(`^(evidence:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}|sha256:[0-9a-f]{64})$`)
)

func isControlledEvidenceRef(value string) bool {
	return controlledEvidenceRefPattern.MatchString(value)
}

type ProductUpdate struct {
	PriceMinor     int64  `json:"price_minor"`
	PeriodDays     int    `json:"period_days"`
	Channel        string `json:"channel"`
	CredentialMode string `json:"credential_mode"`
	ForSale        bool   `json:"for_sale"`
	Paused         bool   `json:"paused"`
	PollSeconds    int    `json:"poll_seconds"`
	WaitSeconds    int    `json:"wait_seconds"`
	ETAMinutes     int    `json:"eta_minutes"`
}

func (e *Engine) UpdateProduct(ctx context.Context, actor, sku string, in ProductUpdate) error {
	if in.PriceMinor < 0 || in.PriceMinor > 10000000 || in.PeriodDays < 1 || in.PeriodDays > 366 || in.PollSeconds < 5 || in.PollSeconds > 3600 || in.WaitSeconds < 60 || in.WaitSeconds > 86400 || in.ETAMinutes < 1 || in.ETAMinutes > 1440 {
		return ErrInvalid
	}
	if in.Channel != "gpt" && in.Channel != "gptpro" || in.CredentialMode != "account_id" && in.CredentialMode != "session" || in.Channel == "gptpro" && in.CredentialMode != "session" {
		return ErrInvalid
	}
	if in.ForSale {
		if !e.PaymentsEnabled {
			return ErrUnavailable
		}
		if err := e.Ready(ctx); err != nil {
			return err
		}
	}
	return e.transaction(ctx, func(tx *sql.Tx) error {
		var channel, mode string
		var days int
		if err := tx.QueryRowContext(ctx, `SELECT channel,credential_mode,period_days FROM membership_products WHERE sku=$1 FOR UPDATE`, sku).Scan(&channel, &mode, &days); err != nil {
			return err
		}
		changed := channel != in.Channel || mode != in.CredentialMode || days != in.PeriodDays
		if changed {
			var count int
			if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM membership_orders o LEFT JOIN membership_tasks t ON t.order_id=o.id WHERE o.sku=$1 AND o.payment_state NOT IN ('closed','refunded') AND COALESCE(t.state,'awaiting_input') NOT IN ('succeeded','failed','canceled')`, sku).Scan(&count); err != nil {
				return err
			}
			if count > 0 || in.ForSale {
				return ErrConflict
			}
		}
		res, err := tx.ExecContext(ctx, `UPDATE membership_products SET price_minor=$2,period_days=$3,channel=$4,credential_mode=$5,for_sale=$6,paused=$7,poll_seconds=$8,wait_seconds=$9,eta_minutes=$10,
            verified_at=CASE WHEN $11 THEN NULL ELSE verified_at END,verified_run_id=CASE WHEN $11 THEN NULL ELSE verified_run_id END,updated_at=now() WHERE sku=$1`, sku, in.PriceMinor, in.PeriodDays, in.Channel, in.CredentialMode, in.ForSale, in.Paused, in.PollSeconds, in.WaitSeconds, in.ETAMinutes, changed)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n != 1 {
			return ErrNotFound
		}
		return event(ctx, tx, "", "product_updated", actor, "", "", sku, false)
	})
}

func (e *Engine) CheckAvailability(ctx context.Context, actor, sku string) error {
	if e.Browser == nil {
		return ErrUnavailable
	}
	var channel string
	if err := e.DB.QueryRowContext(ctx, `SELECT channel FROM membership_products WHERE sku=$1`, sku).Scan(&channel); err != nil {
		return publicError(err)
	}
	out, err := e.Browser.Execute(ctx, BrowserInput{Operation: "checkAvailability", Channel: channel, SKU: sku})
	if err != nil || out.Available == nil || *out.Available < 0 {
		_, dbErr := e.DB.ExecContext(ctx, `UPDATE membership_products SET upstream_available=NULL,inventory_checked_at=NULL,paused=true WHERE sku=$1`, sku)
		if dbErr != nil {
			return dbErr
		}
		return ErrUnavailable
	}
	return e.transaction(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE membership_products SET upstream_available=$2,inventory_checked_at=now() WHERE sku=$1`, sku, *out.Available); err != nil {
			return err
		}
		return event(ctx, tx, "", "availability_checked", actor, "", "", sku, false)
	})
}

func (e *Engine) VerifyProduct(ctx context.Context, actor, sku, runID, evidence string) error {
	if !isControlledEvidenceRef(evidence) {
		return ErrInvalid
	}
	return e.transaction(ctx, func(tx *sql.Tx) error {
		var valid bool
		if err := tx.QueryRowContext(ctx, `SELECT o.kind='validation' AND t.state='succeeded' AND o.sku=p.sku AND o.channel=p.channel AND o.credential_mode=p.credential_mode AND o.period_days=p.period_days
            FROM membership_orders o JOIN membership_tasks t ON t.order_id=o.id JOIN membership_products p ON p.sku=$2 WHERE o.id=$1 FOR UPDATE OF p`, runID, sku).Scan(&valid); err != nil {
			return err
		}
		if !valid {
			return ErrConflict
		}
		if _, err := tx.ExecContext(ctx, `UPDATE membership_products SET verified_run_id=$2,verified_at=now(),failures=0,updated_at=now() WHERE sku=$1`, sku, runID); err != nil {
			return err
		}
		return event(ctx, tx, runID, "product_verified", actor, "succeeded", "", evidence, false)
	})
}

func (e *Engine) Review(ctx context.Context, actor, id, action, evidence string) error {
	if !isControlledEvidenceRef(evidence) {
		return ErrInvalid
	}
	if action != "query" && action != "confirm_success" && action != "confirm_not_submitted" && action != "retry" && action != "cancel" {
		return ErrInvalid
	}
	err := e.transaction(ctx, func(tx *sql.Tx) error {
		var task, state, cdk, ref string
		var lease *time.Time
		var payment, kind string
		var refund *time.Time
		if err := tx.QueryRowContext(ctx, `SELECT t.id,t.state,t.cdk_id,COALESCE(o.credential_ref,''),t.lease_until,o.payment_state,o.kind,o.refund_requested_at FROM membership_orders o JOIN membership_tasks t ON t.order_id=o.id WHERE o.id=$1 FOR UPDATE OF o,t`, id).Scan(&task, &state, &cdk, &ref, &lease, &payment, &kind, &refund); err != nil {
			return err
		}
		if lease != nil && lease.After(time.Now()) {
			return ErrConflict
		}
		if state == "succeeded" || state == "canceled" {
			return ErrConflict
		}
		var attemptState string
		err := tx.QueryRowContext(ctx, `SELECT state FROM membership_attempts WHERE task_id=$1 ORDER BY created_at DESC LIMIT 1`, task).Scan(&attemptState)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		next := state
		switch action {
		case "query":
			if attemptState == "" || attemptState == "not_submitted" {
				return ErrConflict
			}
			next = "processing"
		case "confirm_success":
			if attemptState == "" || attemptState == "not_submitted" {
				return ErrConflict
			}
			next = "succeeded"
			if _, err = tx.ExecContext(ctx, `UPDATE membership_attempts SET state='succeeded',updated_at=now() WHERE id=(SELECT id FROM membership_attempts WHERE task_id=$1 ORDER BY created_at DESC LIMIT 1)`, task); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `UPDATE membership_cdks SET state='spent' WHERE id=$1`, cdk); err != nil {
				return err
			}
			if kind == "customer" {
				if _, err = tx.ExecContext(ctx, `INSERT INTO membership_ledger(id,order_id,kind,amount_minor,reference) SELECT $1,$2,'supplier_cost',cost_minor,id::text FROM membership_cdks WHERE id=$3 ON CONFLICT DO NOTHING`, uuid.NewString(), id, cdk); err != nil {
					return err
				}
			}
		case "confirm_not_submitted":
			if state != "review_required" && state != "failed" {
				return ErrConflict
			}
			if _, err = tx.ExecContext(ctx, `UPDATE membership_attempts SET state='not_submitted',updated_at=now() WHERE task_id=$1 AND state<>'succeeded'`, task); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `UPDATE membership_cdks SET state='reserved' WHERE id=$1`, cdk); err != nil {
				return err
			}
			next = "review_required"
		case "retry":
			if attemptState != "" && attemptState != "not_submitted" {
				return ErrReview
			}
			if refund != nil || kind == "customer" && payment != "paid" {
				return ErrConflict
			}
			next = "awaiting_input"
		case "cancel":
			if attemptState != "" && attemptState != "not_submitted" {
				return ErrReview
			}
			next = "canceled"
			if _, err = tx.ExecContext(ctx, `UPDATE membership_cdks SET state='available',order_id=NULL WHERE id=$1`, cdk); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, `UPDATE membership_tasks SET state=$2,lease_token=NULL,lease_until=NULL,error_code='',deadline_at=CASE WHEN $2='processing' THEN now()+interval '10 minutes' ELSE deadline_at END,next_run_at=now(),updated_at=now() WHERE id=$1`, task, next); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE membership_orders SET input_required=CASE WHEN $2='awaiting_input' THEN true ELSE input_required END,updated_at=now() WHERE id=$1`, id, next); err != nil {
			return err
		}
		return event(ctx, tx, id, "review_"+action, actor, next, "", evidence, true)
	})
	if err != nil {
		return err
	}
	return e.CleanupCredentials(ctx)
}

func (e *Engine) RequestRefund(ctx context.Context, userID int64, id, reason string) error {
	// Reasons are codes, not free text, so credentials cannot enter a support log.
	if reason != "not_delivered" && reason != "wrong_plan" && reason != "cancel_request" {
		return ErrInvalid
	}
	err := e.transaction(ctx, func(tx *sql.Tx) error {
		var payment string
		if err := tx.QueryRowContext(ctx, `SELECT payment_state FROM membership_orders WHERE id=$1 AND user_id=$2 AND kind='customer' FOR UPDATE`, id, userID).Scan(&payment); err != nil {
			return err
		}
		if payment != "paid" {
			return ErrConflict
		}
		if _, err := tx.ExecContext(ctx, `UPDATE membership_orders SET refund_requested_at=COALESCE(refund_requested_at,now()),refund_reason=$2,updated_at=now() WHERE id=$1`, id, reason); err != nil {
			return err
		}
		// Freeze the worker before the payment team evaluates the refund. Any
		// attempt already outside our process remains review-only, never retried.
		if _, err := tx.ExecContext(ctx, `UPDATE membership_tasks SET state='review_required',error_code='REVIEW_REQUIRED',lease_token=NULL,lease_until=NULL,updated_at=now() WHERE order_id=$1 AND state NOT IN ('succeeded','failed','canceled')`, id); err != nil {
			return err
		}
		return event(ctx, tx, id, "refund_requested", actorID(userID), "", "", reason, true)
	})
	if err != nil {
		return err
	}
	return e.CleanupCredentials(ctx)
}

func (e *Engine) SaveCoupon(ctx context.Context, actor, code string, discount int64, max int, expires time.Time) error {
	if !evidencePattern.MatchString(code) || discount <= 0 || max <= 0 || !expires.After(time.Now()) {
		return ErrInvalid
	}
	return e.transaction(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO membership_coupons(code,discount_minor,max_uses,expires_at) VALUES($1,$2,$3,$4) ON CONFLICT(code) DO UPDATE SET discount_minor=EXCLUDED.discount_minor,max_uses=EXCLUDED.max_uses,expires_at=EXCLUDED.expires_at`, code, discount, max, expires); err != nil {
			return err
		}
		return event(ctx, tx, "", "coupon_updated", actor, "", "", code, false)
	})
}

type AdminStats struct {
	Total, Success, Review, Queued int
	MeanSeconds                    float64
}

// AdminOverview is an internal query result. The handler maps it to the
// closed AdminOverviewDTO response before it crosses the HTTP boundary.
type AdminOverview struct {
	Products        []Product
	Orders          []Order
	Stats           AdminStats
	Ledger          map[string]int64
	RuntimeReady    bool
	PaymentsEnabled bool
}

func (e *Engine) AdminOverview(ctx context.Context) (AdminOverview, error) {
	products, err := e.Products(ctx)
	if err != nil {
		return AdminOverview{}, err
	}
	orders, err := e.Orders(ctx, 0, true)
	if err != nil {
		return AdminOverview{}, err
	}
	var stats AdminStats
	if err = e.DB.QueryRowContext(ctx, `SELECT count(*),count(*) FILTER(WHERE state='succeeded'),count(*) FILTER(WHERE state='review_required'),count(*) FILTER(WHERE state IN ('queued','submitted','processing')),COALESCE(avg(extract(epoch FROM updated_at-created_at)) FILTER(WHERE state='succeeded'),0) FROM membership_tasks`).Scan(&stats.Total, &stats.Success, &stats.Review, &stats.Queued, &stats.MeanSeconds); err != nil {
		return AdminOverview{}, err
	}
	rows, err := e.DB.QueryContext(ctx, `SELECT kind,sum(amount_minor) FROM membership_ledger GROUP BY kind`)
	if err != nil {
		return AdminOverview{}, err
	}
	defer closeResource(rows)
	ledger := map[string]int64{}
	for rows.Next() {
		var kind string
		var amount int64
		if err = rows.Scan(&kind, &amount); err != nil {
			return AdminOverview{}, err
		}
		ledger[kind] = amount
	}
	if err = rows.Err(); err != nil {
		return AdminOverview{}, err
	}
	ledger["profit"] = ledger["payment"] - ledger["fee"] - ledger["supplier_cost"] - ledger["refund"]
	return AdminOverview{Products: products, Orders: orders, Stats: stats, Ledger: ledger, RuntimeReady: e.Ready(ctx) == nil, PaymentsEnabled: e.PaymentsEnabled}, nil
}
