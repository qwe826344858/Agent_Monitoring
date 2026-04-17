// ===== Agent Teams 监控面板 - 前端应用 =====

(function () {
  "use strict";

  // ===== 状态 =====
  var currentTeam = null;
  var ws = null;
  var currentTokenData = null; // 缓存 token 数据，供成员卡片显示
  var wsReconnectTimer = null;
  var wsReconnectCountdown = 0;
  var wsCountdownInterval = null;
  var userHasScrolled = false;

  // 头像颜色列表
  var avatarColors = ["blue", "green", "purple", "orange", "pink", "cyan", "indigo", "rose"];
  var barColors = ["bar-blue", "bar-green", "bar-purple", "bar-orange", "bar-pink", "bar-cyan", "bar-indigo", "bar-rose"];

  // ===== DOM 引用 =====
  var teamSelectPage = document.getElementById("team-select-page");
  var dashboardPage = document.getElementById("dashboard-page");
  var teamsGrid = document.getElementById("teams-grid");
  var dashboardTeamName = document.getElementById("dashboard-team-name");
  var membersGrid = document.getElementById("members-grid");
  var kanbanPending = document.getElementById("kanban-pending");
  var kanbanInProgress = document.getElementById("kanban-in-progress");
  var kanbanCompleted = document.getElementById("kanban-completed");
  var kanbanPendingCount = document.getElementById("kanban-pending-count");
  var kanbanProgressCount = document.getElementById("kanban-progress-count");
  var kanbanDoneCount = document.getElementById("kanban-done-count");
  var tokensContent = document.getElementById("tokens-content");
  var messagesTimeline = document.getElementById("messages-timeline");
  var wsStatusDot = document.getElementById("ws-status-dot");
  var wsStatusText = document.getElementById("ws-status-text");
  var btnBack = document.getElementById("btn-back");
  var btnRefresh = document.getElementById("btn-refresh");

  // ===== 工具函数 =====

  function formatTokens(n) {
    if (n == null) return "0";
    if (n >= 1000000) return (n / 1000000).toFixed(1).replace(/\.0$/, "") + "M";
    if (n >= 1000) return (n / 1000).toFixed(1).replace(/\.0$/, "") + "K";
    return String(n);
  }

  function formatCost(v) {
    if (v == null) return "$0.00";
    return "$" + Number(v).toFixed(2);
  }

  function formatTime(ts) {
    if (!ts) return "--:--:--";
    var d = new Date(ts);
    if (isNaN(d.getTime())) return ts;
    return d.toLocaleTimeString("zh-CN", { hour12: false });
  }

  function formatDate(ts) {
    if (!ts) return "";
    var d = new Date(ts);
    if (isNaN(d.getTime())) return "";
    return d.toLocaleDateString("zh-CN");
  }

  function escapeHtml(str) {
    if (typeof str !== "string") return "";
    var div = document.createElement("div");
    div.appendChild(document.createTextNode(str));
    return div.innerHTML;
  }

  function tryParseJSON(str) {
    if (typeof str !== "string") return null;
    var trimmed = str.trim();
    if ((trimmed[0] === "{" && trimmed[trimmed.length - 1] === "}") ||
        (trimmed[0] === "[" && trimmed[trimmed.length - 1] === "]")) {
      try { return JSON.parse(trimmed); } catch (e) { return null; }
    }
    return null;
  }

  function detectMessageType(msg) {
    var text = msg.text || "";
    var parsed = tryParseJSON(text);
    if (parsed && parsed.type === "shutdown_request") return "shutdown";
    if (parsed && parsed.type === "shutdown_response") return "shutdown";
    if (parsed && parsed.type === "idle_notification") return "idle";
    if (text.indexOf("[idle") !== -1) return "idle";
    if (text.indexOf("[shutdown") !== -1) return "shutdown";
    if (msg.inbox === "*" || (msg.to && msg.to === "*")) return "broadcast";
    if (parsed && parsed.type && parsed.type.indexOf("task") !== -1) return "task";
    if (text.match(/task\s*#?\d+/i)) return "task";
    return "normal";
  }

  function getAvatarColor(index) {
    return avatarColors[index % avatarColors.length];
  }

  function getBarColor(index) {
    return barColors[index % barColors.length];
  }

  function getInitial(name) {
    if (!name) return "?";
    return name.charAt(0).toUpperCase();
  }

  // ===== API =====

  function apiGet(path) {
    return fetch(path).then(function (res) {
      if (!res.ok) throw new Error("API error: " + res.status);
      return res.json();
    });
  }

  // ===== 页面切换 =====

  function showTeamSelectPage() {
    closeWebSocket();
    currentTeam = null;
    dashboardPage.classList.add("hidden");
    teamSelectPage.classList.remove("hidden");
    loadTeams();
  }

  function showDashboard(teamName) {
    currentTeam = teamName;
    teamSelectPage.classList.add("hidden");
    dashboardPage.classList.remove("hidden");
    dashboardTeamName.textContent = teamName;
    loadTeamData(teamName);
  }

  // ===== 团队选择页 =====

  function loadTeams() {
    apiGet("/api/teams").then(function (teams) {
      if (!Array.isArray(teams) || teams.length === 0) {
        teamsGrid.innerHTML =
          '<div class="col-span-full text-center text-slate-500 py-20">' +
            '<svg class="w-12 h-12 mx-auto mb-3 text-slate-600" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M18 18.72a9.094 9.094 0 003.741-.479 3 3 0 00-4.682-2.72m.94 3.198l.001.031c0 .225-.012.447-.037.666A11.944 11.944 0 0112 21c-2.17 0-4.207-.576-5.963-1.584A6.062 6.062 0 016 18.719m12 0a5.971 5.971 0 00-.941-3.197m0 0A5.995 5.995 0 0012 12.75a5.995 5.995 0 00-5.058 2.772m0 0a3 3 0 00-4.681 2.72 8.986 8.986 0 003.74.477m.94-3.197a5.971 5.971 0 00-.94 3.197M15 6.75a3 3 0 11-6 0 3 3 0 016 0zm6 3a2.25 2.25 0 11-4.5 0 2.25 2.25 0 014.5 0zm-13.5 0a2.25 2.25 0 11-4.5 0 2.25 2.25 0 014.5 0z"/></svg>' +
            '<p class="text-base">暂无活跃团队</p>' +
            '<p class="text-sm mt-1">请先启动一个 Agent Team</p>' +
          '</div>';
        return;
      }

      teamsGrid.innerHTML = "";
      teams.forEach(function (team) {
        var card = document.createElement("div");
        card.className = "team-card-hover bg-slate-800 border border-slate-700 rounded-xl p-5 cursor-pointer";
        card.onclick = function () { showDashboard(team.name); };

        var membersCount = team.members_count != null ? team.members_count : "?";
        var desc = team.description || "Agent 协作团队";
        var cwd = team.cwd || "";
        var createdAt = team.created_at ? formatDate(team.created_at) : "";

        card.innerHTML =
          '<div class="flex items-start justify-between mb-3">' +
            '<div class="w-10 h-10 rounded-lg bg-blue-500/20 flex items-center justify-center">' +
              '<svg class="w-5 h-5 text-blue-400" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M18 18.72a9.094 9.094 0 003.741-.479 3 3 0 00-4.682-2.72m.94 3.198l.001.031c0 .225-.012.447-.037.666A11.944 11.944 0 0112 21c-2.17 0-4.207-.576-5.963-1.584A6.062 6.062 0 016 18.719m12 0a5.971 5.971 0 00-.941-3.197m0 0A5.995 5.995 0 0012 12.75a5.995 5.995 0 00-5.058 2.772m0 0a3 3 0 00-4.681 2.72 8.986 8.986 0 003.74.477m.94-3.197a5.971 5.971 0 00-.94 3.197M15 6.75a3 3 0 11-6 0 3 3 0 016 0zm6 3a2.25 2.25 0 11-4.5 0 2.25 2.25 0 014.5 0zm-13.5 0a2.25 2.25 0 11-4.5 0 2.25 2.25 0 014.5 0z"/></svg>' +
            '</div>' +
            '<span class="text-xs bg-slate-700 text-slate-300 px-2 py-0.5 rounded-full">' + escapeHtml(String(membersCount)) + ' 成员</span>' +
          '</div>' +
          '<h3 class="text-base font-semibold text-white mb-1">' + escapeHtml(team.name) + '</h3>' +
          '<p class="text-sm text-slate-400 mb-3">' + escapeHtml(desc) + '</p>' +
          (cwd ? '<div class="text-xs text-slate-500 truncate mb-1" title="' + escapeHtml(cwd) + '"><span class="text-slate-600">目录:</span> ' + escapeHtml(cwd) + '</div>' : '') +
          (createdAt ? '<div class="text-xs text-slate-500"><span class="text-slate-600">创建:</span> ' + escapeHtml(createdAt) + '</div>' : '');

        teamsGrid.appendChild(card);
      });

      // 如果只有一个团队，自动进入
      if (teams.length === 1) {
        showDashboard(teams[0].name);
      }
    }).catch(function (err) {
      console.error("加载团队列表失败:", err);
      teamsGrid.innerHTML =
        '<div class="col-span-full text-center text-red-400 py-20">' +
          '<p class="text-base">加载失败</p>' +
          '<p class="text-sm text-slate-500 mt-1">' + escapeHtml(err.message) + '</p>' +
          '<button onclick="location.reload()" class="mt-3 text-sm text-blue-400 hover:text-blue-300">重试</button>' +
        '</div>';
    });
  }

  // ===== Dashboard 数据加载 =====

  function loadTeamData(teamName) {
    membersGrid.innerHTML = '<p class="text-slate-500 text-sm col-span-full text-center py-8">加载中...</p>';
    messagesTimeline.innerHTML = '<p class="text-slate-500 text-sm text-center py-8">加载中...</p>';
    tokensContent.innerHTML = '<p class="text-slate-500 text-sm text-center py-4">加载中...</p>';

    Promise.all([
      apiGet("/api/teams/" + encodeURIComponent(teamName)),
      apiGet("/api/teams/" + encodeURIComponent(teamName) + "/tasks"),
      apiGet("/api/teams/" + encodeURIComponent(teamName) + "/messages"),
      apiGet("/api/teams/" + encodeURIComponent(teamName) + "/tokens")
    ]).then(function (results) {
      currentTokenData = results[3];
      renderMembers(results[0], currentTokenData);
      renderKanban(results[1]);
      renderMessages(results[2]);
      renderTokens(results[3]);
      connectWebSocket(teamName);
    }).catch(function (err) {
      console.error("加载团队数据失败:", err);
    });
  }

  // ===== 渲染：成员卡片 =====

  // 从 token 数据中查找某个成员的消耗
  function findMemberToken(tokenData, memberName) {
    if (!tokenData || !tokenData.members) return null;
    var members = tokenData.members;
    for (var i = 0; i < members.length; i++) {
      if (members[i].name === memberName) return members[i];
    }
    // 模糊匹配：成员名包含在 token 数据的 name 中
    for (var i = 0; i < members.length; i++) {
      if (members[i].name && members[i].name.indexOf(memberName) !== -1) return members[i];
    }
    return null;
  }

  function renderMembers(teamData, tokenData) {
    var members = [];
    if (teamData && teamData.members) members = teamData.members;
    else if (Array.isArray(teamData)) members = teamData;

    if (members.length === 0) {
      membersGrid.innerHTML = '<p class="text-slate-500 text-sm col-span-full text-center py-8">暂无成员</p>';
      return;
    }

    membersGrid.innerHTML = "";
    members.forEach(function (m, index) {
      var card = document.createElement("div");
      card.className = "member-card-hover bg-slate-800 border border-slate-700 rounded-lg p-4";
      card.dataset.agentId = m.agentId || m.name || "";
      card.dataset.memberName = m.name || "";

      var status = m.status || "active";
      var statusDotClass = "status-dot-" + (status === "idle" ? "idle" : (status === "offline" ? "offline" : "active"));
      var statusLabel = status === "idle" ? "空闲" : (status === "offline" ? "离线" : "活跃");

      var modelShort = (m.model || "unknown").replace(/\[.*\]/, "");
      var name = m.name || "unknown";
      var role = m.agentType || m.agent_type || "member";
      var colorClass = "avatar-" + getAvatarColor(index);

      // 运行模式
      var runMode = m.tmuxPaneId ? "tmux" : "in-process";

      // 查找该成员的 token 消耗
      var tokenInfo = findMemberToken(tokenData || currentTokenData, name);
      var tokenHtml = "";
      if (tokenInfo && (tokenInfo.input_tokens > 0 || tokenInfo.output_tokens > 0)) {
        var totalTokens = (tokenInfo.input_tokens || 0) + (tokenInfo.output_tokens || 0);
        var cost = tokenInfo.estimated_cost_usd || 0;
        tokenHtml =
          '<div class="token-display mt-2 pt-2 border-t border-slate-700/50">' +
            '<div class="text-xs text-slate-500 mb-1">Token 消耗</div>' +
            '<div class="flex gap-3 text-xs">' +
              '<span class="text-slate-500">入 <span class="text-slate-400 font-mono">' + formatTokens(tokenInfo.input_tokens) + '</span></span>' +
              '<span class="text-slate-500">出 <span class="text-slate-400 font-mono">' + formatTokens(tokenInfo.output_tokens) + '</span></span>' +
            '</div>' +
            // 小型进度条显示在团队中的占比
            (tokenData && tokenData.team_total ?
              (function() {
                var teamTotal = (tokenData.team_total.input_tokens || 0) + (tokenData.team_total.output_tokens || 0);
                var pct = teamTotal > 0 ? Math.round(totalTokens / teamTotal * 100) : 0;
                return '<div class="mt-1.5 h-1 bg-slate-700 rounded-full overflow-hidden">' +
                  '<div class="h-full bg-blue-500 rounded-full" style="width:' + pct + '%"></div>' +
                '</div>' +
                '<div class="text-right text-[10px] text-slate-600 mt-0.5">' + pct + '% 占比</div>';
              })() : '') +
          '</div>';
      }

      card.innerHTML =
        '<div class="flex items-start gap-3">' +
          '<div class="' + colorClass + ' w-10 h-10 rounded-full flex items-center justify-center text-white font-bold text-sm shrink-0">' +
            escapeHtml(getInitial(name)) +
          '</div>' +
          '<div class="flex-1 min-w-0">' +
            '<div class="flex items-center gap-2 mb-1">' +
              '<span class="font-semibold text-sm text-white truncate">' + escapeHtml(name) + '</span>' +
              '<span class="' + statusDotClass + '"></span>' +
            '</div>' +
            '<div class="text-xs text-slate-400 mb-0.5">' + escapeHtml(role) + '</div>' +
            '<div class="text-xs text-slate-500 font-mono">' + escapeHtml(modelShort) + '</div>' +
            '<div class="text-xs text-slate-600 mt-1">' + escapeHtml(runMode) + ' · ' + escapeHtml(statusLabel) + '</div>' +
          '</div>' +
        '</div>' +
        tokenHtml;

      membersGrid.appendChild(card);
    });
  }

  // ===== 渲染：任务看板 =====

  function renderKanban(tasks) {
    kanbanPending.innerHTML = "";
    kanbanInProgress.innerHTML = "";
    kanbanCompleted.innerHTML = "";

    var counts = { pending: 0, progress: 0, done: 0 };

    if (Array.isArray(tasks)) {
      tasks.forEach(function (task) {
        var card = createTaskCard(task);
        var status = (task.status || "").toLowerCase();
        if (status === "completed" || status === "done") {
          kanbanCompleted.appendChild(card);
          counts.done++;
        } else if (status === "in_progress" || status === "in-progress") {
          kanbanInProgress.appendChild(card);
          counts.progress++;
        } else {
          kanbanPending.appendChild(card);
          counts.pending++;
        }
      });
    }

    kanbanPendingCount.textContent = counts.pending;
    kanbanProgressCount.textContent = counts.progress;
    kanbanDoneCount.textContent = counts.done;
  }

  function createTaskCard(task) {
    var card = document.createElement("div");
    card.className = "task-card-animated bg-slate-900/60 border border-slate-700/50 rounded-md p-2.5";
    card.dataset.taskId = task.id || task.task_id || "";

    var taskId = task.id || task.task_id || "";
    var title = task.title || task.subject || "未命名任务";
    var owner = task.owner || "未分配";

    card.innerHTML =
      (taskId ? '<span class="text-[10px] font-mono text-slate-500 bg-slate-800 px-1.5 py-0.5 rounded">#' + escapeHtml(String(taskId)) + '</span> ' : '') +
      '<div class="text-xs font-medium text-slate-300 mt-1 leading-relaxed">' + escapeHtml(title) + '</div>' +
      '<div class="text-[10px] text-slate-500 mt-1">' + escapeHtml(owner) + '</div>' +
      (task.blockedBy && task.blockedBy.length > 0
        ? '<div class="text-[10px] text-slate-600 mt-1 italic">依赖: #' + escapeHtml(task.blockedBy.join(", #")) + '</div>'
        : '');

    return card;
  }

  function updateKanbanCounts() {
    kanbanPendingCount.textContent = kanbanPending.children.length;
    kanbanProgressCount.textContent = kanbanInProgress.children.length;
    kanbanDoneCount.textContent = kanbanCompleted.children.length;
  }

  // ===== 渲染：Token 统计 =====

  function renderTokens(data) {
    if (!data || (!data.team_total && !data.members)) {
      tokensContent.innerHTML = '<p class="text-slate-500 text-sm text-center py-4">暂无 Token 数据</p>';
      return;
    }

    var total = data.team_total || {};
    var members = data.members || [];
    var totalAllTokens = (total.input_tokens || 0) + (total.output_tokens || 0);

    // Cache hit rate
    var cacheRead = total.cache_read_input_tokens || 0;
    var inputTokens = total.input_tokens || 0;
    var cacheHitRate = (cacheRead + inputTokens) > 0
      ? Math.round((cacheRead / (cacheRead + inputTokens)) * 100)
      : 0;

    var html = '';

    // 顶部统计行：费用突出 + 环形进度
    html += '<div class="flex items-start gap-6 mb-5">';

    // 环形进度条
    html += '<div class="shrink-0">';
    html += '<div class="donut-chart" style="background: conic-gradient(#22c55e ' + cacheHitRate + '%, #334155 ' + cacheHitRate + '%)">';
    html += '<span class="donut-label">' + cacheHitRate + '%</span>';
    html += '</div>';
    html += '<div class="text-[10px] text-slate-500 text-center mt-1">Cache 命中率</div>';
    html += '</div>';

    // 统计数字
    html += '<div class="flex-1">';
    html += '<div class="text-2xl font-bold text-blue-400 mb-2">' + formatTokens((total.input_tokens || 0) + (total.output_tokens || 0)) + '</div>';
    html += '<div class="text-xs text-slate-500 mb-1">总 Token 消耗</div>';
    html += '<div class="grid grid-cols-2 gap-3 mt-3">';
    html += '<div><div class="text-xs text-slate-500">输入 Token</div><div class="text-sm font-semibold text-slate-300">' + formatTokens(total.input_tokens) + '</div></div>';
    html += '<div><div class="text-xs text-slate-500">输出 Token</div><div class="text-sm font-semibold text-slate-300">' + formatTokens(total.output_tokens) + '</div></div>';
    html += '<div><div class="text-xs text-slate-500">Cache 创建</div><div class="text-sm font-semibold text-slate-300">' + formatTokens(total.cache_creation_input_tokens) + '</div></div>';
    html += '<div><div class="text-xs text-slate-500">Cache 读取</div><div class="text-sm font-semibold text-green-400">' + formatTokens(total.cache_read_input_tokens) + '</div></div>';
    html += '</div>';
    html += '</div>';
    html += '</div>';

    // 成员占比柱状图
    if (members.length > 0) {
      html += '<div class="border-t border-slate-700 pt-4 space-y-3">';
      html += '<div class="text-xs text-slate-500 font-semibold uppercase tracking-wider mb-2">成员消耗占比</div>';

      members.forEach(function (m, index) {
        var memberTotal = (m.input_tokens || 0) + (m.output_tokens || 0);
        var pct = totalAllTokens > 0 ? Math.round((memberTotal / totalAllTokens) * 100) : 0;
        var modelShort = (m.model || "unknown").replace(/\[.*\]/, "");
        var barColorClass = getBarColor(index);

        html += '<div class="flex items-center gap-3">';
        html += '<div class="w-20 text-xs text-slate-400 truncate shrink-0" title="' + escapeHtml(m.name || "unknown") + '">' + escapeHtml(m.name || "unknown") + '</div>';
        html += '<div class="flex-1 h-5 bg-slate-900/60 rounded-full overflow-hidden relative">';
        html += '<div class="h-full rounded-full ' + barColorClass + ' transition-all duration-500" style="width:' + pct + '%"></div>';
        html += '</div>';
        html += '<div class="w-28 text-right text-xs shrink-0">';
        html += '<span class="text-slate-400">' + pct + '%</span>';
        html += '<span class="text-slate-500 ml-2">' + formatTokens(memberTotal) + '</span>';
        html += '</div>';
        html += '</div>';
      });

      html += '</div>';
    }

    // 脚注
    html += '<div class="text-[10px] text-slate-600 mt-4 pt-3 border-t border-slate-700/50">* Token 数据来自 JSONL 流式记录，仅供趋势参考</div>';

    tokensContent.innerHTML = html;
  }

  // ===== 渲染：消息流 =====

  function renderMessages(data) {
    var messages = Array.isArray(data) ? data : (data && data.messages ? data.messages : []);

    if (messages.length === 0) {
      messagesTimeline.innerHTML = '<p class="text-slate-500 text-sm text-center py-8">暂无消息</p>';
      return;
    }

    messagesTimeline.innerHTML = "";
    messages.forEach(function (msg) {
      appendMessage(msg, false);
    });
    scrollMessagesToBottom(true);
  }

  function appendMessage(msg, animate) {
    var placeholder = messagesTimeline.querySelector("p");
    if (placeholder) placeholder.remove();

    var msgType = detectMessageType(msg);
    var from = msg.from || "unknown";
    var to = msg.inbox || msg.to || "?";
    var timeStr = formatTime(msg.timestamp);

    // 系统消息（idle/shutdown）居中显示
    if (msgType === "idle" || msgType === "shutdown") {
      var sysDiv = document.createElement("div");
      sysDiv.className = "flex justify-center" + (animate !== false ? " message-bubble" : "");

      var bgClass = msgType === "shutdown" ? "bg-red-500/10 text-red-400 border-red-500/20" : "bg-yellow-500/10 text-yellow-400 border-yellow-500/20";

      var parsed = tryParseJSON(msg.text || "");
      var sysLabel = msgType === "shutdown" ? "shutdown" : "idle";
      var sysDetail = "";
      if (parsed) {
        if (parsed.reason) sysDetail = parsed.reason;
        else if (parsed.type) sysDetail = parsed.type;
      }

      sysDiv.innerHTML =
        '<div class="text-xs border rounded-full px-3 py-1 ' + bgClass + '">' +
          '<span class="font-mono">' + escapeHtml(timeStr) + '</span> ' +
          '<span class="font-semibold">[' + sysLabel + ']</span> ' +
          escapeHtml(from) +
          (sysDetail ? ' — ' + escapeHtml(sysDetail) : '') +
        '</div>';

      messagesTimeline.appendChild(sysDiv);
      return;
    }

    var bubble = document.createElement("div");
    bubble.className = (animate !== false ? "message-bubble " : "") + "flex gap-2.5";

    // 消息气泡颜色
    var bubbleBg = "bg-slate-700/50";
    var routeColor = "text-blue-400";
    var tagHtml = "";

    if (msgType === "broadcast") {
      bubbleBg = "bg-purple-500/10 border border-purple-500/20";
      routeColor = "text-purple-400";
      tagHtml = '<span class="text-[10px] bg-purple-500/20 text-purple-400 px-1.5 py-0.5 rounded ml-2">广播</span>';
    } else if (msgType === "task") {
      routeColor = "text-green-400";
      tagHtml = '<span class="text-[10px] bg-green-500/20 text-green-400 px-1.5 py-0.5 rounded ml-2">任务</span>';
    }

    // 发送方头像
    var avatarIndex = simpleHash(from) % avatarColors.length;
    var avatarColorClass = "avatar-" + avatarColors[avatarIndex];

    // 消息内容
    var contentHtml = "";
    var parsed = tryParseJSON(msg.text || "");
    if (parsed) {
      contentHtml = '<div class="text-xs text-slate-400 bg-slate-900/50 rounded p-2 mt-1 font-mono break-all whitespace-pre-wrap">' + escapeHtml(JSON.stringify(parsed, null, 2)) + '</div>';
    } else {
      contentHtml = '<div class="text-sm text-slate-300 leading-relaxed break-words whitespace-pre-wrap">' + escapeHtml(msg.text || msg.summary || "") + '</div>';
    }

    // 路由
    var routeHtml = escapeHtml(from);
    if (msgType === "broadcast") {
      routeHtml += ' <span class="text-slate-600">&rarr;</span> <span class="text-purple-400">* (全体)</span>';
    } else {
      routeHtml += ' <span class="text-slate-600">&rarr;</span> ' + escapeHtml(to);
    }

    bubble.innerHTML =
      '<div class="' + avatarColorClass + ' w-7 h-7 rounded-full flex items-center justify-center text-white text-xs font-bold shrink-0 mt-0.5">' +
        escapeHtml(getInitial(from)) +
      '</div>' +
      '<div class="flex-1 min-w-0">' +
        '<div class="flex items-center gap-2 mb-1">' +
          '<span class="text-xs font-semibold ' + routeColor + '">' + routeHtml + '</span>' +
          tagHtml +
          '<span class="text-[10px] text-slate-600 ml-auto font-mono shrink-0">' + escapeHtml(timeStr) + '</span>' +
        '</div>' +
        '<div class="rounded-lg ' + bubbleBg + ' px-3 py-2">' +
          contentHtml +
        '</div>' +
      '</div>';

    messagesTimeline.appendChild(bubble);
  }

  function simpleHash(str) {
    var hash = 0;
    for (var i = 0; i < str.length; i++) {
      hash = ((hash << 5) - hash) + str.charCodeAt(i);
      hash |= 0;
    }
    return Math.abs(hash);
  }

  // 自动滚动控制
  function setupScrollListener() {
    messagesTimeline.addEventListener("scroll", function () {
      var el = messagesTimeline;
      var isAtBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 50;
      userHasScrolled = !isAtBottom;
    });
  }

  function scrollMessagesToBottom(force) {
    if (!force && userHasScrolled) return;
    messagesTimeline.scrollTop = messagesTimeline.scrollHeight;
  }

  // ===== WebSocket =====

  function setWsStatus(status) {
    if (status === "connected") {
      wsStatusDot.className = "w-2 h-2 rounded-full bg-green-400";
      wsStatusText.textContent = "已连接";
      wsStatusText.className = "ws-connected";
    } else if (status === "reconnecting") {
      wsStatusDot.className = "w-2 h-2 rounded-full bg-yellow-400";
      wsStatusText.className = "ws-reconnecting";
    } else {
      wsStatusDot.className = "w-2 h-2 rounded-full bg-red-400";
      wsStatusText.textContent = "已断开";
      wsStatusText.className = "ws-disconnected";
    }
  }

  function closeWebSocket() {
    if (ws) { ws.close(); ws = null; }
    if (wsReconnectTimer) { clearTimeout(wsReconnectTimer); wsReconnectTimer = null; }
    if (wsCountdownInterval) { clearInterval(wsCountdownInterval); wsCountdownInterval = null; }
    setWsStatus("disconnected");
  }

  function connectWebSocket(teamName) {
    closeWebSocket();

    var protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    var url = protocol + "//" + window.location.host + "/ws/" + encodeURIComponent(teamName);
    ws = new WebSocket(url);

    ws.onopen = function () {
      setWsStatus("connected");
    };

    ws.onmessage = function (evt) {
      try {
        var payload = JSON.parse(evt.data);
        handleWebSocketEvent(payload);
      } catch (e) {
        console.error("解析 WebSocket 消息失败:", e);
      }
    };

    ws.onerror = function () {
      setWsStatus("disconnected");
    };

    ws.onclose = function () {
      if (currentTeam === teamName) {
        wsReconnectCountdown = 3;
        setWsStatus("reconnecting");
        wsStatusText.textContent = "重连中 (" + wsReconnectCountdown + "s)";

        wsCountdownInterval = setInterval(function () {
          wsReconnectCountdown--;
          if (wsReconnectCountdown > 0) {
            wsStatusText.textContent = "重连中 (" + wsReconnectCountdown + "s)";
          } else {
            clearInterval(wsCountdownInterval);
            wsCountdownInterval = null;
          }
        }, 1000);

        wsReconnectTimer = setTimeout(function () {
          if (currentTeam === teamName) {
            connectWebSocket(teamName);
          }
        }, 3000);
      } else {
        setWsStatus("disconnected");
      }
    };
  }

  function handleWebSocketEvent(payload) {
    var event = payload.event;
    var data = payload.data;

    switch (event) {
      case "member_update":
        if (data && data.members) {
          renderMembers({ members: data.members });
        }
        break;

      case "new_message":
        if (data && data.message) {
          var msg = data.message;
          if (data.inbox) msg.inbox = data.inbox;
          appendMessage(msg, true);
          scrollMessagesToBottom(false);
        }
        break;

      case "task_update":
        if (data) {
          updateKanbanTask(data);
        }
        break;

      case "token_update":
        if (data) {
          currentTokenData = data;
          renderTokens(data);
          updateMemberTokens(data);
        }
        break;

      default:
        console.log("未知 WebSocket 事件:", event);
    }
  }

  // 实时更新成员卡片上的 token 显示（不重新渲染整个列表）
  function updateMemberTokens(tokenData) {
    if (!tokenData || !tokenData.members) return;
    var cards = membersGrid.querySelectorAll("[data-member-name]");
    cards.forEach(function (card) {
      var name = card.dataset.memberName;
      var tokenInfo = findMemberToken(tokenData, name);
      // 找到卡片中的 token 区域并更新
      var tokenDiv = card.querySelector(".token-display");
      if (tokenInfo && (tokenInfo.input_tokens > 0 || tokenInfo.output_tokens > 0)) {
        var teamTotal = (tokenData.team_total.input_tokens || 0) + (tokenData.team_total.output_tokens || 0);
        var totalTokens = (tokenInfo.input_tokens || 0) + (tokenInfo.output_tokens || 0);
        var pct = teamTotal > 0 ? Math.round(totalTokens / teamTotal * 100) : 0;
        var cost = tokenInfo.estimated_cost_usd || 0;

        var html =
          '<div class="text-xs text-slate-500 mb-1">Token 消耗</div>' +
          '<div class="flex gap-3 text-xs">' +
            '<span class="text-slate-500">入 <span class="text-slate-400 font-mono">' + formatTokens(tokenInfo.input_tokens) + '</span></span>' +
            '<span class="text-slate-500">出 <span class="text-slate-400 font-mono">' + formatTokens(tokenInfo.output_tokens) + '</span></span>' +
          '</div>' +
          '<div class="mt-1.5 h-1 bg-slate-700 rounded-full overflow-hidden">' +
            '<div class="h-full bg-blue-500 rounded-full" style="width:' + pct + '%"></div>' +
          '</div>' +
          '<div class="text-right text-[10px] text-slate-600 mt-0.5">' + pct + '% 占比</div>';

        if (tokenDiv) {
          tokenDiv.innerHTML = html;
        } else {
          // 创建 token 区域
          tokenDiv = document.createElement("div");
          tokenDiv.className = "token-display mt-2 pt-2 border-t border-slate-700/50";
          tokenDiv.innerHTML = html;
          card.appendChild(tokenDiv);
        }
      }
    });
  }

  function updateKanbanTask(taskData) {
    var taskId = taskData.task_id || taskData.id || "";
    var existing = document.querySelector('.task-card-animated[data-task-id="' + taskId + '"]');
    if (existing) existing.remove();

    var card = createTaskCard(taskData);
    var status = (taskData.status || "").toLowerCase();
    if (status === "completed" || status === "done") {
      kanbanCompleted.appendChild(card);
    } else if (status === "in_progress" || status === "in-progress") {
      kanbanInProgress.appendChild(card);
    } else {
      kanbanPending.appendChild(card);
    }
    updateKanbanCounts();
  }

  // ===== 事件绑定 =====

  btnBack.addEventListener("click", function () {
    showTeamSelectPage();
  });

  btnRefresh.addEventListener("click", function () {
    if (currentTeam) {
      loadTeamData(currentTeam);
    }
  });

  setupScrollListener();

  // ===== 初始化 =====

  loadTeams();

})();
