# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
# Go Server — build and start (requires MySQL)
cd server && go build -o sms-server .
PORT=8080 DB_HOST=127.0.0.1 DB_PORT=3306 DB_USER=smsuser DB_PASS=smspass DB_NAME=sms_rec JWT_SECRET=your-secret ./sms-server

# Windows WPF client — build (requires .NET 8 SDK)
cd windows/SmsNotifier && dotnet build

# Android client — open android/ in Android Studio, sync Gradle, build APK
```

## Project Architecture

SMS forwarding system: Android phone receives SMS → Go Server forwards via WebSocket → Windows desktop toast notification. Multi-user with registration/login.

| Layer | Dir | Protocol |
|-------|-----|----------|
| Server | `server/` | REST + WebSocket (port from env) |
| Android | `android/` | OkHttp WS client + Retrofit REST |
| Windows | `windows/SmsNotifier/` | ClientWebSocket + HttpClient |

## Server Internals

Go server uses `net/http` with Go 1.22+ enhanced mux routing (`"POST /api/register"` syntax).

**Startup flow** in `main.go`: config env vars → MySQL connect + auto-migrate tables → start Hub goroutine → register REST + WebSocket routes → CORS wrapper.

**WebSocket Hub** (`hub/hub.go`): connection pool indexed by `userID`, channel-based concurrency (`Register`/`Unregister`/`Broadcast`). `WritePump` sends with ping keepalive (30s). `readPump` in `api/api.go` handles incoming messages — on `sms_received`, stores to DB then broadcasts `sms_deliver` to same user's other devices.

**Message format**: `{"type":"...","data":{...}}`. Types: `sms_received` (Android→Server), `sms_deliver` (Server→Windows), `ack`, `connection_status`, `ping`/`pong`.

**Auth**: bcrypt password hashing, JWT with 72h expiry. Public endpoints: `POST /api/register`, `POST /api/login`. All other `/api/` routes go through JWT middleware (Bearer token in Authorization header). WebSocket authenticates via `?token=<JWT>` query param.

## MySQL Setup

```sql
CREATE DATABASE IF NOT EXISTS sms_rec CHARACTER SET utf8mb4;
CREATE USER IF NOT EXISTS 'smsuser'@'localhost' IDENTIFIED BY 'smspass';
GRANT ALL ON sms_rec.* TO 'smsuser'@'localhost';
```

Tables (auto-created by `store.Migrate()`): `users`, `devices`, `sms_logs`, `connection_logs`.

## Config (env vars)

`PORT`, `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASS`, `DB_NAME`, `JWT_SECRET` — all with defaults in `config/config.go`.

## Remote Access

- Android emulator uses `10.0.2.2` to reach host; physical device needs server LAN IP
- Windows client defaults to `http://localhost:8080`
- Server accessible at `http://<server-ip>:<port>`
