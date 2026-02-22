package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type sqlStore struct {
	db            *sql.DB
	table         string
	driverName    string
	prefix        string
	defaultTTL    time.Duration
	getStmt       *sql.Stmt
	upsertStmt    *sql.Stmt
	addInsertStmt *sql.Stmt
	addReuseStmt  *sql.Stmt
	deleteStmt    *sql.Stmt
	flushStmt     *sql.Stmt
}

var sqlIdentPartRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func newSQLStore(cfg StoreConfig) (Store, error) {
	if cfg.SQLDriverName == "" || cfg.SQLDSN == "" {
		return nil, errors.New("sql driver requires driver name and dsn")
	}
	db, err := sql.Open(cfg.SQLDriverName, cfg.SQLDSN)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	table := cfg.SQLTable
	if table == "" {
		table = "cache_entries"
	}
	if err := validateSQLTableName(table); err != nil {
		return nil, err
	}
	ttl := cfg.DefaultTTL
	if ttl <= 0 {
		ttl = defaultCacheTTL
	}
	s := &sqlStore{
		db:         db,
		table:      table,
		driverName: cfg.SQLDriverName,
		prefix:     cfg.Prefix,
		defaultTTL: ttl,
	}
	if err := s.ensureSchema(); err != nil {
		return nil, err
	}
	if err := s.prepareStatements(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *sqlStore) Driver() Driver { return DriverSQL }

func (s *sqlStore) ensureSchema() error {
	var stmt string
	switch s.driverName {
	case "postgres", "pgx":
		stmt = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			k TEXT PRIMARY KEY,
			v BYTEA NOT NULL,
			ea BIGINT NOT NULL
		);`, s.table)
	case "mysql":
		stmt = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			k VARBINARY(255) PRIMARY KEY,
			v LONGBLOB NOT NULL,
			ea BIGINT NOT NULL
		) ENGINE=InnoDB;`, s.table)
	default: // sqlite
		stmt = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			k TEXT PRIMARY KEY,
			v BLOB NOT NULL,
			ea INTEGER NOT NULL
		);`, s.table)
	}
	_, err := s.db.Exec(stmt)
	return err
}

func (s *sqlStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	var v []byte
	var exp int64
	err := s.getStmt.QueryRowContext(ctx, s.cacheKey(key)).Scan(&v, &exp)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if time.Now().UnixMilli() > exp {
		_ = s.Delete(ctx, key)
		return nil, false, nil
	}
	return cloneBytes(v), true, nil
}

func (s *sqlStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	exp := time.Now().Add(ttl).UnixMilli()
	_, err := s.upsertStmt.ExecContext(ctx, s.cacheKey(key), value, exp, value, exp)
	return err
}

func (s *sqlStore) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	nowMs := time.Now().UnixMilli()
	exp := time.UnixMilli(nowMs).Add(ttl).UnixMilli()
	cacheKey := s.cacheKey(key)
	_, err := s.addInsertStmt.ExecContext(ctx, cacheKey, value, exp)
	if err != nil {
		if isDuplicateErr(err, s.driverName) {
			// Treat logically expired rows as absent so Add semantics match stores
			// that expire keys eagerly and so lock helpers can reacquire after TTL.
			res, updateErr := s.addReuseStmt.ExecContext(ctx, value, exp, cacheKey, nowMs)
			if updateErr != nil {
				return false, updateErr
			}
			rows, rowsErr := res.RowsAffected()
			if rowsErr != nil {
				return false, rowsErr
			}
			return rows > 0, nil
		}
		return false, err
	}
	return true, nil
}

func (s *sqlStore) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var v []byte
	var exp int64
	selectSQL := fmt.Sprintf("SELECT v, ea FROM %s WHERE k = %s", s.table, s.ph(1))
	if s.driverName == "postgres" || s.driverName == "pgx" || s.driverName == "mysql" {
		selectSQL += " FOR UPDATE"
	}
	err = tx.QueryRowContext(ctx, selectSQL, s.cacheKey(key)).Scan(&v, &exp)

	current := int64(0)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	if err == nil {
		if time.Now().UnixMilli() > exp {
			current = 0
		} else {
			current, err = strconv.ParseInt(string(v), 10, 64)
			if err != nil {
				return 0, fmt.Errorf("cache key %q does not contain a numeric value", key)
			}
		}
	}

	next := current + delta
	exp = time.Now().Add(ttl).UnixMilli()
	upsertStmt := tx.StmtContext(ctx, s.upsertStmt)
	defer upsertStmt.Close()
	_, err = upsertStmt.ExecContext(ctx, s.cacheKey(key), []byte(strconv.FormatInt(next, 10)), exp, []byte(strconv.FormatInt(next, 10)), exp)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return next, nil
}

func (s *sqlStore) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.Increment(ctx, key, -delta, ttl)
}

func (s *sqlStore) Delete(ctx context.Context, key string) error {
	_, err := s.deleteStmt.ExecContext(ctx, s.cacheKey(key))
	return err
}

func (s *sqlStore) DeleteMany(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	placeholders := make([]string, 0, len(keys))
	for i := range keys {
		placeholders = append(placeholders, s.ph(i+1))
	}
	args := make([]any, 0, len(keys))
	for _, k := range keys {
		args = append(args, s.cacheKey(k))
	}
	_, err := s.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE k IN (%s)", s.table, strings.Join(placeholders, ",")), args...)
	return err
}

func (s *sqlStore) Flush(ctx context.Context) error {
	_, err := s.flushStmt.ExecContext(ctx)
	return err
}

func (s *sqlStore) cacheKey(key string) string {
	if s.prefix == "" {
		return key
	}
	return s.prefix + ":" + key
}

func (s *sqlStore) upsertSQL() string {
	// Placeholders must be positional for postgres/pgx.
	p1, p2, p3, p4, p5 := s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5)
	switch s.driverName {
	case "postgres", "pgx":
		return fmt.Sprintf("INSERT INTO %s (k, v, ea) VALUES (%s, %s, %s) ON CONFLICT (k) DO UPDATE SET v = %s, ea = %s", s.table, p1, p2, p3, p4, p5)
	case "mysql":
		return fmt.Sprintf("INSERT INTO %s (k, v, ea) VALUES (%s, %s, %s) ON DUPLICATE KEY UPDATE v = %s, ea = %s", s.table, p1, p2, p3, p4, p5)
	default: // sqlite
		return fmt.Sprintf("INSERT INTO %s (k, v, ea) VALUES (%s, %s, %s) ON CONFLICT(k) DO UPDATE SET v = %s, ea = %s", s.table, p1, p2, p3, p4, p5)
	}
}

func (s *sqlStore) getSQL() string {
	return fmt.Sprintf("SELECT v, ea FROM %s WHERE k = %s", s.table, s.ph(1))
}

func (s *sqlStore) addInsertSQL() string {
	return fmt.Sprintf("INSERT INTO %s (k, v, ea) VALUES (%s, %s, %s)", s.table, s.ph(1), s.ph(2), s.ph(3))
}

func (s *sqlStore) addReuseExpiredSQL() string {
	return fmt.Sprintf("UPDATE %s SET v = %s, ea = %s WHERE k = %s AND ea < %s", s.table, s.ph(1), s.ph(2), s.ph(3), s.ph(4))
}

func (s *sqlStore) deleteSQL() string {
	return fmt.Sprintf("DELETE FROM %s WHERE k = %s", s.table, s.ph(1))
}

func (s *sqlStore) flushSQL() string {
	return fmt.Sprintf("DELETE FROM %s", s.table)
}

func (s *sqlStore) prepareStatements() error {
	var err error
	if s.getStmt, err = s.db.Prepare(s.getSQL()); err != nil {
		return err
	}
	if s.upsertStmt, err = s.db.Prepare(s.upsertSQL()); err != nil {
		return err
	}
	if s.addInsertStmt, err = s.db.Prepare(s.addInsertSQL()); err != nil {
		return err
	}
	if s.addReuseStmt, err = s.db.Prepare(s.addReuseExpiredSQL()); err != nil {
		return err
	}
	if s.deleteStmt, err = s.db.Prepare(s.deleteSQL()); err != nil {
		return err
	}
	if s.flushStmt, err = s.db.Prepare(s.flushSQL()); err != nil {
		return err
	}
	return nil
}

func (s *sqlStore) ph(i int) string {
	if s.driverName == "postgres" || s.driverName == "pgx" {
		return fmt.Sprintf("$%d", i)
	}
	return "?"
}

func isDuplicateErr(err error, driver string) bool {
	msg := err.Error()
	switch driver {
	case "postgres", "pgx":
		return strings.Contains(msg, "duplicate key value")
	case "mysql":
		return strings.Contains(msg, "Duplicate entry")
	default:
		return strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "unique constraint")
	}
}

func validateSQLTableName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("sql table name is required")
	}
	for _, part := range strings.Split(name, ".") {
		if !sqlIdentPartRE.MatchString(part) {
			return fmt.Errorf("invalid sql table name %q", name)
		}
	}
	return nil
}
