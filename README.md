# Sion Backend

LoL 사이온 궁극기를 구현한 AGV 프로젝트의 백엔드 서버.
프론트엔드는 [sion-frontend](https://github.com/tolelom/sion-frontend), 로봇 제어는 [sion](https://github.com/tolelom/sion) 참고.

## Tech Stack

- **Language**: Go
- **Framework**: Fiber v2
- **WebSocket**: gofiber/websocket
- **AI**: Ollama (llama3.2)
- **DB**: MySQL + GORM

## Features

- 실시간 WebSocket 통신 (AGV ↔ 서버 ↔ 웹)
- A* 경로 탐색
- LLM 기반 AGV 행동 해설 / 채팅 (클템 스타일)
- AGV 시뮬레이터
- 로그 버퍼링 + 재시도 (MySQL)
- RESTful API

## Getting Started

```bash
git clone https://github.com/tolelom/sion-backend.git
cd sion-backend

# .env 파일에 MYSQL_*, OLLAMA_* 환경변수 설정
go run main.go
```

## License

MIT
