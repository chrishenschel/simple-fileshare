# Fileshare Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans (inline; user requested immediate implementation).

**Goal:** Ship a lightweight Go file-share service with master-password upload, PIN-protected short links, expiry cleanup, and Docker Compose.

**Architecture:** Single Go binary (`cmd/fileshare`) with stdlib HTTP, HMAC session cookies, in-memory rate limits, filesystem storage under `/data`, and a 60s cleanup ticker.

**Tech Stack:** Go 1.24, stdlib only, Docker multi-stage build, Compose volume `./data:/data`.

## Global Constraints

- Max upload: 200 MB
- Share id: 5 chars `[A-Za-z0-9]`
- Expiry options: Never, 1h, 3h, 10h, 24h, 7d
- Rate limits: 5 / 15 min (login by IP; PIN by IP+id)
- Env required: `MASTER_PASSWORD`, `DOMAIN`, `SESSION_SECRET`
- Storage: `/data/{id}/info.json` + file; `/data/expirations.json`

## File map

| Path | Responsibility |
|------|----------------|
| `cmd/fileshare/main.go` | Wire config, storage, cleanup, HTTP server |
| `internal/config/config.go` | Load/validate env |
| `internal/auth/session.go` | Sign/verify session cookies |
| `internal/ratelimit/limiter.go` | Sliding/window rate limiter |
| `internal/idgen/idgen.go` | Random 5-char ids |
| `internal/storage/storage.go` | Shares + expirations index |
| `internal/cleanup/cleanup.go` | Ticker deleting expired shares |
| `internal/web/*.go` + `templates/` | Handlers + HTML |
| `Dockerfile`, `docker-compose.yml`, `.env.example` | Deploy |

### Task 1: Core packages (config, id, rate limit, auth, storage, cleanup)

- [x] Implement packages with unit tests for id, ratelimit, config validation, expiry helpers
- [x] Commit

### Task 2: HTTP handlers + templates

- [x] Login, upload, download PIN flow, logout
- [x] Commit

### Task 3: Main, Docker, README

- [x] `cmd/fileshare/main.go`, Docker assets, `.gitignore`, README
- [x] Commit + push to origin
---
