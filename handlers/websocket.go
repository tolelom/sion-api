package handlers

import (
	"encoding/json"
	"log"
	"sion-backend/models"
	"sion-backend/services"
	"sync"
	"time"

	"github.com/gofiber/websocket/v2"
)

// Deprecated: currentAGVStatus is kept for backward compatibility until websocket.go is removed (Task 5)
var currentAGVStatus *models.AGVStatus

type Client struct {
	Conn       *websocket.Conn
	ClientType string // "agv" 또는 "web"
}

// 클라이언트 관리자
type ClientManager struct {
	clients    map[*websocket.Conn]*Client
	broadcast  chan models.WebSocketMessage
	register   chan *Client
	unregister chan *websocket.Conn
	mutex      sync.RWMutex
}

// 전역 클라이언트 관리자
var Manager = &ClientManager{
	clients:    make(map[*websocket.Conn]*Client),
	broadcast:  make(chan models.WebSocketMessage, 100),
	register:   make(chan *Client),
	unregister: make(chan *websocket.Conn),
}

// 클라이언트 관리 시작
func (manager *ClientManager) Start() {
	for {
		select {
		case client := <-manager.register:
			manager.mutex.Lock()
			manager.clients[client.Conn] = client
			manager.mutex.Unlock()
			log.Printf("클라이언트 등록: %s (%s)", client.ClientType, client.Conn.RemoteAddr())

		case conn := <-manager.unregister:
			manager.mutex.Lock()
			if client, ok := manager.clients[conn]; ok {
				delete(manager.clients, conn)
				_ = conn.Close()
				log.Printf("클라이언트 해제: %s (%s)", client.ClientType, conn.RemoteAddr())
			}
			manager.mutex.Unlock()
		case message := <-manager.broadcast:
			manager.handleBroadcast(message)
		}
	}
}

func (manager *ClientManager) handleBroadcast(message models.WebSocketMessage) {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	for conn, client := range manager.clients {
		// 메시지 타입에 따라 전송 대상 결정
		shouldSend := false

		switch message.Type {
		case models.MessageTypePosition,
			models.MessageTypeStatus,
			models.MessageTypeLog,
			models.MessageTypeTargetFound,
			models.MessageTypePathUpdate,
			models.MessageTypeChatResponse,
			models.MessageTypeAGVEvent:
			// AGV에서 Web으로 전송
			if client.ClientType == "web" {
				shouldSend = true
			}
		case models.MessageTypeCommand,
			models.MessageTypeModeChange,
			models.MessageTypeEmergencyStop:
			// Web에서 AGV로 전송
			if client.ClientType == "agv" {
				shouldSend = true
			}
		case models.MessageTypeLLMExplanation,
			models.MessageTypeTTS,
			models.MessageTypeMapUpdate,
			models.MessageTypeSystemInfo:
			// 모든 Web 클라이언트에게 전송
			if client.ClientType == "web" {
				shouldSend = true
			}
		}

		if shouldSend {
			err := conn.WriteJSON(message)
			if err != nil {
				log.Printf("전송 실패 (%s): %v", client.ClientType, err)
				manager.unregister <- conn
			}
		}
	}
}

// 외부에서 호출할 수 있는 브로드캐스트 메서드
func (manager *ClientManager) BroadcastMessage(msg models.WebSocketMessage) {
	manager.broadcast <- msg
}

func (manager *ClientManager) GetClientCount() map[string]int {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	count := map[string]int{
		"agv": 0,
		"web": 0,
	}

	for _, client := range manager.clients {
		count[client.ClientType]++
	}

	return count
}

// AGV WebSocket Handler
func HandleAGVWebSocket(c *websocket.Conn) {
	client := &Client{
		Conn:       c,
		ClientType: "agv",
	}

	Manager.register <- client

	defer func() {
		Manager.unregister <- c
	}()

	for {
		var msg models.WebSocketMessage
		err := c.ReadJSON(&msg)
		if err != nil {
			log.Printf("AGV 메시지 읽기 오류: %v", err)
			break
		}

		// 타임스탬프 추가
		if msg.Timestamp == 0 {
			msg.Timestamp = time.Now().UnixMilli()
		}

		// 🆕 경로 업데이트 메시지 상세 로깅
		if msg.Type == models.MessageTypePathUpdate {
			logPathUpdate(msg)
		}

		// 로깅 추가
		go services.LogAGVEvent(msg, "sion-001", "agv")

		log.Printf("AGV 메시지: %s - Timestamp: %d", msg.Type, msg.Timestamp)

		// 모든 메시지 브로드캐스트
		Manager.broadcast <- msg
	}
}

// 🆕 경로 업데이트 메시지 상세 로깅
func logPathUpdate(msg models.WebSocketMessage) {
	log.Printf("\n=== 🗺️  경로 업데이트 수신 ===")

	if dataMap, ok := msg.Data.(map[string]interface{}); ok {
		// 경로 정보 추출
		if points, ok := dataMap["points"].([]interface{}); ok {
			log.Printf("📍 경로 포인트 개수: %d", len(points))

			// 각 포인트 상세 출력
			for i, p := range points {
				if pointMap, ok := p.(map[string]interface{}); ok {
					if x, ok := pointMap["x"].(float64); ok {
						if y, ok := pointMap["y"].(float64); ok {
							if angle, ok := pointMap["angle"].(float64); ok {
								log.Printf("  [%d] x=%.2f, y=%.2f, angle=%.4f", i, x, y, angle)
							}
						}
					}
				}
			}
		}

		if length, ok := dataMap["length"].(float64); ok {
			log.Printf("📏 경로 길이: %.2f", length)
		}

		if algorithm, ok := dataMap["algorithm"].(string); ok {
			log.Printf("🔧 알고리즘: %s", algorithm)
		}
	}

	// 전체 데이터 JSON 출력
	if jsonData, err := json.MarshalIndent(msg.Data, "", "  "); err == nil {
		log.Printf("📄 전체 경로 데이터:\n%s", string(jsonData))
	}

	log.Printf("✅ AGV 경로가 프론트엔드로 전달됩니다\n")
}

// Web 클라이언트 WebSocket Handler (채팅 + LLM 연동)
func HandleWebClientWebSocket(c *websocket.Conn) {
	client := &Client{
		Conn:       c,
		ClientType: "web",
	}

	Manager.register <- client

	defer func() {
		Manager.unregister <- c
	}()

	// 연결 확인 메시지 전송
	welcomeMsg := models.WebSocketMessage{
		Type: models.MessageTypeSystemInfo,
		Data: map[string]interface{}{
			"message":      "웹 클라이언트 연결됨",
			"connected_at": time.Now().Format(time.RFC3339),
		},
		Timestamp: time.Now().UnixMilli(),
	}
	_ = c.WriteJSON(welcomeMsg)

	for {
		var msg models.WebSocketMessage
		err := c.ReadJSON(&msg)
		if err != nil {
			log.Printf("웹 메시지 읽기 오류: %v", err)
			break
		}

		// 타임스탬프 추가
		if msg.Timestamp == 0 {
			msg.Timestamp = time.Now().UnixMilli()
		}

		// 로깅 추가
		go services.LogAGVEvent(msg, "sion-001", "web-user")

		log.Printf("웹 메시지: %s - %+v", msg.Type, msg.Data)

		// 🆕 채팅 메시지 처리 (LLM AnswerQuestion 호출)
		switch msg.Type {
		case models.MessageTypeChat:
			if chatData, ok := msg.Data.(map[string]interface{}); ok {
				if message, ok := chatData["message"].(string); ok {
					log.Printf("💬 사용자 질문: %s", message)

					go func() {
						// LLM 서비스만 체크, AGV 상태는 선택적
						if llmService == nil {
							log.Printf("⚠️ LLM 서비스가 초기화되지 않음")
							return
						}

						// AGV 상태가 있으면 전달, 없으면 nil로 전달
						var status *models.AGVStatus
						if currentAGVStatus != nil {
							status = currentAGVStatus
							log.Printf("📊 AGV 상태 포함하여 LLM 호출")
						} else {
							log.Printf("📝 AGV 상태 없이 LLM 호출")
						}

						response, err := llmService.AnswerQuestion(message, status)
						if err != nil {
							log.Printf("❌ LLM 응답 실패: %v", err)
							return
						}

						responseMsg := models.WebSocketMessage{
							Type: models.MessageTypeChatResponse,
							Data: models.ChatResponseData{
								Message:   response,
								Model:     llmService.Model,
								Timestamp: time.Now().UnixMilli(),
							},
							Timestamp: time.Now().UnixMilli(),
						}

						Manager.BroadcastMessage(responseMsg)
						log.Printf("✅ AI 응답 전송: %s", response[:min(50, len(response))]+"...")
					}()

				}
			}

		case models.MessageTypeCommand,
			models.MessageTypeModeChange,
			models.MessageTypeEmergencyStop:
			// 명령 메시지는 AGV로 브로드캐스트
			Manager.broadcast <- msg

		default:
			log.Printf("알 수 없는 메시지 타입: %s", msg.Type)
		}
	}
}
