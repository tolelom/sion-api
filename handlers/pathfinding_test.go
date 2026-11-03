package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sion-backend/contract"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func newPathfindingApp() *fiber.App {
	app := fiber.New()
	app.Post("/api/pathfinding", HandlePathfinding)
	return app
}

func doPathfinding(t *testing.T, app *fiber.App, body any) (int, PathfindingResponse) {
	t.Helper()

	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("request 인코딩 실패: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/pathfinding", &buf)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test 실패: %v", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("응답 읽기 실패: %v", err)
	}

	var out PathfindingResponse
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("응답 디코딩 실패: %v (body=%s)", err, string(raw))
		}
	}
	return resp.StatusCode, out
}

type pathfindingFixture struct {
	Name     string          `json:"name"`
	Request  json.RawMessage `json:"request"`
	Raw      string          `json:"raw"`
	Status   int             `json:"status"`
	Response json.RawMessage `json:"response"`
}

func TestHandlePathfinding_ContractFixtures(t *testing.T) {
	var fixtures []pathfindingFixture
	contract.LoadJSON(t, "fixtures/pathfinding.json", &fixtures)
	app := newPathfindingApp()
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			body := f.Raw
			if f.Request != nil {
				body = string(f.Request)
				err := contract.Check(t, "PathfindingRequest", f.Request)
				if wantValid := f.Status == http.StatusOK; (err == nil) != wantValid {
					t.Fatalf("request schema valid=%v, want %v for HTTP %d: %v", err == nil, wantValid, f.Status, err)
				}
			}
			contract.Validate(t, "PathfindingResponse", f.Response)

			req := httptest.NewRequest(http.MethodPost, "/api/pathfinding", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			raw, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != f.Status {
				t.Fatalf("HTTP %d, want %d (body=%s)", resp.StatusCode, f.Status, raw)
			}
			contract.Validate(t, "PathfindingResponse", raw)
			var got, want any
			json.Unmarshal(raw, &got)
			json.Unmarshal(f.Response, &want)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("response\n%s\nwant\n%s", raw, f.Response)
			}
		})
	}
}
