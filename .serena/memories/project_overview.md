# Project Overview

CCX is a multi-upstream AI API proxy and protocol conversion gateway. It currently supports five channel categories: messages, chat, responses, gemini, and images.

Main structure:
- `backend-go/`: Go Gin backend service for routing, auth, scheduling, protocol conversion, metrics, and logs.
- `frontend/`: Vue 3 + Vite + Vuetify admin UI.
- `dist/`: release artifacts; do not edit manually.
- `.config/`: runtime config and persistence such as config.json, metrics.db, backups.
- `refs/`: external reference projects, default read-only.

Versioning:
- Root `VERSION` is the single release version source.
- Backend runtime version is injected by `backend-go/Makefile` through ldflags.

Important route source:
- Actual proxy and admin routes should be verified against `backend-go/main.go`.