package token

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"agent-monitor/internal/model"
)

// 按模型的 token 费率（每百万 token 的美元价格）
var modelPricing = map[string]struct {
	InputPerM  float64
	OutputPerM float64
}{
	"claude-opus-4-6":   {InputPerM: 15.0, OutputPerM: 75.0},
	"claude-sonnet-4-6": {InputPerM: 3.0, OutputPerM: 15.0},
	"claude-haiku-4-5":  {InputPerM: 0.80, OutputPerM: 4.0},
}

// Cache 读取的成本为输入 token 的 10%
const cacheReadCostRatio = 0.1

// jsonlMessage JSONL 中 message 字段的嵌套结构
type jsonlMessage struct {
	Role  string            `json:"role"`
	Usage *model.TokenUsage `json:"usage"`
}

// jsonlRecord JSONL 文件中单行的结构
// 实际格式: {"type":"assistant", "message":{"role":"assistant","usage":{...}}, ...}
type jsonlRecord struct {
	Type    string        `json:"type"`
	Message jsonlMessage  `json:"message"`
}

// fileState 跟踪单个 JSONL 文件的读取偏移量
type fileState struct {
	Offset int64
	Usage  model.TokenUsage
}

// memberInfo 追踪某个成员的 JSONL 路径和身份信息
type memberInfo struct {
	Name  string
	Model string
	Path  string
}

// Tracker 定时轮询 JSONL 文件，累加 token 消耗
type Tracker struct {
	projectsDir string // ~/.claude/projects
	teamsDir    string // ~/.claude/teams
	teamName    string

	// 每个 JSONL 文件的读取状态
	fileStates map[string]*fileState
	mu         sync.RWMutex

	interval time.Duration
	done     chan struct{}
	onUpdate func(model.TokenReport) // token 更新回调
}

// NewTracker 创建 token 追踪器
// projectsDir: ~/.claude/projects 路径
// teamsDir: ~/.claude/teams 路径
// teamName: 团队名称
func NewTracker(projectsDir, teamsDir, teamName string) *Tracker {
	return &Tracker{
		projectsDir: projectsDir,
		teamsDir:    teamsDir,
		teamName:    teamName,
		fileStates:  make(map[string]*fileState),
		interval:    5 * time.Second,
		done:        make(chan struct{}),
	}
}

// OnUpdate 设置 token 更新的回调函数
func (t *Tracker) OnUpdate(fn func(model.TokenReport)) {
	t.onUpdate = fn
}

// Start 启动定时轮询，每 5 秒扫描一次 JSONL 文件
func (t *Tracker) Start() {
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	// 首次立即扫描
	t.poll()

	for {
		select {
		case <-ticker.C:
			t.poll()
		case <-t.done:
			return
		}
	}
}

// Stop 停止定时轮询
func (t *Tracker) Stop() {
	close(t.done)
}

// GetReport 获取当前 token 统计报告（线程安全）
func (t *Tracker) GetReport() model.TokenReport {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.buildReport()
}

// GetRealModels 返回从 JSONL 中检测到的真实模型映射（agentIndex -> model）
// 用于覆盖 config.json 中可能不准确的 model 字段
func (t *Tracker) GetRealModels() map[int]string {
	members, _ := t.discoverJSONLFiles()
	result := make(map[int]string)
	// index 0 是 lead，后续是 subagents
	for i, m := range members {
		if m.Model != "" && m.Model != "claude-sonnet-4-6" { // 排除默认值
			result[i] = m.Model
		}
	}
	return result
}

// poll 执行一次 JSONL 文件扫描
func (t *Tracker) poll() {
	members, sessionDir := t.discoverJSONLFiles()
	if sessionDir == "" {
		return
	}

	changed := false
	for _, m := range members {
		if t.scanFile(m.Path) {
			changed = true
		}
	}

	if changed && t.onUpdate != nil {
		t.mu.RLock()
		report := t.buildReport()
		t.mu.RUnlock()
		t.onUpdate(report)
	}
}

// discoverJSONLFiles 通过 config.json 的 leadSessionId 定位 JSONL 文件
// 返回所有成员的 JSONL 信息和 session 目录路径
func (t *Tracker) discoverJSONLFiles() ([]memberInfo, string) {
	// 读取团队配置获取 leadSessionId
	configPath := filepath.Join(t.teamsDir, t.teamName, "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Printf("[token] 读取 config.json 失败: %v", err)
		return nil, ""
	}

	var config model.TeamConfig
	if err := json.Unmarshal(data, &config); err != nil {
		log.Printf("[token] 解析 config.json 失败: %v", err)
		return nil, ""
	}

	if config.LeadSessionID == "" {
		return nil, ""
	}

	// 构造项目哈希路径：/ 替换为 -
	// 通过成员的 cwd 获取项目路径
	projectPath := ""
	for _, m := range config.Members {
		if m.Cwd != "" {
			projectPath = m.Cwd
			break
		}
	}
	if projectPath == "" {
		return nil, ""
	}

	// 项目路径转 hash：去掉开头的 /，然后将 / 和 _ 替换为 -
	// Claude Code 会将路径中的 / 和 _ 都替换为 -
	hash := strings.TrimPrefix(projectPath, "/")
	hash = strings.ReplaceAll(hash, "/", "-")
	hash = strings.ReplaceAll(hash, "_", "-")
	projectHash := "-" + hash

	// 如果精确匹配不存在，尝试扫描 projects 目录模糊匹配
	projectDir := filepath.Join(t.projectsDir, projectHash)
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		// 模糊匹配：遍历 projects 目录找最接近的
		if entries, err := os.ReadDir(t.projectsDir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() && strings.Contains(entry.Name(), filepath.Base(projectPath)) {
					// 检查该目录下是否存在 sessionId 对应的文件
					candidate := filepath.Join(t.projectsDir, entry.Name())
					if _, err := os.Stat(filepath.Join(candidate, config.LeadSessionID+".jsonl")); err == nil {
						projectDir = candidate
						break
					}
				}
			}
		}
	}

	// 构建成员名称和模型的映射
	memberMap := make(map[string]string) // name -> model
	for _, m := range config.Members {
		// 简化模型名：去掉方括号后缀，如 claude-opus-4-6[1m] -> claude-opus-4-6
		modelName := m.Model
		if idx := strings.Index(modelName, "["); idx != -1 {
			modelName = modelName[:idx]
		}
		memberMap[m.Name] = modelName
	}

	var members []memberInfo

	// Lead 的 JSONL: {projectDir}/{sessionId}.jsonl
	leadPath := filepath.Join(projectDir, config.LeadSessionID+".jsonl")
	leadModel := ""
	for _, m := range config.Members {
		if m.AgentType == "team-lead" {
			leadModel = m.Model
			if idx := strings.Index(leadModel, "["); idx != -1 {
				leadModel = leadModel[:idx]
			}
			break
		}
	}
	if _, err := os.Stat(leadPath); err == nil {
		members = append(members, memberInfo{
			Name:  "team-lead",
			Model: leadModel,
			Path:  leadPath,
		})
		log.Printf("[token] 找到 lead JSONL: %s", leadPath)
	} else {
		log.Printf("[token] lead JSONL 不存在: %s (projectDir=%s)", leadPath, projectDir)
	}

	// Subagents: {projectDir}/{sessionId}/subagents/*.jsonl
	sessionDir := filepath.Join(projectDir, config.LeadSessionID)
	subagentsDir := filepath.Join(sessionDir, "subagents")
	entries, err := os.ReadDir(subagentsDir)
	if err != nil {
		// subagents 目录可能不存在
		return members, sessionDir
	}

	// 构建 agentId -> member 的映射，用于将 subagent 文件名关联到实际成员
	agentIdToMember := make(map[string]model.Member)
	for _, m := range config.Members {
		agentIdToMember[m.AgentID] = m
	}

	for i, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		jsonlPath := filepath.Join(subagentsDir, entry.Name())

		// 尝试从文件内容检测模型
		agentModel := "claude-sonnet-4-6" // 默认模型
		if detectedModel := t.detectModelFromFile(jsonlPath); detectedModel != "" {
			agentModel = detectedModel
		}

		// 为 subagent 分配名称：优先使用成员名（跳过 lead），否则用文件名
		name := strings.TrimSuffix(entry.Name(), ".jsonl")
		memberIdx := i + 1 // 跳过 lead (index 0)
		if memberIdx < len(config.Members) {
			m := config.Members[memberIdx]
			if m.AgentType != "team-lead" {
				name = m.Name
				modelName := m.Model
				if idx := strings.Index(modelName, "["); idx != -1 {
					modelName = modelName[:idx]
				}
				if modelName != "" {
					agentModel = modelName
				}
			}
		}

		members = append(members, memberInfo{
			Name:  name,
			Model: agentModel,
			Path:  jsonlPath,
		})
	}

	return members, sessionDir
}

// detectModelFromFile 尝试从 JSONL 文件内容检测使用的模型
// 模型信息在 message.model 字段中
func (t *Tracker) detectModelFromFile(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		var record struct {
			Message struct {
				Model string `json:"model"`
			} `json:"message"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &record); err == nil && record.Message.Model != "" {
			model := record.Message.Model
			// 跳过 synthetic 等特殊标记
			if model == "<synthetic>" {
				continue
			}
			if idx := strings.Index(model, "["); idx != -1 {
				model = model[:idx]
			}
			return model
		}
	}
	return ""
}

// scanFile 扫描单个 JSONL 文件的新增内容，返回是否有新数据
func (t *Tracker) scanFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	t.mu.Lock()
	state, exists := t.fileStates[path]
	if !exists {
		state = &fileState{}
		t.fileStates[path] = state
	}
	currentOffset := state.Offset
	t.mu.Unlock()

	// 从上次偏移位置继续读取
	if _, err := f.Seek(currentOffset, 0); err != nil {
		return false
	}

	scanner := bufio.NewScanner(f)
	// JSONL 行可能很长，设置足够大的缓冲区
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	hasNew := false
	newOffset := currentOffset

	lineCount := 0
	usageCount := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		newOffset += int64(len(line)) + 1 // +1 for newline
		lineCount++

		var record jsonlRecord
		if err := json.Unmarshal(line, &record); err != nil {
			// JSON 容错：跳过解析失败的行
			continue
		}

		// 只处理 assistant 消息中的 usage 字段
		// usage 在 message.usage 中
		if record.Message.Role != "assistant" || record.Message.Usage == nil {
			continue
		}

		usage := record.Message.Usage
		t.mu.Lock()
		state.Usage.InputTokens += usage.InputTokens
		state.Usage.OutputTokens += usage.OutputTokens
		state.Usage.CacheCreationInputTokens += usage.CacheCreationInputTokens
		state.Usage.CacheReadInputTokens += usage.CacheReadInputTokens
		t.mu.Unlock()

		usageCount++
		hasNew = true
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[token] 扫描文件出错 %s: %v (read %d lines)", path, err, lineCount)
	}

	if lineCount > 0 {
		log.Printf("[token] 扫描 %s: %d 行, %d 条 usage, offset %d->%d", filepath.Base(path), lineCount, usageCount, currentOffset, newOffset)
	}

	t.mu.Lock()
	state.Offset = newOffset
	t.mu.Unlock()

	return hasNew
}

// buildReport 构建当前 token 统计报告（调用方需持有读锁）
func (t *Tracker) buildReport() model.TokenReport {
	members, _ := t.discoverJSONLFiles()

	report := model.TokenReport{}
	for _, m := range members {
		state, exists := t.fileStates[m.Path]
		if !exists {
			continue
		}

		cost := estimateCost(m.Model, state.Usage)
		memberUsage := model.MemberTokenUsage{
			Name:                     m.Name,
			Model:                    m.Model,
			InputTokens:              state.Usage.InputTokens,
			OutputTokens:             state.Usage.OutputTokens,
			CacheCreationInputTokens: state.Usage.CacheCreationInputTokens,
			CacheReadInputTokens:     state.Usage.CacheReadInputTokens,
			EstimatedCostUSD:         cost,
			JSONLPath:                m.Path,
		}
		report.Members = append(report.Members, memberUsage)

		// 累加团队总计
		report.TeamTotal.InputTokens += state.Usage.InputTokens
		report.TeamTotal.OutputTokens += state.Usage.OutputTokens
		report.TeamTotal.CacheCreationInputTokens += state.Usage.CacheCreationInputTokens
		report.TeamTotal.CacheReadInputTokens += state.Usage.CacheReadInputTokens
		report.TeamTotal.EstimatedCostUSD += cost
	}

	return report
}

// estimateCost 根据模型和 token 用量估算费用（美元）
func estimateCost(modelName string, usage model.TokenUsage) float64 {
	pricing, ok := modelPricing[modelName]
	if !ok {
		// 未知模型使用 sonnet 价格作为默认
		pricing = modelPricing["claude-sonnet-4-6"]
	}

	// 常规输入 token 费用
	inputCost := float64(usage.InputTokens) / 1_000_000 * pricing.InputPerM
	// 输出 token 费用
	outputCost := float64(usage.OutputTokens) / 1_000_000 * pricing.OutputPerM
	// Cache 创建 token 按输入价格计算
	cacheCreateCost := float64(usage.CacheCreationInputTokens) / 1_000_000 * pricing.InputPerM
	// Cache 读取 token 成本为输入的 10%
	cacheReadCost := float64(usage.CacheReadInputTokens) / 1_000_000 * pricing.InputPerM * cacheReadCostRatio

	return inputCost + outputCost + cacheCreateCost + cacheReadCost
}
