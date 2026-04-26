# Security Review — Public Deployment Readiness

**Date:** 2026-04-26
**Scope:** Authentication, session management, and hardening of `internal/server/`

## Architecture Summary

- Single shared token (`--token` flag, required at startup)
- Session-based auth via HMAC-SHA256 signed cookie (`kb-session`)
- Three auth methods: session cookie, Bearer token, HTTP Basic Auth (git endpoints)
- Auth middleware wraps all routes except `/healthz`, `/login`, `/static/`

## Strengths

| Area | Detail | File |
|------|--------|------|
| Auth coverage | Every route goes through `authMiddleware`; no unprotected endpoints | `auth.go:14-55` |
| Timing safety | `subtle.ConstantTimeCompare` on all token comparisons | `auth.go:25,35`, `handlers.go:30` |
| Cookie signing | Cookie stores `HMAC-SHA256(token)`, not the raw token | `auth.go:57-61` |
| Cookie flags | `HttpOnly`, `SameSite=Lax` | `handlers.go:38-41` |
| File reads | `ReadBlob` reads from git object store, not filesystem — no path traversal | `gitrepo/repo.go:65` |
| Static assets | Embedded via `embed.FS` — no directory traversal | `server.go:23-24` |
| Startup guard | Server refuses to start with empty token | `server.go:72-73` |

## Issues

### 1. CRITICAL — `Secure` cookie flag not set behind reverse proxy

**Location:** `handlers.go:39`

```go
Secure: r.TLS != nil,
```

Behind a TLS-terminating proxy (Caddy, Cloudflare Tunnel, nginx), `r.TLS` is always `nil`. The `Secure` flag is never set, so the cookie is sent over plain HTTP too.

**Fix:** Force `Secure: true` when deployed behind TLS, or detect via `X-Forwarded-Proto: https`. Consider a `--behind-tls` flag.

### 2. HIGH — No rate limiting on `/login`

No per-IP throttling or backoff. An attacker can brute-force the token with unlimited attempts.

**Fix:** In-memory per-IP rate limiter (e.g. token bucket) on `POST /login`. Return `429 Too Many Requests` after threshold.

### 3. MEDIUM — No logout / session invalidation

Sessions last 30 days (`MaxAge: 86400 * 30`). No `/logout` endpoint exists. If a session is compromised, there is no way to revoke it without rotating the token (which invalidates all sessions).

**Fix:** Add `POST /logout` that clears the cookie (`MaxAge: -1`). Optionally track issued sessions server-side for true revocation.

### 4. MEDIUM — No security headers

Missing headers that harden against common web attacks:

| Header | Purpose |
|--------|---------|
| `Strict-Transport-Security` | Force HTTPS |
| `X-Content-Type-Options: nosniff` | Prevent MIME sniffing |
| `X-Frame-Options: DENY` | Prevent clickjacking |
| `Content-Security-Policy` | Restrict script/style sources |

**Fix:** Add a middleware that sets these on every response.

### 5. LOW — CSRF residual risk

`SameSite=Lax` blocks cross-origin `POST` cookies in modern browsers, which covers `POST /api/settings/pull`, `POST /api/settings/reindex`, and `POST /flashcards/review/{hash}`. `PUT`/`DELETE` on `/api/bookmarks` trigger CORS preflight, so they are safe.

Residual risk: very old browsers without `SameSite` support. Acceptable for a personal knowledge base.

### 6. LOW — Error detail in toasts

`settings.go:42-46` returns raw `err.Error()` and git CLI output in toast HTML. Not exploitable (endpoint is authenticated) but could leak internal paths to authenticated users who share a screen.

## Recommendation

Minimum changes before public deployment:

1. Force `Secure` cookie flag (or add `--behind-tls` / detect `X-Forwarded-Proto`)
2. Rate limit `POST /login`
3. Add `POST /logout`
4. Add security headers middleware
