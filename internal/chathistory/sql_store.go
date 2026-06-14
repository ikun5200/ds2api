package chathistory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	sqlStateID            = "default"
	defaultSQLTablePrefix = "ds2api_"
)

type SQLConfig struct {
	Type            string
	DSN             string
	TablePrefix     string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type sqlDialect struct {
	name   string
	driver string
}

type sqlStore struct {
	mu           sync.Mutex
	db           *sql.DB
	dialect      sqlDialect
	stateTable   string
	entriesTable string
	err          error
}

type sqlRunner interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func NewSQL(cfg SQLConfig) *Store {
	return &Store{sql: newSQLStore(cfg)}
}

func newSQLStore(cfg SQLConfig) *sqlStore {
	store := &sqlStore{}
	store.err = store.open(cfg)
	return store
}

func (s *sqlStore) open(cfg SQLConfig) (err error) {
	dialect, err := normalizeSQLDialect(cfg.Type)
	if err != nil {
		return err
	}
	dsn, err := normalizeSQLDSN(dialect, cfg.DSN)
	if err != nil {
		return err
	}
	if strings.TrimSpace(dsn) == "" {
		return errors.New("external database dsn is required")
	}
	prefix := strings.ToLower(strings.TrimSpace(cfg.TablePrefix))
	if prefix == "" {
		prefix = defaultSQLTablePrefix
	}
	stateTable, err := sqlTableName(prefix, "chat_history_state")
	if err != nil {
		return err
	}
	entriesTable, err := sqlTableName(prefix, "chat_history_entries")
	if err != nil {
		return err
	}

	db, err := sql.Open(dialect.driver, dsn)
	if err != nil {
		return fmt.Errorf("open chat history database: %w", err)
	}
	defer func() {
		if err != nil {
			if closeErr := db.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("close chat history database after init failure: %w", closeErr))
			}
		}
	}()
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}

	s.db = db
	s.dialect = dialect
	s.stateTable = stateTable
	s.entriesTable = entriesTable

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping chat history database: %w", err)
	}
	if err := s.initSchema(ctx); err != nil {
		return err
	}
	return nil
}

func (s *sqlStore) Path() string {
	if s == nil {
		return ""
	}
	return "sql:" + s.dialect.name + ":" + s.entriesTable
}

func (s *sqlStore) Err() error {
	if s == nil {
		return errors.New("chat history sql store is nil")
	}
	return s.err
}

func (s *sqlStore) Snapshot() (File, error) {
	if err := s.available(); err != nil {
		return File{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx := context.Background()
	state, err := s.loadState(ctx, s.db, false)
	if err != nil {
		return File{}, err
	}
	entries, err := s.loadEntries(ctx, s.db)
	if err != nil {
		return File{}, err
	}
	items := make([]SummaryEntry, 0, len(entries))
	for _, entry := range entries {
		items = append(items, summaryFromEntry(entry))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt == items[j].CreatedAt {
			if items[i].Revision == items[j].Revision {
				return items[i].UpdatedAt > items[j].UpdatedAt
			}
			return items[i].Revision > items[j].Revision
		}
		return items[i].CreatedAt > items[j].CreatedAt
	})
	if state.Limit != DisabledLimit && len(items) > state.Limit {
		items = items[:state.Limit]
	}
	return cloneFile(File{
		Version:  FileVersion,
		Limit:    state.Limit,
		Revision: state.Revision,
		Items:    items,
	}), nil
}

func (s *sqlStore) Revision() (int64, error) {
	if err := s.available(); err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.loadState(context.Background(), s.db, false)
	if err != nil {
		return 0, err
	}
	return state.Revision, nil
}

func (s *sqlStore) Enabled() bool {
	if err := s.available(); err != nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.loadState(context.Background(), s.db, false)
	return err == nil && state.Limit != DisabledLimit
}

func (s *sqlStore) Get(id string) (Entry, error) {
	if err := s.available(); err != nil {
		return Entry{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok, err := s.getEntry(context.Background(), s.db, strings.TrimSpace(id))
	if err != nil {
		return Entry{}, err
	}
	if !ok {
		return Entry{}, errors.New("chat history entry not found")
	}
	return cloneEntry(entry), nil
}

func (s *sqlStore) DetailRevision(id string) (int64, error) {
	if err := s.available(); err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok, err := s.getEntry(context.Background(), s.db, strings.TrimSpace(id))
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, errors.New("chat history entry not found")
	}
	return entry.Revision, nil
}

func (s *sqlStore) Start(params StartParams) (Entry, error) {
	if err := s.available(); err != nil {
		return Entry{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	var entry Entry
	err := s.withTx(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		state, err := s.loadState(ctx, tx, true)
		if err != nil {
			return err
		}
		if state.Limit == DisabledLimit {
			return ErrDisabled
		}
		now := time.Now().UnixMilli()
		revision := nextSQLRevision(state.Revision)
		entry = Entry{
			ID:          "chat_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
			Revision:    revision,
			CreatedAt:   now,
			UpdatedAt:   now,
			Status:      "streaming",
			CallerID:    strings.TrimSpace(params.CallerID),
			AccountID:   strings.TrimSpace(params.AccountID),
			Surface:     strings.TrimSpace(params.Surface),
			Model:       strings.TrimSpace(params.Model),
			Stream:      params.Stream,
			UserInput:   strings.TrimSpace(params.UserInput),
			Messages:    cloneMessages(params.Messages),
			HistoryText: params.HistoryText,
			FinalPrompt: strings.TrimSpace(params.FinalPrompt),
		}
		if err := s.upsertEntry(ctx, tx, entry); err != nil {
			return err
		}
		state.Revision = revision
		if err := s.applyRetention(ctx, tx, state.Limit); err != nil {
			return err
		}
		return s.upsertState(ctx, tx, state)
	})
	if err != nil {
		return cloneEntry(entry), err
	}
	return cloneEntry(entry), nil
}

func (s *sqlStore) Update(id string, params UpdateParams) (Entry, error) {
	if err := s.available(); err != nil {
		return Entry{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	var updated Entry
	err := s.withTx(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		state, err := s.loadState(ctx, tx, true)
		if err != nil {
			return err
		}
		target := strings.TrimSpace(id)
		if target == "" {
			return errors.New("history id is required")
		}
		item, ok, err := s.getEntry(ctx, tx, target)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("chat history entry not found")
		}
		now := time.Now().UnixMilli()
		item.Revision = nextSQLRevision(state.Revision)
		item.UpdatedAt = now
		if params.Status != "" {
			item.Status = params.Status
		}
		if params.ReasoningContent != "" || item.ReasoningContent == "" {
			item.ReasoningContent = params.ReasoningContent
		}
		if params.Content != "" || item.Content == "" {
			item.Content = params.Content
		}
		item.Error = strings.TrimSpace(params.Error)
		item.StatusCode = params.StatusCode
		item.ElapsedMs = params.ElapsedMs
		item.FinishReason = strings.TrimSpace(params.FinishReason)
		if params.Usage != nil {
			item.Usage = cloneMap(params.Usage)
		}
		if params.Completed {
			item.CompletedAt = now
		}
		if err := s.upsertEntry(ctx, tx, item); err != nil {
			return err
		}
		state.Revision = item.Revision
		if err := s.applyRetention(ctx, tx, state.Limit); err != nil {
			return err
		}
		if err := s.upsertState(ctx, tx, state); err != nil {
			return err
		}
		updated = item
		return nil
	})
	if err != nil {
		return Entry{}, err
	}
	return cloneEntry(updated), nil
}

func (s *sqlStore) Delete(id string) error {
	if err := s.available(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.withTx(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		state, err := s.loadState(ctx, tx, true)
		if err != nil {
			return err
		}
		target := strings.TrimSpace(id)
		if target == "" {
			return errors.New("history id is required")
		}
		res, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE id = %s", s.entriesTable, s.dialect.placeholder(1)), target)
		if err != nil {
			return fmt.Errorf("delete chat history entry: %w", err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("check deleted chat history entry: %w", err)
		}
		if affected == 0 {
			return errors.New("chat history entry not found")
		}
		state.Revision = nextSQLRevision(state.Revision)
		return s.upsertState(ctx, tx, state)
	})
}

func (s *sqlStore) Clear() error {
	if err := s.available(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.withTx(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		state, err := s.loadState(ctx, tx, true)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s", s.entriesTable)); err != nil {
			return fmt.Errorf("clear chat history entries: %w", err)
		}
		state.Revision = nextSQLRevision(state.Revision)
		return s.upsertState(ctx, tx, state)
	})
}

func (s *sqlStore) SetLimit(limit int) (File, error) {
	if err := s.available(); err != nil {
		return File{}, err
	}
	if !isAllowedLimit(limit) {
		return File{}, fmt.Errorf("unsupported chat history limit: %d", limit)
	}
	err := func() error {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.withTx(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
			state, err := s.loadState(ctx, tx, true)
			if err != nil {
				return err
			}
			state.Limit = limit
			state.Revision = nextSQLRevision(state.Revision)
			if err := s.applyRetention(ctx, tx, state.Limit); err != nil {
				return err
			}
			return s.upsertState(ctx, tx, state)
		})
	}()
	if err != nil {
		return File{}, err
	}
	return s.Snapshot()
}

func (s *sqlStore) initSchema(ctx context.Context) error {
	stmts := []string{
		s.dialect.createStateTableSQL(s.stateTable),
		s.dialect.createEntriesTableSQL(s.entriesTable),
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("init chat history database schema: %w", err)
		}
	}
	state, err := s.loadState(ctx, s.db, false)
	if errors.Is(err, sql.ErrNoRows) {
		state = defaultSQLState()
		revision, revErr := s.maxEntryRevision(ctx, s.db)
		if revErr != nil {
			return revErr
		}
		state.Revision = revision
		return s.upsertState(ctx, s.db, state)
	}
	if err != nil {
		return err
	}
	return s.upsertState(ctx, s.db, state)
}

func (s *sqlStore) available() error {
	if s == nil {
		return errors.New("chat history sql store is nil")
	}
	if s.err != nil {
		return s.err
	}
	if s.db == nil {
		return errors.New("chat history database is not open")
	}
	return nil
}

func (s *sqlStore) withTx(ctx context.Context, fn func(context.Context, *sql.Tx) error) (err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin chat history database transaction: %w", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("rollback chat history database transaction: %w", rollbackErr))
		}
	}()
	if err := fn(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit chat history database transaction: %w", err)
	}
	committed = true
	return nil
}

func (s *sqlStore) loadState(ctx context.Context, runner sqlRunner, lock bool) (File, error) {
	query := fmt.Sprintf(
		"SELECT version, limit_value, revision FROM %s WHERE id = %s%s",
		s.stateTable,
		s.dialect.placeholder(1),
		s.dialect.lockSuffix(lock),
	)
	state := defaultSQLState()
	if err := runner.QueryRowContext(ctx, query, sqlStateID).Scan(&state.Version, &state.Limit, &state.Revision); err != nil {
		return File{}, err
	}
	return normalizeSQLState(state), nil
}

func (s *sqlStore) upsertState(ctx context.Context, runner sqlRunner, state File) error {
	state = normalizeSQLState(state)
	query := s.dialect.upsertStateSQL(s.stateTable)
	if _, err := runner.ExecContext(ctx, query, sqlStateID, state.Version, state.Limit, state.Revision); err != nil {
		return fmt.Errorf("save chat history database state: %w", err)
	}
	return nil
}

func (s *sqlStore) maxEntryRevision(ctx context.Context, runner sqlRunner) (int64, error) {
	var revision int64
	query := fmt.Sprintf("SELECT COALESCE(MAX(revision), 0) FROM %s", s.entriesTable)
	if err := runner.QueryRowContext(ctx, query).Scan(&revision); err != nil {
		return 0, fmt.Errorf("load max chat history revision: %w", err)
	}
	return revision, nil
}

func (s *sqlStore) loadEntries(ctx context.Context, runner sqlRunner) (entries []Entry, err error) {
	query := fmt.Sprintf("SELECT payload FROM %s ORDER BY created_at DESC, revision DESC, updated_at DESC", s.entriesTable)
	rows, err := runner.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("load chat history entries: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close chat history entry rows: %w", closeErr))
		}
	}()
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, fmt.Errorf("scan chat history entry: %w", err)
		}
		entry, err := decodeSQLEntry(payload)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(entry.ID) != "" {
			entries = append(entries, entry)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat history entries: %w", err)
	}
	return entries, nil
}

func (s *sqlStore) getEntry(ctx context.Context, runner sqlRunner, id string) (Entry, bool, error) {
	if id == "" {
		return Entry{}, false, nil
	}
	var payload string
	query := fmt.Sprintf("SELECT payload FROM %s WHERE id = %s", s.entriesTable, s.dialect.placeholder(1))
	if err := runner.QueryRowContext(ctx, query, id).Scan(&payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Entry{}, false, nil
		}
		return Entry{}, false, fmt.Errorf("load chat history entry: %w", err)
	}
	entry, err := decodeSQLEntry(payload)
	if err != nil {
		return Entry{}, false, err
	}
	return entry, true, nil
}

func (s *sqlStore) upsertEntry(ctx context.Context, runner sqlRunner, entry Entry) error {
	payload, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encode chat history entry: %w", err)
	}
	query := s.dialect.upsertEntrySQL(s.entriesTable)
	if _, err := runner.ExecContext(ctx, query, entry.ID, entry.Revision, entry.CreatedAt, entry.UpdatedAt, string(payload)); err != nil {
		return fmt.Errorf("save chat history entry: %w", err)
	}
	return nil
}

func (s *sqlStore) applyRetention(ctx context.Context, tx *sql.Tx, limit int) (err error) {
	if limit == DisabledLimit {
		return nil
	}
	query := fmt.Sprintf("SELECT id FROM %s ORDER BY created_at DESC, revision DESC, updated_at DESC", s.entriesTable)
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("load retained chat history ids: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close retained chat history rows: %w", closeErr))
		}
	}()

	position := 0
	deleteIDs := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("scan retained chat history id: %w", err)
		}
		if position >= limit {
			deleteIDs = append(deleteIDs, id)
		}
		position++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate retained chat history ids: %w", err)
	}
	if len(deleteIDs) == 0 {
		return nil
	}
	return s.deleteEntriesByID(ctx, tx, deleteIDs)
}

func (s *sqlStore) deleteEntriesByID(ctx context.Context, runner sqlRunner, ids []string) error {
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = s.dialect.placeholder(i + 1)
		args[i] = id
	}
	query := fmt.Sprintf("DELETE FROM %s WHERE id IN (%s)", s.entriesTable, strings.Join(placeholders, ", "))
	if _, err := runner.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("delete old chat history entries: %w", err)
	}
	return nil
}

func decodeSQLEntry(payload string) (Entry, error) {
	var entry Entry
	if err := json.Unmarshal([]byte(payload), &entry); err != nil {
		return Entry{}, fmt.Errorf("decode chat history entry: %w", err)
	}
	return cloneEntry(entry), nil
}

func defaultSQLState() File {
	return File{
		Version:  FileVersion,
		Limit:    DefaultLimit,
		Revision: 0,
		Items:    []SummaryEntry{},
	}
}

func normalizeSQLState(state File) File {
	state.Version = FileVersion
	if !isAllowedLimit(state.Limit) {
		state.Limit = DefaultLimit
	}
	state.Items = nil
	return state
}

func nextSQLRevision(current int64) int64 {
	next := time.Now().UnixNano()
	if next <= current {
		return current + 1
	}
	return next
}

func normalizeSQLDialect(dbType string) (sqlDialect, error) {
	switch strings.ToLower(strings.TrimSpace(dbType)) {
	case "postgres", "postgresql", "pg", "pgx":
		return sqlDialect{name: "postgres", driver: "pgx"}, nil
	case "mysql", "mariadb":
		return sqlDialect{name: "mysql", driver: "mysql"}, nil
	default:
		if strings.TrimSpace(dbType) == "" {
			return sqlDialect{}, errors.New("external database type is required")
		}
		return sqlDialect{}, fmt.Errorf("unsupported external database type: %s", dbType)
	}
}

func normalizeSQLDSN(dialect sqlDialect, dsn string) (string, error) {
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

func sqlTableName(prefix, suffix string) (string, error) {
	name := strings.TrimSpace(prefix) + suffix
	if err := validateSQLIdentifier(name); err != nil {
		return "", err
	}
	return name, nil
}

func validateSQLIdentifier(name string) error {
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

func (d sqlDialect) placeholder(position int) string {
	if d.name == "postgres" {
		return fmt.Sprintf("$%d", position)
	}
	return "?"
}

func (d sqlDialect) lockSuffix(lock bool) string {
	if !lock {
		return ""
	}
	switch d.name {
	case "postgres", "mysql":
		return " FOR UPDATE"
	default:
		return ""
	}
}

func (d sqlDialect) upsertStateSQL(table string) string {
	switch d.name {
	case "postgres":
		return fmt.Sprintf(
			"INSERT INTO %s (id, version, limit_value, revision) "+
				"VALUES ($1, $2, $3, $4) "+
				"ON CONFLICT (id) DO UPDATE SET "+
				"version = EXCLUDED.version, "+
				"limit_value = EXCLUDED.limit_value, "+
				"revision = EXCLUDED.revision",
			table,
		)
	default:
		return fmt.Sprintf(
			"INSERT INTO %s (id, version, limit_value, revision) "+
				"VALUES (?, ?, ?, ?) "+
				"ON DUPLICATE KEY UPDATE "+
				"version = VALUES(version), "+
				"limit_value = VALUES(limit_value), "+
				"revision = VALUES(revision)",
			table,
		)
	}
}

func (d sqlDialect) upsertEntrySQL(table string) string {
	switch d.name {
	case "postgres":
		return fmt.Sprintf(
			"INSERT INTO %s (id, revision, created_at, updated_at, payload) "+
				"VALUES ($1, $2, $3, $4, $5) "+
				"ON CONFLICT (id) DO UPDATE SET "+
				"revision = EXCLUDED.revision, "+
				"created_at = EXCLUDED.created_at, "+
				"updated_at = EXCLUDED.updated_at, "+
				"payload = EXCLUDED.payload",
			table,
		)
	default:
		return fmt.Sprintf(
			"INSERT INTO %s (id, revision, created_at, updated_at, payload) "+
				"VALUES (?, ?, ?, ?, ?) "+
				"ON DUPLICATE KEY UPDATE "+
				"revision = VALUES(revision), "+
				"created_at = VALUES(created_at), "+
				"updated_at = VALUES(updated_at), "+
				"payload = VALUES(payload)",
			table,
		)
	}
}

func (d sqlDialect) createStateTableSQL(table string) string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
id VARCHAR(64) PRIMARY KEY,
version INTEGER NOT NULL,
limit_value INTEGER NOT NULL,
revision BIGINT NOT NULL
)`, table)
}

func (d sqlDialect) createEntriesTableSQL(table string) string {
	payloadType := "TEXT"
	if d.name == "mysql" {
		payloadType = "LONGTEXT"
	}
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
id VARCHAR(96) PRIMARY KEY,
revision BIGINT NOT NULL,
created_at BIGINT NOT NULL,
updated_at BIGINT NOT NULL,
payload %s NOT NULL
)`, table, payloadType)
}
