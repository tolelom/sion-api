package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sion-backend/handlers"
	"sion-backend/models"
	"sion-backend/services"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/websocket/v2"
)

const shutdownTimeout = 5 * time.Second

func main() {
	cm := services.NewClientManager(services.DefaultWriteTimeout)
	sim := services.NewAGVSimulator(func(msg models.WebSocketMessage) { cm.BroadcastJSON(msg) }, func() bool { return cm.Count() > 0 })
	sim.Start()
	defer sim.Stop()

	app := newApp(cm, sim, envOr("ALLOWED_ORIGINS", "http://localhost:5173"))
	port := envOr("PORT", "8001")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	listenErr := make(chan error, 1)
	go func() { listenErr <- app.Listen(":" + port) }()
	log.Printf("[INFO] 서버 시작: http://localhost:%s", port)

	select {
	case err := <-listenErr:
		log.Printf("[ERROR] 서버 실행 실패: %v", err)
		sim.Stop()
		os.Exit(1)
	case <-ctx.Done():
	}
	log.Printf("[INFO] 종료 신호를 받아 서버를 멈춘다")
	// 업그레이드된 WebSocket 연결은 기다리지 않는다. 진행 중인 HTTP 요청만 마무리한다.
	if err := app.ShutdownWithTimeout(shutdownTimeout); err != nil {
		log.Printf("[WARN] 서버 종료 실패: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func newApp(cm *services.ClientManager, sim *services.AGVSimulator, allowedOrigins string) *fiber.App {
	app := fiber.New(fiber.Config{BodyLimit: 64 * 1024})
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: allowedOrigins,
		AllowHeaders: "Origin, Content-Type, Accept",
		AllowMethods: "GET, POST, OPTIONS",
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Sion WebSocket 서버가 실행 중입니다.")
	})

	api := app.Group("/api")
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "OK",
			"clients": cm.Count(),
			"time":    time.Now().Format(time.RFC3339),
		})
	})
	api.Post("/pathfinding", handlers.HandlePathfinding)

	app.Use("/websocket", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/websocket/web", websocket.New(handlers.NewWebHandler(cm, sim), websocket.Config{
		Origins: originList(allowedOrigins),
	}))
	return app
}

func originList(origins string) []string {
	var out []string
	for _, o := range strings.Split(origins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			out = append(out, o)
		}
	}
	return out
}
