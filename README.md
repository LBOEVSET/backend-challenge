# Backend Challenge — Implementation

## Quick Start

### Option 1 — Docker Compose (recommended for local dev)

Runs the API + MongoDB together with one command. Credentials and URIs are wired automatically.

```bash
docker compose up --build
# API available at http://localhost:8080
# gRPC available at localhost:9090
```

Data is persisted in a Docker volume (`mongo_data`) between restarts.

### Option 2 — Local (requires Go 1.23+)

```bash
# Start MongoDB with credentials
docker run -d --name mongo-local -p 27017:27017 \
  -e MONGO_INITDB_ROOT_USERNAME=<username> \
  -e MONGO_INITDB_ROOT_PASSWORD=<password> \
  mongo:7

# Run the backend
export MONGODB_URI="mongodb://<username>:<password>@localhost:27017"
export JWT_SECRET="your-secret"
go run ./cmd/server
```

To reuse an existing container:
```bash
docker start mongo-local
```

### Tests

```bash
go test ./... -v
```

---

## Architecture — Hexagonal (Ports & Adapters)

```
cmd/server/
  main.go           ← Entry point: wires everything together
  grpc.go           ← gRPC server start (build tag: grpc)
  grpc_stub.go      ← No-op stub for normal builds

internal/
  domain/           ← Core entity (User) — no external deps
  port/             ← Interface: UserRepository (the port)
  application/      ← Use cases: Register, Login, CRUD, CountUsers
  adapter/
    mongodb/        ← MongoDB adapter (implements port.UserRepository)
    http/           ← Gin HTTP adapter (handlers, middleware, router)
    grpc/           ← gRPC adapter (build tag: grpc)

proto/user/
  user.proto        ← gRPC service definition
  user.pb.go        ← Generated at Docker build time
  user_grpc.pb.go   ← Generated at Docker build time

pkg/
  auth/             ← JWT utilities (HS256, 24h TTL)
  hash/             ← bcrypt helpers

test/
  mock/             ← Testify mock for UserRepository
  service/          ← Unit tests (no DB required)

design/
  lottery-search-system.md ← Lottery Search System design doc
```

The application layer depends only on `port.UserRepository` — never on the MongoDB driver directly. This allows tests to swap in a mock without a real database.

---

## Environment Variables

| Variable      | Default                     | Description               |
|---------------|-----------------------------|---------------------------|
| `MONGODB_URI` | `mongodb://localhost:27017` | MongoDB connection string |
| `MONGODB_DB`  | `assignment`                | Database name             |
| `JWT_SECRET`  | `change-me-in-production`   | HMAC signing key          |
| `PORT`        | `8080`                      | HTTP listen port          |
| `GRPC_PORT`   | `9090`                      | gRPC listen port          |

---

## JWT — How to Generate & Use Tokens

### 1. Register

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","email":"alice@example.com","password":"secret123"}'
```

Response:
```json
{"id":"...","name":"Alice","email":"alice@example.com","created_at":"2024-01-01T00:00:00Z"}
```

### 2. Login → get token

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","password":"secret123"}'
```

Response:
```json
{"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."}
```

Token properties: HS256, 24-hour TTL, contains `user_id` claim.

### 3. Use the token

Add `Authorization: Bearer <token>` to every protected request.

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

### Sample requests

```bash
TOKEN="eyJ..."

# List users
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/users

# Get user
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/users/<id>

# Update
curl -X PUT http://localhost:8080/api/v1/users/<id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice Updated"}'

# Delete
curl -X DELETE -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/users/<id>
```

---

## gRPC (Bonus)

gRPC is enabled at build time via the `grpc` build tag. The Docker build generates stubs automatically from `proto/user/user.proto`.

Defined RPCs:
- `UserService.CreateUser`
- `UserService.GetUser`

To build and run locally with gRPC:

```bash
# Install protoc and plugins (once)
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

## GKE Deployment

Push to `main` or `dev` → GitHub Actions:
1. Builds with `docker buildx --platform linux/amd64`
2. Pushes to `asia-southeast1-docker.pkg.dev/agentassistant-496719/assignment/backend-challenge`
3. Rolls out to GKE namespace `assignment`

MongoDB runs as a pod in the same namespace (`mongo:27017`) backed by a 2Gi PersistentVolumeClaim. The backend connects to it via K8s internal DNS — no external exposure needed.

**To update the MongoDB credentials secret in GKE:**
```bash
kubectl create secret generic backend-challenge-secret \
  --namespace=assignment \
  --from-literal=MONGODB_URI="mongodb://<user>:<pass>@mongo:27017" \
  --from-literal=JWT_SECRET="<your-secret>" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl rollout restart deployment/backend-challenge -n assignment
```

**Required GitHub secrets:** `GCP_PROJECT_ID`, `GKE_CLUSTER`, `GKE_ZONE`, `WIF_PROVIDER`, `MONGODB_URI`, `JWT_SECRET`

**Live URL:** `http://8.233.137.90`

---

## Lottery Search System

See [`design/lottery-search-system.md`](design/lottery-search-system.md) for the full design proposal.

---

# Original Assignment

## Overview

| Section | Focus | Submission Type |
|---------|-------|-----------------|
| User Management API | Build a Golang user management API with MongoDB and JWT authentication | Code implementation |
| Lottery Search System | Design a real-world lottery ticket search solution with wildcard matching | Design proposal only (no code) |

## User Management API

### Requirements

1. **User Model** — ID, Name, Email, Password (hashed), CreatedAt
2. **Authentication** — Register, Login → JWT (HS256), middleware-protected routes
3. **User Operations** — Create, Get by ID, List, Update, Delete
4. **MongoDB Integration** — official Go driver, persist/retrieve users
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

## Lottery Search System

See [`design/lottery-search-system.md`](design/lottery-search-system.md).
