package main

import (
	"net/http"
	"net/http/httptest"
	"sion-backend/handlers"
	"sion-backend/services"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func newTestRouteApp(t *testing.T) *fiber.App {
	t.Helper()
	cm := services.NewClientManager()
	br := services.NewBroker(cm)
	chatH := handlers.NewChatHandler(nil, br)

	app := fiber.New()
	api := app.Group("/api")
	registerTestRoutes(api, br, chatH)
	return app
}

func postTestRoute(t *testing.T, app *fiber.App, path string) int {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(http.MethodPost, path, nil), -1)
	if err != nil {
		t.Fatalf("app.Test 실패: %v", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

var testRoutePaths = []string{
	"/api/test/position",
	"/api/test/status",
	"/api/test/event",
}

func TestRegisterTestRoutes_기본값은_비활성(t *testing.T) {
	app := newTestRouteApp(t)

	for _, path := range testRoutePaths {
		if status := postTestRoute(t, app, path); status != http.StatusNotFound {
			t.Errorf("%s: 플래그 미설정 시 404 기대, got %d", path, status)
		}
	}
}

func TestRegisterTestRoutes_Falsy값도_비활성(t *testing.T) {
	for _, v := range []string{"", "false", "0", "no", "maybe"} {
		t.Run("SION_ENABLE_TEST_API="+v, func(t *testing.T) {
			t.Setenv("SION_ENABLE_TEST_API", v)
			app := newTestRouteApp(t)

			for _, path := range testRoutePaths {
				if status := postTestRoute(t, app, path); status != http.StatusNotFound {
					t.Errorf("%s: %q일 때 404 기대, got %d", path, v, status)
				}
			}
		})
	}
}

func TestRegisterTestRoutes_활성화시_동작(t *testing.T) {
	for _, v := range []string{"1", "true", "TRUE", "yes"} {
		t.Run("SION_ENABLE_TEST_API="+v, func(t *testing.T) {
			t.Setenv("SION_ENABLE_TEST_API", v)
			app := newTestRouteApp(t)

			for _, path := range testRoutePaths {
				if status := postTestRoute(t, app, path); status != http.StatusOK {
					t.Errorf("%s: %q일 때 200 기대, got %d", path, v, status)
				}
			}
		})
	}
}

func TestRegisterTestRoutes_활성_여부를_반환(t *testing.T) {
	cm := services.NewClientManager()
	br := services.NewBroker(cm)
	chatH := handlers.NewChatHandler(nil, br)

	app := fiber.New()
	if enabled := registerTestRoutes(app.Group("/api"), br, chatH); enabled {
		t.Error("플래그 미설정 시 false 기대, got true")
	}

	t.Setenv("SION_ENABLE_TEST_API", "true")
	app2 := fiber.New()
	if enabled := registerTestRoutes(app2.Group("/api"), br, chatH); !enabled {
		t.Error("플래그 설정 시 true 기대, got false")
	}
}
