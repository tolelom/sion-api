# 기여

```bash
go test -race ./...   # 테스트
go vet ./...          # 정적 검사
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o server .   # 컨테이너용 바이너리(Dockerfile이 server를 복사)
```

저장소 역할과 코드 지도는 `ARCHITECTURE.md`에 있습니다.

## 계약

sion-web과 주고받는 모양은 `contract/`가 정합니다.

| 파일 | 내용 |
|---|---|
| `contract/sion.schema.json` | JSON Schema(draft 2020-12). WebSocket 서버 메시지(`ServerMessage`), 대시보드 명령(`ClientCommand`), `PathfindingRequest`·`PathfindingResponse` |
| `contract/fixtures/server/*.json` | 서버 메시지 종류별 예시 |
| `contract/fixtures/client/*.json` | 명령 종류별로 받아들이는 예시(`accepted`, 뒤따르는 시뮬레이터 상태 `then`)와 거절하는 예시(`rejected`, 오류 문구 `error`). `invalid.json`은 형식이 틀린 메시지 |
| `contract/fixtures/pathfinding.json` | 경로 탐색 요청과 HTTP 상태·응답 |
| `contract/pathfinding-cases.json` | sion-robot `astar.py`와 같이 쓰는 A* 케이스(비용·칸 수, 유일할 때만 경로) |

`go test ./...`가 실제 브로드캐스트·접속 메시지·오류 응답을 스키마로 검사하고, 픽스처를 Go 타입과 실제 핸들러에 통과시킵니다. 서버 메시지는 스키마에 없는 필드를 허용하지 않으므로 필드를 더하거나 바꾸면 스키마를 같이 고쳐야 테스트가 통과합니다. 거절 예시는 스키마도 어겨야 합니다.

계약을 바꿀 때:

1. `contract/`의 스키마와 픽스처를 고치고 `go test ./...`를 통과시킵니다.
2. sion-web에서 `npm run contract:sync`로 복사하고 `npm run contract:types`로 타입을 다시 만듭니다(sion-web `CONTRIBUTING.md` 참고).
3. A* 케이스를 바꿨다면 sion-robot `tests/fixtures/pathfinding-cases.json`에도 복사합니다.

## 배포

GitHub Actions(`.github/workflows/ci.yml`)가 PR과 push마다 정적 검사, 테스트, arm64 빌드를 돌립니다. `main`에 push하면 arm64 이미지 `ghcr.io/tolelom/sion-api:latest`, `ghcr.io/tolelom/sion-api:main`을 올립니다.

`.github/workflows/vuln.yml`은 `main` push, PR, 매주 월요일에 govulncheck를 돌려 호출 경로에 알려진 취약점이 있으면 실패합니다.
