package watcher

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"agent-monitor/internal/model"

	"github.com/fsnotify/fsnotify"
)

// EventType 文件变化事件类型
type EventType string

const (
	EventMemberUpdate EventType = "member_update"
	EventNewMessage   EventType = "new_message"
	EventTaskUpdate   EventType = "task_update"
)

// FileEvent 文件监听产生的变化事件
type FileEvent struct {
	Type      EventType
	TeamName  string
	Timestamp time.Time
	Data      interface{}
}

// NewMessageData 新消息事件的数据
type NewMessageData struct {
	Inbox   string             `json:"inbox"`
	Message model.InboxMessage `json:"message"`
}

// Watcher 文件监听器，监控团队配置、收件箱和任务文件的变化
type Watcher struct {
	teamsDir string // ~/.claude/teams
	tasksDir string // ~/.claude/tasks
	teamName string

	events chan FileEvent
	done   chan struct{}

	// 消息去重：缓存每个 inbox 的上次消息数量
	inboxCounts map[string]int
	mu          sync.Mutex

	// 防抖计时器
	debounceTimers map[string]*time.Timer
	debounceMu     sync.Mutex
}

// New 创建新的文件监听器
// teamsDir: ~/.claude/teams 路径
// tasksDir: ~/.claude/tasks 路径
// teamName: 要监听的团队名称
func New(teamsDir, tasksDir, teamName string) *Watcher {
	return &Watcher{
		teamsDir:       teamsDir,
		tasksDir:       tasksDir,
		teamName:       teamName,
		events:         make(chan FileEvent, 100),
		done:           make(chan struct{}),
		inboxCounts:    make(map[string]int),
		debounceTimers: make(map[string]*time.Timer),
	}
}

// Events 返回事件通道，调用者从中读取变化事件
func (w *Watcher) Events() <-chan FileEvent {
	return w.events
}

// Start 启动文件监听，阻塞直到 Stop 被调用
func (w *Watcher) Start() error {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer fsw.Close()

	// 监听团队 config.json 所在目录
	teamDir := filepath.Join(w.teamsDir, w.teamName)
	if err := w.addWatchIfExists(fsw, teamDir); err != nil {
		log.Printf("[watcher] 无法监听团队目录 %s: %v", teamDir, err)
	}

	// 监听 inboxes 目录
	inboxDir := filepath.Join(w.teamsDir, w.teamName, "inboxes")
	if err := w.addWatchIfExists(fsw, inboxDir); err != nil {
		log.Printf("[watcher] 无法监听收件箱目录 %s: %v", inboxDir, err)
	}

	// 监听 tasks 目录
	taskDir := filepath.Join(w.tasksDir, w.teamName)
	if err := w.addWatchIfExists(fsw, taskDir); err != nil {
		log.Printf("[watcher] 无法监听任务目录 %s: %v", taskDir, err)
	}

	log.Printf("[watcher] 开始监听团队 %s 的文件变化", w.teamName)

	for {
		select {
		case event, ok := <-fsw.Events:
			if !ok {
				return nil
			}
			// 仅处理写入和创建事件
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				w.debounce(event.Name, func() {
					w.handleFileChange(event.Name)
				})
			}

		case err, ok := <-fsw.Errors:
			if !ok {
				return nil
			}
			log.Printf("[watcher] fsnotify 错误: %v", err)

		case <-w.done:
			return nil
		}
	}
}

// Stop 停止文件监听
func (w *Watcher) Stop() {
	close(w.done)
}

// debounce 200ms 防抖合并，相同文件路径在 200ms 内只触发一次处理
func (w *Watcher) debounce(path string, fn func()) {
	w.debounceMu.Lock()
	defer w.debounceMu.Unlock()

	if timer, exists := w.debounceTimers[path]; exists {
		timer.Stop()
	}
	w.debounceTimers[path] = time.AfterFunc(200*time.Millisecond, func() {
		fn()
		w.debounceMu.Lock()
		delete(w.debounceTimers, path)
		w.debounceMu.Unlock()
	})
}

// handleFileChange 根据文件路径判断变化类型并生成事件
func (w *Watcher) handleFileChange(path string) {
	teamDir := filepath.Join(w.teamsDir, w.teamName)
	taskDir := filepath.Join(w.tasksDir, w.teamName)
	inboxDir := filepath.Join(teamDir, "inboxes")

	switch {
	case path == filepath.Join(teamDir, "config.json"):
		w.handleConfigChange(path)

	case strings.HasPrefix(path, inboxDir) && strings.HasSuffix(path, ".json"):
		w.handleInboxChange(path)

	case strings.HasPrefix(path, taskDir) && strings.HasSuffix(path, ".json"):
		w.handleTaskChange(path)
	}
}

// handleConfigChange 处理 config.json 变化，推送 member_update 事件
func (w *Watcher) handleConfigChange(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[watcher] 读取 config.json 失败: %v", err)
		return
	}

	var config model.TeamConfig
	if err := json.Unmarshal(data, &config); err != nil {
		// JSON 容错：写入过程中可能读到不完整的 JSON
		log.Printf("[watcher] 解析 config.json 失败（可能正在写入）: %v", err)
		return
	}

	w.events <- FileEvent{
		Type:      EventMemberUpdate,
		TeamName:  w.teamName,
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"members": config.Members},
	}
}

// handleInboxChange 处理收件箱文件变化，仅推送增量消息
func (w *Watcher) handleInboxChange(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[watcher] 读取收件箱文件失败: %v", err)
		return
	}

	var messages []model.InboxMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		// JSON 容错
		log.Printf("[watcher] 解析收件箱文件失败（可能正在写入）: %v", err)
		return
	}

	// 从文件名提取 inbox 名称（去掉 .json 后缀）
	inbox := strings.TrimSuffix(filepath.Base(path), ".json")

	w.mu.Lock()
	prevCount := w.inboxCounts[inbox]
	currentCount := len(messages)
	w.inboxCounts[inbox] = currentCount
	w.mu.Unlock()

	// 只推送新增消息（消息去重）
	if currentCount > prevCount {
		for i := prevCount; i < currentCount; i++ {
			w.events <- FileEvent{
				Type:      EventNewMessage,
				TeamName:  w.teamName,
				Timestamp: time.Now(),
				Data: NewMessageData{
					Inbox:   inbox,
					Message: messages[i],
				},
			}
		}
	}
}

// handleTaskChange 处理任务文件变化
func (w *Watcher) handleTaskChange(path string) {
	// 忽略辅助文件（.lock, .highwatermark）
	base := filepath.Base(path)
	if !strings.HasSuffix(base, ".json") {
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[watcher] 读取任务文件失败: %v", err)
		return
	}

	var task model.Task
	if err := json.Unmarshal(data, &task); err != nil {
		// JSON 容错
		log.Printf("[watcher] 解析任务文件失败（可能正在写入）: %v", err)
		return
	}

	w.events <- FileEvent{
		Type:      EventTaskUpdate,
		TeamName:  w.teamName,
		Timestamp: time.Now(),
		Data:      task,
	}
}

// addWatchIfExists 如果目录存在则添加监听
func (w *Watcher) addWatchIfExists(fsw *fsnotify.Watcher, dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		log.Printf("[watcher] 目录不存在，跳过监听: %s", dir)
		return nil
	}
	return fsw.Add(dir)
}

// InitInboxCounts 初始化收件箱消息计数缓存，避免首次启动时推送历史消息
func (w *Watcher) InitInboxCounts() {
	inboxDir := filepath.Join(w.teamsDir, w.teamName, "inboxes")
	entries, err := os.ReadDir(inboxDir)
	if err != nil {
		log.Printf("[watcher] 读取收件箱目录失败: %v", err)
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

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
		inbox := strings.TrimSuffix(entry.Name(), ".json")
		w.inboxCounts[inbox] = len(messages)
	}
	log.Printf("[watcher] 初始化收件箱计数完成，共 %d 个收件箱", len(w.inboxCounts))
}
