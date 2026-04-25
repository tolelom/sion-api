package services

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"sion-backend/models"
	"sync"
	"sync/atomic"
	"time"
)

type AGVSimulator struct {
	mu             sync.RWMutex
	Status         *models.AGVStatus
	MapWidth       float64
	MapHeight      float64
	Enemies        []models.Enemy
	Obstacles      []models.Obstacle
	UpdateInterval time.Duration
	BroadcastFunc  func(models.WebSocketMessage)

	running  atomic.Bool
	stopChan chan struct{}
	doneChan chan struct{}
}

func NewAGVSimulator(broadcastFunc func(models.WebSocketMessage)) *AGVSimulator {
	return &AGVSimulator{
		Status: &models.AGVStatus{
			ID:   "sion-001",
			Name: "사이온",
			Position: models.PositionData{
				X:     5.0,
				Y:     5.0,
				Angle: 0,
			},
			Mode:    models.ModeAuto,
			State:   models.StateIdle,
			Speed:   0,
			Battery: 100,
		},
		MapWidth:       30.0,
		MapHeight:      30.0,
		Enemies:        generateRandomEnemies(5, 30, 30),
		Obstacles:      generateRandomObstacles(10, 30, 30),
		UpdateInterval: 500 * time.Millisecond,
		BroadcastFunc:  broadcastFunc,
	}
}

// IsRunning은 외부 핸들러가 시뮬레이터 상태를 안전하게 읽기 위한 접근자.
func (sim *AGVSimulator) IsRunning() bool {
	return sim.running.Load()
}

// Snapshot은 외부에서 읽을 수 있는 현재 상태 사본을 반환한다.
// (시뮬레이터 고루틴이 매 틱 Status를 변경하므로 직접 노출하지 않는다.)
func (sim *AGVSimulator) Snapshot() (status models.AGVStatus, enemies []models.Enemy, mapW, mapH float64) {
	sim.mu.RLock()
	defer sim.mu.RUnlock()
	if sim.Status != nil {
		status = *sim.Status
	}
	enemies = make([]models.Enemy, len(sim.Enemies))
	copy(enemies, sim.Enemies)
	return status, enemies, sim.MapWidth, sim.MapHeight
}

func (sim *AGVSimulator) Start() {
	if !sim.running.CompareAndSwap(false, true) {
		log.Println("[WARN] 시뮬레이터가 이미 실행 중")
		return
	}
	sim.stopChan = make(chan struct{})
	sim.doneChan = make(chan struct{})
	log.Println("[INFO] AGV 시뮬레이터 시작")
	go sim.runSimulation()
}

func (sim *AGVSimulator) Stop() {
	if !sim.running.CompareAndSwap(true, false) {
		return
	}
	close(sim.stopChan)
	<-sim.doneChan
	log.Println("[INFO] AGV 시뮬레이터 중지")
}

func (sim *AGVSimulator) runSimulation() {
	defer close(sim.doneChan)
	ticker := time.NewTicker(sim.UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sim.update()
		case <-sim.stopChan:
			return
		}
	}
}

func (sim *AGVSimulator) update() {
	sim.mu.Lock()
	defer sim.mu.Unlock()

	detectedEnemies := sim.detectEnemiesLocked()
	sim.Status.DetectedEnemies = detectedEnemies

	if len(detectedEnemies) > 0 && sim.Status.Mode == models.ModeAuto {
		lowestHPEnemy := sim.findLowestHPEnemy(detectedEnemies)
		sim.Status.TargetEnemy = &lowestHPEnemy
		sim.Status.State = models.StateCharging
		sim.Status.Speed = 2.5
		LogTargetFound(sim.Status.ID, &lowestHPEnemy)
	} else {
		sim.Status.TargetEnemy = nil
		sim.Status.State = models.StateSearching
		sim.Status.Speed = 1.0
	}

	if sim.Status.Mode == models.ModeAuto {
		if sim.Status.TargetEnemy != nil {
			sim.moveTowardsLocked(sim.Status.TargetEnemy.Position.X, sim.Status.TargetEnemy.Position.Y)
			dist := sim.distanceToLocked(sim.Status.TargetEnemy.Position.X, sim.Status.TargetEnemy.Position.Y)
			if dist < 2.0 {
				sim.attackTargetLocked()
			}
		} else {
			sim.randomWalkLocked()
		}
	}

	sim.consumeBatteryLocked()
	statusMsg, positionMsg := sim.buildBroadcastMessagesLocked()
	statusCopy := *sim.Status

	// DB 로그·브로드캐스트는 잠금 밖에서 수행
	go LogAGVStatus(statusCopy.ID, &statusCopy)
	if sim.BroadcastFunc != nil {
		sim.BroadcastFunc(statusMsg)
		sim.BroadcastFunc(positionMsg)
	}
}

func (sim *AGVSimulator) detectEnemiesLocked() []models.Enemy {
	const detectionRange = 10.0
	var detected []models.Enemy
	for _, enemy := range sim.Enemies {
		dist := sim.distanceToLocked(enemy.Position.X, enemy.Position.Y)
		if dist <= detectionRange && enemy.HP > 0 {
			detected = append(detected, enemy)
		}
	}
	return detected
}

func (sim *AGVSimulator) findLowestHPEnemy(enemies []models.Enemy) models.Enemy {
	lowest := enemies[0]
	for _, enemy := range enemies {
		if enemy.HP < lowest.HP {
			lowest = enemy
		}
	}
	return lowest
}

func (sim *AGVSimulator) moveTowardsLocked(targetX, targetY float64) {
	dx := targetX - sim.Status.Position.X
	dy := targetY - sim.Status.Position.Y
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist > 0.1 {
		dx /= dist
		dy /= dist
		moveSpeed := sim.Status.Speed * 0.5
		sim.Status.Position.X += dx * moveSpeed
		sim.Status.Position.Y += dy * moveSpeed
		sim.Status.Position.Angle = math.Atan2(dy, dx)
		sim.clampPositionLocked()
	}
}

func (sim *AGVSimulator) randomWalkLocked() {
	if rand.Float64() < 0.1 {
		sim.Status.Position.Angle = rand.Float64() * 2 * math.Pi
	}
	moveSpeed := sim.Status.Speed * 0.5
	sim.Status.Position.X += math.Cos(sim.Status.Position.Angle) * moveSpeed
	sim.Status.Position.Y += math.Sin(sim.Status.Position.Angle) * moveSpeed
	sim.clampPositionLocked()
}

func (sim *AGVSimulator) attackTargetLocked() {
	if sim.Status.TargetEnemy == nil {
		return
	}
	if rand.Float64() >= 0.2 {
		return
	}
	for i := range sim.Enemies {
		if sim.Enemies[i].ID != sim.Status.TargetEnemy.ID {
			continue
		}
		sim.Enemies[i].HP -= 10
		if sim.Enemies[i].HP < 0 {
			sim.Enemies[i].HP = 0
		}
		sim.Status.TargetEnemy.HP = sim.Enemies[i].HP
		log.Printf("[INFO] 타겟 공격: %s HP: %d", sim.Enemies[i].Name, sim.Enemies[i].HP)
		if sim.Enemies[i].HP == 0 {
			log.Printf("[INFO] 타겟 제거: %s", sim.Enemies[i].Name)
			sim.Status.TargetEnemy = nil
		}
		return
	}
}

func (sim *AGVSimulator) consumeBatteryLocked() {
	if sim.Status.Speed > 0 {
		sim.Status.Battery -= 1
		if sim.Status.Battery < 0 {
			sim.Status.Battery = 0
			sim.Status.State = models.StateStopped
			sim.Status.Speed = 0
			log.Println("[WARN] 배터리 방전, AGV 정지")
		}
	}
	if sim.Status.Battery <= 20 && sim.Status.Battery > 0 {
		if rand.Float64() < 0.05 {
			log.Printf("[WARN] 배터리 부족: %d%%", sim.Status.Battery)
		}
	}
}

func (sim *AGVSimulator) clampPositionLocked() {
	if sim.Status.Position.X < 0 {
		sim.Status.Position.X = 0
	}
	if sim.Status.Position.X > sim.MapWidth {
		sim.Status.Position.X = sim.MapWidth
	}
	if sim.Status.Position.Y < 0 {
		sim.Status.Position.Y = 0
	}
	if sim.Status.Position.Y > sim.MapHeight {
		sim.Status.Position.Y = sim.MapHeight
	}
}

func (sim *AGVSimulator) distanceToLocked(x, y float64) float64 {
	dx := x - sim.Status.Position.X
	dy := y - sim.Status.Position.Y
	return math.Sqrt(dx*dx + dy*dy)
}

func (sim *AGVSimulator) buildBroadcastMessagesLocked() (statusMsg, positionMsg models.WebSocketMessage) {
	flatEnemies := make([]map[string]interface{}, len(sim.Status.DetectedEnemies))
	for i, enemy := range sim.Status.DetectedEnemies {
		flatEnemies[i] = map[string]interface{}{
			"id":   enemy.ID,
			"name": enemy.Name,
			"hp":   enemy.HP,
			"x":    enemy.Position.X,
			"y":    enemy.Position.Y,
		}
	}

	var flatTarget map[string]interface{}
	if sim.Status.TargetEnemy != nil {
		flatTarget = map[string]interface{}{
			"id":   sim.Status.TargetEnemy.ID,
			"name": sim.Status.TargetEnemy.Name,
			"hp":   sim.Status.TargetEnemy.HP,
			"x":    sim.Status.TargetEnemy.Position.X,
			"y":    sim.Status.TargetEnemy.Position.Y,
		}
	}

	now := time.Now()
	statusMsg = models.WebSocketMessage{
		Type: models.MessageTypeStatus,
		Data: map[string]interface{}{
			"battery":          sim.Status.Battery,
			"speed":            sim.Status.Speed,
			"mode":             sim.Status.Mode,
			"state":            sim.Status.State,
			"detected_enemies": flatEnemies,
			"target_enemy":     flatTarget,
		},
		Timestamp: now.UnixMilli(),
	}
	positionMsg = models.WebSocketMessage{
		Type: models.MessageTypePosition,
		Data: models.PositionData{
			X:         sim.Status.Position.X,
			Y:         sim.Status.Position.Y,
			Angle:     sim.Status.Position.Angle,
			Timestamp: now,
		},
		Timestamp: now.UnixMilli(),
	}
	return statusMsg, positionMsg
}

func generateRandomEnemies(count int, mapWidth, mapHeight float64) []models.Enemy {
	enemyNames := []string{"아리", "야스오", "지글스", "룩스", "제드"}
	enemies := make([]models.Enemy, count)

	for i := 0; i < count; i++ {
		enemies[i] = models.Enemy{
			ID:   fmt.Sprintf("enemy-%d", i+1),
			Name: enemyNames[rand.Intn(len(enemyNames))],
			HP:   rand.Intn(81) + 20,
			Position: models.PositionData{
				X: rand.Float64() * mapWidth,
				Y: rand.Float64() * mapHeight,
			},
		}
	}
	return enemies
}

func generateRandomObstacles(count int, mapWidth, mapHeight float64) []models.Obstacle {
	obstacles := make([]models.Obstacle, count)
	for i := 0; i < count; i++ {
		obstacles[i] = models.Obstacle{
			ID:   fmt.Sprintf("obstacle-%d", i+1),
			Type: "static",
			Position: models.GridCoordinate{
				Row: rand.Intn(int(mapHeight)),
				Col: rand.Intn(int(mapWidth)),
			},
			Size: 1,
		}
	}
	return obstacles
}
