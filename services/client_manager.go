package services

import (
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/gofiber/websocket/v2"
)

var errClientClosed = errors.New("클라이언트 연결이 이미 닫혔습니다")

// gofiber/websocket은 핸들러가 끝나면 *websocket.Conn을 풀에 돌려보내 재사용한다.
// 그래서 핸들러 밖의 쓰기는 withOpenConn으로만 하고, 핸들러는 closed가 세워진 뒤에야 반환한다.
type clientEntry struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
	closed  bool
}

func (e *clientEntry) withOpenConn(write func(*websocket.Conn) error) error {
	e.writeMu.Lock()
	defer e.writeMu.Unlock()
	if e.closed {
		return errClientClosed
	}
	return write(e.conn)
}

func (e *clientEntry) close() {
	e.writeMu.Lock()
	defer e.writeMu.Unlock()
	if e.closed {
		return
	}
	e.closed = true
	_ = e.conn.Close()
}

type ClientManager struct {
	clients      map[*websocket.Conn]*clientEntry
	mutex        sync.RWMutex
	writeTimeout time.Duration
}

const DefaultWriteTimeout = 5 * time.Second

func NewClientManager(writeTimeout time.Duration) *ClientManager {
	return &ClientManager{
		clients:      make(map[*websocket.Conn]*clientEntry),
		writeTimeout: writeTimeout,
	}
}

func (m *ClientManager) Register(conn *websocket.Conn) {
	m.mutex.Lock()
	m.clients[conn] = &clientEntry{conn: conn}
	m.mutex.Unlock()
	log.Printf("[INFO] 클라이언트 등록: %s", conn.RemoteAddr())
}

// Unregister는 핸들러가 반환하기 전에 핸들러 고루틴에서 불러야 한다.
func (m *ClientManager) Unregister(conn *websocket.Conn) {
	m.mutex.Lock()
	entry, ok := m.clients[conn]
	delete(m.clients, conn)
	m.mutex.Unlock()
	if !ok {
		return
	}
	entry.close()
	log.Printf("[INFO] 클라이언트 해제: %s", conn.RemoteAddr())
}

func (m *ClientManager) Count() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.clients)
}

func (m *ClientManager) entry(conn *websocket.Conn) (*clientEntry, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	entry, ok := m.clients[conn]
	return entry, ok
}

func (m *ClientManager) snapshotClients() []*clientEntry {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	out := make([]*clientEntry, 0, len(m.clients))
	for _, e := range m.clients {
		out = append(out, e)
	}
	return out
}

func (m *ClientManager) writeText(entry *clientEntry, data []byte) error {
	return entry.withOpenConn(func(conn *websocket.Conn) error {
		if err := conn.SetWriteDeadline(time.Now().Add(m.writeTimeout)); err != nil {
			return err
		}
		return conn.WriteMessage(websocket.TextMessage, data)
	})
}

func (m *ClientManager) WriteJSON(conn *websocket.Conn, v any) error {
	entry, ok := m.entry(conn)
	if !ok {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return m.writeText(entry, data)
}

func (m *ClientManager) WriteControl(conn *websocket.Conn, messageType int, data []byte, deadline time.Time) error {
	entry, ok := m.entry(conn)
	if !ok {
		return nil
	}
	return entry.withOpenConn(func(conn *websocket.Conn) error {
		return conn.WriteControl(messageType, data, deadline)
	})
}

func (m *ClientManager) BroadcastJSON(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("[ERROR] 브로드캐스트 marshal 실패: %v", err)
		return
	}
	m.Broadcast(data)
}

func (m *ClientManager) Broadcast(data []byte) {
	for _, entry := range m.snapshotClients() {
		err := m.writeText(entry, data)
		if err == nil || errors.Is(err, errClientClosed) {
			continue
		}
		log.Printf("[WARN] 브로드캐스트 전송 실패: %v", err)
		m.drop(entry)
	}
}

// 먼저 닫아야 목록에서 빠진 것을 본 Unregister가 곧장 반환해도 안전하다.
// 같은 conn 키가 풀에서 재사용돼 새 클라이언트를 가리킬 수 있어, 같은 entry일 때만 뺀다.
func (m *ClientManager) drop(entry *clientEntry) {
	entry.close()
	m.mutex.Lock()
	if m.clients[entry.conn] == entry {
		delete(m.clients, entry.conn)
	}
	m.mutex.Unlock()
}
