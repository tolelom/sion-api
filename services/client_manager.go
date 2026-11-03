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

// clientEntry는 연결 하나의 쓰기를 직렬화한다.
//
// gofiber/websocket은 핸들러가 끝나면 *websocket.Conn을 풀에 돌려보내 내부 연결을 nil로 비우고,
// 다음 접속에 같은 포인터를 다시 쓴다. 그래서 핸들러 밖의 고루틴(브로드캐스트)은 conn에 손대기 전에
// 반드시 writeMu를 잡고 closed를 확인한다. 핸들러는 closed가 세워진 것을 확인한 뒤에야 끝나므로
// (Unregister가 직접 세우거나, 이미 drop이 세우고 목록에서 뺐거나), 그 뒤로는 어떤 고루틴도 conn을 건드리지 않는다.
type clientEntry struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
	closed  bool
}

// close는 연결을 닫고 이후 쓰기를 막는다. 여러 번 불러도 된다.
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

func NewClientManager() *ClientManager {
	return &ClientManager{
		clients:      make(map[*websocket.Conn]*clientEntry),
		writeTimeout: 5 * time.Second,
	}
}

// SetWriteTimeout은 연결을 받기 전에만 불러야 한다.
func (m *ClientManager) SetWriteTimeout(d time.Duration) {
	m.writeTimeout = d
}

func (m *ClientManager) Register(conn *websocket.Conn) {
	m.mutex.Lock()
	m.clients[conn] = &clientEntry{conn: conn}
	m.mutex.Unlock()
	log.Printf("[INFO] 클라이언트 등록: %s", conn.RemoteAddr())
}

// Unregister는 conn을 목록에서 빼고 닫는다. 핸들러가 끝나기 전에 핸들러 고루틴에서 불러야 한다.
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

func (m *ClientManager) safeWrite(entry *clientEntry, data []byte) error {
	entry.writeMu.Lock()
	defer entry.writeMu.Unlock()
	if entry.closed {
		return errClientClosed
	}
	if err := entry.conn.SetWriteDeadline(time.Now().Add(m.writeTimeout)); err != nil {
		return err
	}
	return entry.conn.WriteMessage(websocket.TextMessage, data)
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
	return m.safeWrite(entry, data)
}

func (m *ClientManager) WriteControl(conn *websocket.Conn, messageType int, data []byte, deadline time.Time) error {
	entry, ok := m.entry(conn)
	if !ok {
		return nil
	}
	entry.writeMu.Lock()
	defer entry.writeMu.Unlock()
	if entry.closed {
		return errClientClosed
	}
	return entry.conn.WriteControl(messageType, data, deadline)
}

func (m *ClientManager) BroadcastJSON(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("[ERROR] 브로드캐스트 marshal 실패: %v", err)
		return
	}
	m.BroadcastToWeb(data)
}

// BroadcastToWeb은 등록된 모든 클라이언트에 data를 보낸다. 쓰기에 실패한 연결은 닫고 목록에서 뺀다.
func (m *ClientManager) BroadcastToWeb(data []byte) {
	for _, entry := range m.snapshotClients() {
		err := m.safeWrite(entry, data)
		if err == nil || errors.Is(err, errClientClosed) {
			continue
		}
		log.Printf("[WARN] 브로드캐스트 전송 실패: %v", err)
		m.drop(entry)
	}
}

// drop은 entry를 닫고, 목록이 아직 그 entry를 가리킬 때만 뺀다.
// conn 키만 보고 지우면 풀에서 재사용된 같은 포인터의 새 클라이언트를 지울 수 있다.
// closed를 먼저 세우므로, 목록에서 빠진 것을 본 핸들러의 Unregister가 곧바로 끝나도 안전하다.
func (m *ClientManager) drop(entry *clientEntry) {
	entry.close()
	m.mutex.Lock()
	if m.clients[entry.conn] == entry {
		delete(m.clients, entry.conn)
	}
	m.mutex.Unlock()
}
