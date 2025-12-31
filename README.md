# Real-Time Chat Analytics Platform

A scalable real-time messaging platform with analytics capabilities, built with WebSocket technology and Hub-and-Spoke architecture.

## Overview

This platform provides real-time chat functionality with comprehensive analytics tracking. The system is designed to handle concurrent connections efficiently while collecting and analyzing message patterns, user engagement, and system performance metrics.

## Architecture

The platform currently consists of:

- **Backend**: Go-based WebSocket server with Hub-and-Spoke architecture
- **Analytics** (planned): Real-time message analytics and user behavior tracking
- **Frontend** (planned): Web-based chat interface

### Current Implementation

```
┌─────────────────────────────────────────┐
│          WebSocket Clients              │
│  (Browser, Mobile, Desktop)             │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│         Hub (Central Manager)           │
│  • Connection management                │
│  • Message broadcasting                 │
│  • Channel-based communication          │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│         Client Connections              │
│  • Read/Write goroutines                │
│  • Message validation                   │
│  • Ping/Pong keepalive                  │
└─────────────────────────────────────────┘
```

## Features

### Current
- Real-time bidirectional messaging
- WebSocket-based communication
- Hub-and-Spoke connection management
- Type-safe message validation
- Automatic connection cleanup
- Graceful shutdown
- Health check endpoint
- Comprehensive test coverage

### Planned
- Message persistence
- User analytics dashboard
- Active user tracking
- Message rate analytics
- Performance metrics
- Private messaging
- Authentication & authorization
- Frontend chat interface

## Quick Start

### Prerequisites
- Go 1.21 or higher
- Git

### Installation

```bash
# Clone the repository
git clone https://github.com/EPW80/Chat-Analytics-Platform.git
cd Chat-Analytics-Platform

# Navigate to backend
cd backend

# Install dependencies
go mod download

# Run the server
go run cmd/server/main.go
```

The server will start on port 8080 by default.

### Testing the Server

#### Health Check
```bash
curl http://localhost:8080/health
# Response: {"status":"ok","clients":0}
```

#### WebSocket Connection
Install wscat:
```bash
npm install -g wscat
```

Connect and send messages:
```bash
# Terminal 1
wscat -c "ws://localhost:8080/ws?userId=alice&username=Alice"

# Terminal 2
wscat -c "ws://localhost:8080/ws?userId=bob&username=Bob"

# Send a message (in either terminal)
{"type":"chat","content":"Hello everyone!"}
```

## Project Structure

```
RealTimeChatAnalyticsPlatform/
├── backend/                 # Go WebSocket server
│   ├── cmd/
│   │   └── server/         # Main server entry point
│   ├── pkg/
│   │   ├── client/         # WebSocket client handler
│   │   ├── hub/            # Connection manager
│   │   └── message/        # Message types & validation
│   ├── go.mod
│   └── README.md           # Backend documentation
├── .gitignore
└── README.md               # This file
```

## API Documentation

### Endpoints

#### GET /health
Health check endpoint that returns server status and connected client count.

**Response:**
```json
{
  "status": "ok",
  "clients": 5
}
```

#### WS /ws
WebSocket endpoint for real-time chat connections.

**Query Parameters:**
- `userId` (optional): User identifier (defaults to "anonymous")
- `username` (optional): Display name (defaults to "Anonymous")

**Example:**
```
ws://localhost:8080/ws?userId=user123&username=Alice
```

### Message Format

All messages follow this JSON structure:

```json
{
  "type": "chat",
  "userId": "user123",
  "username": "Alice",
  "content": "Hello world",
  "timestamp": "2025-12-30T10:30:00Z"
}
```

**Message Types:**
- `chat`: User chat message
- `system`: System announcement
- `join`: User joined notification
- `leave`: User left notification

**Validation Rules:**
- Username: Required, max 50 characters
- Content (for chat messages): Required, max 1000 characters
- Timestamp: Automatically set to UTC

## Development

### Running Tests
```bash
cd backend

# Run all tests
go test ./...

# Run with race detection
go test -race ./...

# Test coverage
go test -cover ./...
```

### Building
```bash
cd backend
go build -o chat-server cmd/server/main.go
./chat-server
```

### Environment Variables
- `PORT`: Server port (default: 8080)

## Performance

- **Capacity**: 100+ concurrent connections
- **Latency**: <100ms message delivery (p95)
- **Throughput**: Non-blocking broadcast to all clients
- **Reliability**: Automatic connection cleanup, no goroutine leaks

## Technology Stack

### Backend
- **Language**: Go 1.21+
- **WebSocket**: gorilla/websocket v1.5.3
- **Concurrency**: Goroutines and channels
- **Logging**: slog (structured logging)

## Roadmap

- [ ] Frontend chat interface (React/Vue)
- [ ] Message persistence (DynamoDB/PostgreSQL)
- [ ] Real-time analytics dashboard
- [ ] User authentication & authorization
- [ ] Private messaging
- [ ] Rate limiting
- [ ] Docker containerization
- [ ] Kubernetes deployment
- [ ] Load testing suite
- [ ] Horizontal scaling with Redis pub/sub

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Security Notes

This is currently a development implementation. For production use:

1. **Authentication**: Implement proper JWT/OAuth instead of query parameters
2. **CORS**: Restrict allowed origins
3. **Rate Limiting**: Add per-client message rate limits
4. **Input Validation**: Add content filtering and sanitization
5. **TLS**: Use WSS (WebSocket Secure) with proper certificates
6. **Environment Variables**: Use secure secret management

## Documentation

For detailed backend documentation, see [backend/README.md](backend/README.md).

## License

MIT

## Author

Erik Williams ([@EPW80](https://github.com/EPW80))
