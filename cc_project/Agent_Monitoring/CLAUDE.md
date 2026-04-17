# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Agent Teams 实时监控面板 — 可视化展示 Claude Code Agent Teams 中每个 Agent 的运行状态、任务进度和实时通信流。

## Tech Stack

- **Backend**: Go (1.25) + Gin + gorilla/websocket + fsnotify
- **Frontend**: HTML/CSS/JS, embedded via `go:embed`
- **Deployment**: Single binary, `go build` then run

## Data Sources

监控数据全部来自 Claude Code 的本地 JSON 文件：

| 数据 | 路径 | 说明 |
|------|------|------|
| 团队成员 | `~/.claude/teams/{team}/config.json` | 成员列表、状态、模型 |
| 消息通信 | `~/.claude/teams/{team}/inboxes/{name}.json` | Agent 间的邮箱消息 |
| 任务状态 | `~/.claude/tasks/{team}/*.json` | 任务创建、分配、完成 |

## Architecture

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

## Build & Run

```bash
go build -o agent-monitor .
./agent-monitor
# Open http://localhost:8080
```

## Skills

- `agent-teams-module-delivery`: 多Agent协作模块化交付流程（Leader拆分 → 开发Agent并行 → 测试Agent验证）
- `regression-verify`: 基于《回归验证功能点清单》的全量自动化回归验证

## Verification Workflow

代码编写完成后，必须通过远程机器验证：

```bash
ssh yunxigu@192.168.2.66
# 密码: yunxigu2025
cd /Users/yunxigu/cc_project/Agent_Monitoring
```

- 验证操作在远程机器的 **docker 环境**中进行
- 日常调试**禁止使用 `--build`**，通过 volume 挂载代码到容器
- 仅在修改了 `requirements.txt`、`Dockerfile` 或首次部署时才允许重建镜像

## Language

与用户使用中文沟通。
