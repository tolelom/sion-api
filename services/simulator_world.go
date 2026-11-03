package services

import (
	"fmt"
	"math/rand"
	"sion-backend/models"
)

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
			Name:     enemyNames[rand.Intn(len(enemyNames))],
			HP:       minEnemyHP + rand.Intn(maxEnemyHP-minEnemyHP+1),
			Position: models.PositionData{X: x, Y: y},
		})
	}
	return enemies
}
