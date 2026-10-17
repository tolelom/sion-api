package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sion-backend/models"
	"sync"
	"sync/atomic"
	"time"
)

const (
	MapSize           = 20
	startX            = 2.0
	startY            = 2.0
	maxPathPoints     = 400
	batteryDrainTicks = 5
	obstacleCount     = 25
	enemyCount        = 5
	detectionRange    = 10.0
	attackRange       = 2.0
)

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
		updateInterval: 500 * time.Millisecond,
		broadcastFunc:  broadcast,
		hasListeners:   hasListeners,
	}
	sim.obstacles = generateObstacles(obstacleCount)
	sim.resetLocked()
	return sim
}

func initialStatus() *models.AGVStatus {
	return &models.AGVStatus{
		ID:       "sion-001",
		Name:     "사이온",
		Position: models.PositionData{X: startX, Y: startY},
		Mode:     models.ModeAuto,
		State:    models.StateIdle,
		Battery:  100,
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

func (sim *AGVSimulator) Snapshot() (status models.AGVStatus, enemies []models.Enemy, mapW, mapH float64) {
	sim.mu.RLock()
	defer sim.mu.RUnlock()
	status = *sim.status
	enemies = make([]models.Enemy, len(sim.enemies))
	copy(enemies, sim.enemies)
	return status, enemies, MapSize, MapSize
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

func (sim *AGVSimulator) followPathLocked() {
	sim.status.TargetEnemy = nil
	sim.status.DetectedEnemies = nil
	if len(sim.path) == 0 {
		sim.status.Speed = 0
		sim.status.State = models.StateIdle
		return
	}
	sim.status.Speed = 1.0
	sim.status.State = models.StateMoving
	next := sim.path[0]
	if sim.moveTowardsLocked(next.X, next.Y) {
		sim.path = sim.path[1:]
	}
}

func (sim *AGVSimulator) huntLocked() {
	detected := sim.detectEnemiesLocked()
	sim.status.DetectedEnemies = detected
	if len(detected) == 0 {
		sim.status.TargetEnemy = nil
		sim.status.State = models.StateSearching
		sim.status.Speed = 1.0
		sim.randomWalkLocked()
		return
	}
	target := lowestHP(detected)
	sim.status.TargetEnemy = &target
	sim.status.State = models.StateCharging
	sim.status.Speed = 2.5
	sim.moveTowardsLocked(target.Position.X, target.Position.Y)
	if sim.distanceToLocked(target.Position.X, target.Position.Y) < attackRange {
		sim.attackTargetLocked()
	}
}

func (sim *AGVSimulator) HandleCommand(msg models.WebSocketMessage) error {
	switch msg.Type {
	case models.MessageTypeEmergencyStop:
		sim.mu.Lock()
		sim.status.State = models.StateEmergency
		sim.status.Speed = 0
		sim.path = nil
		sim.mu.Unlock()
		return nil
	case models.MessageTypeModeChange:
		var cmd struct {
			Mode string `json:"mode"`
		}
		if err := decodeData(msg.Data, &cmd); err != nil {
			return err
		}
		if cmd.Mode != models.ModeAuto && cmd.Mode != models.ModeManual {
			return fmt.Errorf("알 수 없는 모드: %q", cmd.Mode)
		}
		sim.mu.Lock()
		sim.status.Mode = cmd.Mode
		sim.path = nil
		sim.mu.Unlock()
		return nil
	case models.MessageTypeCommand:
		return sim.handleMoveCommand(msg.Data)
	default:
		return fmt.Errorf("지원하지 않는 메시지 타입: %q", msg.Type)
	}
}

func (sim *AGVSimulator) handleMoveCommand(data any) error {
	var cmd struct {
		Action string            `json:"action"`
		Path   []models.Waypoint `json:"path"`
	}
	if err := decodeData(data, &cmd); err != nil {
		return err
	}
	if cmd.Action == "reset" {
		sim.mu.Lock()
		sim.resetLocked()
		sim.mu.Unlock()
		return nil
	}
	if len(cmd.Path) == 0 {
		return errors.New("경로가 비어 있습니다")
	}
	if len(cmd.Path) > maxPathPoints {
		return fmt.Errorf("경로는 최대 %d개 지점까지입니다", maxPathPoints)
	}
	for _, p := range cmd.Path {
		if p.X < 0 || p.Y < 0 || p.X > MapSize || p.Y > MapSize {
			return errors.New("경로가 맵을 벗어납니다")
		}
	}
	sim.mu.Lock()
	defer sim.mu.Unlock()
	if sim.status.State == models.StateEmergency || sim.status.Battery == 0 {
		return errors.New("정지 상태입니다. 초기화 후 다시 시도하세요")
	}
	sim.status.Mode = models.ModeManual
	sim.path = cmd.Path
	return nil
}

func decodeData(data any, dst any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return errors.New("잘못된 명령 형식")
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return errors.New("잘못된 명령 형식")
	}
	return nil
}

func (sim *AGVSimulator) detectEnemiesLocked() []models.Enemy {
	var detected []models.Enemy
	for _, enemy := range sim.enemies {
		if enemy.HP > 0 && sim.distanceToLocked(enemy.Position.X, enemy.Position.Y) <= detectionRange {
			detected = append(detected, enemy)
		}
	}
	return detected
}

func lowestHP(enemies []models.Enemy) models.Enemy {
	lowest := enemies[0]
	for _, enemy := range enemies {
		if enemy.HP < lowest.HP {
			lowest = enemy
		}
	}
	return lowest
}

func (sim *AGVSimulator) moveTowardsLocked(targetX, targetY float64) bool {
	dx := targetX - sim.status.Position.X
	dy := targetY - sim.status.Position.Y
	dist := math.Hypot(dx, dy)
	stepLen := sim.status.Speed * 0.5
	if dist <= stepLen {
		sim.status.Position.X, sim.status.Position.Y = targetX, targetY
		sim.clampPositionLocked()
		return true
	}
	sim.status.Position.X += dx / dist * stepLen
	sim.status.Position.Y += dy / dist * stepLen
	sim.status.Position.Angle = math.Atan2(dy, dx)
	sim.clampPositionLocked()
	return false
}

func (sim *AGVSimulator) randomWalkLocked() {
	if rand.Float64() < 0.1 {
		sim.status.Position.Angle = rand.Float64() * 2 * math.Pi
	}
	stepLen := sim.status.Speed * 0.5
	nx := sim.status.Position.X + math.Cos(sim.status.Position.Angle)*stepLen
	ny := sim.status.Position.Y + math.Sin(sim.status.Position.Angle)*stepLen
	if nx < 0 || ny < 0 || nx > MapSize || ny > MapSize || sim.isObstacleLocked(nx, ny) {
		sim.status.Position.Angle = rand.Float64() * 2 * math.Pi
		return
	}
	sim.status.Position.X, sim.status.Position.Y = nx, ny
}

func (sim *AGVSimulator) isObstacleLocked(x, y float64) bool {
	cx, cy := int(math.Floor(x)), int(math.Floor(y))
	for _, o := range sim.obstacles {
		if o.X == cx && o.Y == cy {
			return true
		}
	}
	return false
}

func (sim *AGVSimulator) attackTargetLocked() {
	if sim.status.TargetEnemy == nil || rand.Float64() >= 0.2 {
		return
	}
	for i := range sim.enemies {
		if sim.enemies[i].ID != sim.status.TargetEnemy.ID {
			continue
		}
		sim.enemies[i].HP = max(sim.enemies[i].HP-10, 0)
		sim.status.TargetEnemy.HP = sim.enemies[i].HP
		if sim.enemies[i].HP == 0 {
			sim.status.TargetEnemy = nil
		}
		return
	}
}

func (sim *AGVSimulator) consumeBatteryLocked() {
	if sim.status.Speed > 0 && sim.tick%batteryDrainTicks == 0 {
		sim.status.Battery--
	}
	if sim.status.Battery <= 0 {
		sim.status.Battery = 0
		sim.status.State = models.StateStopped
		sim.status.Speed = 0
		sim.path = nil
	}
}

func (sim *AGVSimulator) clampPositionLocked() {
	sim.status.Position.X = math.Min(math.Max(sim.status.Position.X, 0), MapSize)
	sim.status.Position.Y = math.Min(math.Max(sim.status.Position.Y, 0), MapSize)
}

func (sim *AGVSimulator) distanceToLocked(x, y float64) float64 {
	return math.Hypot(x-sim.status.Position.X, y-sim.status.Position.Y)
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

func generateObstacles(count int) []models.Cell {
	taken := map[models.Cell]bool{}
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			taken[models.Cell{X: int(startX) + dx, Y: int(startY) + dy}] = true
		}
	}
	obstacles := make([]models.Cell, 0, count)
	for len(obstacles) < count {
		c := models.Cell{X: rand.Intn(MapSize), Y: rand.Intn(MapSize)}
		if taken[c] {
			continue
		}
		taken[c] = true
		obstacles = append(obstacles, c)
	}
	return obstacles
}

func generateEnemies(count int, obstacles []models.Cell) []models.Enemy {
	names := []string{"아리", "야스오", "지글스", "룩스", "제드"}
	blocked := map[models.Cell]bool{}
	for _, o := range obstacles {
		blocked[o] = true
	}
	enemies := make([]models.Enemy, 0, count)
	for len(enemies) < count {
		x, y := rand.Float64()*MapSize, rand.Float64()*MapSize
		if blocked[models.Cell{X: int(x), Y: int(y)}] {
			continue
		}
		enemies = append(enemies, models.Enemy{
			ID:       fmt.Sprintf("enemy-%d", len(enemies)+1),
			Name:     names[rand.Intn(len(names))],
			HP:       rand.Intn(81) + 20,
			Position: models.PositionData{X: x, Y: y},
		})
	}
	return enemies
}
