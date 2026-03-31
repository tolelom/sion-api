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

func (b *Broker) OnAGVMessage(msg models.WebSocketMessage, rawBytes []byte) {
	switch msg.Type {
	case models.MessageTypeStatus:
		dataRaw, err := json.Marshal(msg.Data)
		if err != nil {
			log.Printf("WARN: status marshal 실패: %v", err)
			break
		}
		var status models.AGVStatus
		if err := json.Unmarshal(dataRaw, &status); err != nil {
			log.Printf("WARN: status 파싱 실패: %v", err)
			break
		}
		b.setAGVStatus(&status)
	}

	b.cm.BroadcastToWeb(rawBytes)
}

func (b *Broker) OnWebMessage(msg models.WebSocketMessage) {
	raw, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Broker.OnWebMessage marshal 실패: %v", err)
		return
	}
	b.cm.WriteToAGV(raw)
}

func (b *Broker) BroadcastToWeb(msg models.WebSocketMessage) {
	raw, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Broker.BroadcastToWeb marshal 실패: %v", err)
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

	raw, err := json.Marshal(models.WebSocketMessage{
		Type:      msgType,
		Data:      map[string]interface{}{"connected": connected},
		Timestamp: time.Now().UnixMilli(),
	})
	if err != nil {
		log.Printf("SetAGVConnected marshal 실패: %v", err)
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
