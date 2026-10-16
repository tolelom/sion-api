package services

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gofiber/websocket/v2"
)

type ClientType string

const (
	AGVClient ClientType = "agv"
	WebClient ClientType = "web"
)

// clientEntry는 각 conn마다 write 직렬화를 위한 mutex를 함께 보관한다.
// websocket conn은 동시 WriteMessage 호출이 안전하지 않으므로 conn당 단일 writer를 보장해야 한다.
type clientEntry struct {
	ct      ClientType
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

func (m *ClientManager) Register(conn *websocket.Conn, ct ClientType) {
	m.mutex.Lock()
	m.clients[conn] = &clientEntry{ct: ct}
	m.mutex.Unlock()
	log.Printf("[INFO] 클라이언트 등록: %s (%s)", ct, conn.RemoteAddr())
}

func (m *ClientManager) Unregister(conn *websocket.Conn) {
	m.mutex.Lock()
	entry, ok := m.clients[conn]
	if ok {
		delete(m.clients, conn)
	}
	m.mutex.Unlock()
	if !ok {
		return
	}
	_ = conn.Close()
	log.Printf("[INFO] 클라이언트 해제: %s (%s)", entry.ct, conn.RemoteAddr())
}

func (m *ClientManager) GetClientCount() map[string]int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	count := map[string]int{"agv": 0, "web": 0}
	for _, e := range m.clients {
		count[string(e.ct)]++
	}
	return count
}

// safeWrite는 conn별 mutex를 잡고 메시지를 전송한다. 호출자가 락을 직접 다루지 않게 한다.
func (m *ClientManager) safeWrite(conn *websocket.Conn, entry *clientEntry, data []byte) error {
	entry.writeMu.Lock()
	defer entry.writeMu.Unlock()
	if err := conn.SetWriteDeadline(time.Now().Add(m.writeTimeout)); err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}

// snapshotClients는 RLock 안에서 (conn, entry) 쌍을 복사해 반환한다.
// write는 RLock 밖에서 conn별 mutex를 잡고 수행해 RWMutex와 conn write의 호출 순서를 분리한다.
func (m *ClientManager) snapshotClients(filter ClientType) []struct {
	conn  *websocket.Conn
	entry *clientEntry
} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	out := make([]struct {
		conn  *websocket.Conn
		entry *clientEntry
	}, 0, len(m.clients))
	for c, e := range m.clients {
		if e.ct == filter {
			out = append(out, struct {
				conn  *websocket.Conn
				entry *clientEntry
			}{c, e})
		}
	}
	return out
}

// WriteJSON은 특정 conn에 JSON 페이로드를 직렬화해 안전하게 전송한다.
// (conn별 mutex로 동시 쓰기를 직렬화)
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

// WriteControl은 ping/pong 등 컨트롤 프레임을 conn별 writeMu 안에서 전송한다.
// 일반 WriteMessage와 직렬화돼야 fasthttp/websocket의 동시 write 깨짐을 방지할 수 있다.
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

func (m *ClientManager) BroadcastToWeb(data []byte) {
	targets := m.snapshotClients(WebClient)
	var failed []*websocket.Conn
	for _, t := range targets {
		if err := m.safeWrite(t.conn, t.entry, data); err != nil {
			log.Printf("[ERROR] BroadcastToWeb 전송 실패: %v", err)
			failed = append(failed, t.conn)
		}
	}
	for _, conn := range failed {
		m.Unregister(conn)
	}
}

func (m *ClientManager) WriteToAGV(data []byte) {
	targets := m.snapshotClients(AGVClient)
	if len(targets) == 0 {
		log.Println("[WARN] WriteToAGV: AGV 연결 없음")
		return
	}
	t := targets[0]
	if err := m.safeWrite(t.conn, t.entry, data); err != nil {
		log.Printf("[ERROR] WriteToAGV 전송 실패: %v", err)
		m.Unregister(t.conn)
	}
}
