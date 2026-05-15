package rules

import "github.com/Samantha09/mcpguard/internal/models"

// BuiltInRules 返回 MVP 内置的 10 条默认安全规则
func BuiltInRules() []Rule {
	return []Rule{
		{ID: "builtin-001", Name: "禁止递归删除", Type: RuleTypeKeyword, Pattern: "rm -rf", Action: models.ActionBlock, Enabled: true},
		{ID: "builtin-002", Name: "禁止强制删除根目录", Type: RuleTypeKeyword, Pattern: "rm -rf /", Action: models.ActionBlock, Enabled: true},
		{ID: "builtin-003", Name: "禁止格式化磁盘", Type: RuleTypeKeyword, Pattern: "mkfs", Action: models.ActionBlock, Enabled: true},
		{ID: "builtin-004", Name: "禁止覆盖磁盘", Type: RuleTypeKeyword, Pattern: "dd if=/dev/zero", Action: models.ActionBlock, Enabled: true},
		{ID: "builtin-005", Name: "禁止访问 SSH 密钥", Type: RuleTypeKeyword, Pattern: "~/.ssh", Action: models.ActionBlock, Enabled: true},
		{ID: "builtin-006", Name: "禁止访问密码文件", Type: RuleTypeKeyword, Pattern: "/etc/passwd", Action: models.ActionBlock, Enabled: true},
		{ID: "builtin-007", Name: "禁止 sudo 提权", Type: RuleTypeKeyword, Pattern: "sudo", Action: models.ActionBlock, Enabled: true},
		{ID: "builtin-008", Name: "禁止 su 切换用户", Type: RuleTypeKeyword, Pattern: "su -", Action: models.ActionBlock, Enabled: true},
		{ID: "builtin-009", Name: "禁止执行命令工具", Type: RuleTypeToolName, Pattern: "execute_command", Action: models.ActionWarn, Enabled: true},
		{ID: "builtin-010", Name: "禁止写文件工具", Type: RuleTypeToolName, Pattern: "write_file", Action: models.ActionWarn, Enabled: true},
	}
}
