package services

import (
	"sion-backend/models"
	"sync"
	"sync/atomic"
	"time"
)

const (
	MapSize        = 20
	startX         = 2.0
	startY         = 2.0
	maxPathPoints  = 400
	obstacleCount  = 25
	enemyCount     = 5
	detectionRange = 10.0
	attackRange    = 2.0

	agvID   = "sion-001"
	agvName = "사이온"

	defaultUpdateInterval = 500 * time.Millisecond
	// stepPerSpeed는 속도 1일 때 한 틱에 움직이는 거리다. 기본 틱 0.5초에 맞춘 값이다.
	stepPerSpeed = 0.5
	cruiseSpeed  = 1.0 // 수색·경로 추종 속도
	chargeSpeed  = 2.5 // 표적을 향해 돌진하는 속도
	// turnChance는 수색 중 매 틱 방향을 무작위로 바꿀 확률이다.
	turnChance = 0.1

	attackChance = 0.2
	attackDamage = 10
	minEnemyHP   = 20
	maxEnemyHP   = 100

	fullBattery       = 100
	batteryDrainTicks = 5 // 움직이는 동안 이 틱마다 배터리 1 감소
)

var enemyNames = []string{"아리", "야스오", "지글스", "룩스", "제드"}

// AGVSimulator는 공개 데모에서 실제 AGV 대신 움직이는 가상 AGV다.
// 틱마다 상태를 한 걸음 진행하고(step) 상태·위치 메시지를 브로드캐스트한다.
//
// 파일 구성: simulator.go(수명주기·틱·메시지), simulator_commands.go(WS 명령),
// simulator_behavior.go(이동·전투·배터리), simulator_world.go(장애물·적 생성).
//
// 이름이 Locked로 끝나는 메서드는 호출자가 mu를 쥐고 있어야 한다.
// 브로드캐스트는 느린 클라이언트가 명령 처리를 막지 않도록 mu 밖에서 보낸다.
type AGVSimulator struct {
	mu        sync.RWMutex
	status    *models.AGVStatus
	enemies   []models.Enemy
	obstacles []models.Cell
	path      []models.Waypoint
	tick      int

	updateInterval time.Duration
	broadcastFunc  func(models.WebSocketMessage)
	hasListeners   func() bool

	running  atomic.Bool
	lifeMu   sync.Mutex
	stopChan chan struct{}
	doneChan chan struct{}
}

func NewAGVSimulator(broadcast func(models.WebSocketMessage), hasListeners func() bool) *AGVSimulator {
	sim := &AGVSimulator{
		updateInterval: defaultUpdateInterval,
		broadcastFunc:  broadcast,
		hasListeners:   hasListeners,
	}
	sim.obstacles = generateObstacles(obstacleCount)
	sim.resetLocked()
	return sim
}

func initialStatus() *models.AGVStatus {
	return &models.AGVStatus{
		ID:       agvID,
		Name:     agvName,
		Position: models.PositionData{X: startX, Y: startY},
		Mode:     models.ModeAuto,
		State:    models.StateIdle,
		Battery:  fullBattery,
	}
}

func (sim *AGVSimulator) resetLocked() {
	sim.status = initialStatus()
	sim.enemies = generateEnemies(enemyCount, sim.obstacles)
	sim.path = nil
	sim.tick = 0
}

func (sim *AGVSimulator) IsRunning() bool {
	return sim.running.Load()
}

// Snapshot은 현재 AGV 상태와 적 목록의 복사본을 돌려준다.
func (sim *AGVSimulator) Snapshot() (models.AGVStatus, []models.Enemy) {
	sim.mu.RLock()
	defer sim.mu.RUnlock()
	enemies := make([]models.Enemy, len(sim.enemies))
	copy(enemies, sim.enemies)
	return *sim.status, enemies
}

func (sim *AGVSimulator) MapUpdateMessage() models.WebSocketMessage {
	sim.mu.RLock()
	obstacles := make([]models.Cell, len(sim.obstacles))
	copy(obstacles, sim.obstacles)
	sim.mu.RUnlock()
	return models.WebSocketMessage{
		Type: models.MessageTypeMapUpdate,
		Data: map[string]interface{}{
			"obstacles": obstacles,
			"width":     MapSize,
			"height":    MapSize,
		},
		Timestamp: time.Now().UnixMilli(),
	}
}

func (sim *AGVSimulator) Start() {
	sim.lifeMu.Lock()
	defer sim.lifeMu.Unlock()
	if sim.running.Load() {
		return
	}
	sim.stopChan = make(chan struct{})
	sim.doneChan = make(chan struct{})
	sim.running.Store(true)
	go sim.runSimulation(sim.stopChan, sim.doneChan)
}

func (sim *AGVSimulator) Stop() {
	sim.lifeMu.Lock()
	defer sim.lifeMu.Unlock()
	if !sim.running.Load() {
		return
	}
	close(sim.stopChan)
	<-sim.doneChan
	sim.running.Store(false)
}

func (sim *AGVSimulator) runSimulation(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(sim.updateInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			sim.update()
		case <-stop:
			return
		}
	}
}

func (sim *AGVSimulator) update() {
	if sim.hasListeners != nil && !sim.hasListeners() {
		return
	}
	statusMsg, positionMsg := sim.step()
	if sim.broadcastFunc != nil {
		sim.broadcastFunc(statusMsg)
		sim.broadcastFunc(positionMsg)
	}
}

func (sim *AGVSimulator) step() (statusMsg, positionMsg models.WebSocketMessage) {
	sim.mu.Lock()
	defer sim.mu.Unlock()
	sim.tick++

	switch {
	case sim.status.State == models.StateEmergency, sim.status.Battery == 0:
		sim.status.Speed = 0
	case sim.status.Mode == models.ModeManual:
		sim.followPathLocked()
	default:
		sim.huntLocked()
	}

	sim.consumeBatteryLocked()
	return sim.buildBroadcastMessagesLocked()
}

func flatEnemy(e models.Enemy) map[string]interface{} {
	return map[string]interface{}{
		"id":   e.ID,
		"name": e.Name,
		"hp":   e.HP,
		"x":    e.Position.X,
		"y":    e.Position.Y,
	}
}

func (sim *AGVSimulator) buildBroadcastMessagesLocked() (statusMsg, positionMsg models.WebSocketMessage) {
	flatEnemies := make([]map[string]interface{}, len(sim.status.DetectedEnemies))
	for i, enemy := range sim.status.DetectedEnemies {
		flatEnemies[i] = flatEnemy(enemy)
	}
	var flatTarget map[string]interface{}
	if sim.status.TargetEnemy != nil {
		flatTarget = flatEnemy(*sim.status.TargetEnemy)
	}

	now := time.Now()
	statusMsg = models.WebSocketMessage{
		Type: models.MessageTypeStatus,
		Data: map[string]interface{}{
			"battery":          sim.status.Battery,
			"speed":            sim.status.Speed,
			"mode":             sim.status.Mode,
			"state":            sim.status.State,
			"detected_enemies": flatEnemies,
			"target_enemy":     flatTarget,
		},
		Timestamp: now.UnixMilli(),
	}
	positionMsg = models.WebSocketMessage{
		Type: models.MessageTypePosition,
		Data: models.PositionData{
			X:         sim.status.Position.X,
			Y:         sim.status.Position.Y,
			Angle:     sim.status.Position.Angle,
			Timestamp: now,
		},
		Timestamp: now.UnixMilli(),
	}
	return statusMsg, positionMsg
}
