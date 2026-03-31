# Sion Backend

LoL 사이온 궁극기를 구현한 AGV 프로젝트의 백엔드 서버.
프론트엔드는 [sion-frontend](https://github.com/tolelom/sion-frontend), 로봇 제어는 [sion](https://github.com/tolelom/sion) 참고.

## Tech Stack

- **Language**: Go
- **Framework**: Fiber v2
- **WebSocket**: gofiber/websocket
- **AI**: OpenAI GPT-4o-mini

## Features

- 실시간 WebSocket 통신 (AGV ↔ 서버 ↔ 웹)
- A* 경로 탐색
- LLM 기반 AGV 행동 해설 / 채팅
- RESTful API

## Getting Started

```bash
git clone https://github.com/tolelom/sion-backend.git
cd sion-backend

# .env 파일에 OPENAI_API_KEY 설정
go run main.go
```

## License

MIT
