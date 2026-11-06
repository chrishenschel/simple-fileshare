# Fileshare Design

Lightweight personal file-sharing service: password-gated uploads, PIN-protected short download links, automatic expiry cleanup.

## Goals

- Share a single file via a short URL like `{DOMAIN}/f/AbcdE`
- Protect uploads with a master password (session cookie after login)
- Protect downloads with a 4-digit numeric PIN
- Really delete expired files from disk via a background job
- Stay lightweight: one Go binary in Docker Compose

## Non-goals

- Multi-user accounts / admin UI
- Email notifications
- Virus scanning
- Multi-file uploads or custom short codes
- Persistent rate-limit state across restarts

## Architecture

Single Go HTTP service (stdlib `net/http`) listening on `:8080`, with an in-process cleanup ticker.

```
Browser
  │
  ▼
Go HTTP server (:8080)
  ├── /login          master password → signed session cookie
  ├── /logout          upload UI (auth required)
  ├── /logout          logout
  ├── /f/{id}         PIN gate → download
  └── cleanup ticker  reads root expiry index, deletes folders
  │
  ▼
Volume /data
  ├── expirations.json
  └── {id}/
        ├── info.json
        └── {original filename}
```

### Environment

| Variable | Required | Purpose |
|----------|----------|---------|
| `MASTER_PASSWORD` | yes | Password for upload login |
| `DOMAIN` | yes | Base URL shown in share links (e.g. `https://share.example.com`) |
| `SESSION_SECRET` | yes | HMAC key for session cookies (stable across restarts) |
| `DATA_DIR` | no | Defaults to `/data` |
| `LISTEN_ADDR` | no | Defaults to `:8080` |

### Docker Compose

- Build from local Dockerfile (multi-stage: build Go → scratch/alpine runtime)
- Port `8080:8080`
- `env_file: .env`
- Volume `./data:/data`
- `restart: unless-stopped`

## Auth

- `GET /login` — password form
- `POST /login` — constant-time compare against `MASTER_PASSWORD`
- Success: HMAC-signed session cookie (`HttpOnly`, `SameSite=Lax`; `Secure` when request is HTTPS / `X-Forwarded-Proto=https`)
- Session lifetime: 7 days
- `POST /logout` clears cookie
- `/` (upload) requires valid session; otherwise redirect to `/login`

## Rate limiting

In-memory, per process (acceptable for single-instance deploy):

| Action | Key | Limit |
|--------|-----|-------|
| Master login | client IP | 5 attempts / 15 minutes |
| PIN check | client IP + share id | 5 attempts / 15 minutes |

On limit exceeded: HTTP 429 with a clear “too many attempts, try later” message.

Client IP: prefer `X-Forwarded-For` first hop when present (behind reverse proxy), else `RemoteAddr`.

## Upload flow

Authenticated `GET /` shows:

1. File input
2. 4-digit numeric PIN field
3. Expiry select: **Never**, **1h**, **3h**, **10h**, **24h**, **7d**

`POST /` (multipart):

- Reject files larger than **200 MB**
- Validate PIN is exactly 4 digits (`0000`–`9999` allowed)
- Generate share id: 5 characters from `[A-Za-z0-9]`; on collision, regenerate
- Create `/data/{id}/`, write file under sanitized original basename, write `info.json`
- If expiry ≠ Never: add `{id: expires_at}` to `expirations.json` (atomic rewrite)
- Redirect/render success page with full link `{DOMAIN}/f/{id}` and the PIN

### `info.json` schema

```json
{
  "pin": "1234",
  "filename": "report.pdf",
  "created_at": "2026-11-06T14:00:00Z",
  "expires_at": "2026-11-06T17:00:00Z"
}
```

`expires_at` is `null` when expiry is Never.

### `expirations.json` schema

```json
{
  "AbcdE": "2026-11-06T17:00:00Z"
}
```

Only shares that expire appear here. Never-expiring shares are omitted so the cleanup job does not scan folders.

## Download flow

- `GET /f/{id}` — if folder missing or already past expiry (lazy check), show “file not found or expired”; else show PIN form
- `POST /f/{id}` — rate-limited PIN check against `info.json`; on success stream file with `Content-Disposition: attachment; filename="..."`
- Unlimited downloads until expiry or manual deletion by cleanup
- Wrong PIN: “incorrect PIN”

## Cleanup job

- Ticker every **60 seconds**
- Read `expirations.json`
- For each entry with `expires_at <= now`: `RemoveAll(/data/{id})`, drop key from map
- Write updated `expirations.json` atomically (temp file + rename)
- File lock / single goroutine ownership so upload and cleanup do not corrupt the index (mutex around expirations read/write)

## UI

Server-rendered HTML templates (no SPA):

- Login
- Upload + success with copyable link
- PIN entry + error states
- Rate-limit / not-found pages

Functional, minimal styling. No JS framework required; optional tiny inline JS only if needed for UX (e.g. copy button).

## Error handling summary

| Case | Behavior |
|------|----------|
| Oversize upload | 400 + clear message |
| Invalid PIN format on upload | 400 |
| Rate limited | 429 |
| Wrong download PIN | 403 / form error “incorrect PIN” |
| Missing / expired share | 404 “file not found or expired” |
| Unauthenticated upload | redirect to `/login` |

## Testing notes

- Unit: share id generation, PIN validation, expiry calculation, sanitization
- Integration-ish: upload → PIN download → expiry cleanup deletes folder and index entry
- Rate limit: 6th attempt blocked within window

## Decisions locked in

- Language: Go
- Login: session cookie after master password (option A)
- Expiry options: Never, 1h, 3h, 10h, 24h, 7d
- Max upload: 200 MB
- Downloads: unlimited until expiry
- Approach: single binary + in-process cleanup (approach 1)
