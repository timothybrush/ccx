# Task Completion Checklist

For backend code changes:
- Run formatting: `cd backend-go && make fmt` or equivalent `go fmt ./...`.
- Run focused tests first when available.
- Prefer `cd backend-go && make test` for backend verification.

For frontend code changes:
- Run `cd frontend && bun run build`.
- Use `bun run type-check` and `bun run lint` when relevant.

For docs or interface changes:
- Verify facts against `VERSION`, `Makefile`, `backend-go/Makefile`, `frontend/package.json`, and `backend-go/main.go` as applicable.
- Recommended broad checks: `make build`, `cd backend-go && make test`, `cd frontend && bun run build`.

Always report what was changed and what verification was run. If verification could not be run, state why.