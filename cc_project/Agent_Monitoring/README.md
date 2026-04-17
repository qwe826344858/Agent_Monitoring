# Agent_Monitoring

Agent Teams 实时监控面板 — 可视化展示 Claude Code Agent Teams 中每个 Agent 的运行状态、任务进度和实时通信流。

## 技术栈

- **Backend**: Go (1.25) + Gin + gorilla/websocket + fsnotify
- **Frontend**: HTML/CSS/JS, embedded via `go:embed`
- **Deployment**: Single binary, `go build` then run

## 数据来源

监控数据全部来自 Claude Code 的本地 JSON 文件：

| 数据 | 路径 | 说明 |
|------|------|------|
| 团队成员 | `~/.claude/teams/{team}/config.json` | 成员列表、状态、模型 |
| 消息通信 | `~/.claude/teams/{team}/inboxes/{name}.json` | Agent 间的邮箱消息 |
| 任务状态 | `~/.claude/tasks/{team}/*.json` | 任务创建、分配、完成 |

## 快速开始

```bash
go build -o agent-monitor .
./agent-monitor
# Open http://localhost:8080
```

## 架构

```
server (Go)
├── fsnotify watches ~/.claude/teams/ and ~/.claude/tasks/
├── Gin serves REST API (/api/*) + static frontend
└── gorilla/websocket pushes file change events to frontend

frontend (embedded static files)
├── Team selector (scan all teams, show sessionId/cwd for identification)
├── Member status cards (active/idle/shutdown)
├── Task kanban (unassigned → in-progress → completed)
└── Real-time message timeline (from inbox diffs)
```
