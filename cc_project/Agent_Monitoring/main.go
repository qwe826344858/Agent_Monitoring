package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"agent-monitor/internal/handler"
	"agent-monitor/internal/model"
	"agent-monitor/internal/token"
	"agent-monitor/internal/watcher"

	"github.com/gin-gonic/gin"
)

//go:embed static
var staticFS embed.FS

// teamManager 管理所有团队的 watcher 和 tracker，支持动态注册新团队
type teamManager struct {
	teamsDir    string
	tasksDir    string
	projectsDir string
	wsHub       *handler.WSHub

	mu       sync.RWMutex
	trackers map[string]*token.Tracker
	watchers map[string]*watcher.Watcher
}

func newTeamManager(teamsDir, tasksDir, projectsDir string, wsHub *handler.WSHub) *teamManager {
	return &teamManager{
		teamsDir:    teamsDir,
		tasksDir:    tasksDir,
		projectsDir: projectsDir,
		wsHub:       wsHub,
		trackers:    make(map[string]*token.Tracker),
		watchers:    make(map[string]*watcher.Watcher),
	}
}

// registerTeam 为一个团队注册 watcher 和 tracker，如果已注册则跳过
func (tm *teamManager) registerTeam(teamName string) {
	tm.mu.Lock()
	if _, exists := tm.trackers[teamName]; exists {
		tm.mu.Unlock()
		return
	}

	// 创建 tracker
	tracker := token.NewTracker(tm.projectsDir, tm.teamsDir, teamName)
	tracker.OnUpdate(func(report model.TokenReport) {
		tm.wsHub.Broadcast(teamName, model.WSEvent{
			Event:     "token_update",
			Timestamp: time.Now(),
			Data:      report,
		})
	})
	tm.trackers[teamName] = tracker

	// 创建 watcher
	w := watcher.New(tm.teamsDir, tm.tasksDir, teamName)
	w.InitInboxCounts()
	tm.watchers[teamName] = w

	tm.mu.Unlock()

	// 启动 tracker 和 watcher（不持锁）
	go tracker.Start()

	go func() {
		go func() {
			for evt := range w.Events() {
				tm.wsHub.Broadcast(teamName, model.WSEvent{
					Event:     string(evt.Type),
					Timestamp: evt.Timestamp,
					Data:      evt.Data,
				})
			}
		}()
		if err := w.Start(); err != nil {
			log.Printf("[main] watcher 启动失败 (team=%s): %v", teamName, err)
		}
	}()

	log.Printf("[main] 已启动团队 %s 的监听和 token 追踪", teamName)
}

// scanAndRegister 扫描 teams 目录，注册所有新发现的团队
func (tm *teamManager) scanAndRegister() {
	entries, err := os.ReadDir(tm.teamsDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			tm.registerTeam(entry.Name())
		}
	}
}

// startAutoScan 每隔 interval 扫描一次新团队
func (tm *teamManager) startAutoScan(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			tm.scanAndRegister()
		}
	}()
}

// getTrackers 返回当前所有 tracker 的快照（线程安全）
func (tm *teamManager) getTrackers() map[string]*token.Tracker {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	result := make(map[string]*token.Tracker, len(tm.trackers))
	for k, v := range tm.trackers {
		result[k] = v
	}
	return result
}

func main() {
	port := flag.Int("port", 8080, "HTTP server port")
	flag.Parse()

	// 支持通过环境变量自定义 .claude 目录（用于 Docker 环境）
	claudeDir := os.Getenv("CLAUDE_DIR")
	if claudeDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("无法获取用户主目录: %v", err)
		}
		claudeDir = filepath.Join(homeDir, ".claude")
	}

	teamsDir := filepath.Join(claudeDir, "teams")
	tasksDir := filepath.Join(claudeDir, "tasks")
	projectsDir := filepath.Join(claudeDir, "projects")

	// 创建 WebSocket 连接池
	wsHub := handler.NewWSHub()

	// 创建团队管理器并执行首次扫描
	tm := newTeamManager(teamsDir, tasksDir, projectsDir, wsHub)
	tm.scanAndRegister()

	// 每 3 秒扫描一次新团队
	tm.startAutoScan(3 * time.Second)

	// 配置 Gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// 注册 REST API 路由
	// APIHandler 使用 teamManager 的动态 tracker 列表
	apiHandler := &handler.APIHandler{
		TeamsDir:    teamsDir,
		TasksDir:    tasksDir,
		GetTrackers: tm.getTrackers,
	}
	apiHandler.RegisterRoutes(r)

	// 注册 WebSocket 路由
	r.GET("/ws/:name", func(c *gin.Context) {
		// WebSocket 连接时确保该团队已注册
		teamName := c.Param("name")
		tm.registerTeam(teamName)
		wsHub.HandleWS(c)
	})

	// 托管前端静态文件
	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatalf("无法加载静态文件: %v", err)
	}
	r.StaticFS("/static", http.FS(staticSub))

	// 根路径返回 index.html
	r.GET("/", func(c *gin.Context) {
		data, err := staticFS.ReadFile("static/index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "failed to load index.html")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("[main] Agent Monitor 启动在 http://localhost%s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
