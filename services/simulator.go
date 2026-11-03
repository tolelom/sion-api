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

	defaultUpdateInterval = 500 * time.Millisecond
	defaultTickSeconds    = float64(defaultUpdateInterval) / float64(time.Second)
	cruiseSpeed           = 1.0
	chargeSpeed           = 2.5
	randomWalkTurnChance  = 0.1

	attackChance = 0.2
	attackDamage = 10
	minEnemyHP   = 20
	maxEnemyHP   = 100

	fullBattery                  = 100
	movingTicksPerBatteryPercent = 5
)

var enemyNames = []string{"아리", "야스오", "지글스", "룩스", "제드"}

// AGVSimulator는 공개 데모에서 실제 AGV 대신 움직인다. ~Locked 메서드는 mu를 쥔 채 불러야 하며,
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
		Data: models.MapUpdateData{
			Obstacles: obstacles,
			Width:     MapSize,
			Height:    MapSize,
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

func enemyData(e models.Enemy) models.EnemyData {
	return models.EnemyData{ID: e.ID, Name: e.Name, HP: e.HP, X: e.Position.X, Y: e.Position.Y}
}

func (sim *AGVSimulator) buildBroadcastMessagesLocked() (statusMsg, positionMsg models.WebSocketMessage) {
	detected := make([]models.EnemyData, len(sim.status.DetectedEnemies))
	for i, enemy := range sim.status.DetectedEnemies {
		detected[i] = enemyData(enemy)
	}
	var target *models.EnemyData
	if sim.status.TargetEnemy != nil {
		t := enemyData(*sim.status.TargetEnemy)
		target = &t
	}

	now := time.Now()
	statusMsg = models.WebSocketMessage{
		Type: models.MessageTypeStatus,
		Data: models.StatusData{
			Battery:         sim.status.Battery,
			Speed:           sim.status.Speed,
			Mode:            sim.status.Mode,
			State:           sim.status.State,
			DetectedEnemies: detected,
			TargetEnemy:     target,
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
