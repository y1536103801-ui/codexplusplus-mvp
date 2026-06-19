# 00 Environment

Run folder: 20260619-1940-e2e
Status: pass

## Versions And Scope

- Backend/gateway: local Docker service at `http://127.0.0.1:8081`, health check returned `status=ok`.
- Desktop build: `codex-plus-plus-manager.exe` release build timestamp `2026-06-19 19:34:34`, version `1.2.9`.
- Contract/config: client OpenAPI subset, gateway policy seeded cases, and config version `codexplus-mvp-1` verified by local E2E runner.
- MVP scope: Windows-only local MVP. macOS x64 and macOS arm64 are tracked outside this MVP release gate.

## Environment

- Test date: 2026-06-19.
- Executor: local Codex run with owner authorization.
- Environment owner: repository owner authorized isolated Desktop Manager/Codex local E2E.
- Base URLs: local loopback only, `127.0.0.1` endpoints; no production URL used.
- Turnstile status: Turnstile enabled for production-equivalent browser-login policy boundary; local mock uses authorized browser handoff token.

## Redaction

- Redaction result: pass.
- No real API key, JWT, refresh token, Authorization header, upstream provider key, poll token, or session token is present in this evidence file.
