package costing

import (
	"context"
	"database/sql"
	"fmt"
	"math/big"
	"sort"
	"time"
)

// SQLSource only issues SELECT statements in a read-only repeatable-read transaction.
// It never reads account names, credentials, request bodies or user identifiers.
type SQLSource struct {
	Origin string
	Open   func() (*sql.DB, func(), error)
}
type SourceAccount struct {
	ID                    int64   `json:"id"`
	Platform              string  `json:"platform"`
	Type                  string  `json:"type"`
	Status                string  `json:"status"`
	PlanType              string  `json:"plan_type"`
	SubscriptionExpiresAt *string `json:"subscription_expires_at"`
}
type UsageRow struct {
	AccountID            int64   `json:"account_id"`
	Asset                string  `json:"asset"`
	Tier                 string  `json:"tier"`
	Model                string  `json:"model"`
	Requests             int64   `json:"requests"`
	InputTokens          string  `json:"input_tokens"`
	OutputTokens         string  `json:"output_tokens"`
	CacheReadTokens      string  `json:"cache_read_tokens"`
	CacheCreationTokens  string  `json:"cache_creation_tokens"`
	ReferenceCost        string  `json:"reference_cost"`
	RecordedRequests     *string `json:"recorded_requests"`
	RequestDifference    *string `json:"request_difference"`
	ReconciliationStatus string  `json:"reconciliation_status"`
	Start                string  `json:"start"`
	End                  string  `json:"end"`
}
type Reconciliation struct {
	Accounts            []SourceAccount `json:"accounts"`
	SourceLatestUsageAt *string         `json:"source_latest_usage_at"`
	CoverageComplete    bool            `json:"coverage_complete"`
	Status              string          `json:"status"`
	Rows                []UsageRow      `json:"rows"`
	Warnings            []string        `json:"warnings"`
}

func (s *SQLSource) read(ctx context.Context, fn func(*sql.Tx) error) error {
	if s == nil || s.Open == nil {
		return ErrUnavailable
	}
	db, release, err := s.Open()
	if err != nil || db == nil || release == nil {
		return ErrUnavailable
	}
	defer release()
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return ErrUnavailable
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SET LOCAL statement_timeout='7000ms'`); err != nil {
		return ErrUnavailable
	}
	if err = fn(tx); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return ErrUnavailable
	}
	return nil
}
func readSourceAccounts(ctx context.Context, tx *sql.Tx) ([]SourceAccount, error) {
	out := []SourceAccount{}
	rows, err := tx.QueryContext(ctx, `SELECT id,platform,type,status,COALESCE(credentials->>'plan_type',''),credentials->>'subscription_expires_at' FROM accounts WHERE deleted_at IS NULL AND platform='openai' ORDER BY id LIMIT 1001`)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer rows.Close()
	for rows.Next() {
		var a SourceAccount
		if rows.Scan(&a.ID, &a.Platform, &a.Type, &a.Status, &a.PlanType, &a.SubscriptionExpiresAt) != nil {
			return nil, ErrUnavailable
		}
		out = append(out, a)
	}
	if rows.Err() != nil {
		return nil, ErrUnavailable
	}
	if len(out) > 1000 {
		return nil, ErrLimit
	}
	return out, nil
}
func (s *SQLSource) Accounts(ctx context.Context) ([]SourceAccount, error) {
	var out []SourceAccount
	err := s.read(ctx, func(tx *sql.Tx) error { var err error; out, err = readSourceAccounts(ctx, tx); return err })
	return out, err
}
func (s *SQLSource) AccountExists(ctx context.Context, id int64) error {
	return s.read(ctx, func(tx *sql.Tx) error {
		var found bool
		err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM accounts WHERE id=$1 AND deleted_at IS NULL AND platform='openai')`, id).Scan(&found)
		if err != nil {
			return ErrUnavailable
		}
		if !found {
			return ErrNotFound
		}
		return nil
	})
}
func (s *SQLSource) Reconcile(ctx context.Context, l *Ledger, q SummaryQuery) (Reconciliation, error) {
	out := Reconciliation{Status: "GATEWAY_USAGE_ONLY", Accounts: []SourceAccount{}, Rows: []UsageRow{}, Warnings: []string{"REFERENCE_COST_IS_NOT_PROCUREMENT_CASH", "DIRECT_UPSTREAM_USAGE_NOT_OBSERVED", "SOURCE_RETENTION_COVERAGE_NOT_VERIFIED"}}
	if !validPeriod(q.Start, q.End) || (q.Model != "" && !textOK(q.Model, 100)) || instant(q.End).Sub(instant(q.Start)) > 92*24*time.Hour {
		return out, invalid("usage query supports at most 92 days and one model or all models")
	}
	at, err := parseTime(q.AsOf)
	if err != nil || at.After(time.Now().Add(time.Second)) {
		return out, invalid("valid historical knowledge cutoff required")
	}
	if l == nil || l.Store == nil {
		return out, ErrUnavailable
	}
	events, err := l.Store.Snapshot(ctx)
	if err != nil {
		return out, err
	}
	active := activeEvents(events, at)
	a, b := instant(q.Start), instant(q.End)
	if at.Before(b) {
		b = at
	}
	if !b.After(a) {
		return out, nil
	}
	unclassified := false
	err = s.read(ctx, func(tx *sql.Tx) error {
		var err error
		out.Accounts, err = readSourceAccounts(ctx, tx)
		if err != nil {
			return err
		}
		var latest sql.NullTime
		if tx.QueryRowContext(ctx, `SELECT max(created_at) FROM usage_logs WHERE account_id IN (SELECT id FROM accounts WHERE deleted_at IS NULL AND platform='openai')`).Scan(&latest) != nil {
			return ErrUnavailable
		}
		if latest.Valid {
			value := latest.Time.UTC().Format(time.RFC3339Nano)
			out.SourceLatestUsageAt = &value
		}
		for _, account := range out.Accounts {
			intervals := []AccountInterval{}
			for _, event := range events {
				if _, ok := active[event.ID]; ok && event.Command.AccountInterval != nil && event.Command.AccountInterval.AccountID == account.ID {
					p := *event.Command.AccountInterval
					if instant(p.Start).Before(b) && a.Before(instant(p.End)) {
						intervals = append(intervals, p)
					}
				}
			}
			sort.Slice(intervals, func(i, j int) bool { return instant(intervals[i].Start).Before(instant(intervals[j].Start)) })
			portions := []AccountInterval{}
			cursor := a
			addGap := func(end time.Time) {
				if end.After(cursor) {
					portions = append(portions, AccountInterval{AccountID: account.ID, Asset: fmt.Sprintf("account-%d", account.ID), Tier: "unassigned", Start: cursor.Format(time.RFC3339Nano), End: end.Format(time.RFC3339Nano)})
				}
			}
			for _, p := range intervals {
				x, y := instant(p.Start), instant(p.End)
				if x.Before(a) {
					x = a
				}
				if y.After(b) {
					y = b
				}
				addGap(x)
				p.Start = x.Format(time.RFC3339Nano)
				p.End = y.Format(time.RFC3339Nano)
				portions = append(portions, p)
				cursor = y
			}
			addGap(b)
			for _, p := range portions {
				rows, err := tx.QueryContext(ctx, `SELECT model,count(*),COALESCE(sum(input_tokens),0)::text,COALESCE(sum(output_tokens),0)::text,COALESCE(sum(cache_read_tokens),0)::text,COALESCE(sum(cache_creation_tokens),0)::text,COALESCE(sum(total_cost),0)::text FROM usage_logs WHERE account_id=$1 AND ($2='' OR model=$2) AND created_at >= $3 AND created_at < $4 GROUP BY model ORDER BY model`, account.ID, q.Model, p.Start, p.End)
				if err != nil {
					return ErrUnavailable
				}
				for rows.Next() {
					row := UsageRow{AccountID: account.ID, Asset: p.Asset, Tier: p.Tier, Start: p.Start, End: p.End}
					if rows.Scan(&row.Model, &row.Requests, &row.InputTokens, &row.OutputTokens, &row.CacheReadTokens, &row.CacheCreationTokens, &row.ReferenceCost) != nil {
						rows.Close()
						return ErrUnavailable
					}
					compareRecordedRequests(active, &row)
					if p.Tier == "unassigned" {
						row.ReconciliationStatus = "TIER_PERIOD_NOT_CONFIRMED"
						unclassified = true
					}
					out.Rows = append(out.Rows, row)
					if len(out.Rows) > 5000 {
						rows.Close()
						return ErrLimit
					}
				}
				rowErr := rows.Err()
				rows.Close()
				if rowErr != nil {
					return ErrUnavailable
				}
			}
		}
		return nil
	})
	if unclassified {
		out.Warnings = append(out.Warnings, "TIER_PERIOD_NOT_CONFIRMED")
	}
	return out, err
}

// QuotaSnapshots reads already cached gateway observations without refreshing
// upstream tokens, probing providers, redeeming credits or changing source state.
func (s *SQLSource) QuotaSnapshots(ctx context.Context) ([]QuotaObservation, error) {
	out := []QuotaObservation{}
	err := s.read(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT id,extra->>'codex_usage_updated_at',
   (extra->>'codex_5h_used_percent')::numeric,(extra->>'codex_5h_window_minutes')::bigint,extra->>'codex_5h_reset_at',
   (extra->>'codex_7d_used_percent')::numeric,(extra->>'codex_7d_window_minutes')::bigint,extra->>'codex_7d_reset_at'
   FROM accounts WHERE deleted_at IS NULL AND platform='openai' AND extra->>'codex_usage_updated_at' IS NOT NULL ORDER BY id LIMIT 1001`)
		if err != nil {
			return ErrUnavailable
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			var at string
			var p5, p7 sql.NullFloat64
			var m5, m7 sql.NullInt64
			var r5, r7 sql.NullString
			if rows.Scan(&id, &at, &p5, &m5, &r5, &p7, &m7, &r7) != nil {
				return ErrUnavailable
			}
			q := QuotaObservation{AccountID: id, ObservedAt: at, Kind: "snapshot", Windows: []ObservedQuotaWindow{}}
			for _, w := range []struct {
				id string
				p  sql.NullFloat64
				m  sql.NullInt64
				r  sql.NullString
			}{{"5h", p5, m5, r5}, {"7d", p7, m7, r7}} {
				reset, err := parseTime(w.r.String)
				if w.p.Valid && w.m.Valid && w.r.Valid && err == nil {
					q.Windows = append(q.Windows, ObservedQuotaWindow{Pool: "codex", Window: w.id, UsedPercent: w.p.Float64, DurationSeconds: w.m.Int64 * 60, ResetAt: reset.Unix()})
				}
			}
			if len(q.Windows) > 0 {
				out = append(out, q)
			}
		}
		if rows.Err() != nil {
			return ErrUnavailable
		}
		if len(out) > 1000 {
			return ErrLimit
		}
		return nil
	})
	return out, err
}

// Aggregate deliveries can be compared only when they cover the exact bound interval.
// A partial or cross-boundary record is not silently treated as a complete denominator.
func compareRecordedRequests(active map[string]LedgerEvent, row *UsageRow) {
	row.ReconciliationStatus = "NO_RECORDED_REQUESTS"
	deliveries := []*Delivery{}
	a, b := instant(row.Start), instant(row.End)
	for _, event := range active {
		d := event.Command.Delivery
		if d == nil || d.Asset != row.Asset || d.Tier != row.Tier || d.Model != row.Model || d.Unit != "request" {
			continue
		}
		x, y := instant(d.PeriodStart), instant(d.PeriodEnd)
		if !x.Before(b) || !a.Before(y) {
			continue
		}
		if x.Before(a) || y.After(b) {
			row.ReconciliationStatus = "CROSS_BOUNDARY_RECORDS"
			return
		}
		deliveries = append(deliveries, d)
	}
	if len(deliveries) == 0 {
		return
	}
	sort.Slice(deliveries, func(i, j int) bool {
		return instant(deliveries[i].PeriodStart).Before(instant(deliveries[j].PeriodStart))
	})
	end := a
	total := new(big.Rat)
	for _, d := range deliveries {
		if !instant(d.PeriodStart).Equal(end) {
			row.ReconciliationStatus = "RECORDED_SCOPE_INCOMPLETE"
			return
		}
		end = instant(d.PeriodEnd)
		total.Add(total, rat(d.Quantity))
	}
	if !end.Equal(b) {
		row.ReconciliationStatus = "RECORDED_SCOPE_INCOMPLETE"
		return
	}
	value := total.FloatString(6)
	delta := new(big.Rat).Sub(total, new(big.Rat).SetInt64(row.Requests))
	difference := delta.FloatString(6)
	row.RecordedRequests = &value
	row.RequestDifference = &difference
	row.ReconciliationStatus = "MATCH"
	if delta.Sign() != 0 {
		row.ReconciliationStatus = "MISMATCH"
	}
}
