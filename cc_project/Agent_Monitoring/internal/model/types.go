package model

import "time"

// TeamConfig 对应 ~/.claude/teams/{team}/config.json 的完整结构
type TeamConfig struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	LeadAgentID   string   `json:"leadAgentId"`
	LeadSessionID string   `json:"leadSessionId"`
	Members       []Member `json:"members"`
}

// Member 团队中单个 Agent 的信息
type Member struct {
	AgentID       string   `json:"agentId"`
	Name          string   `json:"name"`
	AgentType     string   `json:"agentType"`     // team-lead, teammate 等
	Model         string   `json:"model"`          // claude-opus-4-6[1m], claude-sonnet-4-6 等
	JoinedAt      int64    `json:"joinedAt"`       // Unix 毫秒时间戳
	TmuxPaneID    string   `json:"tmuxPaneId"`     // 非空表示 tmux 模式运行
	Cwd           string   `json:"cwd"`            // 工作目录
	Subscriptions []string `json:"subscriptions"`  // 订阅的消息频道
}

// InboxMessage 收件箱中的单条消息
type InboxMessage struct {
	From      string `json:"from"`
	Text      string `json:"text"`
	Summary   string `json:"summary"`
	Timestamp string `json:"timestamp"` // ISO 8601 格式
	Read      bool   `json:"read"`
}

// Task 任务文件对应的结构
// Claude Code 的 TaskCreate 用 subject 作为标题，blocks/blockedBy 作为依赖
type Task struct {
	ID          string   `json:"id"`
	Subject     string   `json:"subject"`     // Claude Code 实际使用的标题字段
	Title       string   `json:"title"`       // 兼容旧格式
	Description string   `json:"description"`
	ActiveForm  string   `json:"activeForm"`  // 进行中时的展示文本
	Status      string   `json:"status"`      // pending, in_progress, completed
	Owner       string   `json:"owner"`       // 负责人 agent 名称
	Blocks      []string `json:"blocks"`      // 该任务阻塞的任务 ID
	BlockedBy   []string `json:"blockedBy"`   // 阻塞该任务的任务 ID
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}

// DisplayTitle 返回任务的展示标题，优先 subject，其次 title，最后截取 description
func (t Task) DisplayTitle() string {
	if t.Subject != "" {
		return t.Subject
	}
	if t.Title != "" {
		return t.Title
	}
	if t.Description != "" {
		if len(t.Description) > 60 {
			return t.Description[:60] + "..."
		}
		return t.Description
	}
	return "未命名任务"
}

// TokenUsage 单条 JSONL 记录中的 usage 字段
type TokenUsage struct {
	InputTokens                int64 `json:"input_tokens"`
	OutputTokens               int64 `json:"output_tokens"`
	CacheCreationInputTokens   int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens       int64 `json:"cache_read_input_tokens"`
}

// WSEvent WebSocket 推送给前端的事件结构
type WSEvent struct {
	Event     string      `json:"event"`     // member_update, new_message, task_update, token_update
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// MemberTokenUsage 单个成员的 token 消耗统计
type MemberTokenUsage struct {
	Name                       string  `json:"name"`
	Model                      string  `json:"model"`
	InputTokens                int64   `json:"input_tokens"`
	OutputTokens               int64   `json:"output_tokens"`
	CacheCreationInputTokens   int64   `json:"cache_creation_input_tokens"`
	CacheReadInputTokens       int64   `json:"cache_read_input_tokens"`
	EstimatedCostUSD           float64 `json:"estimated_cost_usd"`
	JSONLPath                  string  `json:"jsonl_path"`
}

// TokenReport 团队 token 统计报告
type TokenReport struct {
	TeamTotal TokenUsageSummary  `json:"team_total"`
	Members   []MemberTokenUsage `json:"members"`
}

// TokenUsageSummary 团队总计的 token 消耗汇总
type TokenUsageSummary struct {
	InputTokens                int64   `json:"input_tokens"`
	OutputTokens               int64   `json:"output_tokens"`
	CacheCreationInputTokens   int64   `json:"cache_creation_input_tokens"`
	CacheReadInputTokens       int64   `json:"cache_read_input_tokens"`
	EstimatedCostUSD           float64 `json:"estimated_cost_usd"`
}
