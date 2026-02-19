package cache

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

func (d *fakeDriver) Open(name string) (driver.Conn, error) {
	return &fakeConn{execErr: d.execErr, pingErr: d.pingErr}, nil
}

type fakeConn struct {
	execErr error
	pingErr error
}

func (c *fakeConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not impl") }
func (c *fakeConn) Close() error                        { return nil }
func (c *fakeConn) Begin() (driver.Tx, error)           { return nil, errors.New("not impl") }

func (c *fakeConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return driver.RowsAffected(1), c.execErr
}
func (c *fakeConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return &fakeRows{}, nil
}
func (c *fakeConn) Ping(ctx context.Context) error { return c.pingErr }

type fakeRows struct{}

func (r *fakeRows) Columns() []string              { return []string{} }
func (r *fakeRows) Close() error                   { return nil }
func (r *fakeRows) Next(dest []driver.Value) error { return driver.ErrBadConn }

func init() {
	sql.Register("pgfake", &fakeDriver{})
	sql.Register("mysqlfake", &fakeDriver{})
	sql.Register("pgfail", &fakeDriver{execErr: errors.New("boom")})
	sql.Register("postgres", &fakeDriver{})
	sql.Register("pingfail", &fakeDriver{pingErr: errors.New("ping boom")})
}
