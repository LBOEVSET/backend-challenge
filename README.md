# Backend Challenge

## Quick Start

### Option 1 — Docker Compose (recommended for local dev)

```bash
docker compose up --build
# HTTP API: http://localhost:8080
# gRPC:     localhost:9090
```

Data persists in a Docker volume (`mongo_data`) between restarts.

### Option 2 — Local (Go 1.23+)

```bash
# Start MongoDB
docker run -d --name mongo-local -p 27017:27017 \
  -e MONGO_INITDB_ROOT_USERNAME=<user> \
  -e MONGO_INITDB_ROOT_PASSWORD=<pass> \
  mongo:7

# Run the API
export MONGODB_URI="mongodb://<user>:<pass>@localhost:27017"
export JWT_SECRET="your-secret"
go run ./cmd/server
```

---

## Testing

Tests use **testify/mock** — no real MongoDB required.

```bash
# Run all tests
go test ./... -v

# Run with coverage report
go test ./... \
  -coverprofile=coverage.out \
  -covermode=atomic \
  -coverpkg=./internal/application/...,./internal/domain/...,./internal/adapter/http/...,./pkg/...,./test/service/...

go tool cover -html=coverage.out   # open HTML report
```

Coverage threshold: **80%** (enforced in CI via `-coverpkg`).

Test files:

| Package | File | What it covers |
|---------|------|----------------|
| `test/service/` | `user_service_test.go` | All use cases: Register, Login, CRUD, CountUsers |
| `test/handler/` | `handler_test.go` | Every HTTP route via `httptest` (26 tests) |
| `pkg/auth/` | `jwt_test.go` | Token generation and validation |
| `pkg/hash/` | `password_test.go` | bcrypt hash and check |
| `internal/domain/` | `user_test.go` | Domain entity validation |

---

## Architecture — Hexagonal (Ports & Adapters)

```
cmd/server/
  main.go           ← Entry point: wires everything together
  grpc.go           ← gRPC server start

internal/
  domain/           ← Core entity (User) — no external deps
  port/             ← Interface: UserRepository (the port)
  application/      ← Use cases: Register, Login, CRUD, CountUsers
  adapter/
    mongodb/        ← MongoDB adapter (implements port.UserRepository)
    http/           ← Gin HTTP adapter (handlers, middleware, router)
    grpc/           ← gRPC adapter

proto/user/
  user.proto        ← gRPC service definition
  user.pb.go        ← Generated at Docker build time

pkg/
  auth/             ← JWT utilities (HS256, 24h TTL)
  hash/             ← bcrypt helpers

test/
  mock/             ← Testify mock for UserRepository
  service/          ← Use-case unit tests (no DB)
  handler/          ← HTTP layer unit tests (httptest)

design/
  lottery-search-system.md ← Lottery Search System design doc
```

The application layer depends only on `port.UserRepository` — never on the MongoDB driver directly. Tests swap in the mock without a real database.

---

## Environment Variables

| Variable      | Default                     | Description               |
|---------------|-----------------------------|---------------------------|
| `MONGODB_URI` | `mongodb://localhost:27017` | MongoDB connection string |
| `MONGODB_DB`  | `assignment`                | Database name             |
| `JWT_SECRET`  | `change-me-in-production`   | HMAC signing key          |
| `PORT`        | `8080`                      | HTTP listen port          |
| `GRPC_PORT`   | `9090`                      | gRPC listen port          |

Secrets (`MONGODB_URI`, `MONGODB_DB`, `JWT_SECRET`) are stored in **Infisical** and never hardcoded in any file committed to Git. Locally, create a `.env` file (see `.env.example`); Docker Compose reads it automatically.

---

## API Reference

| Method | Path                  | Auth | Description       |
|--------|-----------------------|------|-------------------|
| GET    | /api/v1/health        | —    | Health check      |
| POST   | /api/v1/auth/register | —    | Register new user |
| POST   | /api/v1/auth/login    | —    | Login → JWT       |
| POST   | /api/v1/users         | ✓    | Create user       |
| GET    | /api/v1/users         | ✓    | List all users    |
| GET    | /api/v1/users/:id     | ✓    | Get user by ID    |
| PUT    | /api/v1/users/:id     | ✓    | Update name/email |
| DELETE | /api/v1/users/:id     | ✓    | Delete user       |

### Example requests

```bash
# Register
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","email":"alice@example.com","password":"secret123"}'

# Login → get token
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","password":"secret123"}'
# → {"token":"eyJ..."}

TOKEN="eyJ..."

# List users
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/users

# Update
curl -X PUT http://localhost:8080/api/v1/users/<id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice Updated"}'

# Delete
curl -X DELETE -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/users/<id>
```

Token: HS256, 24-hour TTL, contains `user_id` claim. Add `Authorization: Bearer <token>` to every protected request.

---

## gRPC (Bonus)

gRPC is enabled via the `grpc` build tag. The Docker build generates stubs from `proto/user/user.proto` automatically.

Defined RPCs: `UserService.CreateUser`, `UserService.GetUser`

```bash
# Install protoc (once)
brew install protobuf
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.34.2
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0

# Generate stubs
make proto

# Build with gRPC enabled
go build -tags grpc -o bin/server ./cmd/server
./bin/server
```

---

## CI/CD (GKE)

Push to `dev` → GitHub Actions runs two jobs:

**Job 1 — Test & SonarQube scan:**
1. `go test ./...` with `-coverpkg` covering application, domain, http adapter, and pkg packages
2. Fails if total coverage < 80%
3. SonarQube scan via `SonarSource/sonarqube-scan-action@v6` — uploads `coverage.out`
4. Quality Gate check — fails build if gate is RED

**Job 2 — Build & deploy** (runs only if Job 1 passes):
1. Fetches secrets from Infisical
2. Authenticates to GCP via Workload Identity Federation
3. Builds with `docker buildx --platform linux/amd64`
4. Pushes to `asia-southeast1-docker.pkg.dev/agentassistant-496719/assignment/backend-challenge`
5. Rolls out to GKE namespace `assignment`

**Required GitHub secrets:**

| Secret | Purpose |
|--------|---------|
| `WIF_PROVIDER` | Workload Identity Federation provider |
| `GCP_PROJECT_ID` | GCP project ID |
| `GKE_CLUSTER` | GKE cluster name |
| `GKE_ZONE` | GKE cluster zone |
| `INFISICAL_CLIENT_ID` | Infisical machine identity ID |
| `INFISICAL_CLIENT_SECRET` | Infisical machine identity secret |
| `INFISICAL_PROJECT_SLUG` | Infisical project slug |
| `SONARQUBE_URL` | `http://<sonarqube-external-ip>:9000` |
| `SONARQUBE_TOKEN` | SonarQube analysis token |

**Live URL:** `http://8.233.137.90`

---

## SonarQube

SonarQube Community Edition (v26.6.0) runs in the `assignment` namespace on GKE at `http://34.143.211.72:9000`.

**What's analyzed:** `internal/application`, `internal/domain`, `internal/adapter/http`, `pkg/auth`, `pkg/hash`

**Excluded from analysis** (require integration tests or are entry points): `cmd/`, `internal/adapter/mongodb/`, `internal/adapter/grpc/`, `test/mock/`

**Quality Gate:** Fails the build if coverage on analyzed files drops below threshold or new bugs/vulnerabilities are introduced.

To re-deploy SonarQube:
```bash
kubectl apply -f k8s/sonarqube.yaml
kubectl get svc sonarqube -n assignment   # wait for external IP
```

---

## Lottery Search System

See [`design/lottery-search-system.md`](design/lottery-search-system.md) for the full design proposal.

---

# Original Assignment

## Overview

| Section | Focus | Submission |
|---------|-------|------------|
| User Management API | Go API with MongoDB and JWT authentication | Code |
| Lottery Search System | Wildcard lottery ticket search — Redis bitmap design | Design doc |

## User Management API Requirements

1. **User Model** — ID, Name, Email, Password (hashed), CreatedAt
2. **Authentication** — Register, Login → JWT (HS256), middleware-protected routes
3. **User Operations** — Create, Get by ID, List, Update, Delete
4. **MongoDB Integration** — official Go driver
5. **Middleware** — logging (method, path, execution time)
6. **Concurrency** — background goroutine every 10s to log user count
7. **Testing** — unit tests with mocked MongoDB

### Bonus (all implemented ✓)

| Bonus | Status |
|-------|--------|
| Docker + docker-compose | ✓ |
| Go interface abstraction (`port.UserRepository`) | ✓ |
| Input validation (`go-playground/validator`) | ✓ |
| Graceful shutdown via `context.Context` | ✓ |
| gRPC (`CreateUser`, `GetUser`) | ✓ |
| Hexagonal architecture (domain / port / application / adapter) | ✓ |
| Unit tests — 80%+ coverage (SonarQube verified) | ✓ |
| SonarQube code quality gate in CI | ✓ |

## Lottery Search System

See [`design/lottery-search-system.md`](design/lottery-search-system.md).
