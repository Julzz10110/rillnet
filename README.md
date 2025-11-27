# RillNet - P2P Live Streaming Mesh

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![WebRTC](https://img.shields.io/badge/WebRTC-Pion-29ABE2?style=flat)](https://github.com/pion/webrtc)

**RillNet** is a high-performance P2P (Peer-to-Peer) CDN solution for live video streaming, designed to reduce bandwidth costs by enabling viewers to exchange video segments with each other instead of receiving streams directly from the server.

## 🎯 Overview

RillNet implements a lightweight Selective Forwarding Unit (SFU) architecture combined with a peer-to-peer mesh network. When viewers connect, the signaling server organizes them into a mesh network where they exchange video segments with each other, significantly reducing server bandwidth consumption.

### Key Features

- **🔄 P2P Mesh Network**: Automatic peer discovery and optimal mesh topology construction
- **📹 WebRTC SFU**: Lightweight Selective Forwarding Unit for video stream ingestion
- **⚡ Adaptive Bitrate**: Automatic quality adjustment based on network conditions
- **📊 Real-time Monitoring**: Prometheus metrics and Grafana dashboards
- **🔒 Production Ready**: Authentication, rate limiting, and security features (in development)
- **📈 Scalable**: Horizontal scaling support with Redis-backed state management
- **🌐 NAT Traversal**: STUN/TURN server support for connectivity behind firewalls

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      RillNet Architecture                    │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────┐      ┌──────────────┐                  │
│  │   Ingest     │      │   Signal     │                  │
│  │   Server     │◄────►│   Server     │                  │
│  │  (SFU)       │      │  (WebSocket) │                  │
│  └──────┬───────┘      └──────┬───────┘                  │
│         │                      │                            │
│         │                      │                            │
│  ┌──────▼──────────────────────▼───────┐                  │
│  │         Mesh Network Service        │                  │
│  │  - Peer Discovery                   │                  │
│  │  - Optimal Path Selection          │                  │
│  │  - Health Monitoring                │                  │
│  └─────────────────────────────────────┘                  │
│                                                              │
│  ┌──────────────┐      ┌──────────────┐                  │
│  │   Redis      │      │  Prometheus  │                  │
│  │  (State)     │      │  (Metrics)   │                  │
│  └──────────────┘      └──────────────┘                  │
│                                                              │
└─────────────────────────────────────────────────────────────┘

         Publisher (Streamer)
                │
                ▼
         ┌──────────────┐
         │  Ingest SFU  │
         └──────┬───────┘
                │
                ▼
    ┌───────────────────────┐
    │   P2P Mesh Network    │
    │                       │
    │  Viewer1 ◄──► Viewer2 │
    │    │           │       │
    │    └─────┬─────┘       │
    │          │             │
    │       Viewer3         │
    └───────────────────────┘
```

### Components

- **Ingest Server**: Receives video streams from publishers via WebRTC, acts as SFU
- **Signal Server**: WebSocket-based signaling server for peer coordination
- **Mesh Service**: Manages P2P connections, peer discovery, and optimal path selection
- **Quality Service**: Monitors network conditions and adjusts stream quality
- **Metrics Service**: Collects and aggregates performance metrics

## 🚀 Quick Start

### Prerequisites

- Go 1.25 or higher
- Redis (for persistent storage)
- Docker & Docker Compose (optional, for containerized deployment)

### Installation

1. **Clone the repository**
   ```bash
   git clone https://github.com/yourusername/rillnet.git
   cd rillnet
   ```

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Build the project**
   ```bash
   make build
   ```

4. **Run services**
   ```bash
   # Start Redis
   docker-compose up -d redis

   # Start all services
   make run
   ```

   Or use Docker Compose for full stack:
   ```bash
   docker-compose up -d
   ```

### Configuration

Copy and modify the configuration file:
```bash
cp configs/config.yaml configs/config.local.yaml
```

Edit `configs/config.local.yaml` with your settings:

```yaml
server:
  address: ":8080"
  read_timeout: 30s
  write_timeout: 30s

signal:
  address: ":8081"
  ping_interval: 30s
  pong_timeout: 60s

webrtc:
  ice_servers:
    - urls:
        - "stun:stun.l.google.com:19302"
        - "turn:your-turn-server.com:3478"
  port_range:
    min: 50000
    max: 60000
  simulcast: true
  max_bitrate: 5000

mesh:
  max_connections: 4
  health_check_interval: 10s
  reconnect_attempts: 3

monitoring:
  prometheus_enabled: true
  prometheus_port: 9090
  metrics_interval: 30s

logging:
  level: "info"
  format: "json"
```

## 📖 Usage

### Starting a Stream (Publisher)

1. **Create a stream**
   ```bash
   curl -X POST http://localhost:8080/api/v1/streams \
     -H "Content-Type: application/json" \
     -d '{
       "name": "My Stream",
       "owner": "publisher-123",
       "max_peers": 100
     }'
   ```

2. **Connect via WebRTC** using the web interface at `http://localhost:80`

### Joining a Stream (Viewer)

1. **Join a stream**
   ```bash
   curl -X POST http://localhost:8080/api/v1/streams/{stream_id}/join \
     -H "Content-Type: application/json" \
     -d '{
       "peer_id": "viewer-456",
       "is_publisher": false,
       "capabilities": {
         "max_bitrate": 2000,
         "codecs": ["VP8", "Opus"]
       }
     }'
   ```

2. **Connect via WebSocket signaling**
   ```javascript
   const ws = new WebSocket('ws://localhost:8081/ws?peer_id=viewer-456');
   ```

## 🔌 API Reference

### Stream Management

- `POST /api/v1/streams` - Create a new stream
- `GET /api/v1/streams` - List all active streams
- `GET /api/v1/streams/:id` - Get stream details
- `POST /api/v1/streams/:id/join` - Join a stream
- `POST /api/v1/streams/:id/leave` - Leave a stream
- `GET /api/v1/streams/:id/stats` - Get stream statistics

### WebRTC Signaling

- `POST /api/v1/streams/:id/publisher/offer` - Create publisher offer
- `POST /api/v1/streams/:id/publisher/answer` - Handle publisher answer
- `POST /api/v1/streams/:id/subscriber/offer` - Create subscriber offer
- `POST /api/v1/streams/:id/subscriber/answer` - Handle subscriber answer

### WebSocket Signaling

- `WS /ws?peer_id={peer_id}` - WebSocket connection for signaling

### Health & Metrics

- `GET /health` - Health check endpoint
- `GET /ready` - Readiness probe
- `GET /metrics` - Prometheus metrics

## 🛠️ Development

### Project Structure

```
rillnet/
├── cmd/                    # Application entry points
│   ├── ingest/            # Ingest server (SFU)
│   ├── signal/            # Signaling server
│   └── monitor/           # Monitoring service
├── internal/              # Internal packages
│   ├── core/              # Core business logic
│   │   ├── domain/        # Domain models
│   │   ├── ports/         # Interfaces
│   │   └── services/      # Business services
│   ├── handlers/          # HTTP/WebSocket handlers
│   └── infrastructure/    # External integrations
│       ├── repositories/  # Data access layer
│       ├── signal/        # Signaling implementation
│       └── webrtc/        # WebRTC implementation
├── pkg/                   # Public packages
│   ├── config/            # Configuration management
│   ├── logger/            # Logging utilities
│   └── webrtc/            # WebRTC utilities
├── web/                   # Web frontend
├── tests/                 # Test suites
├── configs/               # Configuration files
└── deployments/           # Deployment configs
```

### Running Tests

```bash
# Run all tests
make test

# Run unit tests only
make test-unit

# Run integration tests
make test-integration

# Run load tests
make test-load

# Generate coverage report
make test-coverage
```

### Building

```bash
# Build all binaries
make build

# Build specific service
go build -o bin/ingest ./cmd/ingest
go build -o bin/signal ./cmd/signal
```

## 📊 Monitoring

### Prometheus Metrics

RillNet exposes the following metrics:

- `rillnet_peers_connected_total` - Total connected peers
- `rillnet_streams_active_total` - Active streams count
- `rillnet_data_exchanged_bytes_total` - Total data exchanged
- `rillnet_webrtc_connection_duration_seconds` - WebRTC connection duration
- `rillnet_network_latency_seconds` - Network latency
- `rillnet_stream_bitrate_bps` - Stream bitrate per quality
- `rillnet_stream_peer_count` - Peer count per stream
- `rillnet_stream_health_score` - Stream health score (0-100)

### Grafana Dashboards

Pre-configured Grafana dashboards are available in `deployments/monitoring/grafana/dashboards/`.

Access Grafana at `http://localhost:3000` (default credentials: admin/admin)

## 🐳 Docker Deployment

### Using Docker Compose

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

### Kubernetes Deployment

Kubernetes manifests are available in `deployments/k8s/`:

```bash
kubectl apply -f deployments/k8s/
```

## 🧪 Testing

### Unit Tests

```bash
go test ./internal/... -v
```

### Integration Tests

```bash
go test ./tests/integration/... -v
```

### Load Testing

```bash
make load-test
```

## 📈 Performance

### Current Capabilities

- **Concurrent Viewers**: Up to 10,000 per stream (target)
- **Concurrent Streams**: Up to 100 (target)
- **P2P Efficiency**: 70%+ traffic through P2P (target)
- **Latency**: < 200ms p95 (target)

### Benchmarks

See `tests/load/` for load testing scripts and results.

## 🗺️ Roadmap

See [ROADMAP.md](ROADMAP.md) for detailed development plan.

### Current Status: 🚧 In Active Development

**Phase 1 (Weeks 1-4)**: Critical Fixes
- [x] Project structure and architecture
- [ ] YAML configuration loading
- [ ] RTP packet forwarding
- [ ] WebSocket signaling routing
- [ ] Redis persistence

**Phase 2 (Weeks 5-8)**: Security & Stability
- [ ] Authentication & Authorization
- [ ] Rate limiting
- [ ] Graceful shutdown
- [ ] Monitoring & Alerting

**Phase 3 (Weeks 9-12)**: Optimization & Scaling
- [ ] Mesh network optimization
- [ ] Adaptive bitrate
- [ ] Horizontal scaling

**Phase 4 (Weeks 13-16)**: Advanced Features
- [ ] Distributed tracing
- [ ] CDN integration
- [ ] Complete documentation

See [DEVELOPMENT_PLAN.md](DEVELOPMENT_PLAN.md) for detailed roadmap.

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

### Development Guidelines

- Follow Go best practices and conventions
- Write tests for new features
- Update documentation
- Ensure all tests pass before submitting PR

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [Pion WebRTC](https://github.com/pion/webrtc) - Go WebRTC implementation
- [Gin](https://github.com/gin-gonic/gin) - HTTP web framework
- [Prometheus](https://prometheus.io/) - Monitoring and alerting

## 📧 Contact

For questions, issues, or contributions, please open an issue on GitHub.

---

**Note**: This project is currently in active development. Some features may be incomplete or subject to change.

