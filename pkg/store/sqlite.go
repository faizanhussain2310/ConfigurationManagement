package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/faizanhussain/arbiter/pkg/engine"
	_ "modernc.org/sqlite"
)

// Store wraps two SQLite connection pools (read + write) and provides
// all database operations for rules, versions, eval history, and metadata.
type Store struct {
	writeDB *sql.DB
	readDB  *sql.DB

	// Per-rule counters for eval history pruning.
	// Every 100th insert triggers a prune to keep max 1000 entries per rule.
	pruneCounters sync.Map // map[string]*atomic.Int64
}

// New opens a SQLite database and runs migrations.
func New(dbPath string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_txlock=immediate", dbPath)

	writeDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open write db: %w", err)
	}
	writeDB.SetMaxOpenConns(1)

	readDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open read db: %w", err)
	}
	readDB.SetMaxOpenConns(4)

	// Run PRAGMAs on both pools
	for _, db := range []*sql.DB{writeDB, readDB} {
		for _, pragma := range []string{
			"PRAGMA journal_mode = WAL",
			"PRAGMA busy_timeout = 5000",
			"PRAGMA synchronous = NORMAL",
			"PRAGMA foreign_keys = ON",
		} {
			if _, err := db.Exec(pragma); err != nil {
				return nil, fmt.Errorf("pragma %q: %w", pragma, err)
			}
		}
	}

	// Run migrations on write DB
	if _, err := writeDB.Exec(schema); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{writeDB: writeDB, readDB: readDB}, nil
}

// Close closes both database connections.
func (s *Store) Close() error {
	if err := s.writeDB.Close(); err != nil {
		return err
	}
	return s.readDB.Close()
}

// --- Rules CRUD ---

// CreateRule inserts a new rule and its initial version (v1) atomically.
func (s *Store) CreateRule(ctx context.Context, r *engine.Rule, modifiedBy string) error {
	tx, err := s.writeDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	r.Version = 1
	now := time.Now().UTC()
	r.CreatedAt = now
	r.UpdatedAt = now
	if r.Status == "" {
		r.Status = "active"
	}
	if r.Environment == "" {
		r.Environment = "production"
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO rules (id, name, description, type, version, tree, default_value, status, environment, active_from, active_until, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.Name, r.Description, r.Type, r.Version,
		string(r.Tree), nullableString(r.DefaultValue),
		r.Status, r.Environment, nullableTime(r.ActiveFrom), nullableTime(r.ActiveUntil),
		now, now,
	)
	if err != nil {
		return fmt.Errorf("insert rule: %w", err)
	}

	// Insert initial version snapshot
	_, err = tx.ExecContext(ctx,
		`INSERT INTO rule_versions (rule_id, version, name, description, type, tree, default_value, status, environment, active_from, active_until, modified_by, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, 1, r.Name, r.Description, r.Type,
		string(r.Tree), nullableString(r.DefaultValue),
		r.Status, r.Environment, nullableTime(r.ActiveFrom), nullableTime(r.ActiveUntil),
		modifiedBy, now,
	)
	if err != nil {
		return fmt.Errorf("insert version: %w", err)
	}

	return tx.Commit()
}

// GetRule fetches a single rule by ID.
func (s *Store) GetRule(ctx context.Context, id string) (*engine.Rule, error) {
	row := s.readDB.QueryRowContext(ctx,
		`SELECT id, name, description, type, version, tree, default_value, status, environment, active_from, active_until, created_at, updated_at
		 FROM rules WHERE id = ?`, id)
	return scanRule(row)
}

// ListRules returns paginated rules, optionally filtered by environment.
func (s *Store) ListRules(ctx context.Context, limit, offset int, environment string) ([]*engine.Rule, int, error) {
	var total int
	var args []any

	countQuery := `SELECT COUNT(*) FROM rules`
	listQuery := `SELECT id, name, description, type, version, tree, default_value, status, environment, active_from, active_until, created_at, updated_at FROM rules`

	if environment != "" {
		countQuery += ` WHERE environment = ?`
		listQuery += ` WHERE environment = ?`
		args = append(args, environment)
	}

	err := s.readDB.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	listQuery += ` ORDER BY updated_at DESC LIMIT ? OFFSET ?`
	listArgs := append(args, limit, offset)

	rows, err := s.readDB.QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var rules []*engine.Rule
	for rows.Next() {
		r, err := scanRuleRows(rows)
		if err != nil {
			return nil, 0, err
		}
		rules = append(rules, r)
	}
	if rules == nil {
		rules = []*engine.Rule{}
	}
	return rules, total, rows.Err()
}

// UpdateRule updates a rule and creates a new version atomically.
func (s *Store) UpdateRule(ctx context.Context, r *engine.Rule, modifiedBy string) error {
	tx, err := s.writeDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get current version
	var currentVersion int
	err = tx.QueryRowContext(ctx, `SELECT version FROM rules WHERE id = ?`, r.ID).Scan(&currentVersion)
	if err != nil {
		return fmt.Errorf("rule not found: %w", err)
	}

	newVersion := currentVersion + 1
	now := time.Now().UTC()

	if r.Environment == "" {
		r.Environment = "production"
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE rules SET name=?, description=?, type=?, version=?, tree=?, default_value=?, status=?, environment=?, active_from=?, active_until=?, updated_at=?
		 WHERE id=?`,
		r.Name, r.Description, r.Type, newVersion,
		string(r.Tree), nullableString(r.DefaultValue),
		r.Status, r.Environment, nullableTime(r.ActiveFrom), nullableTime(r.ActiveUntil),
		now, r.ID,
	)
	if err != nil {
		return fmt.Errorf("update rule: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO rule_versions (rule_id, version, name, description, type, tree, default_value, status, environment, active_from, active_until, modified_by, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, newVersion, r.Name, r.Description, r.Type,
		string(r.Tree), nullableString(r.DefaultValue),
		r.Status, r.Environment, nullableTime(r.ActiveFrom), nullableTime(r.ActiveUntil),
		modifiedBy, now,
	)
	if err != nil {
		return fmt.Errorf("insert version: %w", err)
	}

	r.Version = newVersion
	r.UpdatedAt = now
	return tx.Commit()
}

// DeleteRule removes a rule and all its versions and history (cascade).
func (s *Store) DeleteRule(ctx context.Context, id string) error {
	result, err := s.writeDB.ExecContext(ctx, `DELETE FROM rules WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	// Clean up prune counter
	s.pruneCounters.Delete(id)
	return nil
}

// --- Versions ---

// ListVersions returns version summaries for a rule.
func (s *Store) ListVersions(ctx context.Context, ruleID string) ([]engine.RuleVersionSummary, error) {
	rows, err := s.readDB.QueryContext(ctx,
		`SELECT version, name, status, modified_by, created_at FROM rule_versions
		 WHERE rule_id = ? ORDER BY version DESC`, ruleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []engine.RuleVersionSummary
	for rows.Next() {
		var v engine.RuleVersionSummary
		if err := rows.Scan(&v.Version, &v.Name, &v.Status, &v.ModifiedBy, &v.CreatedAt); err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	if versions == nil {
		versions = []engine.RuleVersionSummary{}
	}
	return versions, rows.Err()
}

// GetVersion returns a full version snapshot.
func (s *Store) GetVersion(ctx context.Context, ruleID string, version int) (*engine.RuleVersion, error) {
	row := s.readDB.QueryRowContext(ctx,
		`SELECT id, rule_id, version, name, description, type, tree, default_value, status, environment, active_from, active_until, modified_by, created_at
		 FROM rule_versions WHERE rule_id = ? AND version = ?`, ruleID, version)

	var v engine.RuleVersion
	var treeStr string
	var defVal, modBy sql.NullString
	var activeFrom, activeUntil sql.NullTime
	err := row.Scan(&v.ID, &v.RuleID, &v.Version, &v.Name, &v.Description,
		&v.Type, &treeStr, &defVal, &v.Status, &v.Environment,
		&activeFrom, &activeUntil, &modBy, &v.CreatedAt)
	if err != nil {
		return nil, err
	}
	v.Tree = json.RawMessage(treeStr)
	if defVal.Valid {
		v.DefaultValue = json.RawMessage(defVal.String)
	}
	if activeFrom.Valid {
		v.ActiveFrom = &activeFrom.Time
	}
	if activeUntil.Valid {
		v.ActiveUntil = &activeUntil.Time
	}
	if modBy.Valid {
		v.ModifiedBy = modBy.String
	}
	return &v, nil
}

// RollbackToVersion copies a version snapshot into the rules table as a new version.
func (s *Store) RollbackToVersion(ctx context.Context, ruleID string, targetVersion int, modifiedBy string) (*engine.Rule, error) {
	tx, err := s.writeDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Get the target version snapshot
	var name, description, ruleType, tree, status, environment string
	var defVal sql.NullString
	var activeFrom, activeUntil sql.NullTime
	err = tx.QueryRowContext(ctx,
		`SELECT name, description, type, tree, default_value, status, environment, active_from, active_until
		 FROM rule_versions WHERE rule_id = ? AND version = ?`,
		ruleID, targetVersion).Scan(&name, &description, &ruleType, &tree, &defVal, &status, &environment, &activeFrom, &activeUntil)
	if err != nil {
		return nil, fmt.Errorf("version %d not found: %w", targetVersion, err)
	}

	// Get current version number
	var currentVersion int
	err = tx.QueryRowContext(ctx, `SELECT version FROM rules WHERE id = ?`, ruleID).Scan(&currentVersion)
	if err != nil {
		return nil, err
	}

	newVersion := currentVersion + 1
	now := time.Now().UTC()

	// Update rules table
	_, err = tx.ExecContext(ctx,
		`UPDATE rules SET name=?, description=?, type=?, version=?, tree=?, default_value=?, status=?, environment=?, active_from=?, active_until=?, updated_at=?
		 WHERE id=?`,
		name, description, ruleType, newVersion, tree, defVal, status, environment, activeFrom, activeUntil, now, ruleID,
	)
	if err != nil {
		return nil, err
	}

	// Insert new version entry
	_, err = tx.ExecContext(ctx,
		`INSERT INTO rule_versions (rule_id, version, name, description, type, tree, default_value, status, environment, active_from, active_until, modified_by, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ruleID, newVersion, name, description, ruleType, tree, defVal, status, environment, activeFrom, activeUntil, modifiedBy, now,
	)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	var defaultValue json.RawMessage
	if defVal.Valid {
		defaultValue = json.RawMessage(defVal.String)
	}

	rule := &engine.Rule{
		ID:           ruleID,
		Name:         name,
		Description:  description,
		Type:         ruleType,
		Version:      newVersion,
		Tree:         json.RawMessage(tree),
		DefaultValue: defaultValue,
		Status:       status,
		Environment:  environment,
		UpdatedAt:    now,
	}
	if activeFrom.Valid {
		rule.ActiveFrom = &activeFrom.Time
	}
	if activeUntil.Valid {
		rule.ActiveUntil = &activeUntil.Time
	}
	return rule, nil
}

// DuplicateRule creates a copy of a rule with a new ID.
func (s *Store) DuplicateRule(ctx context.Context, sourceID, newID, modifiedBy string) (*engine.Rule, error) {
	tx, err := s.writeDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Read source rule
	var name, description, ruleType, tree, status, environment string
	var defVal sql.NullString
	var activeFrom, activeUntil sql.NullTime
	err = tx.QueryRowContext(ctx,
		`SELECT name, description, type, tree, default_value, status, environment, active_from, active_until FROM rules WHERE id = ?`,
		sourceID).Scan(&name, &description, &ruleType, &tree, &defVal, &status, &environment, &activeFrom, &activeUntil)
	if err != nil {
		return nil, fmt.Errorf("source rule not found: %w", err)
	}

	now := time.Now().UTC()
	newName := name + "-copy"

	_, err = tx.ExecContext(ctx,
		`INSERT INTO rules (id, name, description, type, version, tree, default_value, status, environment, active_from, active_until, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?)`,
		newID, newName, description, ruleType, tree, defVal, status, environment, activeFrom, activeUntil, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert duplicate: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO rule_versions (rule_id, version, name, description, type, tree, default_value, status, environment, active_from, active_until, modified_by, created_at)
		 VALUES (?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		newID, newName, description, ruleType, tree, defVal, status, environment, activeFrom, activeUntil, modifiedBy, now,
	)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	var defaultValue json.RawMessage
	if defVal.Valid {
		defaultValue = json.RawMessage(defVal.String)
	}

	rule := &engine.Rule{
		ID:          newID,
		Name:        newName,
		Description: description,
		Type:        ruleType,
		Version:     1,
		Tree:        json.RawMessage(tree),
		DefaultValue: defaultValue,
		Status:      status,
		Environment: environment,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if activeFrom.Valid {
		rule.ActiveFrom = &activeFrom.Time
	}
	if activeUntil.Valid {
		rule.ActiveUntil = &activeUntil.Time
	}
	return rule, nil
}

// --- Eval History ---

// InsertEvalHistory records an evaluation. Prunes to 1000 entries per rule
// every 100th insert.
func (s *Store) InsertEvalHistory(ctx context.Context, ruleID string, evalCtx json.RawMessage, result json.RawMessage) error {
	_, err := s.writeDB.ExecContext(ctx,
		`INSERT INTO eval_history (rule_id, context, result) VALUES (?, ?, ?)`,
		ruleID, string(evalCtx), string(result),
	)
	if err != nil {
		return err
	}

	// Prune check
	counterVal, _ := s.pruneCounters.LoadOrStore(ruleID, &atomic.Int64{})
	counter := counterVal.(*atomic.Int64)
	count := counter.Add(1)
	if count%100 == 0 {
		s.pruneEvalHistory(ctx, ruleID)
	}

	return nil
}

func (s *Store) pruneEvalHistory(ctx context.Context, ruleID string) {
	s.writeDB.ExecContext(ctx,
		`DELETE FROM eval_history WHERE rule_id = ? AND id NOT IN (
			SELECT id FROM eval_history WHERE rule_id = ? ORDER BY created_at DESC LIMIT 1000
		)`, ruleID, ruleID)
}

// ListEvalHistory returns paginated evaluation history for a rule.
func (s *Store) ListEvalHistory(ctx context.Context, ruleID string, limit, offset int) ([]engine.EvalHistoryEntry, int, error) {
	var total int
	err := s.readDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM eval_history WHERE rule_id = ?`, ruleID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.readDB.QueryContext(ctx,
		`SELECT id, rule_id, context, result, created_at FROM eval_history
		 WHERE rule_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		ruleID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entries []engine.EvalHistoryEntry
	for rows.Next() {
		var e engine.EvalHistoryEntry
		var ctxStr, resultStr string
		if err := rows.Scan(&e.ID, &e.RuleID, &ctxStr, &resultStr, &e.CreatedAt); err != nil {
			return nil, 0, err
		}
		e.Context = json.RawMessage(ctxStr)
		e.Result = json.RawMessage(resultStr)
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []engine.EvalHistoryEntry{}
	}
	return entries, total, rows.Err()
}

// --- Meta ---

// GetMeta returns a metadata value by key.
func (s *Store) GetMeta(ctx context.Context, key string) (string, error) {
	var val string
	err := s.readDB.QueryRowContext(ctx, `SELECT value FROM _meta WHERE key = ?`, key).Scan(&val)
	return val, err
}

// SetMeta sets a metadata key-value pair (upsert).
func (s *Store) SetMeta(ctx context.Context, key, value string) error {
	_, err := s.writeDB.ExecContext(ctx,
		`INSERT INTO _meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?`,
		key, value, value)
	return err
}

// RuleExists checks if a rule with the given ID exists.
func (s *Store) RuleExists(ctx context.Context, id string) (bool, error) {
	var count int
	err := s.readDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM rules WHERE id = ?`, id).Scan(&count)
	return count > 0, err
}

// --- Import Support ---

// ImportRule creates or force-replaces a rule from imported JSON.
func (s *Store) ImportRule(ctx context.Context, r *engine.Rule, force bool, modifiedBy string) error {
	exists, err := s.RuleExists(ctx, r.ID)
	if err != nil {
		return err
	}

	if exists && !force {
		return fmt.Errorf("conflict: rule %s already exists", r.ID)
	}

	if exists && force {
		return s.UpdateRule(ctx, r, modifiedBy)
	}

	return s.CreateRule(ctx, r, modifiedBy)
}

// --- Helpers ---

func scanRule(row *sql.Row) (*engine.Rule, error) {
	var r engine.Rule
	var treeStr string
	var defVal sql.NullString
	var activeFrom, activeUntil sql.NullTime
	err := row.Scan(&r.ID, &r.Name, &r.Description, &r.Type, &r.Version,
		&treeStr, &defVal, &r.Status, &r.Environment,
		&activeFrom, &activeUntil, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	r.Tree = json.RawMessage(treeStr)
	if defVal.Valid {
		r.DefaultValue = json.RawMessage(defVal.String)
	}
	if activeFrom.Valid {
		r.ActiveFrom = &activeFrom.Time
	}
	if activeUntil.Valid {
		r.ActiveUntil = &activeUntil.Time
	}
	return &r, nil
}

func scanRuleRows(rows *sql.Rows) (*engine.Rule, error) {
	var r engine.Rule
	var treeStr string
	var defVal sql.NullString
	var activeFrom, activeUntil sql.NullTime
	err := rows.Scan(&r.ID, &r.Name, &r.Description, &r.Type, &r.Version,
		&treeStr, &defVal, &r.Status, &r.Environment,
		&activeFrom, &activeUntil, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	r.Tree = json.RawMessage(treeStr)
	if defVal.Valid {
		r.DefaultValue = json.RawMessage(defVal.String)
	}
	if activeFrom.Valid {
		r.ActiveFrom = &activeFrom.Time
	}
	if activeUntil.Valid {
		r.ActiveUntil = &activeUntil.Time
	}
	return &r, nil
}

func nullableString(data json.RawMessage) sql.NullString {
	if len(data) == 0 || string(data) == "null" {
		return sql.NullString{}
	}
	return sql.NullString{String: string(data), Valid: true}
}

func nullableTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}
