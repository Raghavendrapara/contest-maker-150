# Contest Maker 150

A modern, containerized application for generating coding contests from the NeetCode 150 problem set.

![Contest Maker 150](https://img.shields.io/badge/version-1.0.0-blue)
![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)
![React](https://img.shields.io/badge/React-19-61DAFB?logo=react)
![TypeScript](https://img.shields.io/badge/TypeScript-5.6-3178C6?logo=typescript)
![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker)

## Features

- **Problem Set Generation**: Select problems from the curated NeetCode 150 list
- **Gradual Difficulty Progression**: Problems automatically arranged from Easy → Medium → Hard
- **Unique Problem Selection**: Never repeat solved questions for individual users  
- **Timed Contests**: Customizable contest duration with real-time countdown
- **Progress Tracking**: Track solved problems across topics and difficulty levels
- **Full Observability**: OpenTelemetry tracing + Prometheus metrics + Grafana dashboards

## Tech Stack

### Backend
- **Language**: Go 1.22+
- **Framework**: Gin (HTTP router)
- **ORM**: GORM with PostgreSQL
- **Auth**: JWT (access + refresh tokens)
- **Observability**: OpenTelemetry (traces), Prometheus (metrics)
- **Architecture**: Clean Architecture (Hexagonal)

### Frontend  
- **Framework**: React 18 with TypeScript
- **Build Tool**: Vite 5
- **Styling**: Tailwind CSS 4
- **State Management**: Zustand + React Query
- **Icons**: Lucide React

### Infrastructure
- **Database**: PostgreSQL 16
- **Container Runtime**: Docker + Docker Compose
- **Web Server**: Nginx (production frontend)
- **Tracing**: Jaeger
- **Metrics**: Prometheus + Grafana

## Quick Start

### Prerequisites
- Docker & Docker Compose
- Go 1.22+ (for local development)
- Node.js 20+ (for local development)

### Running with Docker

1. Clone the repository:
```bash
git clone https://github.com/yourusername/contest-maker-150.git
cd contest-maker-150
```

2. Create environment file:
```bash
cp .env.example .env
# Edit .env with your settings
```

3. Start all services:
```bash
docker-compose up -d
```

4. Access the application:
- **Frontend**: http://localhost:3000
- **Backend API**: http://localhost:8080
- **Jaeger UI**: http://localhost:16686
- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3001 (admin/admin)

### Local Development

#### Backend
```bash
cd backend
go mod download
go run cmd/api/main.go
```

#### Frontend
```bash
cd frontend
npm install
npm run dev
```

## API Endpoints

### Authentication
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/auth/signup` | Register new user |
| POST | `/api/auth/login` | Login user |
| POST | `/api/auth/refresh` | Refresh access token |

### Users
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/users/me` | Get current user |
| GET | `/api/users/me/progress` | Get user progress stats |

### Problems
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/problems` | List all problems |
| GET | `/api/problems/stats` | Get problem statistics |
| GET | `/api/problems/:id` | Get single problem |

### Contests
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/contests` | Create new contest |
| GET | `/api/contests` | List user's contests |
| GET | `/api/contests/active` | Get active contest |
| GET | `/api/contests/:id` | Get contest by ID |
| PATCH | `/api/contests/:id/problems/:problemId` | Mark problem complete |
| POST | `/api/contests/:id/complete` | Complete contest |
| POST | `/api/contests/:id/abandon` | Abandon contest |

## Project Structure

```
contest-maker-150/
├── backend/
│   ├── cmd/api/              # Application entry point
│   ├── internal/
│   │   ├── domain/           # Business entities & interfaces
│   │   ├── handler/          # HTTP handlers
│   │   ├── infrastructure/   # Database, logging, config
│   │   ├── middleware/       # Auth, CORS, logging, tracing
│   │   ├── repository/       # Data access layer
│   │   ├── service/          # Business logic
│   │   └── data/             # Problem data & seeder
│   ├── Dockerfile
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── components/       # React components
│   │   ├── hooks/            # Custom hooks
│   │   ├── pages/            # Page components
│   │   ├── services/         # API client
│   │   ├── stores/           # Zustand stores
│   │   └── types/            # TypeScript types
│   ├── Dockerfile
│   └── package.json
├── monitoring/
│   ├── prometheus.yml
│   └── grafana/
├── docs/                     # Documentation
├── docker-compose.yml
└── README.md
```

## Documentation

- [Backend Architecture](docs/backend-architecture.md) - Clean architecture patterns
- [Concurrency Patterns](docs/concurrency-patterns.md) - Go concurrency deep dive
- [API Reference](docs/api-reference.md) - Complete API documentation
- [Deployment Guide](docs/deployment.md) - Production deployment

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `SERVER_PORT` | API server port | `8080` |
| `SERVER_ENVIRONMENT` | `development` or `production` | `development` |
| `DATABASE_HOST` | PostgreSQL host | `localhost` |
| `DATABASE_PORT` | PostgreSQL port | `5432` |
| `DATABASE_USER` | Database username | `contestmaker` |
| `DATABASE_PASSWORD` | Database password | - |
| `DATABASE_NAME` | Database name | `contestmaker` |
| `JWT_SECRET` | JWT signing secret | - |
| `JWT_ACCESS_EXPIRY` | Access token expiry | `15m` |
| `JWT_REFRESH_EXPIRY` | Refresh token expiry | `168h` |
| `TELEMETRY_ENABLED` | Enable observability | `true` |
| `TELEMETRY_OTEL_ENDPOINT` | OpenTelemetry collector | `http://localhost:4318` |

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [NeetCode](https://neetcode.io/) for the curated problem list
- [LeetCode](https://leetcode.com/) for hosting the problems
