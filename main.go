package main

import (
	"log"
	"os"
	"sion-backend/handlers"
	"sion-backend/models"
	"sion-backend/services"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/websocket/v2"
	"github.com/joho/godotenv"
)

// envInt는 환경변수를 int로 파싱한다. 미설정/파싱 실패/0 이하면 fallback을 사용한다.
func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		log.Printf("[WARN] env %s=%q 파싱 실패, fallback=%d 사용", key, v, fallback)
		return fallback
	}
	return n
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("[WARN] .env 파일을 찾을 수 없습니다")
	}

	if err := services.InitDatabase(); err != nil {
		log.Fatalf("[FATAL] DB 초기화 실패: %v", err)
	}

	services.InitLogging(services.LogConfig{
		FlushSize:     envInt("LOG_FLUSH_SIZE", 50),
		FlushInterval: time.Duration(envInt("LOG_FLUSH_INTERVAL_SEC", 10)) * time.Second,
		MaxRetries:    envInt("LOG_MAX_RETRIES", 3),
		MaxFailedLogs: envInt("LOG_MAX_FAILED", 500),
	})
	defer services.StopLogging()

	cm := services.NewClientManager()
	br := services.NewBroker(cm)

	llm := services.NewLLMServiceFromEnv()
	chatH := handlers.NewChatHandler(llm, br)

	sim := services.NewAGVSimulator(func(msg models.WebSocketMessage) {
		br.BroadcastToWeb(msg)
	})

	app := fiber.New()
	app.Use(logger.New())
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = "http://localhost:5173"
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins: allowedOrigins,
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

	api.Post("/chat", chatH.HandleChat)
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
	testAPI.Post("/event", handlers.NewTestEventHandler(br, chatH))

	app.Use("/websocket", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/websocket/agv", websocket.New(handlers.NewAGVHandler(cm, br)))
	app.Get("/websocket/web", websocket.New(handlers.NewWebHandler(cm, br, llm)))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8001"
	}
	log.Printf("[INFO] 서버 시작: http://localhost:%s", port)
	log.Fatal(app.Listen(":" + port))
}
