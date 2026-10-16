package main

import (
	"log"
	"os"
	"sion-backend/handlers"
	"sion-backend/models"
	"sion-backend/services"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/websocket/v2"
)

func main() {
	cm := services.NewClientManager()
	sim := services.NewAGVSimulator(func(msg models.WebSocketMessage) { cm.BroadcastJSON(msg) }, func() bool { return cm.Count() > 0 })
	sim.Start()

	app := fiber.New(fiber.Config{BodyLimit: 64 * 1024})
	app.Use(recover.New())
	app.Use(logger.New())
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = "http://localhost:5173"
	}
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

	port := os.Getenv("PORT")
	if port == "" {
		port = "8001"
	}
	log.Printf("[INFO] 서버 시작: http://localhost:%s", port)
	log.Fatal(app.Listen(":" + port))
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
