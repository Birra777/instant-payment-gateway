# Instant Payment Gateway

[![CI](https://github.com/ponti/instant-payment-gateway/actions/workflows/ci.yml/badge.svg)](https://github.com/ponti/instant-payment-gateway/actions/workflows/ci.yml)
[![Coverage](https://codecov.io/gh/ponti/instant-payment-gateway/branch/main/graph/badge.svg)](https://codecov.io/gh/ponti/instant-payment-gateway)

A simplified instant payment processing system built with Go, React, PostgreSQL, and Redis. Implements double-entry bookkeeping, idempotent payment processing, real-time WebSocket updates, and a transaction monitoring dashboard.

## Architecture

```
                    ┌─────────────────┐
                    │  React Dashboard │
                    │   (Port 3000)   │
                    └────────┬────────┘
                             │ REST + WebSocket
                    ┌────────▼────────┐
                    │   Go API Server  │
                    │   (Port 8080)    │
                    │                  │
                    │ ┌──────────────┐ │
                    │ │  Middleware   │ │
                    │ │ Rate Limit   │ │
                    │ │ Auth / HMAC  │ │
                    │ │ Logging      │ │
                    │ └──────┬───────┘ │
                    │ ┌──────▼───────┐ │
                    │ │   Services   │ │
                    │ │ Payment      │ │
                    │ │ Webhook      │ │
                    │ └──────┬───────┘ │
                    └────────┼────────┘
               ┌─────────────┼─────────────┐
        ┌──────▼──────┐             ┌──────▼──────┐
        │ PostgreSQL  │             │    Redis    │
        │  - accounts │             │ - rate limit│
        │  - txns     │             │ - cache     │
        │  - ledger   │             └─────────────┘
        │  - audit    │
        └─────────────┘
```

## Features

- **Payment Processing**: Initiate, authorize, and settle payments via REST API
- **Double-Entry Bookkeeping**: Every transaction creates balanced debit/credit ledger entries
- **Idempotency**: Duplicate payment requests with the same key return the existing transaction
- **Rate Limiting**: Per-client rate limiting by IP or API key
- **Request Signing**: HMAC-SHA256 signature verification for secure API calls
- **Real-Time Updates**: WebSocket endpoint streams transaction status changes
- **Structured Logging**: Request tracing with unique trace IDs
- **Analytics**: Daily transaction volume, success/failure rates
- **Webhook Support**: Inbound webhooks for Stripe and MTN MoMo
- **React Dashboard**: Transaction history, filtering, real-time charts

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/accounts` | Create account |
| `GET` | `/api/v1/accounts` | List accounts |
| `GET` | `/api/v1/accounts/:id` | Get account |
| `POST` | `/api/v1/payments` | Initiate payment |
| `POST` | `/api/v1/payments/:id/authorize` | Authorize payment |
| `POST` | `/api/v1/payments/:id/settle` | Settle payment |
| `GET` | `/api/v1/payments` | List transactions (with filters) |
| `GET` | `/api/v1/payments/:id` | Get transaction |
| `GET` | `/api/v1/payments/:id/ledger` | Get ledger entries |
| `GET` | `/api/v1/analytics` | Get daily analytics |
| `POST` | `/api/v1/webhooks/stripe` | Stripe webhook handler |
| `POST` | `/api/v1/webhooks/momo` | MoMo webhook handler |
| `GET` | `/ws` | WebSocket connection |
| `GET` | `/health` | Health check |

## Quick Start

### Prerequisites
- Docker & Docker Compose
- Go 1.22+ (for local development)
- Node.js 20+ (for dashboard development)

### Run with Docker Compose

```bash
# Start all services
docker compose up -d --build

# API:       http://localhost:8080
# Dashboard: http://localhost:3000
# Health:    http://localhost:8080/health
```

### Local Development

```bash
# Copy environment config
cp .env.example .env

# Start dependencies
docker compose up -d db redis

# Run migrations
make migrate

# Start the API server
make run

# In another terminal, start the dashboard
cd web && npm install && npm start
```

### Run the Payment Simulator

```bash
# Generates test accounts and mock transactions
make simulate
```

## Database Schema

| Table | Description |
|-------|-------------|
| `accounts` | User/merchant accounts with balances |
| `transactions` | Payment records with status tracking |
| `ledger_entries` | Double-entry bookkeeping records |
| `audit_log` | Immutable event log |

## Testing

```bash
# Run all Go tests
make test

# View coverage report
make coverage

# Run dashboard tests
cd web && npm test
```

## Project Structure

```
instant-payment-gateway/
├── cmd/server/          # Application entrypoint
├── internal/
│   ├── handler/         # HTTP route handlers
│   ├── service/         # Business logic layer
│   ├── repository/      # Database access layer
│   ├── middleware/       # Auth, rate limiting, logging, CORS
│   ├── model/           # Data models and DTOs
│   └── websocket/       # WebSocket hub for real-time updates
├── web/                 # React dashboard
│   └── src/
│       ├── components/  # UI components
│       └── hooks/       # Custom React hooks
├── migrations/          # SQL schema migrations
├── scripts/             # Simulator, seed data
├── .github/workflows/   # CI/CD pipeline
├── docker-compose.yml   # Multi-service orchestration
├── Dockerfile           # API container
└── Makefile             # Development commands
```

## License

MIT
