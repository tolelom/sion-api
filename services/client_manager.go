package services

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gofiber/websocket/v2"
)

type clientEntry struct {
	writeMu sync.Mutex
}

type ClientManager struct {
	clients      map[*websocket.Conn]*clientEntry
	mutex        sync.RWMutex
	writeTimeout time.Duration
}

func NewClientManager() *ClientManager {
	return &ClientManager{
		clients:      make(map[*websocket.Conn]*clientEntry),
		writeTimeout: 5 * time.Second,
	}
}

func (m *ClientManager) SetWriteTimeout(d time.Duration) {
	m.writeTimeout = d
}

func (m *ClientManager) Register(conn *websocket.Conn) {
	m.mutex.Lock()
	m.clients[conn] = &clientEntry{}
	m.mutex.Unlock()
	log.Printf("[INFO] 클라이언트 등록: %s", conn.RemoteAddr())
}

func (m *ClientManager) Unregister(conn *websocket.Conn) {
	m.mutex.Lock()
	_, ok := m.clients[conn]
	if ok {
		delete(m.clients, conn)
	}
	m.mutex.Unlock()
	if !ok {
		return
	}
	_ = conn.Close()
	log.Printf("[INFO] 클라이언트 해제: %s", conn.RemoteAddr())
}

func (m *ClientManager) Count() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.clients)
}

func (m *ClientManager) safeWrite(conn *websocket.Conn, entry *clientEntry, data []byte) error {
	entry.writeMu.Lock()
	defer entry.writeMu.Unlock()
	if err := conn.SetWriteDeadline(time.Now().Add(m.writeTimeout)); err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}

type clientTarget struct {
	conn  *websocket.Conn
	entry *clientEntry
}

func (m *ClientManager) snapshotClients() []clientTarget {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	out := make([]clientTarget, 0, len(m.clients))
	for c, e := range m.clients {
		out = append(out, clientTarget{c, e})
	}
	return out
}

func (m *ClientManager) WriteJSON(conn *websocket.Conn, v any) error {
	m.mutex.RLock()
	entry, ok := m.clients[conn]
	m.mutex.RUnlock()
	if !ok {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return m.safeWrite(conn, entry, data)
}

func (m *ClientManager) WriteControl(conn *websocket.Conn, messageType int, data []byte, deadline time.Time) error {
	m.mutex.RLock()
	entry, ok := m.clients[conn]
	m.mutex.RUnlock()
	if !ok {
		return nil
	}
	entry.writeMu.Lock()
	defer entry.writeMu.Unlock()
	return conn.WriteControl(messageType, data, deadline)
}

func (m *ClientManager) BroadcastJSON(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("[ERROR] 브로드캐스트 marshal 실패: %v", err)
		return
	}
	m.BroadcastToWeb(data)
}

func (m *ClientManager) BroadcastToWeb(data []byte) {
	var failed []*websocket.Conn
	for _, t := range m.snapshotClients() {
		if err := m.safeWrite(t.conn, t.entry, data); err != nil {
			log.Printf("[WARN] 브로드캐스트 전송 실패: %v", err)
			failed = append(failed, t.conn)
		}
	}
	for _, conn := range failed {
		m.Unregister(conn)
	}
}
