#!/bin/bash
# Agent Teams 模拟测试脚本
# 用法: ./simulate_team.sh          # 运行模拟
#       ./simulate_team.sh --cleanup # 清理测试数据

TEAMS_DIR="$HOME/.claude/teams/demo-team"
TASKS_DIR="$HOME/.claude/tasks/demo-team"

START_TIME=$(date +%s)

# 打印带时间偏移的日志
log() {
    local now=$(date +%s)
    local elapsed=$((now - START_TIME))
    local min=$((elapsed / 60))
    local sec=$((elapsed % 60))
    printf "[%02d:%02d] %s\n" "$min" "$sec" "$1"
}

# 生成 ISO 时间戳
timestamp() {
    date -u +"%Y-%m-%dT%H:%M:%SZ"
}

# 向 inbox 追加一条消息
append_message() {
    local recipient="$1"
    local from="$2"
    local text="$3"
    local summary="$4"
    local inbox_file="$TEAMS_DIR/inboxes/${recipient}.json"
    local ts
    ts=$(timestamp)

    local new_msg
    new_msg=$(cat <<EOFMSG
{"from":"${from}","text":"${text}","summary":"${summary}","timestamp":"${ts}","read":false}
EOFMSG
)

    # 读取现有数组，追加新消息，写回文件
    local existing
    existing=$(cat "$inbox_file")
    if [ "$existing" = "[]" ]; then
        echo "[${new_msg}]" > "$inbox_file"
    else
        # 移除末尾的 ]，追加新消息
        echo "${existing%]},${new_msg}]" > "$inbox_file"
    fi
}

# 更新任务状态
update_task() {
    local task_id="$1"
    local status="$2"
    local owner="$3"
    local task_file="$TASKS_DIR/${task_id}.json"

    # 使用 python3 就地更新 JSON（保证格式正确）
    python3 -c "
import json, sys
with open('${task_file}', 'r') as f:
    task = json.load(f)
task['status'] = '${status}'
if '${owner}':
    task['owner'] = '${owner}'
with open('${task_file}', 'w') as f:
    json.dump(task, f, ensure_ascii=False, indent=2)
"
}

# --cleanup: 清理测试数据
if [ "$1" = "--cleanup" ]; then
    echo "Cleaning up demo-team data..."
    rm -rf "$TEAMS_DIR"
    rm -rf "$TASKS_DIR"
    echo "Done."
    exit 0
fi

# ============================================================
# 1. 初始化
# ============================================================

log "🚀 初始化 demo-team..."

mkdir -p "$TEAMS_DIR/inboxes"
mkdir -p "$TASKS_DIR"

# 创建 config.json
cat > "$TEAMS_DIR/config.json" <<'EOFCONFIG'
{
  "name": "demo-team",
  "description": "模拟测试团队",
  "leadAgentId": "team-lead@demo-team",
  "leadSessionId": "sim-00000000-0000-0000-0000-000000000000",
  "members": [
    {
      "agentId": "team-lead@demo-team",
      "name": "team-lead",
      "agentType": "team-lead",
      "model": "claude-opus-4-6[1m]",
      "joinedAt": 1774863762841,
      "tmuxPaneId": "",
      "cwd": "/home/peiqi/tmp/Agent_Monitoring",
      "subscriptions": []
    },
    {
      "agentId": "backend-dev@demo-team",
      "name": "backend-dev",
      "agentType": "general-purpose",
      "model": "claude-sonnet-4-6[1m]",
      "joinedAt": 1774863762842,
      "tmuxPaneId": "",
      "cwd": "/home/peiqi/tmp/Agent_Monitoring",
      "subscriptions": []
    },
    {
      "agentId": "frontend-dev@demo-team",
      "name": "frontend-dev",
      "agentType": "general-purpose",
      "model": "claude-sonnet-4-6[1m]",
      "joinedAt": 1774863762843,
      "tmuxPaneId": "",
      "cwd": "/home/peiqi/tmp/Agent_Monitoring",
      "subscriptions": []
    },
    {
      "agentId": "tester@demo-team",
      "name": "tester",
      "agentType": "general-purpose",
      "model": "claude-haiku-4-5[1m]",
      "joinedAt": 1774863762844,
      "tmuxPaneId": "",
      "cwd": "/home/peiqi/tmp/Agent_Monitoring",
      "subscriptions": []
    }
  ]
}
EOFCONFIG

log "👥 创建 4 个成员: team-lead, backend-dev, frontend-dev, tester"

# 创建空 inbox 文件
for member in team-lead backend-dev frontend-dev tester; do
    echo "[]" > "$TEAMS_DIR/inboxes/${member}.json"
done

log "📬 创建 4 个 inbox 文件"

# 创建 5 个任务文件
cat > "$TASKS_DIR/1.json" <<'EOF'
{
  "id": "1",
  "title": "项目初始化",
  "description": "创建项目基础代码结构，初始化开发环境",
  "status": "pending",
  "owner": "",
  "dependencies": []
}
EOF

cat > "$TASKS_DIR/2.json" <<'EOF'
{
  "id": "2",
  "title": "后端 API 开发",
  "description": "开发 REST API 接口，包括路由、处理器和数据层",
  "status": "pending",
  "owner": "",
  "dependencies": [1]
}
EOF

cat > "$TASKS_DIR/3.json" <<'EOF'
{
  "id": "3",
  "title": "前端页面开发",
  "description": "开发 Dashboard 前端页面，包括四个核心面板",
  "status": "pending",
  "owner": "",
  "dependencies": [1]
}
EOF

cat > "$TASKS_DIR/4.json" <<'EOF'
{
  "id": "4",
  "title": "集成测试",
  "description": "前后端集成测试，验证所有 API 端点和页面功能",
  "status": "pending",
  "owner": "",
  "dependencies": [2, 3]
}
EOF

cat > "$TASKS_DIR/5.json" <<'EOF'
{
  "id": "5",
  "title": "部署上线",
  "description": "构建生产版本并部署到服务器",
  "status": "pending",
  "owner": "",
  "dependencies": [4]
}
EOF

log "📋 创建 5 个任务"

# ============================================================
# 2. 模拟活动
# ============================================================

# T+0s: Lead 分配任务
log "✉️  team-lead → backend-dev: 请开始项目初始化，创建基础代码结构"
append_message "backend-dev" "team-lead" "请开始项目初始化，创建基础代码结构" "分配任务1：项目初始化"
update_task "1" "in_progress" "backend-dev"
log "📋 任务 #1 → in_progress (backend-dev)"

sleep 5

# T+5s: backend-dev 接受并回复
log "✉️  backend-dev → team-lead: 收到，开始执行"
append_message "team-lead" "backend-dev" "收到，开始执行项目初始化" "确认接收任务1"
log "✉️  backend-dev → frontend-dev: 初始化完成后会通知你"
append_message "frontend-dev" "backend-dev" "初始化完成后会通知你，预计5分钟" "通知前端等待"

sleep 5

# T+10s: 任务1完成，分配任务2和3
log "📋 任务 #1 → completed"
update_task "1" "completed" "backend-dev"

sleep 1

log "📋 任务 #2 → in_progress (backend-dev)"
update_task "2" "in_progress" "backend-dev"
log "📋 任务 #3 → in_progress (frontend-dev)"
update_task "3" "in_progress" "frontend-dev"

log "✉️  team-lead → backend-dev: 任务1已完成，请开始后端API开发"
append_message "backend-dev" "team-lead" "任务1已完成，请开始后端API开发。需要实现5个REST接口" "分配任务2：后端API开发"
log "✉️  team-lead → frontend-dev: 项目已初始化，请开始前端页面开发"
append_message "frontend-dev" "team-lead" "项目已初始化，请开始前端页面开发。需要实现四个核心面板" "分配任务3：前端页面开发"

sleep 5

# T+15s: 开发中的交流
log "✉️  frontend-dev → backend-dev: API 接口文档在哪里？"
append_message "backend-dev" "frontend-dev" "API 接口文档在哪里？我需要知道数据格式来对接前端" "询问API文档位置"
sleep 2
log "✉️  backend-dev → frontend-dev: 文档在 docs/api.md，主要有5个接口"
append_message "frontend-dev" "backend-dev" "文档在 docs/api.md，主要有5个接口：teams列表、团队详情、消息、任务、token统计" "回复API文档位置"

sleep 3

# T+20s: Lead 广播
log "✉️  team-lead → * (广播): 进度更新：后端完成60%，前端完成40%"
for member in team-lead backend-dev frontend-dev tester; do
    append_message "$member" "team-lead" "进度更新：后端完成60%，前端完成40%。整体进度符合预期，继续保持" "团队进度广播"
done

sleep 5

# T+25s: 任务2完成
log "📋 任务 #2 → completed"
update_task "2" "completed" "backend-dev"
log "✉️  backend-dev → team-lead: 后端API开发完成，所有接口已通过单元测试"
append_message "team-lead" "backend-dev" "后端API开发完成，所有5个接口已通过单元测试。覆盖率92%" "任务2完成报告"

sleep 5

# T+30s: 任务3完成
log "📋 任务 #3 → completed"
update_task "3" "completed" "frontend-dev"
log "✉️  frontend-dev → team-lead: 前端页面开发完成，四个面板全部就绪"
append_message "team-lead" "frontend-dev" "前端页面开发完成，四个面板全部就绪：成员状态、任务看板、通信流、Token统计" "任务3完成报告"

sleep 5

# T+35s: 开始集成测试
log "📋 任务 #4 → in_progress (tester)"
update_task "4" "in_progress" "tester"
log "✉️  team-lead → tester: 请开始集成测试"
append_message "tester" "team-lead" "前后端开发已完成，请开始集成测试。重点验证WebSocket实时推送和API数据一致性" "分配任务4：集成测试"

sleep 5

# T+40s: 测试完成
log "📋 任务 #4 → completed"
update_task "4" "completed" "tester"
log "✉️  tester → team-lead: 集成测试全部通过，6个API端点验证OK"
append_message "team-lead" "tester" "集成测试全部通过！6个API端点验证OK，WebSocket推送延迟<200ms，无内存泄漏" "任务4完成报告"

sleep 5

# T+45s: 部署
log "📋 任务 #5 → in_progress (backend-dev)"
update_task "5" "in_progress" "backend-dev"
log "✉️  team-lead → backend-dev: 测试通过，请开始部署上线"
append_message "backend-dev" "team-lead" "测试全部通过，请开始构建生产版本并部署到服务器" "分配任务5：部署上线"

sleep 3

log "📋 任务 #5 → completed"
update_task "5" "completed" "backend-dev"
log "✉️  backend-dev → team-lead: 部署完成，服务已上线"
append_message "team-lead" "backend-dev" "部署完成！生产版本已上线，服务运行在 http://localhost:8080" "任务5完成报告"

sleep 2

# T+50s: 完成广播
log "✉️  team-lead → * (广播): 所有任务已完成，项目交付成功！"
for member in team-lead backend-dev frontend-dev tester; do
    append_message "$member" "team-lead" "所有任务已完成，项目交付成功！感谢大家的协作，总耗时约50秒完成了完整的开发-测试-部署流程" "项目完成通知"
done

log "✅ 模拟完成！"
echo ""
echo "数据目录："
echo "  团队配置: $TEAMS_DIR/config.json"
echo "  消息 inbox: $TEAMS_DIR/inboxes/"
echo "  任务文件: $TASKS_DIR/"
echo ""
echo "清理命令: $0 --cleanup"
