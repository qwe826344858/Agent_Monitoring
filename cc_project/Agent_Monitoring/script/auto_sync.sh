#!/bin/bash
# 每10秒执行一次 sync.sh 的脚本

while true; do
    echo "=== $(date '+%Y-%m-%d %H:%M:%S') 开始同步 ==="
    rsync -zar -e "ssh -p 22 " --exclude=.svn --exclude=.cvs --exclude=.idea --exclude=.DS_Store --exclude=.git --exclude=.hg --exclude=*.hprof --exclude=*.pyc peiqi@192.168.2.20:/home/peiqi/tmp/Agent_Monitoring/ .
    echo "=== 同步完成，等待7秒... ==="
    echo ""
    sleep 7
done

#rsync -zar -e "ssh -p 22 " --exclude=.svn --exclude=.cvs --exclude=.idea --exclude=.DS_Store --exclude=.git --exclude=.hg --exclude=*.hprof --exclude=*.pyc zhaoting@192.168.2.20:/home/zhaoting/go_project/kcp/ .
