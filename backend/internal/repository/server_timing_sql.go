package repository

import (
	"context"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/servertiming"
)

type serverTimingConnector struct {
	base driver.Connector
	now  func() time.Time
}

func newServerTimingConnector(base driver.Connector) driver.Connector {
	return newServerTimingConnectorWithNow(base, time.Now)
}

func newServerTimingConnectorWithNow(base driver.Connector, now func() time.Time) driver.Connector {
	if now == nil {
		now = time.Now
	}
	return &serverTimingConnector{base: base, now: now}
}

func (c *serverTimingConnector) Connect(ctx context.Context) (driver.Conn, error) {
	startedAt := c.now()
	conn, err := c.base.Connect(ctx)
	servertiming.RecordInterval(ctx, servertiming.MetricDatabase, startedAt, c.now())
	if err != nil {
		return nil, err
	}
	return &serverTimingConn{Conn: conn, now: c.now}, nil
}

func (c *serverTimingConnector) Driver() driver.Driver {
	return c.base.Driver()
}

type serverTimingConn struct {
	driver.Conn
	now func() time.Time
}

func (c *serverTimingConn) Prepare(query string) (driver.Stmt, error) {
	stmt, err := c.Conn.Prepare(query)
	if err != nil {
		return nil, err
	}
	return &serverTimingStmt{Stmt: stmt, now: c.now}, nil
}

func (c *serverTimingConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	startedAt := c.now()
	var (
		stmt driver.Stmt
		err  error
	)
	if preparer, ok := c.Conn.(driver.ConnPrepareContext); ok {
		stmt, err = preparer.PrepareContext(ctx, query)
	} else {
		stmt, err = c.Conn.Prepare(query)
	}
	servertiming.Record(ctx, servertiming.MetricDatabase, startedAt, c.now(), 1)
	if err != nil {
		return nil, err
	}
	return &serverTimingStmt{Stmt: stmt, now: c.now}, nil
}

func (c *serverTimingConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	execer, ok := c.Conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	startedAt := c.now()
	result, err := execer.ExecContext(ctx, query, args)
	servertiming.Record(ctx, servertiming.MetricDatabase, startedAt, c.now(), 1)
	return result, err
}

func (c *serverTimingConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	queryer, ok := c.Conn.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	startedAt := c.now()
	rows, err := queryer.QueryContext(ctx, query, args)
	servertiming.Record(ctx, servertiming.MetricDatabase, startedAt, c.now(), 1)
	if err != nil || rows == nil {
		return rows, err
	}
	return newServerTimingRows(ctx, rows, c.now), nil
}

func (c *serverTimingConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	startedAt := c.now()
	var (
		tx  driver.Tx
		err error
	)
	if beginner, ok := c.Conn.(driver.ConnBeginTx); ok {
		tx, err = beginner.BeginTx(ctx, opts)
	} else {
		if opts.Isolation != driver.IsolationLevel(0) {
			return nil, errors.New("driver does not support non-default isolation")
		}
		if opts.ReadOnly {
			return nil, errors.New("driver does not support read-only transactions")
		}
		// The wrapper exposes ConnBeginTx, so it must retain database/sql's
		// legacy fallback for drivers that only implement Conn.Begin.
		tx, err = c.Conn.Begin() //nolint:staticcheck // Required driver compatibility fallback.
	}
	servertiming.RecordInterval(ctx, servertiming.MetricDatabase, startedAt, c.now())
	if err != nil || tx == nil {
		return tx, err
	}
	return &serverTimingTx{Tx: tx, ctx: ctx, now: c.now}, nil
}

func (c *serverTimingConn) Ping(ctx context.Context) error {
	if pinger, ok := c.Conn.(driver.Pinger); ok {
		startedAt := c.now()
		err := pinger.Ping(ctx)
		servertiming.RecordInterval(ctx, servertiming.MetricDatabase, startedAt, c.now())
		return err
	}
	return nil
}

func (c *serverTimingConn) ResetSession(ctx context.Context) error {
	if resetter, ok := c.Conn.(driver.SessionResetter); ok {
		startedAt := c.now()
		err := resetter.ResetSession(ctx)
		servertiming.RecordInterval(ctx, servertiming.MetricDatabase, startedAt, c.now())
		return err
	}
	return nil
}

func (c *serverTimingConn) IsValid() bool {
	if validator, ok := c.Conn.(driver.Validator); ok {
		return validator.IsValid()
	}
	return true
}

func (c *serverTimingConn) CheckNamedValue(value *driver.NamedValue) error {
	if checker, ok := c.Conn.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(value)
	}
	return driver.ErrSkip
}

type serverTimingStmt struct {
	driver.Stmt
	now func() time.Time
}

func (s *serverTimingStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	startedAt := s.now()
	var (
		result driver.Result
		err    error
	)
	if execer, ok := s.Stmt.(driver.StmtExecContext); ok {
		result, err = execer.ExecContext(ctx, args)
	} else {
		var values []driver.Value
		values, err = namedValues(args)
		if err == nil {
			// The wrapper exposes StmtExecContext and must preserve the fallback
			// database/sql would use for a legacy driver statement.
			result, err = s.Stmt.Exec(values) //nolint:staticcheck // Required driver compatibility fallback.
		}
	}
	servertiming.Record(ctx, servertiming.MetricDatabase, startedAt, s.now(), 1)
	return result, err
}

func (s *serverTimingStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	startedAt := s.now()
	var (
		rows driver.Rows
		err  error
	)
	if queryer, ok := s.Stmt.(driver.StmtQueryContext); ok {
		rows, err = queryer.QueryContext(ctx, args)
	} else {
		var values []driver.Value
		values, err = namedValues(args)
		if err == nil {
			// The wrapper exposes StmtQueryContext and must preserve the fallback
			// database/sql would use for a legacy driver statement.
			rows, err = s.Stmt.Query(values) //nolint:staticcheck // Required driver compatibility fallback.
		}
	}
	servertiming.Record(ctx, servertiming.MetricDatabase, startedAt, s.now(), 1)
	if err != nil || rows == nil {
		return rows, err
	}
	return newServerTimingRows(ctx, rows, s.now), nil
}

func (s *serverTimingStmt) CheckNamedValue(value *driver.NamedValue) error {
	if checker, ok := s.Stmt.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(value)
	}
	return driver.ErrSkip
}

func namedValues(args []driver.NamedValue) ([]driver.Value, error) {
	values := make([]driver.Value, len(args))
	for i, arg := range args {
		if arg.Name != "" {
			return nil, errors.New("named parameters are not supported")
		}
		values[i] = arg.Value
	}
	return values, nil
}

type serverTimingRows struct {
	driver.Rows
	ctx context.Context
	now func() time.Time
}

func newServerTimingRows(ctx context.Context, rows driver.Rows, now func() time.Time) *serverTimingRows {
	return &serverTimingRows{Rows: rows, ctx: ctx, now: now}
}

func (r *serverTimingRows) Close() error {
	startedAt := r.now()
	err := r.Rows.Close()
	servertiming.RecordInterval(r.ctx, servertiming.MetricDatabase, startedAt, r.now())
	return err
}

func (r *serverTimingRows) Next(dest []driver.Value) error {
	startedAt := r.now()
	err := r.Rows.Next(dest)
	servertiming.RecordInterval(r.ctx, servertiming.MetricDatabase, startedAt, r.now())
	return err
}

func (r *serverTimingRows) HasNextResultSet() bool {
	if rows, ok := r.Rows.(driver.RowsNextResultSet); ok {
		return rows.HasNextResultSet()
	}
	return false
}

func (r *serverTimingRows) NextResultSet() error {
	rows, ok := r.Rows.(driver.RowsNextResultSet)
	if !ok {
		return io.EOF
	}
	startedAt := r.now()
	err := rows.NextResultSet()
	servertiming.RecordInterval(r.ctx, servertiming.MetricDatabase, startedAt, r.now())
	return err
}

func (r *serverTimingRows) ColumnTypeScanType(index int) reflect.Type {
	if rows, ok := r.Rows.(driver.RowsColumnTypeScanType); ok {
		return rows.ColumnTypeScanType(index)
	}
	return reflect.TypeOf(new(any)).Elem()
}

func (r *serverTimingRows) ColumnTypeDatabaseTypeName(index int) string {
	if rows, ok := r.Rows.(driver.RowsColumnTypeDatabaseTypeName); ok {
		return rows.ColumnTypeDatabaseTypeName(index)
	}
	return ""
}

func (r *serverTimingRows) ColumnTypeLength(index int) (int64, bool) {
	if rows, ok := r.Rows.(driver.RowsColumnTypeLength); ok {
		return rows.ColumnTypeLength(index)
	}
	return 0, false
}

func (r *serverTimingRows) ColumnTypeNullable(index int) (bool, bool) {
	if rows, ok := r.Rows.(driver.RowsColumnTypeNullable); ok {
		return rows.ColumnTypeNullable(index)
	}
	return false, false
}

func (r *serverTimingRows) ColumnTypePrecisionScale(index int) (int64, int64, bool) {
	if rows, ok := r.Rows.(driver.RowsColumnTypePrecisionScale); ok {
		return rows.ColumnTypePrecisionScale(index)
	}
	return 0, 0, false
}

type serverTimingTx struct {
	driver.Tx
	ctx context.Context
	now func() time.Time
}

func (t *serverTimingTx) Commit() error {
	startedAt := t.now()
	err := t.Tx.Commit()
	servertiming.RecordInterval(t.ctx, servertiming.MetricDatabase, startedAt, t.now())
	return err
}

func (t *serverTimingTx) Rollback() error {
	startedAt := t.now()
	err := t.Tx.Rollback()
	servertiming.RecordInterval(t.ctx, servertiming.MetricDatabase, startedAt, t.now())
	return err
}
