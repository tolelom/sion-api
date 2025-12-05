package services

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"sion-backend/models"
	"time"
)

// AGVSimulator - AGV 시뮬레이터
type AGVSimulator struct {
	Status         *models.AGVStatus
	MapWidth       float64
	MapHeight      float64
	Enemies        []models.Enemy
	Obstacles      []models.Obstacle // 🔁 models.Position → models.Obstacle
	IsRunning      bool
	UpdateInterval time.Duration
	BroadcastFunc  func(models.WebSocketMessage)
	stopChan       chan bool
}

// NewAGVSimulator - 시뮬레이터 생성
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
		IsRunning:      false,
		UpdateInterval: 500 * time.Millisecond,
		BroadcastFunc:  broadcastFunc,
		stopChan:       make(chan bool),
	}
}

// Start - 시뮬레이터 시작
func (sim *AGVSimulator) Start() {
	if sim.IsRunning {
		log.Println("⚠️ 시뮬레이터가 이미 실행 중")
		return
	}

	sim.IsRunning = true
	log.Println("🤖 AGV 시뮬레이터 시작")

	go sim.runSimulation()
}

// Stop - 시뮬레이터 중지
func (sim *AGVSimulator) Stop() {
	if !sim.IsRunning {
		return
	}

	sim.IsRunning = false
	sim.stopChan <- true
	log.Println("🛑 AGV 시뮬레이터 중지")
}

// runSimulation - 메인 시뮬레이션 루프
func (sim *AGVSimulator) runSimulation() {
	ticker := time.NewTicker(sim.UpdateInterval)
	defer ticker.Stop()

	for sim.IsRunning {
		select {
		case <-ticker.C:
			sim.update()
		case <-sim.stopChan:
			return
		}
	}
}

// update - 매 틱마다 호출
func (sim *AGVSimulator) update() {
	detectedEnemies := sim.detectEnemies()
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
			sim.moveTowards(sim.Status.TargetEnemy.Position.X, sim.Status.TargetEnemy.Position.Y)
			dist := sim.distanceTo(sim.Status.TargetEnemy.Position.X, sim.Status.TargetEnemy.Position.Y)
			if dist < 2.0 {
				sim.attackTarget()
			}
		} else {
			sim.randomWalk()
		}
	}

	sim.consumeBattery()
	sim.broadcastStatus()
	LogAGVStatus(sim.Status.ID, sim.Status)
}

func (sim *AGVSimulator) detectEnemies() []models.Enemy {
	detectionRange := 10.0
	var detected []models.Enemy

	for _, enemy := range sim.Enemies {
		dist := sim.distanceTo(enemy.Position.X, enemy.Position.Y)
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

func (sim *AGVSimulator) moveTowards(targetX, targetY float64) {
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
		sim.clampPosition()
	}
}

func (sim *AGVSimulator) randomWalk() {
	if rand.Float64() < 0.1 {
		sim.Status.Position.Angle = rand.Float64() * 2 * math.Pi
	}
	moveSpeed := sim.Status.Speed * 0.5
	sim.Status.Position.X += math.Cos(sim.Status.Position.Angle) * moveSpeed
	sim.Status.Position.Y += math.Sin(sim.Status.Position.Angle) * moveSpeed
	sim.clampPosition()
}

func (sim *AGVSimulator) attackTarget() {
	if sim.Status.TargetEnemy == nil {
		return
	}

	if rand.Float64() < 0.2 {
		for i := range sim.Enemies {
			if sim.Enemies[i].ID == sim.Status.TargetEnemy.ID {
				sim.Enemies[i].HP -= 10
				if sim.Enemies[i].HP < 0 {
					sim.Enemies[i].HP = 0
				}
				sim.Status.TargetEnemy.HP = sim.Enemies[i].HP
				log.Printf("⚔️ 타겟 공격! %s HP: %d", sim.Enemies[i].Name, sim.Enemies[i].HP)
				if sim.Enemies[i].HP == 0 {
					log.Printf("🎯 타겟 제거: %s", sim.Enemies[i].Name)
					sim.Status.TargetEnemy = nil
				}
				break
			}
		}
	}
}

func (sim *AGVSimulator) consumeBattery() {
	if sim.Status.Speed > 0 {
		// 0.1을 float64로 명시해 암묵적 int 변환 경고 방지
		sim.Status.Battery -= 0.1
		if sim.Status.Battery < 0 {
			sim.Status.Battery = 0
			sim.Status.State = models.StateStopped
			sim.Status.Speed = 0
			log.Println("🪫 배터리 방전! AGV 정지")
		}
	}

	if sim.Status.Battery <= 20 && sim.Status.Battery > 0 {
		if rand.Float64() < 0.05 {
			log.Printf("⚠️ 배터리 부족: %.1f%%", sim.Status.Battery)
		}
	}
}

func (sim *AGVSimulator) clampPosition() {
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

func (sim *AGVSimulator) distanceTo(x, y float64) float64 {
	dx := x - sim.Status.Position.X
	dy := y - sim.Status.Position.Y
	return math.Sqrt(dx*dx + dy*dy)
}

func (sim *AGVSimulator) broadcastStatus() {
	if sim.BroadcastFunc == nil {
		return
	}

	statusMsg := models.WebSocketMessage{
		Type: models.MessageTypeStatus,
		Data: map[string]interface{}{
			"battery":          sim.Status.Battery,
			"speed":            sim.Status.Speed,
			"mode":             sim.Status.Mode,
			"state":            sim.Status.State,
			"detected_enemies": sim.Status.DetectedEnemies,
			"target_enemy":     sim.Status.TargetEnemy,
		},
		Timestamp: time.Now().UnixMilli(),
	}

	positionMsg := models.WebSocketMessage{
		Type: models.MessageTypePosition,
		Data: models.PositionData{
			X:         sim.Status.Position.X,
			Y:         sim.Status.Position.Y,
			Angle:     sim.Status.Position.Angle,
			Timestamp: time.Now(),
		},
		Timestamp: time.Now().UnixMilli(),
	}

	sim.BroadcastFunc(statusMsg)
	sim.BroadcastFunc(positionMsg)
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

// generateRandomObstacles - 랜덤 장애물 생성
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
