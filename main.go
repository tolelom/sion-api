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

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("[WARN] .env 파일을 찾을 수 없습니다")
	}

	if err := services.InitDatabase(); err != nil {
		log.Fatalf("[FATAL] DB 초기화 실패: %v", err)
	}

	services.InitLogging(50, 10*time.Second)
	defer services.StopLogging()

	cm := services.NewClientManager()
	br := services.NewBroker(cm)

	handlers.InitLLMService()
	handlers.InitBroker(br)

	sim := services.NewAGVSimulator(func(msg models.WebSocketMessage) {
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

	api.Post("/chat", handlers.HandleChat)
	api.Post("/pathfinding", handlers.HandlePathfinding)

	logsAPI := api.Group("/logs")
	logsAPI.Get("/recent", handlers.HandleGetRecentLogs)
	logsAPI.Get("/range", handlers.HandleGetLogsByTimeRange)
	logsAPI.Get("/type", handlers.HandleGetLogsByEventType)
	logsAPI.Get("/stats", handlers.HandleGetLogStats)

	simAPI := api.Group("/simulator")
	simAPI.Post("/start", handlers.NewSimulatorStartHandler(sim))
	simAPI.Post("/stop", handlers.NewSimulatorStopHandler(sim))
	simAPI.Get("/status", handlers.NewSimulatorStatusHandler(sim))

	testAPI := api.Group("/test")
	testAPI.Post("/position", handlers.NewTestPositionHandler(br))
	testAPI.Post("/status", handlers.NewTestStatusHandler(br))
	testAPI.Post("/event", handlers.NewTestEventHandler(br))

	app.Use("/websocket", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/websocket/agv", websocket.New(handlers.NewAGVHandler(cm, br)))
	app.Get("/websocket/web", websocket.New(handlers.NewWebHandler(cm, br, handlers.GetLLMService())))

	log.Println("[INFO] 서버 시작: http://localhost:8001")
	log.Fatal(app.Listen(":8001"))
}
