# Suggested Commands

Full stack:
- `make dev`: run frontend dev server and backend hot reload.
- `make run`: run the project.
- `make build`: build the project.
- `make frontend-dev`: run only frontend dev from root.
- `make clean`: clean build outputs.

Backend:
- `cd backend-go && make dev`
- `cd backend-go && make run`
- `cd backend-go && make build`
- `cd backend-go && make build-local`
- `cd backend-go && make test`
- `cd backend-go && make test-cover`
- `cd backend-go && make fmt`
- `cd backend-go && make lint`
- `cd backend-go && make deps`

Frontend:
- `cd frontend && bun install`
- `cd frontend && bun run dev`
- `cd frontend && bun run build`
- `cd frontend && bun run preview`
- `cd frontend && bun run type-check`
- `cd frontend && bun run lint`

Docker:
- `docker-compose up -d`

Useful Darwin CLI:
- Prefer `rg` over `grep` for search.
- Use quoted paths for paths with possible spaces.
- Avoid destructive git operations unless explicitly requested.