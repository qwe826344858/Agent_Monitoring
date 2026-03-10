#!/bin/bash
# 每10秒执行一次 sync.sh 的脚本

while true; do
    echo "=== $(date '+%Y-%m-%d %H:%M:%S') 开始同步 ==="
    bash sync.sh
    echo "=== 同步完成，等待10秒... ==="
    echo ""
    sleep 10
done

