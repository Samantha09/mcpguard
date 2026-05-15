// 存储层 — SQLite 持久化，审计日志和规则存储
package store

import (
	"context"
	"time"

	"github.com/Samantha09/mcpguard/internal/models"
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
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	Action    *models.Action `json:"action,omitempty"`
	Method    string     `json:"method,omitempty"`
	ToolName  string     `json:"tool_name,omitempty"`
	Limit     int        `json:"limit,omitempty"`
	Offset    int        `json:"offset,omitempty"`
}

// SQLiteStore SQLite 实现
type SQLiteStore struct {
	dbPath string
}

// NewSQLiteStore 创建 SQLite 存储
func NewSQLiteStore(dbPath string) *SQLiteStore {
	return &SQLiteStore{dbPath: dbPath}
}

func (s *SQLiteStore) Init(ctx context.Context) error  { return nil }
func (s *SQLiteStore) Close() error                     { return nil }
func (s *SQLiteStore) InsertLog(ctx context.Context, entry *models.LogEntry) error { return nil }
func (s *SQLiteStore) QueryLogs(ctx context.Context, filter LogFilter) ([]*models.LogEntry, error) {
	return nil, nil
}
func (s *SQLiteStore) UpsertPolicy(ctx context.Context, policy *models.Policy) error { return nil }
func (s *SQLiteStore) GetPolicy(ctx context.Context, id string) (*models.Policy, error) {
	return nil, nil
}
func (s *SQLiteStore) ListPolicies(ctx context.Context) ([]*models.Policy, error) {
	return nil, nil
}
func (s *SQLiteStore) DeletePolicy(ctx context.Context, id string) error { return nil }
