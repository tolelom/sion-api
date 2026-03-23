package main

import (
	"log"
	"sion-backend/handlers"
	"sion-backend/models"
	"sion-backend/services"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/websocket/v2"
	"github.com/joho/godotenv"
)

var agvSimulator *services.AGVSimulator

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  .env 파일을 찾을 수 없습니다.")
	}

	if err := services.InitDatabase(); err != nil {
		log.Fatalf("❌ DB 초기화 실패: %v", err)
	}

	services.InitLogging(50, 10*time.Second)
	defer services.StopLogging()

	// ClientManager + Broker 생성
	cm := services.NewClientManager()
	br := services.NewBroker(cm)

	// LLM 서비스 + Broker 초기화
	handlers.InitLLMService()
	handlers.InitBroker(br)

	// 시뮬레이터 초기화
	agvSimulator = services.NewAGVSimulator(func(msg models.WebSocketMessage) {
		br.BroadcastToWeb(msg)
	})

	app := fiber.New()

	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:5173, http://localhost:8001, http://sion.tolelom.xyz",
		AllowHeaders: "Origin, Content-Type, Accept",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Sion WebSocket 서버가 실행 중입니다.")
	})

	api := app.Group("/api")

	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "OK",
			"clients": cm.GetClientCount(),
			"time":    time.Now().Format(time.RFC3339),
		})
	})

	// 채팅 엔드포인트
	api.Post("/chat", handlers.HandleChat)

	// 경로 탐색
	api.Post("/pathfinding", handlers.HandlePathfinding)

	// 로그 조회 API
	logsAPI := api.Group("/logs")
	logsAPI.Get("/recent", handlers.HandleGetRecentLogs)
	logsAPI.Get("/range", handlers.HandleGetLogsByTimeRange)
	logsAPI.Get("/type", handlers.HandleGetLogsByEventType)
	logsAPI.Get("/stats", handlers.HandleGetLogStats)

	// 시뮬레이터 API
	simAPI := api.Group("/simulator")
	simAPI.Post("/start", func(c *fiber.Ctx) error {
		if agvSimulator.IsRunning {
			return c.Status(400).JSON(fiber.Map{
				"success": false,
				"message": "시뮬레이터가 이미 실행 중입니다",
			})
		}
		agvSimulator.Start()
		return c.JSON(fiber.Map{
			"success": true,
			"message": "AGV 시뮬레이터 시작",
		})
	})

	simAPI.Post("/stop", func(c *fiber.Ctx) error {
		if !agvSimulator.IsRunning {
			return c.Status(400).JSON(fiber.Map{
				"success": false,
				"message": "시뮬레이터가 실행 중이 아닙니다",
			})
		}
		agvSimulator.Stop()
		return c.JSON(fiber.Map{
			"success": true,
			"message": "AGV 시뮬레이터 중지",
		})
	})

	simAPI.Get("/status", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success":   true,
			"running":   agvSimulator.IsRunning,
			"agv_state": agvSimulator.Status,
			"enemies":   agvSimulator.Enemies,
			"map_size": fiber.Map{
				"width":  agvSimulator.MapWidth,
				"height": agvSimulator.MapHeight,
			},
		})
	})

	// 테스트용 위치 데이터 전송
	api.Post("/test/position", func(c *fiber.Ctx) error {
		testMsg := models.WebSocketMessage{
			Type: models.MessageTypePosition,
			Data: models.PositionData{
				X:         10.5,
				Y:         15.2,
				Angle:     1.57,
				Timestamp: time.Now(),
			},
			Timestamp: time.Now().UnixMilli(),
		}

		br.BroadcastToWeb(testMsg)
		services.LogAGVPosition("sion-001", testMsg.Data.(models.PositionData))

		return c.JSON(fiber.Map{
			"success": true,
			"message": "테스트 데이터 전송 성공",
		})
	})

	// 테스트용 상태 데이터 전송
	api.Post("/test/status", func(c *fiber.Ctx) error {
		testMsg := models.WebSocketMessage{
			Type: models.MessageTypeStatus,
			Data: map[string]interface{}{
				"battery": 85,
				"speed":   1.5,
				"mode":    "auto",
				"state":   "moving",
			},
			Timestamp: time.Now().UnixMilli(),
		}

		br.BroadcastToWeb(testMsg)
		services.LogWebSocketMessage("sion-001", testMsg)

		return c.JSON(fiber.Map{
			"success": true,
			"message": "상태 데이터 전송 성공",
		})
	})

	// 테스트용 AGV 이벤트 트리거
	api.Post("/test/event", func(c *fiber.Ctx) error {
		testStatus := &models.AGVStatus{
			ID:   "sion-001",
			Name: "사이온",
			Position: models.PositionData{
				X: 10.5, Y: 15.2, Angle: 0.785,
			},
			Mode:    models.ModeAuto,
			State:   models.StateCharging,
			Speed:   2.5,
			Battery: 85,
			TargetEnemy: &models.Enemy{
				ID:       "enemy-1",
				Name:     "아리",
				HP:       30,
				Position: models.PositionData{X: 15, Y: 12},
			},
			DetectedEnemies: []models.Enemy{
				{
					ID:       "enemy-1",
					Name:     "아리",
					HP:       30,
					Position: models.PositionData{X: 15, Y: 12},
				},
				{
					ID:       "enemy-2",
					Name:     "아리",
					HP:       80,
					Position: models.PositionData{X: 20, Y: 18},
				},
			},
		}

		services.LogAGVStatus("sion-001", testStatus)

		if testStatus.TargetEnemy != nil {
			services.LogTargetFound("sion-001", testStatus.TargetEnemy)
		}

		handlers.ExplainAGVEvent("target_change", testStatus)

		return c.JSON(fiber.Map{
			"success": true,
			"message": "이벤트 설명 생성 중...",
		})
	})

	// WebSocket
	app.Use("/websocket", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/websocket/agv", websocket.New(handlers.NewAGVHandler(cm, br)))
	app.Get("/websocket/web", websocket.New(handlers.NewWebHandler(cm, br, handlers.GetLLMService())))

	log.Println("🚀 서버 시작: http://localhost:8001")
	log.Println("📡 WebSocket: ws://localhost:8001/websocket/web")
	log.Println("💬 채팅 API: POST http://localhost:8001/api/chat")
	log.Println("🧪 이벤트 테스트: POST http://localhost:8001/api/test/event")
	log.Println("💾 로그 API: GET http://localhost:8001/api/logs/*")
	log.Println("🤖 시뮬레이터 API: POST http://localhost:8001/api/simulator/*")
	log.Fatal(app.Listen(":8001"))
}
