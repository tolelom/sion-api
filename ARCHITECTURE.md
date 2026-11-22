# 아키텍처

## 시스템 구성

| 저장소 | 역할 |
|---|---|
| [sion-web](https://github.com/tolelom/sion-web) | 관제 대시보드 |
| [sion-api](https://github.com/tolelom/sion-api) (이 저장소) | 대시보드용 WebSocket·REST 서버. 내장 시뮬레이터가 AGV 역할 |
| [sion-robot](https://github.com/tolelom/sion-robot) | JETANK 제어. 경로 계획, 경로 추종, 적 돌진. 서버와 연결 없음 |

구성도는 `README.md` 맨 위에 있습니다. 대시보드와 주고받는 메시지 모양은 이 저장소의 `contract/`가 정하고, sion-web과 sion-robot은 그 복사본을 씁니다.

## 코드 지도

| 경로 | 책임 |
|---|---|
| `main.go` | 앱 구성: 라우트, CORS, 본문 한도, 시뮬레이터 시작, 종료 신호 처리 |
| `handlers/websocket_web.go` | 대시보드 WebSocket: 접속 메시지, 명령 수신, 명령 속도 제한 |
| `handlers/ws_keepalive.go` | ping/pong keepalive |
| `handlers/pathfinding.go` | `POST /api/pathfinding` 요청 검증과 A* 호출 |
| `services/simulator.go` | AGV 시뮬레이터: 수명주기, 틱 루프(접속자가 없으면 건너뜀), 브로드캐스트 메시지 |
| `services/simulator_commands.go` | 시뮬레이터 WebSocket 명령 처리 |
| `services/simulator_behavior.go` | 적 탐지·돌진, 경로 추종, 배터리 소모 |
| `services/simulator_world.go` | 장애물·적 생성 |
| `services/client_manager.go` | 접속 관리, 쓰기 타임아웃, 브로드캐스트 |
| `algorithms/astar.go` | 격자 A* |
| `models/` | 메시지 본문, 상태, 맵 타입 |
| `contract/` | 대시보드와의 계약: JSON Schema, 메시지 예시, 경로 탐색 픽스처, sion-robot과 같이 쓰는 A* 케이스 |
