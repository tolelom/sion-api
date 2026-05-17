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

// 시뮬레이션 파라미터. 운영 중 값을 조정하려면 const → var로 바꾸거나
// NewAGVSimulator에 옵션 인자를 도입한다.
const (
	defaultMapSize          = 30.0
	defaultEnemyCount       = 5
	defaultObstacleCount    = 10
	defaultUpdateIntervalMs = 500
	defaultBattery          = 100
	defaultInitialX         = 5.0
	defaultInitialY         = 5.0

	detectionRange       = 10.0
	targetEngageDistance = 2.0
	chargingSpeed        = 2.5
	searchingSpeed       = 1.0
	moveSpeedFactor      = 0.5
	randomTurnChance     = 0.1
	attackChance         = 0.2
	attackDamage         = 10
	batteryDrainPerTick  = 1
	lowBatteryThreshold  = 20
	lowBatteryWarnChance = 0.05
)

// AGVSimulator의 모든 가변 상태는 unexport이며 mu/atomic으로 보호된다.
// 외부 접근은 Snapshot()/IsRunning() 등 접근자만 사용한다.
type AGVSimulator struct {
	mu             sync.RWMutex
	status         *models.AGVStatus
	mapWidth       float64
	mapHeight      float64
	enemies        []models.Enemy
	obstacles      []models.Obstacle
	updateInterval time.Duration
	broadcastFunc  func(models.WebSocketMessage)

	// rng는 인스턴스 단위 PRNG. 전역 math/rand 대신 사용해 다른 인스턴스/패키지의 호출과
	// 분리한다. 결정론적 테스트가 필요해지면 NewAGVSimulator에 시드 옵션을 추가한다.
	rng *rand.Rand

	running  atomic.Bool
	stopChan chan struct{}
	doneChan chan struct{}
}

func NewAGVSimulator(broadcastFunc func(models.WebSocketMessage)) *AGVSimulator {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return &AGVSimulator{
		status: &models.AGVStatus{
			ID:   "sion-001",
			Name: "사이온",
			Position: models.PositionData{
				X:     defaultInitialX,
				Y:     defaultInitialY,
				Angle: 0,
			},
			Mode:    models.ModeAuto,
			State:   models.StateIdle,
			Speed:   0,
			Battery: defaultBattery,
		},
		mapWidth:       defaultMapSize,
		mapHeight:      defaultMapSize,
		enemies:        generateRandomEnemies(rng, defaultEnemyCount, defaultMapSize, defaultMapSize),
		obstacles:      generateRandomObstacles(rng, defaultObstacleCount, defaultMapSize, defaultMapSize),
		updateInterval: defaultUpdateIntervalMs * time.Millisecond,
		broadcastFunc:  broadcastFunc,
		rng:            rng,
	}
}

// SetUpdateInterval은 다음 Start 시점부터 적용된다.
// 이미 실행 중인 경우 현 ticker는 영향 없고, Stop→Start 후 적용된다.
// 주로 테스트에서 빠른 틱을 쓰기 위해 사용한다.
func (sim *AGVSimulator) SetUpdateInterval(d time.Duration) {
	sim.mu.Lock()
	sim.updateInterval = d
	sim.mu.Unlock()
}

// IsRunning은 외부 핸들러가 시뮬레이터 상태를 안전하게 읽기 위한 접근자.
func (sim *AGVSimulator) IsRunning() bool {
	return sim.running.Load()
}

// Snapshot은 외부에서 읽을 수 있는 현재 상태 사본을 반환한다.
// (시뮬레이터 고루틴이 매 틱 status를 변경하므로 직접 노출하지 않는다.)
func (sim *AGVSimulator) Snapshot() (status models.AGVStatus, enemies []models.Enemy, mapW, mapH float64) {
	sim.mu.RLock()
	defer sim.mu.RUnlock()
	if sim.status != nil {
		status = *sim.status
	}
	enemies = make([]models.Enemy, len(sim.enemies))
	copy(enemies, sim.enemies)
	return status, enemies, sim.mapWidth, sim.mapHeight
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
	sim.mu.RLock()
	interval := sim.updateInterval
	sim.mu.RUnlock()
	ticker := time.NewTicker(interval)
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

// update는 한 시뮬레이션 틱을 처리한다.
// 상태 갱신은 tick()에서 잠금 안에 묶고, DB 로그·브로드캐스트(외부 IO)는
// 잠금 밖에서 수행해 broadcastFunc가 다시 sim에 접근해도 데드락이 없도록 한다.
func (sim *AGVSimulator) update() {
	statusMsg, positionMsg, statusCopy, broadcastFn := sim.tick()

	go LogAGVStatus(statusCopy.ID, &statusCopy)
	if broadcastFn != nil {
		broadcastFn(statusMsg)
		broadcastFn(positionMsg)
	}
}

// tick은 sense → decide → act → consume → build 순서로 한 틱을 처리하고,
// 잠금 밖에서 사용할 메시지/상태 사본과 broadcast 함수 참조를 반환한다.
// 모든 mutation은 이 잠금 영역 안에서 일어난다.
func (sim *AGVSimulator) tick() (statusMsg, positionMsg models.WebSocketMessage, statusCopy models.AGVStatus, broadcastFn func(models.WebSocketMessage)) {
	sim.mu.Lock()
	defer sim.mu.Unlock()

	sim.senseLocked()
	sim.decideLocked()
	sim.actLocked()
	sim.consumeBatteryLocked()

	statusMsg, positionMsg = sim.buildBroadcastMessagesLocked()
	statusCopy = *sim.status
	broadcastFn = sim.broadcastFunc
	return
}

// senseLocked는 감지 결과를 status.DetectedEnemies에 반영한다.
func (sim *AGVSimulator) senseLocked() {
	sim.status.DetectedEnemies = sim.detectEnemiesLocked()
}

// decideLocked는 detection/mode를 보고 target/state/speed를 갱신한다.
// target이 새로 선정될 때만 LogTargetFound를 호출한다(매 틱 X).
func (sim *AGVSimulator) decideLocked() {
	if len(sim.status.DetectedEnemies) > 0 && sim.status.Mode == models.ModeAuto {
		lowest := findLowestHPEnemy(sim.status.DetectedEnemies)
		prevID := ""
		if sim.status.TargetEnemy != nil {
			prevID = sim.status.TargetEnemy.ID
		}
		sim.status.TargetEnemy = &lowest
		sim.status.State = models.StateCharging
		sim.status.Speed = chargingSpeed
		if prevID != lowest.ID {
			LogTargetFound(sim.status.ID, &lowest)
		}
		return
	}
	sim.status.TargetEnemy = nil
	sim.status.State = models.StateSearching
	sim.status.Speed = searchingSpeed
}

// actLocked는 결정된 target을 향한 이동/공격 또는 random walk를 수행한다.
func (sim *AGVSimulator) actLocked() {
	if sim.status.Mode != models.ModeAuto {
		return
	}
	if sim.status.TargetEnemy != nil {
		tx := sim.status.TargetEnemy.Position.X
		ty := sim.status.TargetEnemy.Position.Y
		sim.moveTowardsLocked(tx, ty)
		if sim.distanceToLocked(tx, ty) < targetEngageDistance {
			sim.attackTargetLocked()
		}
		return
	}
	sim.randomWalkLocked()
}

func (sim *AGVSimulator) detectEnemiesLocked() []models.Enemy {
	var detected []models.Enemy
	for _, enemy := range sim.enemies {
		dist := sim.distanceToLocked(enemy.Position.X, enemy.Position.Y)
		if dist <= detectionRange && enemy.HP > 0 {
			detected = append(detected, enemy)
		}
	}
	return detected
}

func findLowestHPEnemy(enemies []models.Enemy) models.Enemy {
	lowest := enemies[0]
	for _, enemy := range enemies {
		if enemy.HP < lowest.HP {
			lowest = enemy
		}
	}
	return lowest
}

func (sim *AGVSimulator) moveTowardsLocked(targetX, targetY float64) {
	dx := targetX - sim.status.Position.X
	dy := targetY - sim.status.Position.Y
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist > 0.1 {
		dx /= dist
		dy /= dist
		moveSpeed := sim.status.Speed * moveSpeedFactor
		sim.status.Position.X += dx * moveSpeed
		sim.status.Position.Y += dy * moveSpeed
		sim.status.Position.Angle = math.Atan2(dy, dx)
		sim.clampPositionLocked()
	}
}

func (sim *AGVSimulator) randomWalkLocked() {
	if sim.rng.Float64() < randomTurnChance {
		sim.status.Position.Angle = sim.rng.Float64() * 2 * math.Pi
	}
	moveSpeed := sim.status.Speed * moveSpeedFactor
	sim.status.Position.X += math.Cos(sim.status.Position.Angle) * moveSpeed
	sim.status.Position.Y += math.Sin(sim.status.Position.Angle) * moveSpeed
	sim.clampPositionLocked()
}

func (sim *AGVSimulator) attackTargetLocked() {
	if sim.status.TargetEnemy == nil {
		return
	}
	if sim.rng.Float64() >= attackChance {
		return
	}
	for i := range sim.enemies {
		if sim.enemies[i].ID != sim.status.TargetEnemy.ID {
			continue
		}
		sim.enemies[i].HP -= attackDamage
		if sim.enemies[i].HP < 0 {
			sim.enemies[i].HP = 0
		}
		sim.status.TargetEnemy.HP = sim.enemies[i].HP
		log.Printf("[INFO] 타겟 공격: %s HP: %d", sim.enemies[i].Name, sim.enemies[i].HP)
		if sim.enemies[i].HP == 0 {
			log.Printf("[INFO] 타겟 제거: %s", sim.enemies[i].Name)
			sim.status.TargetEnemy = nil
		}
		return
	}
}

func (sim *AGVSimulator) consumeBatteryLocked() {
	if sim.status.Speed > 0 {
		sim.status.Battery -= batteryDrainPerTick
		if sim.status.Battery < 0 {
			sim.status.Battery = 0
			sim.status.State = models.StateStopped
			sim.status.Speed = 0
			log.Println("[WARN] 배터리 방전, AGV 정지")
		}
	}
	if sim.status.Battery <= lowBatteryThreshold && sim.status.Battery > 0 {
		if sim.rng.Float64() < lowBatteryWarnChance {
			log.Printf("[WARN] 배터리 부족: %d%%", sim.status.Battery)
		}
	}
}

func (sim *AGVSimulator) clampPositionLocked() {
	if sim.status.Position.X < 0 {
		sim.status.Position.X = 0
	}
	if sim.status.Position.X > sim.mapWidth {
		sim.status.Position.X = sim.mapWidth
	}
	if sim.status.Position.Y < 0 {
		sim.status.Position.Y = 0
	}
	if sim.status.Position.Y > sim.mapHeight {
		sim.status.Position.Y = sim.mapHeight
	}
}

func (sim *AGVSimulator) distanceToLocked(x, y float64) float64 {
	dx := x - sim.status.Position.X
	dy := y - sim.status.Position.Y
	return math.Sqrt(dx*dx + dy*dy)
}

func (sim *AGVSimulator) buildBroadcastMessagesLocked() (statusMsg, positionMsg models.WebSocketMessage) {
	flatEnemies := make([]map[string]interface{}, len(sim.status.DetectedEnemies))
	for i, enemy := range sim.status.DetectedEnemies {
		flatEnemies[i] = map[string]interface{}{
			"id":   enemy.ID,
			"name": enemy.Name,
			"hp":   enemy.HP,
			"x":    enemy.Position.X,
			"y":    enemy.Position.Y,
		}
	}

	var flatTarget map[string]interface{}
	if sim.status.TargetEnemy != nil {
		flatTarget = map[string]interface{}{
			"id":   sim.status.TargetEnemy.ID,
			"name": sim.status.TargetEnemy.Name,
			"hp":   sim.status.TargetEnemy.HP,
			"x":    sim.status.TargetEnemy.Position.X,
			"y":    sim.status.TargetEnemy.Position.Y,
		}
	}

	now := time.Now()
	statusMsg = models.NewMessage(models.MessageTypeStatus, map[string]interface{}{
		"battery":          sim.status.Battery,
		"speed":            sim.status.Speed,
		"mode":             sim.status.Mode,
		"state":            sim.status.State,
		"detected_enemies": flatEnemies,
		"target_enemy":     flatTarget,
	}, now.UnixMilli())
	positionMsg = models.NewMessage(models.MessageTypePosition, models.PositionData{
		X:         sim.status.Position.X,
		Y:         sim.status.Position.Y,
		Angle:     sim.status.Position.Angle,
		Timestamp: now,
	}, now.UnixMilli())
	return statusMsg, positionMsg
}

func generateRandomEnemies(rng *rand.Rand, count int, mapWidth, mapHeight float64) []models.Enemy {
	enemyNames := []string{"아리", "야스오", "지글스", "룩스", "제드"}
	enemies := make([]models.Enemy, count)

	for i := 0; i < count; i++ {
		enemies[i] = models.Enemy{
			ID:   fmt.Sprintf("enemy-%d", i+1),
			Name: enemyNames[rng.Intn(len(enemyNames))],
			HP:   rng.Intn(81) + 20,
			Position: models.PositionData{
				X: rng.Float64() * mapWidth,
				Y: rng.Float64() * mapHeight,
			},
		}
	}
	return enemies
}

func generateRandomObstacles(rng *rand.Rand, count int, mapWidth, mapHeight float64) []models.Obstacle {
	obstacles := make([]models.Obstacle, count)
	for i := 0; i < count; i++ {
		obstacles[i] = models.Obstacle{
			ID:   fmt.Sprintf("obstacle-%d", i+1),
			Type: "static",
			Position: models.GridCoordinate{
				Row: rng.Intn(int(mapHeight)),
				Col: rng.Intn(int(mapWidth)),
			},
			Size: 1,
		}
	}
	return obstacles
}
