# Task Management Backend (Go + Gin)

WebSocket server for real-time task updates in the task management application.

## Overview

This Go backend provides:
- **WebSocket server** for real-time updates across connected clients
- **JWT authentication** middleware (validates BetterAuth tokens from frontend)
- **Broadcast API** for Next.js Server Actions to trigger real-time updates
- **CORS support** for Next.js frontend

## Architecture

```
Frontend (Next.js)
├── Server Actions (Drizzle ORM) → Database (CRUD operations)
└── WebSocket Client → Go Backend → Broadcast to all clients
```

### Data Flow

1. User performs action in Next.js (create/update/delete task)
2. Next.js Server Action saves to database via Drizzle
3. Server Action calls Go backend `/api/broadcast` endpoint
4. Go backend broadcasts update to all connected WebSocket clients
5. Clients receive update and refresh their Zustand store

## Setup

### Prerequisites

- Go 1.21 or higher
- Same `BETTER_AUTH_SECRET` as frontend (for JWT validation)

### Installation

1. Install dependencies:

```bash
go mod tidy
```

2. Create `.env` file:

```bash
cp .env.example .env
```

3. Configure environment variables in `.env`:

```env
PORT=8080
ALLOWED_ORIGINS=http://localhost:3000
JWT_SECRET=your-secret-key-here-match-frontend
```

**Important**: `JWT_SECRET` must match `BETTER_AUTH_SECRET` from frontend `.env.local`

### Running

```bash
# Development
go run cmd/server/main.go

# Build
go build -o bin/server cmd/server/main.go

# Run binary
./bin/server
```

## API Endpoints

### Health Check

```
GET /health
```

Returns server status and connected client count.

**Response:**
```json
{
  "status": "ok",
  "connectedClients": 3
}
```

### WebSocket Connection

```
GET /ws
Headers: Authorization: Bearer <jwt_token>
```

Upgrades HTTP connection to WebSocket. Requires valid JWT token from BetterAuth.

**Message Format:**
```json
{
  "type": "task_created" | "task_updated" | "task_deleted" | "task_status_changed",
  "payload": {
    "id": "uuid",
    "title": "Task title",
    "statusId": "in-progress",
    ...
  }
}
```

### Broadcast API

```
POST /api/broadcast
Headers:
  Authorization: Bearer <jwt_token>
  Content-Type: application/json

Body:
{
  "type": "task_created",
  "data": {
    "id": "uuid",
    "title": "New task"
  }
}
```

Broadcasts a message to all connected WebSocket clients. Called by Next.js Server Actions after database mutations.

## WebSocket Message Types

| Type | Description |
|------|-------------|
| `task_created` | New task created |
| `task_updated` | Task fields updated |
| `task_deleted` | Task deleted |
| `task_status_changed` | Task moved to different status column |
| `ping` | Client heartbeat |
| `pong` | Server heartbeat response |

## Project Structure

```
backend/
├── cmd/
│   └── server/
│       └── main.go              # Entry point, Gin router setup
├── internal/
│   ├── config/
│   │   └── config.go            # Environment configuration
│   ├── middleware/
│   │   └── auth.go              # JWT authentication middleware
│   └── websocket/
│       ├── hub.go               # WebSocket hub (manages clients)
│       ├── client.go            # WebSocket client
│       └── message.go           # Message types and structures
├── .env.example
├── go.mod
└── README.md
```

## Development Notes

- WebSocket connections require authentication (JWT from BetterAuth)
- CORS is configured to allow frontend origins
- Ping/pong heartbeat keeps connections alive (60s timeout)
- Client disconnects are handled gracefully
- Hub broadcasts messages to all connected clients concurrently

## Security

- JWT tokens are validated against `JWT_SECRET`
- CORS restricts origins to `ALLOWED_ORIGINS`
- WebSocket upgrade requires valid authentication
- Maximum message size: 512 bytes (prevents abuse)

## Production Deployment

1. Set strong `JWT_SECRET` (32+ characters, random)
2. Configure `ALLOWED_ORIGINS` with production frontend URL
3. Enable TLS/SSL for WebSocket (`wss://`)
4. Consider rate limiting for broadcast endpoint
5. Monitor connected client count
6. Set up logging and error tracking

## Troubleshooting

**WebSocket connection fails:**
- Check JWT token is valid and not expired
- Verify `JWT_SECRET` matches frontend `BETTER_AUTH_SECRET`
- Check CORS origins include frontend URL

**Clients not receiving updates:**
- Verify `/api/broadcast` is called after database mutations
- Check server logs for broadcast errors
- Ensure WebSocket connection is established before updates

**High memory usage:**
- Check for disconnected clients not being cleaned up
- Monitor `connectedClients` count via `/health` endpoint
- Review client send channel buffer size
