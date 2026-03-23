package services

import (
	"encoding/json"
	"log"
	"sync"

	"sion-backend/models"
)

// Broker — 메시지 라우팅 + 현재 AGV 상태 보유
type Broker struct {
	cm        *ClientManager
	agvStatus *models.AGVStatus
	mu        sync.RWMutex
}

func NewBroker(cm *ClientManager) *Broker {
	return &Broker{cm: cm}
}

// GetAGVStatus — chat.go 등에서 현재 AGV 상태 조회용
func (b *Broker) GetAGVStatus() *models.AGVStatus {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.agvStatus
}

func (b *Broker) setAGVStatus(status *models.AGVStatus) {
	b.mu.Lock()
	b.agvStatus = status
	b.mu.Unlock()
}

// OnAGVMessage — AGV에서 수신한 메시지 처리: AGVStatus 갱신 + Web으로 브로드캐스트
func (b *Broker) OnAGVMessage(msg models.WebSocketMessage) {
	// AGV 상태 관련 메시지면 갱신
	switch msg.Type {
	case models.MessageTypeStatus:
		if data, ok := msg.Data.(map[string]interface{}); ok {
			status := &models.AGVStatus{}
			if battery, ok := data["battery"].(float64); ok {
				status.Battery = int(battery) // Battery는 int
			}
			if speed, ok := data["speed"].(float64); ok {
				status.Speed = speed
			}
			if mode, ok := data["mode"].(string); ok {
				status.Mode = mode // Mode는 plain string
			}
			if state, ok := data["state"].(string); ok {
				status.State = state // State는 plain string
			}
			b.setAGVStatus(status)
		}
	}

	// Web 클라이언트들에게 브로드캐스트
	raw, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Broker.OnAGVMessage marshal 실패: %v", err)
		return
	}
	b.cm.BroadcastToWeb(raw)
}

// OnWebMessage — Web에서 수신한 명령 처리: AGV로 전달
func (b *Broker) OnWebMessage(msg models.WebSocketMessage) {
	raw, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Broker.OnWebMessage marshal 실패: %v", err)
		return
	}
	b.cm.WriteToAGV(raw)
}

// BroadcastToWeb — 서비스(LLM 응답 등)에서 직접 Web으로 전송
func (b *Broker) BroadcastToWeb(msg models.WebSocketMessage) {
	raw, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Broker.BroadcastToWeb marshal 실패: %v", err)
		return
	}
	b.cm.BroadcastToWeb(raw)
}
