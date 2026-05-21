// 存储层 — SQLite 持久化，审计日志和规则存储
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Samantha09/mcpguard/internal/models"
	_ "modernc.org/sqlite"
)

// Store 存储接口
type Store interface {
	// 初始化（建表等）
	Init(ctx context.Context) error
	// 关闭连接
	Close() error

	// 日志操作
	InsertLog(ctx context.Context, entry *models.LogEntry) error
	QueryLogs(ctx context.Context, filter LogFilter) ([]*models.LogEntry, error)

	// 探针操作
	RegisterProbe(ctx context.Context, probe *models.Probe, tokenHash string) error
	GetProbe(ctx context.Context, id string) (*models.Probe, error)
	ListProbes(ctx context.Context) ([]*models.Probe, error)
	UpdateProbeHeartbeat(ctx context.Context, id string) error
	UpdateProbeStatus(ctx context.Context, id string, status string) error

	// 规则操作
	CreateRule(ctx context.Context, rule *models.Rule) error
	GetRule(ctx context.Context, id string) (*models.Rule, error)
	ListRules(ctx context.Context) ([]*models.Rule, error)
	UpdateRule(ctx context.Context, rule *models.Rule) error
	DeleteRule(ctx context.Context, id string) error

	// 策略操作
	UpsertPolicy(ctx context.Context, policy *models.Policy) error
	GetPolicy(ctx context.Context, id string) (*models.Policy, error)
	ListPolicies(ctx context.Context) ([]*models.Policy, error)
	DeletePolicy(ctx context.Context, id string) error
}

// LogFilter 日志查询过滤条件
type LogFilter struct {
	StartTime *time.Time     `json:"start_time,omitempty"`
	EndTime   *time.Time     `json:"end_time,omitempty"`
	Action    *models.Action `json:"action,omitempty"`
	Method    string         `json:"method,omitempty"`
	ToolName  string         `json:"tool_name,omitempty"`
	Limit     int            `json:"limit,omitempty"`
	Offset    int            `json:"offset,omitempty"`
}

// SQLiteStore SQLite 实现
type SQLiteStore struct {
	dbPath string
	db     *sql.DB
}

// NewSQLiteStore 创建 SQLite 存储
func NewSQLiteStore(dbPath string) *SQLiteStore {
	return &SQLiteStore{dbPath: dbPath}
}

func (s *SQLiteStore) Init(ctx context.Context) error {
	db, err := sql.Open("sqlite", s.dbPath)
	if err != nil {
		return fmt.Errorf("打开 SQLite 失败: %w", err)
	}
	s.db = db

	schema := `
CREATE TABLE IF NOT EXISTS logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
	direction TEXT NOT NULL CHECK(direction IN ('request', 'response')),
	method TEXT NOT NULL,
	tool_name TEXT,
	action TEXT NOT NULL CHECK(action IN ('allow', 'block', 'warn')),
	reason TEXT,
	request TEXT NOT NULL,
	response TEXT,
	client_id TEXT,
	probe_id TEXT,
	detector TEXT,
	rule_id TEXT
);
CREATE INDEX IF NOT EXISTS idx_logs_timestamp ON logs(timestamp);
CREATE INDEX IF NOT EXISTS idx_logs_action ON logs(action);
CREATE INDEX IF NOT EXISTS idx_logs_method ON logs(method);
CREATE INDEX IF NOT EXISTS idx_logs_tool_name ON logs(tool_name);
CREATE INDEX IF NOT EXISTS idx_logs_probe_id ON logs(probe_id);

CREATE TABLE IF NOT EXISTS probes (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	token_hash TEXT NOT NULL,
	hostname TEXT,
	ip TEXT,
	status TEXT DEFAULT 'offline',
	last_heartbeat DATETIME,
	registered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	metadata TEXT
);

CREATE TABLE IF NOT EXISTS rules (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	type TEXT NOT NULL CHECK(type IN ('keyword', 'tool_name', 'regex', 'file_path')),
	pattern TEXT NOT NULL,
	action TEXT NOT NULL CHECK(action IN ('allow', 'block', 'warn')),
	enabled BOOLEAN DEFAULT TRUE,
	description TEXT,
	version INTEGER DEFAULT 1,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

	CREATE TABLE IF NOT EXISTS policies (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		enabled BOOLEAN DEFAULT TRUE,
		rule_ids TEXT,
		llm_enabled BOOLEAN DEFAULT FALSE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("创建表失败: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *SQLiteStore) InsertLog(ctx context.Context, entry *models.LogEntry) error {
	ts := entry.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO logs (timestamp, direction, method, tool_name, action, reason, request, response, client_id, probe_id, detector, rule_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ts, entry.Direction, entry.Method, entry.ToolName, string(entry.Action), entry.Reason,
		entry.Request, entry.Response, entry.ClientID, entry.ProbeID, entry.Detector, entry.RuleID,
	)
	return err
}

func (s *SQLiteStore) QueryLogs(ctx context.Context, filter LogFilter) ([]*models.LogEntry, error) {
	var conditions []string
	var args []any

	if filter.StartTime != nil {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, *filter.StartTime)
	}
	if filter.EndTime != nil {
		conditions = append(conditions, "timestamp <= ?")
		args = append(args, *filter.EndTime)
	}
	if filter.Action != nil {
		conditions = append(conditions, "action = ?")
		args = append(args, string(*filter.Action))
	}
	if filter.Method != "" {
		conditions = append(conditions, "method = ?")
		args = append(args, filter.Method)
	}
	if filter.ToolName != "" {
		conditions = append(conditions, "tool_name = ?")
		args = append(args, filter.ToolName)
	}

	query := "SELECT id, timestamp, direction, method, tool_name, action, reason, request, response, client_id, probe_id, detector, rule_id FROM logs"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY timestamp DESC"

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	query += " LIMIT ? OFFSET ?"
	args = append(args, limit, filter.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*models.LogEntry
	for rows.Next() {
		var e models.LogEntry
		var actionStr string
		err := rows.Scan(
			&e.ID, &e.Timestamp, &e.Direction, &e.Method, &e.ToolName,
			&actionStr, &e.Reason, &e.Request, &e.Response,
			&e.ClientID, &e.ProbeID, &e.Detector, &e.RuleID,
		)
		if err != nil {
			return nil, err
		}
		e.Action = models.Action(actionStr)
		entries = append(entries, &e)
	}
	return entries, rows.Err()
}

// --- Probe CRUD ---

func (s *SQLiteStore) RegisterProbe(ctx context.Context, probe *models.Probe, tokenHash string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO probes (id, name, token_hash, hostname, ip, status, last_heartbeat, metadata)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		probe.ID, probe.Name, tokenHash, probe.Hostname, probe.IP, probe.Status, probe.LastHeartbeat, probe.Metadata,
	)
	return err
}

func (s *SQLiteStore) GetProbe(ctx context.Context, id string) (*models.Probe, error) {
	var p models.Probe
	var lastHb sql.NullTime
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, hostname, ip, status, last_heartbeat, registered_at, metadata FROM probes WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.Hostname, &p.IP, &p.Status, &lastHb, &p.RegisteredAt, &p.Metadata)
	if err != nil {
		return nil, err
	}
	if lastHb.Valid {
		p.LastHeartbeat = lastHb.Time
	}
	return &p, nil
}

func (s *SQLiteStore) ListProbes(ctx context.Context) ([]*models.Probe, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, hostname, ip, status, last_heartbeat, registered_at, metadata FROM probes ORDER BY registered_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var probes []*models.Probe
	for rows.Next() {
		var p models.Probe
		var lastHb sql.NullTime
		if err := rows.Scan(&p.ID, &p.Name, &p.Hostname, &p.IP, &p.Status, &lastHb, &p.RegisteredAt, &p.Metadata); err != nil {
			return nil, err
		}
		if lastHb.Valid {
			p.LastHeartbeat = lastHb.Time
		}
		probes = append(probes, &p)
	}
	return probes, rows.Err()
}

func (s *SQLiteStore) UpdateProbeHeartbeat(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE probes SET last_heartbeat = ? WHERE id = ?`, time.Now(), id)
	return err
}

func (s *SQLiteStore) UpdateProbeStatus(ctx context.Context, id string, status string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE probes SET status = ? WHERE id = ?`, status, id)
	return err
}

// --- Rule CRUD ---

func (s *SQLiteStore) CreateRule(ctx context.Context, rule *models.Rule) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO rules (id, name, type, pattern, action, enabled, description, version)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		rule.ID, rule.Name, rule.Type, rule.Pattern, string(rule.Action), rule.Enabled, rule.Description, rule.Version,
	)
	return err
}

func (s *SQLiteStore) GetRule(ctx context.Context, id string) (*models.Rule, error) {
	var r models.Rule
	var createdAt, updatedAt sql.NullTime
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, type, pattern, action, enabled, description, version, created_at, updated_at FROM rules WHERE id = ?`, id,
	).Scan(&r.ID, &r.Name, &r.Type, &r.Pattern, &r.Action, &r.Enabled, &r.Description, &r.Version, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	if createdAt.Valid {
		r.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		r.UpdatedAt = updatedAt.Time
	}
	return &r, nil
}

func (s *SQLiteStore) ListRules(ctx context.Context) ([]*models.Rule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, type, pattern, action, enabled, description, version, created_at, updated_at FROM rules ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*models.Rule
	for rows.Next() {
		var r models.Rule
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&r.ID, &r.Name, &r.Type, &r.Pattern, &r.Action, &r.Enabled, &r.Description, &r.Version, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		if createdAt.Valid {
			r.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			r.UpdatedAt = updatedAt.Time
		}
		rules = append(rules, &r)
	}
	return rules, rows.Err()
}

func (s *SQLiteStore) UpdateRule(ctx context.Context, rule *models.Rule) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE rules SET name = ?, type = ?, pattern = ?, action = ?, enabled = ?, description = ?, version = ?, updated_at = ? WHERE id = ?`,
		rule.Name, rule.Type, rule.Pattern, string(rule.Action), rule.Enabled, rule.Description, rule.Version, time.Now(), rule.ID,
	)
	return err
}

func (s *SQLiteStore) DeleteRule(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM rules WHERE id = ?`, id)
	return err
}

func (s *SQLiteStore) UpsertPolicy(ctx context.Context, policy *models.Policy) error {
	ruleIDs, _ := json.Marshal(policy.RuleIDs)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO policies (id, name, description, enabled, rule_ids, llm_enabled, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			 name=excluded.name, description=excluded.description, enabled=excluded.enabled,
			 rule_ids=excluded.rule_ids, llm_enabled=excluded.llm_enabled, updated_at=excluded.updated_at`,
		policy.ID, policy.Name, policy.Description, policy.Enabled, string(ruleIDs), policy.LLMEnabled, time.Now(),
	)
	return err
}

func (s *SQLiteStore) GetPolicy(ctx context.Context, id string) (*models.Policy, error) {
	var p models.Policy
	var ruleIDsRaw string
	var createdAt, updatedAt sql.NullTime
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, description, enabled, rule_ids, llm_enabled, created_at, updated_at FROM policies WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.Enabled, &ruleIDsRaw, &p.LLMEnabled, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(ruleIDsRaw), &p.RuleIDs); err != nil {
		return nil, fmt.Errorf("解析 rule_ids 失败: %w", err)
	}
	if createdAt.Valid {
		p.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		p.UpdatedAt = updatedAt.Time
	}
	return &p, nil
}

func (s *SQLiteStore) ListPolicies(ctx context.Context) ([]*models.Policy, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, description, enabled, rule_ids, llm_enabled, created_at, updated_at FROM policies ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []*models.Policy
	for rows.Next() {
		var p models.Policy
		var ruleIDsRaw string
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Enabled, &ruleIDsRaw, &p.LLMEnabled, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(ruleIDsRaw), &p.RuleIDs); err != nil {
			return nil, fmt.Errorf("解析 rule_ids 失败: %w", err)
		}
		if createdAt.Valid {
			p.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			p.UpdatedAt = updatedAt.Time
		}
		policies = append(policies, &p)
	}
	return policies, rows.Err()
}

func (s *SQLiteStore) DeletePolicy(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM policies WHERE id = ?`, id)
	return err
}
