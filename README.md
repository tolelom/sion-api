# Sion Backend

LoL 사이온 궁극기를 구현한 AGV 프로젝트의 백엔드 서버.
프론트엔드는 [sion-frontend](https://github.com/tolelom/sion-frontend), 로봇 제어는 [sion](https://github.com/tolelom/sion) 참고.

## Tech Stack

- **Language**: Go 1.26
- **Framework**: Fiber v2
- **WebSocket**: gofiber/websocket
- **AI**: Ollama (llama3.2) — lazy, only invoked on chat/event
- **DB**: MySQL + GORM (production) / glebarez/sqlite in-memory (dev/E2E)

## Features

- 실시간 WebSocket 통신 (AGV ↔ 서버 ↔ 웹)
- A* 경로 탐색 (`/algorithms`)
- 책임 분리된 services: `Broker` (라우팅), `ClientManager` (연결 풀), `AGVSimulator`
- LLM 기반 AGV 행동 해설 / 채팅 (클템 스타일)
- 로그 버퍼링 + 재시도 큐 (MySQL)
- RESTful API + WebSocket keepalive (ping/pong)

## Getting Started

```bash
git clone https://github.com/tolelom/sion-backend.git
cd sion-backend
cp .env_example .env

# Production: .env에 MYSQL_*, OLLAMA_* 환경변수 설정
go run .

# Dev/E2E: MySQL 없이 in-memory SQLite로 띄우기
SION_USE_IN_MEMORY_DB=true go run .
```

서버는 `localhost:8001` 에서 동작 (PORT env로 변경 가능).

> `SION_USE_IN_MEMORY_DB=true`는 **dev/E2E 전용**입니다. 프로세스 종료 시 모든 로그가 사라지며, 시작 로그에 WARN이 두 줄 출력됩니다.

## Tests

```bash
go test ./...                      # 전체
CGO_ENABLED=0 go test ./...        # CI와 동일 (CGO 불필요 — glebarez/sqlite)
go test -cover ./...               # 커버리지 출력
go test -v ./services/             # 특정 패키지만
```

CI는 GitHub Actions가 매 push마다 `go test ./...` → Docker 이미지 빌드 → GHCR push까지 자동 수행합니다. 배포 서버는 GHCR을 watch해 latest 이미지를 자동 pull.

## API 요약

| Method | Path | 설명 |
|---|---|---|
| GET  | `/api/health` | 헬스체크 + 클라이언트 수 |
| POST | `/api/chat` | LLM 채팅 |
| POST | `/api/pathfinding` | A* 경로 요청 |
| GET  | `/api/logs/recent` `?limit=...` | 최근 AGV 로그 |
| GET  | `/api/logs/range` `?start&end` | 시간 범위 로그 |
| GET  | `/api/logs/type` `?event_type=...` | 이벤트 타입별 |
| GET  | `/api/logs/stats` `?hours=...` | 이벤트 집계 |
| POST | `/api/simulator/start` | AGV 시뮬레이터 시작 |
| POST | `/api/simulator/stop` | 정지 |
| GET  | `/api/simulator/status` | 상태 |
| POST | `/api/test/{position,status,event}` | 테스트용 직접 주입 |
| WS   | `/websocket/agv` | 로봇 클라이언트 |
| WS   | `/websocket/web` | 웹 클라이언트 |

## 구조

- `/handlers` — REST + WS 핸들러 (`websocket_agv`, `websocket_web`, `ws_keepalive`, ...)
- `/services` — `Broker`, `ClientManager`, `AGVSimulator`, `LLMService`, log buffer/events/query, GORM 초기화
- `/algorithms` — `astar.go`
- `/models` — `AGVStatus`, `Enemy`, `AGVLog`, WebSocket 메시지 envelope (`json.RawMessage` 기반)

## 환경 변수

`.env_example` 참고. 주요 항목:

| 변수 | 기본값 | 설명 |
|---|---|---|
| `PORT` | 8001 | 서버 포트 |
| `ALLOWED_ORIGINS` | `http://localhost:5173` | CORS allowed origins |
| `MYSQL_*` | — | DB 접속 정보 (in-memory 모드면 무시) |
| `SION_USE_IN_MEMORY_DB` | false | true이면 pure-Go SQLite 인메모리 |
| `OLLAMA_BASE_URL` | `http://localhost:11434` | LLM 엔드포인트 |
| `OLLAMA_MODEL` | llama3.2 | 모델명 |
| `LLM_TIMEOUT_SEC` | 60 | LLM 호출 타임아웃 |
| `LOG_FLUSH_SIZE` / `LOG_FLUSH_INTERVAL_SEC` / `LOG_MAX_RETRIES` / `LOG_MAX_FAILED` | — | 로그 버퍼 튜닝 |

## License

MIT
