# Sion Backend

LoL 사이온 궁극기를 구현한 AGV 프로젝트의 백엔드 서버.
프론트엔드는 [sion-frontend](https://github.com/tolelom/sion-frontend), 로봇 제어는 [sion](https://github.com/tolelom/sion) 참고.

## Tech Stack

- **Language**: Go
- **Framework**: Fiber v2
- **WebSocket**: gofiber/websocket

## Features

공개 데모 모드로 동작한다. 실제 AGV 대신 서버의 시뮬레이터가 AGV 역할을 하고, 접속자가 있을 때만 움직인다.

- 실시간 WebSocket 통신 (`/websocket/web`): 상태·위치 브로드캐스트, 긴급 정지·모드 변경·경로 이동·초기화 명령
- A* 경로 탐색 (`POST /api/pathfinding`)
- LLM 해설·채팅, MySQL 로그 저장, 실 AGV 연결은 데모에서 뺐다 (git 이력에 남아 있다)

## Getting Started

```bash
git clone https://github.com/tolelom/sion-backend.git
cd sion-backend

# ALLOWED_ORIGINS(쉼표 구분, 기본 http://localhost:5173), PORT(기본 8001)
go run main.go
```

## License

MIT
