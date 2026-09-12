package membership

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"strconv"
	"time"

	"github.com/google/uuid"
)

type work struct {
	TaskID, OrderID, State, Channel, SKU, CDKID, Ciphertext, CredentialRef, Lease, Kind string
	PeriodDays, PollSeconds, WaitSeconds, QueryErrors                                   int
	Deadline                                                                            *time.Time
	Paused                                                                              bool
}

func (e *Engine) Start() {
	e.startOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		e.cancel = cancel
		e.wg.Add(1)
		go func() {
			defer e.wg.Done()
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if e.Enabled {
						if err := e.CleanupCredentials(ctx); err != nil {
							log.Printf("membership worker error operation=cleanup_credentials code=MEMBERSHIP_WORKER_ERROR")
						}
						if err := e.ExpireOrders(ctx); err != nil {
							log.Printf("membership worker error operation=expire_orders code=MEMBERSHIP_WORKER_ERROR")
						}
						for _, channel := range []string{"gpt", "gptpro"} {
							if err := e.RunOne(ctx, channel); err != nil {
								log.Printf("membership worker error operation=run_one channel=%q code=MEMBERSHIP_WORKER_ERROR", channel)
							}
						}
						if err := e.ExportAudit(ctx); err != nil {
							log.Printf("membership worker error operation=export_audit code=MEMBERSHIP_WORKER_ERROR")
						}
						if err := e.SendNotifications(ctx); err != nil {
							log.Printf("membership worker error operation=send_notifications code=MEMBERSHIP_WORKER_ERROR")
						}
					}
				}
			}
		}()
	})
}
func (e *Engine) Stop() {
	e.stopOnce.Do(func() {
		if e.cancel != nil {
			e.cancel()
			e.wg.Wait()
		}
	})
}

func (e *Engine) RunOne(ctx context.Context, channel string) error {
	if err := e.Ready(ctx); err != nil {
		return err
	}
	conn, err := e.DB.Conn(ctx)
	if err != nil {
		return err
	}
	defer closeResource(conn)
	var locked bool
	if err = conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1,0))`, "membership:"+channel).Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return nil
	}
	defer func() {
		if _, err := conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock(hashtextextended($1,0))`, "membership:"+channel); err != nil {
			log.Printf("membership cleanup error operation=advisory_unlock code=MEMBERSHIP_CLEANUP_ERROR")
		}
	}()
	var w work
	w.Lease = uuid.NewString()
	err = e.transaction(ctx, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `SELECT t.id,t.order_id,t.state,t.channel,o.sku,c.id,c.ciphertext,COALESCE(o.credential_ref,''),o.kind,o.period_days,p.poll_seconds,p.wait_seconds,t.query_errors,t.deadline_at,p.paused
            FROM membership_tasks t JOIN membership_orders o ON o.id=t.order_id JOIN membership_products p ON p.sku=o.sku JOIN membership_cdks c ON c.id=t.cdk_id
            WHERE t.channel=$1 AND t.state IN ('queued','submitted','processing') AND t.next_run_at<=now() AND (t.lease_until IS NULL OR t.lease_until<now())
	            AND o.refund_requested_at IS NULL
	            AND (t.state<>'queued' OR (NOT p.paused AND (o.kind='validation' OR o.payment_state='paid')))
            ORDER BY t.next_run_at,t.created_at,t.id FOR UPDATE OF t SKIP LOCKED LIMIT 1`, channel).Scan(&w.TaskID, &w.OrderID, &w.State, &w.Channel, &w.SKU, &w.CDKID, &w.Ciphertext, &w.CredentialRef, &w.Kind, &w.PeriodDays, &w.PollSeconds, &w.WaitSeconds, &w.QueryErrors, &w.Deadline, &w.Paused)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE membership_tasks SET lease_token=$2,lease_until=now()+interval '2 minutes' WHERE id=$1`, w.TaskID, w.Lease)
		return err
	})
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	cdk, err := e.Keys.Decrypt("cdk:"+w.CDKID, w.Ciphertext)
	if err != nil {
		return e.finish(ctx, w, "", BrowserResult{State: "review_required", ErrorCode: "REVIEW_REQUIRED"})
	}
	input := BrowserInput{Channel: w.Channel, SKU: w.SKU, PeriodDays: w.PeriodDays, CDK: cdk}
	if w.State == "queued" {
		credential, err := e.Vault.Get(ctx, w.CredentialRef)
		if err != nil {
			if errors.Is(err, ErrCredential) {
				return e.requireInput(ctx, w)
			}
			return e.retryQueued(ctx, w)
		}
		input.Credential = &credential
		input.Operation = "validateCredential"
		checked, err := e.Browser.Execute(ctx, input)
		if err != nil {
			return e.finish(ctx, w, "", BrowserResult{State: "not_submitted", NotSubmitted: true, ErrorCode: "CHANNEL_UNAVAILABLE"})
		}
		checked = NormalizeResult(checked)
		if checked.State != "valid" || checked.SKU != w.SKU || checked.PeriodDays != w.PeriodDays || checked.AccountID != credential.AccountID {
			code := checked.ErrorCode
			if code == "" {
				code = "PRODUCT_MISMATCH"
			}
			return e.finish(ctx, w, "", BrowserResult{State: "not_submitted", NotSubmitted: true, ErrorCode: code})
		}
		attempt := uuid.NewString()
		err = e.transaction(ctx, func(tx *sql.Tx) error {
			var allowed bool
			if err := tx.QueryRowContext(ctx, `SELECT (NOT p.paused AND (o.kind='validation' OR o.payment_state='paid')) AND o.refund_requested_at IS NULL AND o.credential_expires_at>now()
                FROM membership_orders o JOIN membership_products p ON p.sku=o.sku WHERE o.id=$1 FOR UPDATE OF o`, w.OrderID).Scan(&allowed); err != nil {
				return err
			}
			if !allowed {
				return ErrConflict
			}
			res, err := tx.ExecContext(ctx, `UPDATE membership_tasks SET state='submitted',deadline_at=now()+($3*interval '1 second'),updated_at=now() WHERE id=$1 AND lease_token=$2 AND state='queued'`, w.TaskID, w.Lease, w.WaitSeconds)
			if err != nil {
				return err
			}
			n, _ := res.RowsAffected()
			if n != 1 {
				return ErrConflict
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO membership_attempts(id,task_id,cdk_id,state) VALUES($1,$2,$3,'intent')`, attempt, w.TaskID, w.CDKID); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `UPDATE membership_cdks SET state='submitted' WHERE id=$1`, w.CDKID); err != nil {
				return err
			}
			return event(ctx, tx, w.OrderID, "submit_intent", "worker", "submitted", "", attempt, false)
		})
		if err != nil {
			return err
		}
		// An externally retained intent must exist before any irreversible browser click.
		if err = e.exportIntent(ctx, attempt); err != nil {
			return e.finish(ctx, w, attempt, BrowserResult{State: "not_submitted", NotSubmitted: true, ErrorCode: "CHANNEL_UNAVAILABLE"})
		}
		input.Operation = "submitRecharge"
		input.AttemptID = attempt
		result, err := e.Browser.Execute(ctx, input)
		if err != nil {
			result = BrowserResult{State: "review_required", ErrorCode: "REVIEW_REQUIRED"}
		}
		return e.finish(ctx, w, attempt, NormalizeResult(result))
	}
	if w.Deadline != nil && time.Now().After(*w.Deadline) {
		return e.finish(ctx, w, "", BrowserResult{State: "review_required", ErrorCode: "REVIEW_REQUIRED"})
	}
	var attempt, upstream string
	err = e.DB.QueryRowContext(ctx, `SELECT id,upstream_task_id FROM membership_attempts WHERE task_id=$1 ORDER BY created_at DESC LIMIT 1`, w.TaskID).Scan(&attempt, &upstream)
	if err != nil {
		return e.finish(ctx, w, "", BrowserResult{State: "review_required", ErrorCode: "REVIEW_REQUIRED"})
	}
	input.Operation = "queryStatus"
	input.AttemptID = attempt
	input.UpstreamTaskID = upstream
	result, err := e.Browser.Execute(ctx, input)
	if err != nil {
		if w.QueryErrors < 2 {
			queryErr := err
			if _, err = e.DB.ExecContext(ctx, `UPDATE membership_tasks SET query_errors=query_errors+1,next_run_at=now()+($3*interval '1 second'),lease_token=NULL,lease_until=NULL WHERE id=$1 AND lease_token=$2`, w.TaskID, w.Lease, w.PollSeconds); err != nil {
				return err
			}
			return queryErr
		}
		result = BrowserResult{State: "review_required", ErrorCode: "REVIEW_REQUIRED"}
	}
	return e.finish(ctx, w, attempt, NormalizeResult(result))
}

func (e *Engine) exportIntent(ctx context.Context, attempt string) error {
	var id, raw string
	if err := e.DB.QueryRowContext(ctx, `SELECT id,row_to_json(ev)::text FROM membership_events ev WHERE action='submit_intent' AND evidence_ref=$1`, attempt).Scan(&id, &raw); err != nil {
		return err
	}
	key, err := e.Audit.Store(ctx, id, []byte(raw))
	if err != nil || key == "" {
		if err == nil {
			err = ErrUnavailable
		}
		return err
	}
	_, err = e.DB.ExecContext(ctx, `INSERT INTO membership_event_exports(event_id,object_key) VALUES($1,$2) ON CONFLICT DO NOTHING`, id, key)
	return err
}

func (e *Engine) requireInput(ctx context.Context, w work) error {
	return e.transaction(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE membership_tasks SET state='awaiting_input',error_code='CREDENTIAL_EXPIRED',lease_token=NULL,lease_until=NULL WHERE id=$1 AND lease_token=$2`, w.TaskID, w.Lease); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE membership_orders SET input_required=true,credential_ref=NULL,credential_expires_at=NULL,updated_at=now() WHERE id=$1`, w.OrderID); err != nil {
			return err
		}
		return event(ctx, tx, w.OrderID, "input_required", "worker", "awaiting_input", "CREDENTIAL_EXPIRED", "", true)
	})
}

func (e *Engine) retryQueued(ctx context.Context, w work) error {
	_, err := e.DB.ExecContext(ctx, `UPDATE membership_tasks SET error_code='CHANNEL_UNAVAILABLE',next_run_at=now()+($3*interval '1 second'),lease_token=NULL,lease_until=NULL,updated_at=now() WHERE id=$1 AND lease_token=$2 AND state='queued'`, w.TaskID, w.Lease, w.PollSeconds)
	return err
}

func (e *Engine) finish(ctx context.Context, w work, attempt string, result BrowserResult) error {
	state := result.State
	if state == "not_submitted" {
		state = "review_required"
	}
	if state != "submitted" && state != "processing" && state != "succeeded" && state != "failed" && state != "review_required" {
		state = "review_required"
		result.ErrorCode = "REVIEW_REQUIRED"
	}
	terminal := state == "succeeded" || state == "failed"
	err := e.transaction(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE membership_tasks SET state=$3,error_code=$4,lease_token=NULL,lease_until=NULL,next_run_at=now()+($5*interval '1 second'),query_errors=0,updated_at=now() WHERE id=$1 AND lease_token=$2 AND NOT EXISTS(SELECT 1 FROM membership_orders WHERE id=$6 AND refund_requested_at IS NOT NULL)`, w.TaskID, w.Lease, state, result.ErrorCode, w.PollSeconds, w.OrderID)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n != 1 {
			return ErrConflict
		}
		if attempt != "" {
			attemptState := state
			if result.NotSubmitted {
				attemptState = "not_submitted"
			}
			if _, err = tx.ExecContext(ctx, `UPDATE membership_attempts SET state=$2,error_code=$3,upstream_task_id=CASE WHEN $4='' THEN upstream_task_id ELSE $4 END,updated_at=now() WHERE id=$1`, attempt, attemptState, result.ErrorCode, result.UpstreamTaskID); err != nil {
				return err
			}
		}
		cdkState := "submitted"
		switch state {
		case "succeeded":
			cdkState = "spent"
		case "review_required", "failed":
			cdkState = "uncertain"
		}
		if result.NotSubmitted {
			cdkState = "reserved"
		}
		if _, err = tx.ExecContext(ctx, `UPDATE membership_cdks SET state=$2 WHERE id=$1`, w.CDKID, cdkState); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE membership_orders SET updated_at=now() WHERE id=$1`, w.OrderID); err != nil {
			return err
		}
		if state == "succeeded" && w.Kind == "customer" {
			if _, err = tx.ExecContext(ctx, `INSERT INTO membership_ledger(id,order_id,kind,amount_minor,reference) SELECT $1,$2,'supplier_cost',cost_minor,id::text FROM membership_cdks WHERE id=$3 ON CONFLICT DO NOTHING`, uuid.NewString(), w.OrderID, w.CDKID); err != nil {
				return err
			}
		}
		if state == "review_required" || state == "failed" {
			if _, err = tx.ExecContext(ctx, `UPDATE membership_products SET failures=failures+1,paused=CASE WHEN failures>=2 THEN true ELSE paused END WHERE sku=$1`, w.SKU); err != nil {
				return err
			}
		}
		return event(ctx, tx, w.OrderID, "fulfillment_updated", "worker", state, result.ErrorCode, "", terminal || state == "review_required")
	})
	if err != nil {
		return err
	}
	// A successful submission no longer needs the customer's credential for CDK-based queries.
	if terminal || state == "submitted" || state == "processing" || state == "review_required" {
		if err = e.Vault.Delete(ctx, w.CredentialRef); err != nil {
			return err
		}
		_, err = e.DB.ExecContext(ctx, `UPDATE membership_orders SET credential_ref=NULL,credential_expires_at=NULL WHERE id=$1 AND credential_ref=$2`, w.OrderID, w.CredentialRef)
	}
	return err
}

func (e *Engine) CleanupCredentials(ctx context.Context) error {
	if e.Vault == nil {
		return nil
	}
	rows, err := e.DB.QueryContext(ctx, `SELECT o.id,o.credential_ref FROM membership_orders o LEFT JOIN membership_tasks t ON t.order_id=o.id WHERE o.credential_ref IS NOT NULL AND (o.credential_expires_at<=now() OR t.state IN ('succeeded','failed','canceled','review_required') OR o.payment_state IN ('closed','refund_pending','refunded'))`)
	if err != nil {
		return err
	}
	type item struct{ id, ref string }
	items := []item{}
	for rows.Next() {
		var i item
		if err = rows.Scan(&i.id, &i.ref); err != nil {
			closeResource(rows)
			return err
		}
		items = append(items, i)
	}
	err = rows.Err()
	closeResource(rows)
	if err != nil {
		return err
	}
	for _, i := range items {
		if err = e.Vault.Delete(ctx, i.ref); err != nil {
			return err
		}
		if _, err = e.DB.ExecContext(ctx, `UPDATE membership_orders SET credential_ref=NULL,credential_expires_at=NULL,input_required=true WHERE id=$1 AND credential_ref=$2`, i.id, i.ref); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) SendNotifications(ctx context.Context) error {
	if e.Notify == nil {
		return nil
	}
	conn, err := e.DB.Conn(ctx)
	if err != nil {
		return err
	}
	defer closeResource(conn)
	var locked bool
	if err = conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(hashtextextended('membership:notifications',0))`).Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return nil
	}
	defer func() {
		if _, err := conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock(hashtextextended('membership:notifications',0))`); err != nil {
			log.Printf("membership cleanup error operation=notification_unlock code=MEMBERSHIP_CLEANUP_ERROR")
		}
	}()
	rows, err := e.DB.QueryContext(ctx, `SELECT n.event_id,n.order_id,o.user_id,ev.state FROM membership_notifications n JOIN membership_orders o ON o.id=n.order_id JOIN membership_events ev ON ev.id=n.event_id WHERE n.sent_at IS NULL AND n.next_run_at<=now() AND n.attempts<5 ORDER BY n.next_run_at LIMIT 20`)
	if err != nil {
		return err
	}
	type item struct {
		id, order, state string
		user             int64
	}
	items := []item{}
	for rows.Next() {
		var i item
		if err = rows.Scan(&i.id, &i.order, &i.user, &i.state); err != nil {
			closeResource(rows)
			return err
		}
		items = append(items, i)
	}
	err = rows.Err()
	closeResource(rows)
	if err != nil {
		return err
	}
	for _, i := range items {
		sendErr := e.Notify(ctx, i.user, i.order, i.state)
		if _, err = e.DB.ExecContext(ctx, `UPDATE membership_notifications SET attempts=attempts+1,next_run_at=now()+interval '5 minutes',sent_at=CASE WHEN $2 THEN now() ELSE NULL END WHERE event_id=$1`, i.id, sendErr == nil); err != nil {
			return err
		}
	}
	return nil
}

func actorID(id int64) string { return strconv.FormatInt(id, 10) }
