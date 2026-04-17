package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"agent-monitor/internal/model"
	"agent-monitor/internal/token"

	"github.com/gin-gonic/gin"
)

// APIHandler REST API 处理器
type APIHandler struct {
	TeamsDir    string                                // ~/.claude/teams
	TasksDir    string                                // ~/.claude/tasks
	Trackers    map[string]*token.Tracker             // 静态 tracker（向后兼容）
	GetTrackers func() map[string]*token.Tracker      // 动态获取 tracker（优先使用）
}

// getTracker 获取指定团队的 tracker
func (h *APIHandler) getTracker(name string) *token.Tracker {
	if h.GetTrackers != nil {
		trackers := h.GetTrackers()
		return trackers[name]
	}
	if h.Trackers != nil {
		return h.Trackers[name]
	}
	return nil
}

// TeamSummary 团队列表中的摘要信息
type TeamSummary struct {
	Name         string `json:"name"`
	SessionID    string `json:"session_id"`
	Cwd          string `json:"cwd"`
	MembersCount int    `json:"members_count"`
	Description  string `json:"description"`
}

// MessageWithInbox 带有收件人信息的消息
type MessageWithInbox struct {
	model.InboxMessage
	Inbox string `json:"inbox"`
}

// RegisterRoutes 注册所有 REST API 路由
func (h *APIHandler) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api")
	{
		api.GET("/teams", h.ListTeams)
		api.GET("/teams/:name", h.GetTeam)
		api.GET("/teams/:name/messages", h.GetMessages)
		api.GET("/teams/:name/tasks", h.GetTasks)
		api.GET("/teams/:name/tokens", h.GetTokens)
	}
}

// ListTeams 扫描 ~/.claude/teams/ 下所有子目录，返回团队列表
func (h *APIHandler) ListTeams(c *gin.Context) {
	entries, err := os.ReadDir(h.TeamsDir)
	if err != nil {
		c.JSON(http.StatusOK, []TeamSummary{})
		return
	}

	var teams []TeamSummary
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		configPath := filepath.Join(h.TeamsDir, entry.Name(), "config.json")
		data, err := os.ReadFile(configPath)
		if err != nil {
			continue
		}

		var config model.TeamConfig
		if err := json.Unmarshal(data, &config); err != nil {
			continue
		}

		// 从成员中获取 cwd
		cwd := ""
		for _, m := range config.Members {
			if m.Cwd != "" {
				cwd = m.Cwd
				break
			}
		}

		teams = append(teams, TeamSummary{
			Name:         config.Name,
			SessionID:    config.LeadSessionID,
			Cwd:          cwd,
			MembersCount: len(config.Members),
			Description:  config.Description,
		})
	}

	c.JSON(http.StatusOK, teams)
}

// GetTeam 返回指定团队的完整 config.json 内容
// 会用 JSONL 中检测到的真实模型覆盖 config 中可能不准确的 model 字段
func (h *APIHandler) GetTeam(c *gin.Context) {
	name := c.Param("name")
	configPath := filepath.Join(h.TeamsDir, name, "config.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "team not found"})
		return
	}

	var config model.TeamConfig
	if err := json.Unmarshal(data, &config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse config"})
		return
	}

	// 尝试用 JSONL 中的真实模型覆盖 config 中的 model 字段
	if tracker := h.getTracker(name); tracker != nil {
		realModels := tracker.GetRealModels()
		// lead 对应 index 0，subagents 从 index 1 开始
		// config.members[0] 是 lead
		if leadModel, ok := realModels[0]; ok {
			for i := range config.Members {
				if config.Members[i].AgentType == "team-lead" {
					config.Members[i].Model = leadModel
					break
				}
			}
		}
		// 对于非 lead 成员，如果 JSONL 检测到了统一的模型，覆盖所有
		// 先收集所有检测到的非默认模型
		detectedModels := make(map[string]int)
		for idx, m := range realModels {
			if idx > 0 {
				detectedModels[m]++
			}
		}
		// 如果有一个主要模型，用它覆盖所有非 lead 成员
		if len(detectedModels) > 0 {
			bestModel := ""
			bestCount := 0
			for m, c := range detectedModels {
				if c > bestCount {
					bestModel = m
					bestCount = c
				}
			}
			if bestModel != "" {
				for i := range config.Members {
					if config.Members[i].AgentType != "team-lead" {
						config.Members[i].Model = bestModel
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, config)
}

// GetMessages 读取团队所有 inbox 文件，合并并按时间排序返回
func (h *APIHandler) GetMessages(c *gin.Context) {
	name := c.Param("name")
	inboxDir := filepath.Join(h.TeamsDir, name, "inboxes")

	entries, err := os.ReadDir(inboxDir)
	if err != nil {
		c.JSON(http.StatusOK, []MessageWithInbox{})
		return
	}

	var allMessages []MessageWithInbox
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(inboxDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var messages []model.InboxMessage
		if err := json.Unmarshal(data, &messages); err != nil {
			continue
		}

		// inbox 名称 = 文件名去掉 .json 后缀
		inbox := strings.TrimSuffix(entry.Name(), ".json")
		for _, msg := range messages {
			allMessages = append(allMessages, MessageWithInbox{
				InboxMessage: msg,
				Inbox:        inbox,
			})
		}
	}

	// 按 timestamp 排序
	sort.Slice(allMessages, func(i, j int) bool {
		return allMessages[i].Timestamp < allMessages[j].Timestamp
	})

	c.JSON(http.StatusOK, allMessages)
}

// GetTasks 读取团队的所有任务文件
func (h *APIHandler) GetTasks(c *gin.Context) {
	name := c.Param("name")
	taskDir := filepath.Join(h.TasksDir, name)

	entries, err := os.ReadDir(taskDir)
	if err != nil {
		c.JSON(http.StatusOK, []model.Task{})
		return
	}

	var tasks []model.Task
	for _, entry := range entries {
		// 排除 .lock 和 .highwatermark 等辅助文件
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(taskDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var task model.Task
		if err := json.Unmarshal(data, &task); err != nil {
			continue
		}

		// 确保 title 字段有值（前端用 title 展示）
		if task.Title == "" {
			task.Title = task.DisplayTitle()
		}

		tasks = append(tasks, task)
	}

	c.JSON(http.StatusOK, tasks)
}

// GetTokens 调用 token.Tracker 获取 TokenReport
func (h *APIHandler) GetTokens(c *gin.Context) {
	name := c.Param("name")

	tracker := h.getTracker(name)
	if tracker == nil {
		// 返回空报告
		c.JSON(http.StatusOK, model.TokenReport{})
		return
	}

	report := tracker.GetReport()
	c.JSON(http.StatusOK, report)
}
