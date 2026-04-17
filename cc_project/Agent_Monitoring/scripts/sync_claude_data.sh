#!/bin/bash
# 将本地 ~/.claude 中的 Agent Teams 数据同步到远端机器
# 供远端 Docker 中的监控面板读取
#
# 用法: 在远端机器上运行
#   bash scripts/sync_claude_data.sh
#
# 同步内容:
#   ~/.claude/teams/     → Agent Teams 成员配置和通信消息
#   ~/.claude/tasks/     → 任务数据
#   ~/.claude/projects/  → JSONL 会话记录（token 统计）

REMOTE_USER="peiqi"
REMOTE_HOST="192.168.2.20"
REMOTE_PORT=22
LOCAL_CLAUDE_DIR="$HOME/.claude"
INTERVAL=5

echo "=== Claude Data Sync ==="
echo "从 ${REMOTE_USER}@${REMOTE_HOST} 同步 .claude 数据"
echo "同步间隔: ${INTERVAL}s"
echo ""

while true; do
    # 同步 teams 目录（成员配置 + 收件箱消息）
    rsync -zar --delete \
        -e "ssh -p ${REMOTE_PORT}" \
        "${REMOTE_USER}@${REMOTE_HOST}:/home/${REMOTE_USER}/.claude/teams/" \
        "${LOCAL_CLAUDE_DIR}/teams/" \
        2>/dev/null

    # 同步 tasks 目录（任务数据）
    rsync -zar --delete \
        -e "ssh -p ${REMOTE_PORT}" \
        "${REMOTE_USER}@${REMOTE_HOST}:/home/${REMOTE_USER}/.claude/tasks/" \
        "${LOCAL_CLAUDE_DIR}/tasks/" \
        2>/dev/null

    # 同步 projects 目录（JSONL 会话记录，用于 token 统计）
    # 只同步 .jsonl 文件，排除其他大文件
    rsync -zar \
        -e "ssh -p ${REMOTE_PORT}" \
        --include='*/' \
        --include='*.jsonl' \
        --exclude='*' \
        "${REMOTE_USER}@${REMOTE_HOST}:/home/${REMOTE_USER}/.claude/projects/" \
        "${LOCAL_CLAUDE_DIR}/projects/" \
        2>/dev/null

    sleep ${INTERVAL}
done
