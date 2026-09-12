package membership

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

type Engine struct {
	DB                   *sql.DB
	Keys                 *Keyring
	Vault                Vault
	Browser              Browser
	Audit                AuditSink
	Notify               Notifier
	Enabled              bool
	PaymentsEnabled      bool
	cancel               context.CancelFunc
	wg                   sync.WaitGroup
	credentialRedis      *redis.Client
	redirectTicketRandom func([]byte) (int, error)
	startOnce            sync.Once
	stopOnce             sync.Once
}

func publicError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	var p *pq.Error
	if errors.As(err, &p) && (p.Code == "23505" || p.Code == "23514") {
		return ErrConflict
	}
	return err
}

func (e *Engine) Ready(ctx context.Context) error {
	if !e.Enabled || e.Keys == nil || e.Vault == nil || e.Browser == nil || e.Audit == nil {
		return ErrUnavailable
	}
	return e.Vault.Ready(ctx)
}

func (e *Engine) transaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := e.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackTx(tx)
	if err = fn(tx); err != nil {
		return publicError(err)
	}
	return publicError(tx.Commit())
}

func event(ctx context.Context, tx interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, orderID, action, actor, state, code, evidence string, notify bool) error {
	id := uuid.NewString()
	_, err := tx.ExecContext(ctx, `INSERT INTO membership_events(id,order_id,action,actor,state,error_code,evidence_ref)
        VALUES($1,NULLIF($2,'')::uuid,$3,$4,$5,$6,$7)`, id, orderID, action, actor, state, code, evidence)
	if err != nil {
		return err
	}
	if notify && orderID != "" {
		_, err = tx.ExecContext(ctx, `INSERT INTO membership_notifications(event_id,order_id) VALUES($1,$2)`, id, orderID)
	}
	return err
}

func (e *Engine) Products(ctx context.Context) ([]Product, error) {
	rows, err := e.DB.QueryContext(ctx, `SELECT p.sku,p.name,p.price_minor,p.currency,p.period_days,p.credential_mode,p.for_sale,
        LEAST(COALESCE(p.upstream_available,0), (SELECT count(*) FROM membership_cdks c WHERE c.sku=p.sku AND c.channel=p.channel AND c.state='available')),
        p.eta_minutes,p.channel,p.paused,p.verified_at,p.inventory_checked_at,p.poll_seconds,p.wait_seconds
        FROM membership_products p ORDER BY p.sku`)
	if err != nil {
		return nil, err
	}
	defer closeResource(rows)
	out := []Product{}
	ready := e.Ready(ctx) == nil && e.PaymentsEnabled
	for rows.Next() {
		var p Product
		if err = rows.Scan(&p.SKU, &p.Name, &p.PriceMinor, &p.Currency, &p.PeriodDays, &p.CredentialMode, &p.ForSale, &p.Available, &p.ETAMinutes, &p.Channel, &p.Paused, &p.VerifiedAt, &p.InventoryCheckedAt, &p.PollSeconds, &p.WaitSeconds); err != nil {
			return nil, err
		}
		p.ForSale = ready && p.ForSale && !p.Paused && p.VerifiedAt != nil && p.InventoryCheckedAt != nil && time.Since(*p.InventoryCheckedAt) < 2*time.Minute && p.Available > 0
		out = append(out, p)
	}
	return out, rows.Err()
}

type CreateInput struct {
	SKU            string `json:"sku"`
	IdempotencyKey string `json:"idempotency_key"`
	Coupon         string `json:"coupon"`
}

func (e *Engine) Create(ctx context.Context, userID int64, input CreateInput, validation bool) (string, error) {
	if userID <= 0 || len(input.IdempotencyKey) < 8 || len(input.IdempotencyKey) > 128 {
		return "", ErrInvalid
	}
	var existing string
	err := e.DB.QueryRowContext(ctx, `SELECT id FROM membership_orders WHERE user_id=$1 AND idempotency_key=$2 AND sku=$3 AND kind=$4`, userID, input.IdempotencyKey, input.SKU, kind(validation)).Scan(&existing)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if err = e.Ready(ctx); err != nil {
		return "", err
	}
	id := uuid.NewString()
	err = e.transaction(ctx, func(tx *sql.Tx) error {
		var channel, mode string
		var days int
		var price int64
		var allowed bool
		if err := tx.QueryRowContext(ctx, `SELECT channel,credential_mode,period_days,price_minor,
            for_sale AND NOT paused AND verified_at IS NOT NULL AND inventory_checked_at>now()-interval '2 minutes' AND upstream_available>0
            FROM membership_products WHERE sku=$1 FOR UPDATE`, input.SKU).Scan(&channel, &mode, &days, &price, &allowed); err != nil {
			return err
		}
		if !validation && (!e.PaymentsEnabled || !allowed || price <= 0) {
			return ErrUnavailable
		}
		var stock int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM membership_cdks WHERE sku=$1 AND channel=$2 AND state='available'`, input.SKU, channel).Scan(&stock); err != nil {
			return err
		}
		if stock == 0 {
			return ErrUnavailable
		}
		var discount int64
		if validation {
			price = 0
			input.Coupon = ""
		}
		if input.Coupon != "" {
			if err := tx.QueryRowContext(ctx, `UPDATE membership_coupons SET used=used+1 WHERE code=$1 AND enabled AND expires_at>now() AND used<max_uses RETURNING discount_minor`, input.Coupon).Scan(&discount); err != nil {
				return ErrInvalid
			}
			if discount >= price {
				return ErrInvalid
			}
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO membership_orders(id,user_id,sku,kind,idempotency_key,price_minor,discount_minor,coupon_code,period_days,channel,credential_mode)
            VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),$9,$10,$11)`, id, userID, input.SKU, kind(validation), input.IdempotencyKey, price-discount, discount, input.Coupon, days, channel, mode)
		if err != nil {
			return err
		}
		// Reserve local stock before accepting payment, including validation runs.
		_, err = tx.ExecContext(ctx, `UPDATE membership_cdks SET state='reserved',order_id=$1 WHERE id=(SELECT id FROM membership_cdks WHERE sku=$2 AND channel=$3 AND state='available' ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1)`, id, input.SKU, channel)
		if err != nil {
			return err
		}
		var reserved int
		if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM membership_cdks WHERE order_id=$1`, id).Scan(&reserved); err != nil {
			return err
		}
		if reserved != 1 {
			return ErrUnavailable
		}
		return event(ctx, tx, id, "order_created", strconv.FormatInt(userID, 10), "awaiting_input", "", "", false)
	})
	if errors.Is(err, ErrConflict) {
		if e.DB.QueryRowContext(ctx, `SELECT id FROM membership_orders WHERE user_id=$1 AND idempotency_key=$2 AND sku=$3 AND kind=$4`, userID, input.IdempotencyKey, input.SKU, kind(validation)).Scan(&existing) == nil {
			return existing, nil
		}
	}
	return id, err
}

func kind(validation bool) string {
	if validation {
		return "validation"
	}
	return "customer"
}

func parseCredential(input CredentialInput) (Credential, error) {
	if !input.Consent || input.ConsentVersion != ConsentVersion {
		return Credential{}, ErrConsent
	}
	c := Credential{Mode: input.Mode, Value: strings.TrimSpace(input.Value), AccountID: strings.ToLower(strings.TrimSpace(input.AccountID))}
	switch c.Mode {
	case "account_id":
		c.AccountID = strings.ToLower(c.Value)
	case "session":
		var session struct {
			AccessToken string `json:"accessToken"`
			Account     struct {
				ID string `json:"id"`
			} `json:"account"`
		}
		if len(c.Value) > 65536 || json.Unmarshal([]byte(c.Value), &session) != nil || session.AccessToken == "" || session.Account.ID == "" {
			return Credential{}, ErrInvalid
		}
		embedded := strings.ToLower(session.Account.ID)
		if c.AccountID != "" && c.AccountID != embedded {
			return Credential{}, ErrInvalid
		}
		c.AccountID = embedded
	default:
		return Credential{}, ErrInvalid
	}
	if len(c.AccountID) < 8 || len(c.AccountID) > 128 {
		return Credential{}, ErrInvalid
	}
	for _, r := range c.AccountID {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' && r != '_' {
			return Credential{}, ErrInvalid
		}
	}
	return c, nil
}

func (e *Engine) PutCredential(ctx context.Context, userID int64, id string, input CredentialInput) error {
	if err := e.Ready(ctx); err != nil {
		return err
	}
	c, err := parseCredential(input)
	if err != nil {
		return err
	}
	ref := uuid.NewString()
	old := ""
	if err = e.Vault.Put(ctx, ref, c); err != nil {
		return ErrUnavailable
	}
	err = e.transaction(ctx, func(tx *sql.Tx) error {
		var mode, paymentState, channel, orderKind string
		var target sql.NullString
		var oldRef sql.NullString
		if err := tx.QueryRowContext(ctx, `SELECT credential_mode,payment_state,channel,kind,target_hash,credential_ref FROM membership_orders WHERE id=$1 AND user_id=$2 FOR UPDATE`, id, userID).Scan(&mode, &paymentState, &channel, &orderKind, &target, &oldRef); err != nil {
			return err
		}
		old = oldRef.String
		if mode != c.Mode || paymentState == "closed" || paymentState == "refunded" || paymentState == "refund_pending" {
			return ErrConflict
		}
		hash := e.Keys.Fingerprint("account", c.AccountID)
		if target.Valid && target.String != hash {
			return ErrConflict
		}
		// The partial unique index is the final guard. The transaction lock makes
		// the conflict deterministic before replacing a customer's credential.
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "membership:target:"+hash); err != nil {
			return err
		}
		var active int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM membership_tasks WHERE target_hash=$1 AND order_id<>$2 AND state NOT IN ('succeeded','failed','canceled')`, hash, id).Scan(&active); err != nil {
			return err
		}
		if active != 0 {
			return ErrConflict
		}
		var state string
		err := tx.QueryRowContext(ctx, `SELECT state FROM membership_tasks WHERE order_id=$1 FOR UPDATE`, id).Scan(&state)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if state == "succeeded" || state == "failed" || state == "canceled" {
			return ErrConflict
		}
		masked := c.AccountID[:4] + "..." + c.AccountID[len(c.AccountID)-4:]
		_, err = tx.ExecContext(ctx, `UPDATE membership_orders SET target_hash=$2,target_masked=$3,credential_ref=$4,credential_expires_at=now()+interval '30 minutes',consent_version=$5,consent_at=now(),input_required=false,updated_at=now() WHERE id=$1`, id, hash, masked, ref, ConsentVersion)
		if err != nil {
			return err
		}
		if orderKind == "validation" || paymentState == "paid" {
			if _, err = tx.ExecContext(ctx, `INSERT INTO membership_tasks(id,order_id,state,channel,cdk_id,target_hash) SELECT $1,o.id,'queued',o.channel,c.id,$3 FROM membership_orders o JOIN membership_cdks c ON c.order_id=o.id WHERE o.id=$2 ON CONFLICT(order_id) DO NOTHING`, uuid.NewString(), id, hash); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `UPDATE membership_tasks SET target_hash=$2,state=CASE WHEN state='awaiting_input' THEN 'queued' ELSE state END,next_run_at=now(),updated_at=now() WHERE order_id=$1`, id, hash); err != nil {
				return err
			}
		}
		return event(ctx, tx, id, "credential_received", strconv.FormatInt(userID, 10), state, "", "", false)
	})
	if err != nil {
		_ = e.Vault.Delete(ctx, ref)
		return err
	}
	return e.Vault.Delete(ctx, old)
}

const orderSelect = `SELECT o.id,o.sku,p.name,o.kind,o.payment_state,COALESCE(t.state,'awaiting_input'),o.price_minor,o.discount_minor,o.period_days,o.target_masked,o.credential_mode,o.input_required,COALESCE(t.error_code,''),o.refund_requested_at,o.created_at,o.updated_at FROM membership_orders o JOIN membership_products p ON p.sku=o.sku LEFT JOIN membership_tasks t ON t.order_id=o.id `

type scanner interface{ Scan(...any) error }

func scanOrder(row scanner) (Order, error) {
	var o Order
	err := row.Scan(&o.ID, &o.SKU, &o.Name, &o.Kind, &o.PaymentState, &o.FulfillmentState, &o.PriceMinor, &o.DiscountMinor, &o.PeriodDays, &o.TargetMasked, &o.CredentialMode, &o.InputRequired, &o.ErrorCode, &o.RefundRequestedAt, &o.CreatedAt, &o.UpdatedAt)
	return o, publicError(err)
}

func (e *Engine) Orders(ctx context.Context, userID int64, admin bool) ([]Order, error) {
	rows, err := e.DB.QueryContext(ctx, orderSelect+`WHERE ($1 OR (o.user_id=$2 AND o.kind='customer')) ORDER BY o.created_at DESC LIMIT 100`, admin, userID)
	if err != nil {
		return nil, err
	}
	defer closeResource(rows)
	out := []Order{}
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
func (e *Engine) Order(ctx context.Context, userID int64, id string, admin bool) (Order, error) {
	o, err := scanOrder(e.DB.QueryRowContext(ctx, orderSelect+`WHERE o.id=$1 AND ($2 OR o.user_id=$3)`, id, admin, userID))
	if err != nil {
		return o, err
	}
	rows, err := e.DB.QueryContext(ctx, `SELECT id,action,state,error_code,created_at FROM membership_events WHERE order_id=$1 ORDER BY created_at,id`, id)
	if err != nil {
		return o, err
	}
	defer closeResource(rows)
	o.Events = []Event{}
	for rows.Next() {
		var ev Event
		if err = rows.Scan(&ev.ID, &ev.Action, &ev.State, &ev.ErrorCode, &ev.CreatedAt); err != nil {
			return o, err
		}
		o.Events = append(o.Events, ev)
	}
	return o, rows.Err()
}

func (e *Engine) ImportCDKs(ctx context.Context, actor, sku string, codes []string, cost int64) error {
	if e.Keys == nil || len(codes) == 0 || len(codes) > 100 || cost < 0 {
		return ErrInvalid
	}
	return e.transaction(ctx, func(tx *sql.Tx) error {
		var channel string
		if err := tx.QueryRowContext(ctx, `SELECT channel FROM membership_products WHERE sku=$1`, sku).Scan(&channel); err != nil {
			return err
		}
		for _, code := range codes {
			code = strings.TrimSpace(code)
			if len(code) < 6 || len(code) > 512 {
				return ErrInvalid
			}
			id := uuid.NewString()
			cipher, err := e.Keys.Encrypt("cdk:"+id, code)
			if err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO membership_cdks(id,sku,channel,ciphertext,fingerprint,cost_minor) VALUES($1,$2,$3,$4,$5,$6)`, id, sku, channel, cipher, e.Keys.Fingerprint("cdk", code), cost); err != nil {
				return err
			}
		}
		return event(ctx, tx, "", "stock_imported", actor, "", "", "", false)
	})
}

func (e *Engine) ExportAudit(ctx context.Context) error {
	if e.Audit == nil {
		return ErrUnavailable
	}
	rows, err := e.DB.QueryContext(ctx, `SELECT ev.id,row_to_json(ev)::text FROM membership_events ev LEFT JOIN membership_event_exports x ON x.event_id=ev.id WHERE x.event_id IS NULL ORDER BY ev.created_at LIMIT 100`)
	if err != nil {
		return err
	}
	type entry struct{ id, raw string }
	entries := []entry{}
	for rows.Next() {
		var v entry
		if err = rows.Scan(&v.id, &v.raw); err != nil {
			closeResource(rows)
			return err
		}
		entries = append(entries, v)
	}
	err = rows.Err()
	closeResource(rows)
	if err != nil {
		return err
	}
	for _, v := range entries {
		key, err := e.Audit.Store(ctx, v.id, []byte(v.raw))
		if err != nil || key == "" {
			if err == nil {
				err = ErrUnavailable
			}
			return err
		}
		if _, err = e.DB.ExecContext(ctx, `INSERT INTO membership_event_exports(event_id,object_key) VALUES($1,$2) ON CONFLICT DO NOTHING`, v.id, key); err != nil {
			return err
		}
	}
	return nil
}

// ExpireOrders closes abandoned, unpaid checkouts and returns all reservations
// made by Create. Payment-provider orders are pending and are never closed here.
func (e *Engine) ExpireOrders(ctx context.Context) error {
	type expired struct {
		id, ref string
	}
	var orders []expired
	err := e.transaction(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT id,COALESCE(credential_ref,'') FROM membership_orders
            WHERE payment_state='created' AND created_at<=now()-($1 * interval '1 second')
            ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 100`, int(OrderTTL.Seconds()))
		if err != nil {
			return err
		}
		defer closeResource(rows)
		for rows.Next() {
			var order expired
			if err := rows.Scan(&order.id, &order.ref); err != nil {
				return err
			}
			orders = append(orders, order)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		for _, order := range orders {
			res, err := tx.ExecContext(ctx, `UPDATE membership_orders SET payment_state='closed',credential_ref=NULL,credential_expires_at=NULL,input_required=true,updated_at=now() WHERE id=$1 AND payment_state='created'`, order.id)
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
			if _, err = tx.ExecContext(ctx, `UPDATE membership_cdks SET state='available',order_id=NULL WHERE order_id=$1 AND state='reserved'`, order.id); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `UPDATE membership_coupons SET used=used-1 WHERE code=(SELECT coupon_code FROM membership_orders WHERE id=$1) AND used>0`, order.id); err != nil {
				return err
			}
			if err = event(ctx, tx, order.id, "order_expired", "worker", "closed", "", "", true); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, order := range orders {
		if order.ref != "" && e.Vault != nil {
			if err := e.Vault.Delete(ctx, order.ref); err != nil {
				return err
			}
		}
	}
	return nil
}
