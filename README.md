# Real-Time Chat Analytics Platform

A scalable real-time messaging platform with analytics capabilities, built with WebSocket technology and Hub-and-Spoke architecture.

## Overview

This platform provides real-time chat functionality with comprehensive analytics tracking. The system is designed to handle concurrent connections efficiently while collecting and analyzing message patterns, user engagement, and system performance metrics.

## Architecture

The platform currently consists of:

- **Backend**: Go-based WebSocket server with Hub-and-Spoke architecture
- **Storage**: DynamoDB for message persistence
- **Analytics** (planned): Real-time message analytics and user behavior tracking
- **Frontend** (planned): Web-based chat interface

### Current Implementation (Phase 2A)

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
│  • Non-blocking persistence             │
└──────────┬──────────────────────────────┘
           │
           ▼
┌─────────────────────────────────────────┐
│      DynamoDB (Message Storage)         │
│  • Message history                      │
│  • User-based queries (GSI)             │
│  • Room-based queries (GSI)             │
└─────────────────────────────────────────┘
```

## Features

### Current (Phase 1 + 2A)
- Real-time bidirectional messaging
- WebSocket-based communication
- Hub-and-Spoke connection management
- Type-safe message validation
- Automatic connection cleanup
- Graceful shutdown
- Health check endpoint
- Comprehensive test coverage
- **Message persistence** (DynamoDB) - Phase 2A ✅
- **Non-blocking async storage** - Phase 2A ✅
- **Docker Compose setup** - Phase 2A ✅
- **DynamoDB Local integration** - Phase 2A ✅
- **AWS SDK v2 integration** - Phase 2A ✅

### In Progress (Phase 2A)
- AWS deployment preparation (ECS, CloudFormation)
- Production Dockerfile
- Integration and load tests

### Planned (Phase 2B+)
- User analytics dashboard
- Active user tracking
- Message rate analytics
- Performance metrics
- Private messaging
- Authentication & authorization
- Frontend chat interface (React)

## Quick Start

### Option 1: Docker Compose (Recommended)

**Prerequisites:**
- Docker and Docker Compose
- Git

```bash
# Clone the repository
git clone https://github.com/EPW80/Chat-Analytics-Platform.git
cd Chat-Analytics-Platform

# Start services (Backend + DynamoDB Local)
docker-compose up -d

# Check health
curl http://localhost:8080/health
# Response: {"status":"ok","clients":0}

# Initialize DynamoDB tables (optional - for persistence)
./scripts/init-tables.sh

# View logs
docker-compose logs -f backend

# Stop services
docker-compose down
```

**Services:**
- Backend: `http://localhost:8080`
- DynamoDB Local: `http://localhost:8000`
- WebSocket: `ws://localhost:8080/ws`

### Option 2: Local Go Development

**Prerequisites:**
- Go 1.23 or higher
- Git

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
├── backend/                    # Go WebSocket server
│   ├── cmd/
│   │   └── server/            # Main server entry point
│   ├── pkg/
│   │   ├── client/            # WebSocket client handler
│   │   ├── config/            # Environment configuration
│   │   ├── hub/               # Connection manager
│   │   ├── message/           # Message types & validation
│   │   └── storage/           # DynamoDB persistence (Phase 2A)
│   │       ├── interface.go   # Repository interface
│   │       ├── dynamodb.go    # DynamoDB implementation
│   │       └── schema.go      # Table schema
│   ├── scripts/
│   │   └── init-dynamodb.go   # Table initialization
│   ├── Dockerfile             # Development container
│   ├── .dockerignore
│   ├── go.mod
│   └── README.md              # Backend documentation
├── scripts/
│   ├── wait-for-dynamodb.sh   # Health check script
│   └── init-tables.sh         # Table setup wrapper
├── docker-compose.yml         # Local development stack
├── .env.example               # Environment template
├── .gitignore
├── claude.md                  # Implementation plan
└── README.md                  # This file
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
  "messageId": "550e8400-e29b-41d4-a716-446655440000",
  "roomId": "global",
  "type": "chat",
  "userId": "user123",
  "username": "Alice",
  "content": "Hello world",
  "timestamp": "2025-12-30T10:30:00Z"
}
```

**Message Fields:**
- `messageId`: Unique UUID (auto-generated, Phase 2A)
- `roomId`: Room identifier (defaults to "global", Phase 2A)
- `type`: Message type (see below)
- `userId`: User identifier
- `username`: Display name
- `content`: Message text
- `timestamp`: ISO 8601 timestamp (UTC)

**Message Types:**
- `chat`: User chat message
- `system`: System announcement
- `join`: User joined notification
- `leave`: User left notification

**Validation Rules:**
- Username: Required, max 50 characters
- Content (for chat messages): Required, max 1000 characters
- Timestamp: Automatically set to UTC
- MessageID: Auto-generated UUID v4 if not provided
- RoomID: Defaults to "global" if not provided

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

**Server Configuration:**
- `PORT`: Server port (default: 8080)
- `LOG_LEVEL`: Logging level (debug, info, warn, error; default: info)

**DynamoDB Configuration (Phase 2A):**
- `DYNAMODB_ENDPOINT`: DynamoDB endpoint URL (e.g., http://localhost:8000 for local)
- `DYNAMODB_REGION`: AWS region (default: us-east-1)
- `AWS_ACCESS_KEY_ID`: AWS access key (use "dummy" for local DynamoDB)
- `AWS_SECRET_ACCESS_KEY`: AWS secret key (use "dummy" for local DynamoDB)

**Example (.env.example):**
```bash
PORT=8080
LOG_LEVEL=info
DYNAMODB_ENDPOINT=http://localhost:8000
DYNAMODB_REGION=us-east-1
AWS_ACCESS_KEY_ID=dummy
AWS_SECRET_ACCESS_KEY=dummy
```

## Performance

- **Capacity**: 100+ concurrent connections
- **Latency**: <100ms message delivery (p95)
- **Throughput**: Non-blocking broadcast to all clients
- **Reliability**: Automatic connection cleanup, no goroutine leaks

## Technology Stack

### Backend
- **Language**: Go 1.23+
- **WebSocket**: gorilla/websocket v1.5.3
- **Storage**: AWS SDK v2 for DynamoDB (Phase 2A)
- **Configuration**: github.com/epw80/chat-analytics-platform/pkg/config
- **Concurrency**: Goroutines and channels
- **Logging**: slog (structured logging)
- **UUID Generation**: google/uuid v1.6.0

### Infrastructure (Phase 2A)
- **Containerization**: Docker and Docker Compose
- **Database**: DynamoDB Local (development), AWS DynamoDB (production)
- **Networking**: Docker bridge networks

## Roadmap

### Phase 1 (Complete) ✅
- [x] WebSocket server with Hub-and-Spoke architecture
- [x] Real-time message broadcasting
- [x] Comprehensive test coverage
- [x] Health check endpoint
- [x] Graceful shutdown

### Phase 2A (In Progress - 60% Complete) 🚧
- [x] Message persistence (DynamoDB)
- [x] Docker containerization
- [x] DynamoDB Local integration
- [x] AWS SDK v2 integration
- [x] Configuration management
- [x] Non-blocking async storage
- [ ] AWS deployment files (ECS, CloudFormation)
- [ ] Production Dockerfile
- [ ] Integration and load tests

### Phase 2B (Planned)
- [ ] Real-time analytics backend
- [ ] In-memory metrics tracking
- [ ] Analytics API endpoint
- [ ] Performance metrics collection

### Phase 2C (Planned)
- [ ] Frontend chat interface (React)
- [ ] Analytics dashboard
- [ ] User list component
- [ ] Message history visualization

### Phase 3+ (Future)
- [ ] User authentication & authorization
- [ ] Private messaging
- [ ] Rate limiting
- [ ] Kubernetes deployment
- [ ] Horizontal scaling with Redis pub/sub
- [ ] CloudWatch integration
- [ ] API Gateway + Lambda

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
