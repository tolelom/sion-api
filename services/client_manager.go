package services

import (
	"log"
	"sync"

	"github.com/gofiber/websocket/v2"
)

type ClientType string

const (
	AGVClient ClientType = "agv"
	WebClient ClientType = "web"
)

type ClientManager struct {
	clients map[*websocket.Conn]ClientType
	mutex   sync.RWMutex
}

func NewClientManager() *ClientManager {
	return &ClientManager{
		clients: make(map[*websocket.Conn]ClientType),
	}
}

func (m *ClientManager) Register(conn *websocket.Conn, ct ClientType) {
	m.mutex.Lock()
	m.clients[conn] = ct
	m.mutex.Unlock()
	log.Printf("클라이언트 등록: %s (%s)", ct, conn.RemoteAddr())
}

func (m *ClientManager) Unregister(conn *websocket.Conn) {
	m.mutex.Lock()
	if ct, ok := m.clients[conn]; ok {
		delete(m.clients, conn)
		_ = conn.Close()
		log.Printf("클라이언트 해제: %s (%s)", ct, conn.RemoteAddr())
	}
	m.mutex.Unlock()
}

func (m *ClientManager) GetClientCount() map[string]int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	count := map[string]int{"agv": 0, "web": 0}
	for _, ct := range m.clients {
		count[string(ct)]++
	}
	return count
}

// BroadcastToWeb — 데드락 안전 패턴: RLock 해제 후 실패 연결 정리
func (m *ClientManager) BroadcastToWeb(data []byte) {
	m.mutex.RLock()
	var failed []*websocket.Conn
	for conn, ct := range m.clients {
		if ct == WebClient {
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("BroadcastToWeb 전송 실패: %v", err)
				failed = append(failed, conn)
			}
		}
	}
	m.mutex.RUnlock()

	if len(failed) > 0 {
		m.mutex.Lock()
		for _, conn := range failed {
			delete(m.clients, conn)
			_ = conn.Close()
		}
		m.mutex.Unlock()
	}
}

// WriteToAGV — 첫 번째 AGV 연결에만 전송 (AGV는 단일 연결 가정)
func (m *ClientManager) WriteToAGV(data []byte) {
	m.mutex.RLock()
	var agvConn *websocket.Conn
	for conn, ct := range m.clients {
		if ct == AGVClient {
			agvConn = conn
			break
		}
	}
	m.mutex.RUnlock()

	if agvConn == nil {
		log.Println("WriteToAGV: AGV 연결 없음")
		return
	}
	if err := agvConn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("WriteToAGV 전송 실패: %v", err)
		m.Unregister(agvConn)
	}
}
