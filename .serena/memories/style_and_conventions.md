# Style And Conventions

General:
- Always respond to the user in Simplified Chinese.
- Follow SOLID, KISS, DRY, and YAGNI.
- Prefer root-cause fixes and keep changes scoped.
- Do not plan or execute git commits, branches, pushes, or resets unless the user explicitly asks.

Go:
- Keep package responsibilities focused and interfaces clear.
- Run `go fmt ./...` or `cd backend-go && make fmt` after Go changes.
- Prefer table-driven tests and `httptest` for backend logic.

Frontend:
- Vue 3 + Vite + Vuetify with TypeScript.
- Follow existing Vuetify, TypeScript, and Prettier style.
- Keep strict typing.

Generated artifacts:
- Do not manually edit `dist/`, `frontend/dist/`, or `backend-go/frontend/dist/`.

Security:
- Do not commit real secrets from `.env` or JSON config files.
- Redact API keys, Authorization headers, and multipart-sensitive content in logs and summaries.