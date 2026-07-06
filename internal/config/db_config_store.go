package config

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	dbConfigID        = "default"
	dbConfigSuffix    = "config_store"
	dbDefaultPrefix   = "ds2api_"
	dbDefaultSyncSecs = 5
	dbMinSyncSecs     = 1
	dbMaxSyncSecs     = 300
)

// ---------------------------------------------------------------------------
// Self‑contained SQL dialect helpers (mirrors chathistory internals so we
// don't need to export them).
// ---------------------------------------------------------------------------

type dbDialect struct {
	name   string
	driver string
}

func normalizeConfigDBDialect(dbType string) (dbDialect, error) {
	switch strings.ToLower(strings.TrimSpace(dbType)) {
	case "postgres", "postgresql", "pg", "pgx":
		return dbDialect{name: "postgres", driver: "pgx"}, nil
	case "mysql", "mariadb":
		return dbDialect{name: "mysql", driver: "mysql"}, nil
	default:
		if strings.TrimSpace(dbType) == "" {
			return dbDialect{}, errors.New("external database type is required")
		}
		return dbDialect{}, fmt.Errorf("unsupported external database type: %s", dbType)
	}
}

func normalizeConfigDBDSN(dialect dbDialect, dsn string) (string, error) {
	dsn = strings.TrimSpace(dsn)
	if dialect.name == "mysql" && strings.HasPrefix(strings.ToLower(dsn), "mysql://") {
		return mysqlURLToDSN(dsn)
	}
	return dsn, nil
}

func mysqlURLToDSN(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse mysql database url: %w", err)
	}
	user := parsed.User.Username()
	password, hasPassword := parsed.User.Password()
	auth := user
	if hasPassword {
		auth += ":" + password
	}
	host := parsed.Host
	dbName := strings.TrimPrefix(parsed.Path, "/")
	var b strings.Builder
	if auth != "" {
		b.WriteString(auth)
		b.WriteByte('@')
	}
	if host != "" {
		b.WriteString("tcp(")
		b.WriteString(host)
		b.WriteByte(')')
	}
	b.WriteByte('/')
	b.WriteString(dbName)
	if parsed.RawQuery != "" {
		b.WriteByte('?')
		b.WriteString(parsed.RawQuery)
	}
	return b.String(), nil
}

func validateConfigTableName(name string) error {
	if name == "" {
		return errors.New("database table name is required")
	}
	for i, r := range name {
		if i == 0 && ((r < 'a' || r > 'z') && r != '_') {
			return fmt.Errorf("invalid database table name %q", name)
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return fmt.Errorf("invalid database table name %q", name)
	}
	return nil
}

func configTableName(prefix, suffix string) (string, error) {
	name := strings.TrimSpace(prefix) + suffix
	if err := validateConfigTableName(name); err != nil {
		return "", err
	}
	return name, nil
}

func nextConfigRevision(current int64) int64 {
	next := time.Now().UnixNano()
	if next <= current {
		return current + 1
	}
	return next
}

func (d dbDialect) placeholder(position int) string {
	if d.name == "postgres" {
		return fmt.Sprintf("$%d", position)
	}
	return "?"
}

func (d dbDialect) createConfigTableSQL(table string) string {
	payloadType := "TEXT"
	if d.name == "mysql" {
		payloadType = "LONGTEXT"
	}
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
id VARCHAR(64) PRIMARY KEY,
revision BIGINT NOT NULL,
payload %s NOT NULL,
updated_at BIGINT NOT NULL
)`, table, payloadType)
}

func (d dbDialect) upsertConfigSQL(table string) string {
	switch d.name {
	case "postgres":
		return fmt.Sprintf(
			"INSERT INTO %s (id, revision, payload, updated_at) "+
				"VALUES ($1, $2, $3, $4) "+
				"ON CONFLICT (id) DO UPDATE SET "+
				"revision = EXCLUDED.revision, "+
				"payload = EXCLUDED.payload, "+
				"updated_at = EXCLUDED.updated_at",
			table,
		)
	default:
		return fmt.Sprintf(
			"INSERT INTO %s (id, revision, payload, updated_at) "+
				"VALUES (?, ?, ?, ?) "+
				"ON DUPLICATE KEY UPDATE "+
				"revision = VALUES(revision), "+
				"payload = VALUES(payload), "+
				"updated_at = VALUES(updated_at)",
			table,
		)
	}
}

// ---------------------------------------------------------------------------
// dbConfigStore
// ---------------------------------------------------------------------------

// dbConfigStore persists the full Config as a single JSON row in an external
// SQL database and supports polling-based real‑time sync across instances.
type dbConfigStore struct {
	db          *sql.DB
	dialect     dbDialect
	table       string
	revision    atomic.Int64
	syncTicker  *time.Ticker
	syncDone    chan struct{}
	onReload    func(Config)
	initialised atomic.Bool
}

func openDBConfigStore() (*dbConfigStore, error) {
	cfg := DatabaseFromEnv()
	if !cfg.ExternalEnabled() {
		return nil, nil
	}

	dialect, err := normalizeConfigDBDialect(cfg.Type)
	if err != nil {
		return nil, fmt.Errorf("config db: %w", err)
	}
	dsn, err := normalizeConfigDBDSN(dialect, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("config db dsn: %w", err)
	}
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("config db: external database dsn is required")
	}

	prefix := strings.ToLower(strings.TrimSpace(cfg.TablePrefix))
	if prefix == "" {
		prefix = dbDefaultPrefix
	}
	table, err := configTableName(prefix, dbConfigSuffix)
	if err != nil {
		return nil, fmt.Errorf("config db table name: %w", err)
	}

	db, err := sql.Open(dialect.driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("config db open: %w", err)
	}
	defer func() {
		if err != nil && db != nil {
			closeErr := db.Close()
			err = errors.Join(err, fmt.Errorf("close config database after init failure: %w", closeErr))
		}
	}()

	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetimeSeconds > 0 {
		db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetimeSeconds) * time.Second)
	}

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("config db ping: %w", err)
	}

	s := &dbConfigStore{db: db, dialect: dialect, table: table}
	if err := s.initSchema(ctx); err != nil {
		closeErr := db.Close()
		err = errors.Join(err, fmt.Errorf("close config database after init failure: %w", closeErr))
		return nil, err
	}
	s.initialised.Store(true)
	return s, nil
}

func (s *dbConfigStore) Load(ctx context.Context) (Config, int64, error) {
	if s == nil || s.db == nil {
		return Config{}, 0, errors.New("config db store is nil")
	}
	payload, rev, err := s.loadPayload(ctx, s.db)
	if errors.Is(err, sql.ErrNoRows) {
		return Config{}, 0, nil
	}
	if err != nil {
		return Config{}, 0, err
	}
	var cfg Config
	if err := json.Unmarshal([]byte(payload), &cfg); err != nil {
		return Config{}, 0, fmt.Errorf("config db decode: %w", err)
	}
	cfg.NormalizeCredentials()
	s.revision.Store(rev)
	return cfg, rev, nil
}

func (s *dbConfigStore) Save(ctx context.Context, cfg Config) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("config db store is nil")
	}
	persistCfg := cfg.Clone()
	persistCfg.ClearAccountTokens()
	payload, err := json.Marshal(persistCfg)
	if err != nil {
		return 0, fmt.Errorf("config db marshal: %w", err)
	}
	rev := nextConfigRevision(s.revision.Load())
	if err := s.upsertPayload(ctx, s.db, string(payload), rev); err != nil {
		return 0, err
	}
	s.revision.Store(rev)
	return rev, nil
}

func (s *dbConfigStore) StartWatch(ctx context.Context, onReload func(Config), syncIntervalSecs int) {
	if s == nil || s.db == nil || s.syncTicker != nil {
		return
	}
	if syncIntervalSecs <= 0 {
		syncIntervalSecs = dbDefaultSyncSecs
	} else if syncIntervalSecs < dbMinSyncSecs {
		syncIntervalSecs = dbMinSyncSecs
	} else if syncIntervalSecs > dbMaxSyncSecs {
		syncIntervalSecs = dbMaxSyncSecs
	}
	s.onReload = onReload
	s.syncTicker = time.NewTicker(time.Duration(syncIntervalSecs) * time.Second)
	s.syncDone = make(chan struct{})
	go s.watchLoop(ctx)
}

func (s *dbConfigStore) StopWatch() {
	if s == nil || s.syncTicker == nil {
		return
	}
	s.syncTicker.Stop()
	if s.syncDone != nil {
		close(s.syncDone)
		s.syncDone = nil
	}
	s.syncTicker = nil
}

func (s *dbConfigStore) Close() error {
	s.StopWatch()
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

func (s *dbConfigStore) watchLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.syncDone:
			return
		case <-s.syncTicker.C:
		}
		if err := s.tryReload(ctx); err != nil {
			Logger.Warn("[config_db] sync poll failed", "error", err)
		}
	}
}

func (s *dbConfigStore) tryReload(ctx context.Context) error {
	if s.db == nil {
		return errors.New("db closed")
	}
	payload, rev, err := s.loadPayload(ctx, s.db)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	localRev := s.revision.Load()
	if rev <= localRev {
		return nil
	}
	var cfg Config
	if err := json.Unmarshal([]byte(payload), &cfg); err != nil {
		return fmt.Errorf("config db decode on reload: %w", err)
	}
	cfg.NormalizeCredentials()
	s.revision.Store(rev)
	if s.onReload != nil {
		s.onReload(cfg)
	}
	Logger.Info("[config_db] reloaded config from database",
		"old_revision", localRev,
		"new_revision", rev,
	)
	return nil
}

func (s *dbConfigStore) initSchema(ctx context.Context) error {
	query := s.dialect.createConfigTableSQL(s.table)
	if _, err := s.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("config db init schema: %w", err)
	}
	return nil
}

func (s *dbConfigStore) loadPayload(ctx context.Context, runner interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}) (string, int64, error) {
	query := fmt.Sprintf(
		"SELECT payload, revision FROM %s WHERE id = %s",
		s.table,
		s.dialect.placeholder(1),
	)
	var payload string
	var revision int64
	if err := runner.QueryRowContext(ctx, query, dbConfigID).Scan(&payload, &revision); err != nil {
		return "", 0, err
	}
	return payload, revision, nil
}

func (s *dbConfigStore) upsertPayload(ctx context.Context, runner interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, payload string, revision int64) error {
	now := time.Now().UnixMilli()
	query := s.dialect.upsertConfigSQL(s.table)
	_, err := runner.ExecContext(ctx, query, dbConfigID, revision, payload, now)
	if err != nil {
		return fmt.Errorf("config db upsert: %w", err)
	}
	return nil
}
