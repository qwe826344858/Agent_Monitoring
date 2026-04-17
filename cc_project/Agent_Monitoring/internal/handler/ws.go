package handler

import (
	"log"
	"net/http"
	"sync"
	"time"

	"agent-monitor/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源连接（本地监控工具）
	},
}

// WSHub WebSocket 连接池管理，按团队名称分组
type WSHub struct {
	// teamName -> 该团队的所有 WebSocket 连接
	clients map[string]map[*websocket.Conn]struct{}
	mu      sync.RWMutex
}

// NewWSHub 创建 WebSocket 连接池
func NewWSHub() *WSHub {
	return &WSHub{
		clients: make(map[string]map[*websocket.Conn]struct{}),
	}
}

// HandleWS 处理 WebSocket 连接请求 /ws/:name
func (hub *WSHub) HandleWS(c *gin.Context) {
	teamName := c.Param("name")

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[ws] 升级连接失败: %v", err)
		return
	}

	hub.addClient(teamName, conn)
	log.Printf("[ws] 新连接: team=%s, addr=%s", teamName, conn.RemoteAddr())

	// 读循环：保持连接并检测断开
	// 客户端不需要发送数据，只接收推送
	defer func() {
		hub.removeClient(teamName, conn)
		conn.Close()
		log.Printf("[ws] 连接断开: team=%s, addr=%s", teamName, conn.RemoteAddr())
	}()

	// 设置 pong handler 以维持心跳
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// 启动定时 ping
	go hub.pingLoop(conn)

	for {
		// 阻塞读取，直到连接关闭
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// Broadcast 向指定团队的所有 WebSocket 连接推送事件
func (hub *WSHub) Broadcast(teamName string, event model.WSEvent) {
	hub.mu.RLock()
	clients := hub.clients[teamName]
	hub.mu.RUnlock()

	if len(clients) == 0 {
		return
	}

	// 逐个连接发送，失败则标记清理
	var failed []*websocket.Conn
	for conn := range clients {
		if err := conn.WriteJSON(event); err != nil {
			log.Printf("[ws] 推送失败: %v", err)
			failed = append(failed, conn)
		}
	}

	// 清理失败连接
	for _, conn := range failed {
		hub.removeClient(teamName, conn)
		conn.Close()
	}
}

// addClient 将连接加入指定团队的连接池
func (hub *WSHub) addClient(teamName string, conn *websocket.Conn) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	if hub.clients[teamName] == nil {
		hub.clients[teamName] = make(map[*websocket.Conn]struct{})
	}
	hub.clients[teamName][conn] = struct{}{}
}

// removeClient 从连接池中移除连接
func (hub *WSHub) removeClient(teamName string, conn *websocket.Conn) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	if clients, ok := hub.clients[teamName]; ok {
		delete(clients, conn)
		if len(clients) == 0 {
			delete(hub.clients, teamName)
		}
	}
}

// pingLoop 定时发送 ping 帧以保持连接活跃
func (hub *WSHub) pingLoop(conn *websocket.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if err := conn.WriteControl(
			websocket.PingMessage, nil, time.Now().Add(5*time.Second),
		); err != nil {
			return
		}
	}
}
