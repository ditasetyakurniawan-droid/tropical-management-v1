# General Live Chat

Phase 3 adds a dedicated `chat-service` microservice for authenticated team communication.

## Behavior

- Every authenticated Admin, Auditor, and Staff user can read and send messages in one general room.
- The browser cannot choose the displayed name or role. `api-gateway` verifies the JWT and forwards trusted `X-User-ID`, `X-User-Name`, and `X-User-Role` headers to `chat-service`.
- Messages are persisted in MySQL.
- New messages are pushed in real time with Server-Sent Events (SSE); posting uses normal HTTP `POST`.
- User-name accent colors are deterministic in the UI, while role is shown as a separate badge.
- Message length is limited to 1000 Unicode characters.

## Endpoints

```text
GET  /api/chat/messages?limit=100
POST /api/chat/messages
GET  /api/chat/stream
```

POST payload:

```json
{
  "body": "Kitchen closing checklist sudah selesai."
}
```

## Local database note

Existing local Compose users may already have a populated MySQL volume created before `tropical_chat` existed. To avoid deleting their local data, `docker-compose.yml` currently points `chat-service` at the existing `tropical_auth` schema; the service still owns only its `chat_messages` table.

For production, use a dedicated database and scoped MySQL user, for example `tropical_chat`, with `CHAT_DB_DSN` injected by Vault. The fresh-volume bootstrap SQL already creates `tropical_chat`.

## Local acceptance

```bash
docker compose build chat-service api-gateway web
docker compose up -d chat-service api-gateway web
docker compose ps
```

Login in two separate browsers/incognito sessions using different users and open:

```text
http://localhost:3000/chat
```

A message sent from one session should appear in the other without refreshing the page.
