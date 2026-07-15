package sqlcore

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
)

type fakeDriver struct {
	execErr error
	pingErr error
}

// Open creates a connection carrying the driver's configured failures.
func (d *fakeDriver) Open(name string) (driver.Conn, error) {
	return &fakeConn{execErr: d.execErr, pingErr: d.pingErr}, nil
}

type fakeConn struct {
	execErr error
	pingErr error
}

// Prepare returns a statement sufficient for database/sql compatibility tests.
func (c *fakeConn) Prepare(string) (driver.Stmt, error) { return &fakeStmt{}, nil }

// Close leaves the stateless fake connection ready for reuse.
func (c *fakeConn) Close() error { return nil }

// Begin rejects transactions because the SQL cache does not require them.
func (c *fakeConn) Begin() (driver.Tx, error) { return nil, errors.New("not impl") }

// ExecContext reports one affected row unless the fake was configured to fail.
func (c *fakeConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return driver.RowsAffected(1), c.execErr
}

// QueryContext returns an empty result set for lookup paths.
func (c *fakeConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return &fakeRows{}, nil
}

// Ping returns the configured readiness failure.
func (c *fakeConn) Ping(ctx context.Context) error { return c.pingErr }

type fakeRows struct{}

// Columns returns no metadata because fake queries yield no values.
func (r *fakeRows) Columns() []string { return []string{} }

// Close releases the stateless fake rows without work.
func (r *fakeRows) Close() error { return nil }

// Next terminates iteration using the error expected by the exercised query path.
func (r *fakeRows) Next(dest []driver.Value) error { return driver.ErrBadConn }

type fakeStmt struct{}

// Close releases the stateless fake statement without work.
func (s *fakeStmt) Close() error { return nil }

// NumInput allows any argument count so tests can exercise multiple SQL dialects.
func (s *fakeStmt) NumInput() int { return -1 }

// Exec reports a successful single-row mutation for legacy driver calls.
func (s *fakeStmt) Exec(args []driver.Value) (driver.Result, error) {
	return driver.RowsAffected(1), nil
}

// Query returns an empty result set for legacy driver calls.
func (s *fakeStmt) Query(args []driver.Value) (driver.Rows, error) { return &fakeRows{}, nil }

// init registers isolated database/sql drivers for success and failure scenarios.
func init() {
	sql.Register("pgfake", &fakeDriver{})
	sql.Register("mysqlfake", &fakeDriver{})
	sql.Register("pgfail", &fakeDriver{execErr: errors.New("boom")})
	sql.Register("postgres", &fakeDriver{})
	sql.Register("pingfail", &fakeDriver{pingErr: errors.New("ping boom")})
}
