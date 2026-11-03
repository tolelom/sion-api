package services

import (
	"math"
	"math/rand"
	"sion-backend/models"
)

func (sim *AGVSimulator) followPathLocked() {
	sim.status.TargetEnemy = nil
	sim.status.DetectedEnemies = nil
	if len(sim.path) == 0 {
		sim.status.Speed = 0
		sim.status.State = models.StateIdle
		return
	}
	sim.status.Speed = cruiseSpeed
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
		sim.status.Speed = cruiseSpeed
		sim.randomWalkLocked()
		return
	}
	target := lowestHP(detected)
	sim.status.TargetEnemy = &target
	sim.status.State = models.StateCharging
	sim.status.Speed = chargeSpeed
	sim.moveTowardsLocked(target.Position.X, target.Position.Y)
	if sim.distanceToLocked(target.Position.X, target.Position.Y) < attackRange {
		sim.attackTargetLocked()
	}
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
	stepLen := sim.stepLengthLocked()
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
	if rand.Float64() < turnChance {
		sim.status.Position.Angle = rand.Float64() * 2 * math.Pi
	}
	stepLen := sim.stepLengthLocked()
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
	if sim.status.TargetEnemy == nil || rand.Float64() >= attackChance {
		return
	}
	for i := range sim.enemies {
		if sim.enemies[i].ID != sim.status.TargetEnemy.ID {
			continue
		}
		sim.enemies[i].HP = max(sim.enemies[i].HP-attackDamage, 0)
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

func (sim *AGVSimulator) stepLengthLocked() float64 {
	return sim.status.Speed * stepPerSpeed
}

func (sim *AGVSimulator) distanceToLocked(x, y float64) float64 {
	return math.Hypot(x-sim.status.Position.X, y-sim.status.Position.Y)
}
