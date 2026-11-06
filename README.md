# Simple Fileshare

Lightweight personal file sharing: master-password uploads, PIN-protected short links, automatic expiry deletion.

## Quick start

```bash
cp .env.example .env
# edit MASTER_PASSWORD, DOMAIN, SESSION_SECRET

mkdir -p data
docker compose up --build -d
```

Open `http://localhost:8080`, sign in, upload a file. Share links look like `{DOMAIN}/f/AbcdE`.

## Behaviour

- Upload page (after login): file, 4-digit PIN, expiry (Never / 1h / 3h / 10h / 24h / 7d)
- Max upload size: **200 MB**
- Download: open `/f/{id}`, enter PIN, download starts
- Unlimited downloads until expiry
- Background job deletes expired share folders every minute
- Rate limits: 5 attempts / 15 minutes for login (per IP) and PIN (per IP + share)

## Storage layout

```
data/
  expirations.json          # id → expiry timestamp (Never omitted)
  AbcdE/
    info.json               # pin, filename, created_at, expires_at
    report.pdf
```

## Local development

```bash
export MASTER_PASSWORD=dev
export DOMAIN=http://localhost:8080
export SESSION_SECRET=0123456789abcdef
export DATA_DIR=./data
go run ./cmd/fileshare
```
