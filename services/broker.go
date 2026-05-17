package services

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"sion-backend/models"
)

type Broker struct {
	cm           *ClientManager
	agvStatus    *models.AGVStatus
	agvConnected bool
	mu           sync.RWMutex
}

func NewBroker(cm *ClientManager) *Broker {
	return &Broker{cm: cm}
}

// GetAGVStatus는 현재 AGV 상태의 사본을 반환한다.
// ok=false는 아직 status 메시지를 한 번도 받지 못한 초기 상태.
// 포인터를 반환하던 이전 API는 호출자가 락 밖에서 동일 객체를 공유하는
// 동안 setAGVStatus가 in-place 수정으로 바뀔 경우 race를 만들 수 있었다.
// value-copy는 그 약한 invariant를 제거한다.
func (b *Broker) GetAGVStatus() (models.AGVStatus, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.agvStatus == nil {
		return models.AGVStatus{}, false
	}
	return *b.agvStatus, true
}

func (b *Broker) setAGVStatus(status *models.AGVStatus) {
	b.mu.Lock()
	b.agvStatus = status
	b.mu.Unlock()
}

func (b *Broker) OnAGVMessage(msg models.WebSocketMessage, rawBytes []byte) {
	switch msg.Type {
	case models.MessageTypeStatus:
		// raw에서 한 번에 strongly-typed unmarshal한다.
		// 이전 패턴은 msg.Data(interface{}) → marshal → AGVStatus unmarshal로
		// 같은 데이터를 세 번 직렬화/역직렬화했다.
		var env struct {
			Data models.AGVStatus `json:"data"`
		}
		if err := json.Unmarshal(rawBytes, &env); err != nil {
			log.Printf("[WARN] status 파싱 실패: %v", err)
			break
		}
		b.setAGVStatus(&env.Data)
	}

	b.cm.BroadcastToWeb(rawBytes)
}

func (b *Broker) OnWebMessage(msg models.WebSocketMessage) {
	raw, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[ERROR] Broker.OnWebMessage marshal 실패: %v", err)
		return
	}
	b.cm.WriteToAGV(raw)
}

func (b *Broker) BroadcastToWeb(msg models.WebSocketMessage) {
	raw, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[ERROR] Broker.BroadcastToWeb marshal 실패: %v", err)
		return
	}
	b.cm.BroadcastToWeb(raw)
}

func (b *Broker) SetAGVConnected(connected bool) {
	b.mu.Lock()
	if b.agvConnected == connected {
		b.mu.Unlock()
		return
	}
	b.agvConnected = connected
	b.mu.Unlock()

	msgType := models.MessageTypeAGVConnected
	if !connected {
		msgType = models.MessageTypeAGVDisconnected
	}

	msg := models.NewMessage(msgType, map[string]interface{}{"connected": connected}, time.Now().UnixMilli())
	raw, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[ERROR] SetAGVConnected marshal 실패: %v", err)
		return
	}
	b.cm.BroadcastToWeb(raw)
	log.Printf("[INFO] AGV 연결 상태 변경: %v", connected)
}

func (b *Broker) IsAGVConnected() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.agvConnected
}
