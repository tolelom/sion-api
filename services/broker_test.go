package services

import (
	"encoding/json"
	"sync"
	"testing"

	"sion-backend/models"
)

// Broker는 ClientManager에 의존한다. ClientManager는 실제 websocket.Conn 없이도
// 등록된 conn이 0인 상태에서 BroadcastToWeb/WriteToAGV를 호출하는 데에는 문제가 없다.
// 따라서 broker의 "상태 관리" 동작은 빈 ClientManager로 충분히 테스트할 수 있다.
// (전송 성공 여부는 websocket_integration_test.go에서 실제 websocket으로 검증된다.)

func newTestBroker() *Broker {
	cm := NewClientManager()
	return NewBroker(cm)
}

func TestBroker_GetAGVStatus_InitiallyEmpty(t *testing.T) {
	b := newTestBroker()
	got, ok := b.GetAGVStatus()
	if ok {
		t.Fatalf("초기 상태에서 ok=false 기대, got=%+v ok=true", got)
	}
	// AGVStatus는 슬라이스 필드를 포함해 == 비교 불가. 핵심 필드만 zero인지 검사.
	if got.ID != "" || got.Connected || got.Battery != 0 {
		t.Fatalf("초기 상태는 zero value여야 함, got=%+v", got)
	}
}

func TestBroker_OnAGVMessage_StatusUpdatesInternalSnapshot(t *testing.T) {
	b := newTestBroker()

	status := models.AGVStatus{
		ID:        "agv-001",
		Name:      "Sion",
		Connected: true,
		Mode:      "auto",
		State:     "moving",
		Speed:     0.35,
		Battery:   88,
	}

	envelope := struct {
		Type      string           `json:"type"`
		Data      models.AGVStatus `json:"data"`
		Timestamp int64            `json:"timestamp"`
	}{
		Type:      models.MessageTypeStatus,
		Data:      status,
		Timestamp: 12345,
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("envelope marshal 실패: %v", err)
	}

	b.OnAGVMessage(models.WebSocketMessage{
		Type:      models.MessageTypeStatus,
		Data:      raw, // 사용되지 않지만 호환을 위해 채움
		Timestamp: 12345,
	}, raw)

	got, ok := b.GetAGVStatus()
	if !ok {
		t.Fatal("status 수신 후 ok=true 기대")
	}
	if got.ID != status.ID || got.Battery != status.Battery || got.Mode != status.Mode {
		t.Fatalf("상태 미반영: got=%+v want=%+v", got, status)
	}
}

func TestBroker_OnAGVMessage_NonStatusDoesNotChangeStatus(t *testing.T) {
	b := newTestBroker()
	raw := []byte(`{"type":"position","data":{"x":1,"y":2}}`)
	b.OnAGVMessage(models.WebSocketMessage{Type: models.MessageTypePosition}, raw)

	if _, ok := b.GetAGVStatus(); ok {
		t.Fatal("position 메시지로는 AGVStatus가 채워져선 안 됨")
	}
}

func TestBroker_OnAGVMessage_MalformedStatusKeepsPreviousOk(t *testing.T) {
	b := newTestBroker()
	bad := []byte(`{"type":"status","data":not-json`)
	b.OnAGVMessage(models.WebSocketMessage{Type: models.MessageTypeStatus}, bad)
	if _, ok := b.GetAGVStatus(); ok {
		t.Fatal("파싱 실패시 상태가 채워져선 안 됨")
	}
}

func TestBroker_GetAGVStatus_ReturnsCopySnapshot(t *testing.T) {
	b := newTestBroker()
	status := models.AGVStatus{ID: "a", Battery: 50}
	raw, _ := json.Marshal(struct {
		Data models.AGVStatus `json:"data"`
	}{Data: status})
	b.OnAGVMessage(models.WebSocketMessage{Type: models.MessageTypeStatus}, raw)

	first, ok := b.GetAGVStatus()
	if !ok {
		t.Fatal("ok 기대")
	}
	// 반환된 사본을 수정해도 내부 상태에 영향이 없어야 한다.
	first.Battery = 1
	second, _ := b.GetAGVStatus()
	if second.Battery != 50 {
		t.Fatalf("내부 상태가 호출자 수정에 영향받음: %d", second.Battery)
	}
}

func TestBroker_IsAGVConnected_DefaultsFalse(t *testing.T) {
	b := newTestBroker()
	if b.IsAGVConnected() {
		t.Fatal("초기에는 IsAGVConnected=false 기대")
	}
}

func TestBroker_SetAGVConnected_TogglesFlag(t *testing.T) {
	b := newTestBroker()
	b.SetAGVConnected(true)
	if !b.IsAGVConnected() {
		t.Fatal("true 설정 후 IsAGVConnected=true 기대")
	}
	b.SetAGVConnected(false)
	if b.IsAGVConnected() {
		t.Fatal("false 설정 후 IsAGVConnected=false 기대")
	}
}

func TestBroker_SetAGVConnected_RepeatedSameValueIsNoOp(t *testing.T) {
	// 동일 값으로 여러 번 호출되어도 panic이나 데드락 없이 멱등해야 한다.
	b := newTestBroker()
	for i := 0; i < 5; i++ {
		b.SetAGVConnected(true)
	}
	if !b.IsAGVConnected() {
		t.Fatal("반복 true 호출 후에도 true 유지")
	}
}

func TestBroker_OnWebMessage_AGVNotConnectedDoesNotPanic(t *testing.T) {
	// AGV가 없을 때 OnWebMessage는 "WriteToAGV: AGV 연결 없음"만 로그하고 조용히 반환해야 한다.
	b := newTestBroker()
	msg := models.NewMessage(models.MessageTypeCommand, models.MoveCommand{TargetX: 1, TargetY: 2}, 0)
	b.OnWebMessage(msg) // 패닉 없으면 통과
}

func TestBroker_BroadcastToWeb_NoWebClientsDoesNotPanic(t *testing.T) {
	b := newTestBroker()
	msg := models.NewMessage(models.MessageTypeSystemInfo, models.SystemInfo{AGVConnected: false}, 0)
	b.BroadcastToWeb(msg) // 패닉 없으면 통과
}

func TestBroker_ConcurrentSetAGVConnected_Safe(t *testing.T) {
	// 다수 goroutine이 동시에 SetAGVConnected를 호출해도 race나 panic이 나면 안 된다.
	// (-race 플래그와 함께 돌면 더 강한 검증이 된다.)
	b := newTestBroker()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			b.SetAGVConnected(v%2 == 0)
		}(i)
	}
	wg.Wait()
	// 종료 후 IsAGVConnected는 결정적이지 않지만, 호출은 가능해야 한다.
	_ = b.IsAGVConnected()
}
