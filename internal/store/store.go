// 存储层 — SQLite 持久化，审计日志和规则存储
package store

import (
	"context"
	"database/sql"
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
	detector TEXT,
	rule_id TEXT
);
CREATE INDEX IF NOT EXISTS idx_logs_timestamp ON logs(timestamp);
CREATE INDEX IF NOT EXISTS idx_logs_action ON logs(action);
CREATE INDEX IF NOT EXISTS idx_logs_method ON logs(method);
CREATE INDEX IF NOT EXISTS idx_logs_tool_name ON logs(tool_name);
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
		`INSERT INTO logs (timestamp, direction, method, tool_name, action, reason, request, response, client_id, detector, rule_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ts, entry.Direction, entry.Method, entry.ToolName, string(entry.Action), entry.Reason,
		entry.Request, entry.Response, entry.ClientID, entry.Detector, entry.RuleID,
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

	query := "SELECT id, timestamp, direction, method, tool_name, action, reason, request, response, client_id, detector, rule_id FROM logs"
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
			&e.ClientID, &e.Detector, &e.RuleID,
		)
		if err != nil {
			return nil, err
		}
		e.Action = models.Action(actionStr)
		entries = append(entries, &e)
	}
	return entries, rows.Err()
}

func (s *SQLiteStore) UpsertPolicy(ctx context.Context, policy *models.Policy) error {
	return nil
}
func (s *SQLiteStore) GetPolicy(ctx context.Context, id string) (*models.Policy, error) {
	return nil, nil
}
func (s *SQLiteStore) ListPolicies(ctx context.Context) ([]*models.Policy, error) {
	return nil, nil
}
func (s *SQLiteStore) DeletePolicy(ctx context.Context, id string) error {
	return nil
}
