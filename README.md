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
# Run all tests
go test ./... -v

# Run with coverage report
go test ./... -coverprofile=coverage.out -covermode=atomic
go tool cover -html=coverage.out   # open HTML report in browser
```

Tests live in `test/service/` and use a testify mock — no real MongoDB required. Coverage threshold is **70%** (enforced in CI).

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

### Secrets management (Infisical)

App secrets (`MONGODB_URI`, `MONGODB_DB`, `JWT_SECRET`) are stored in **Infisical** and injected at CI time — never hardcoded in any file committed to Git.

- Locally: create a `.env` file (see `.env.example`). Docker Compose reads it automatically.
- CI: the workflow fetches secrets via `Infisical/secrets-action` before the deploy steps.

**Required GitHub secrets:**

| Secret | Purpose |
|--------|---------|
| `WIF_PROVIDER` | Workload Identity Federation for GCP auth |
| `GCP_PROJECT_ID` | GCP project |
| `GKE_CLUSTER` | GKE cluster name |
| `GKE_ZONE` | GKE cluster zone |
| `INFISICAL_CLIENT_ID` | Infisical machine identity |
| `INFISICAL_CLIENT_SECRET` | Infisical machine identity secret |
| `INFISICAL_PROJECT_SLUG` | Infisical project slug |
| `SONARQUBE_URL` | External URL of the SonarQube instance on GKE |
| `SONARQUBE_TOKEN` | SonarQube analysis token |

**Live URL:** `http://8.233.137.90`

---

## SonarQube (Code Quality & Security)

SonarQube Community Edition runs in the `assignment` namespace on GKE.

**Deploy (first time):**
```bash
kubectl apply -f k8s/sonarqube.yaml
# Wait for the LoadBalancer to get an external IP
kubectl get svc sonarqube -n assignment
```

**Initial setup:**
1. Open the SonarQube UI at `http://<EXTERNAL-IP>:9000` (default login `admin`/`admin`, change on first login).
2. Create a project with key `backend-challenge`.
3. Generate an analysis token and save it as `SONARQUBE_TOKEN` in GitHub secrets.
4. Save `http://<EXTERNAL-IP>:9000` as `SONARQUBE_URL` in GitHub secrets.

**Quality Gate (enforced in CI):**
The `sonarqube-quality-gate-action` fails the build if the default Quality Gate is RED.  Typical causes and fixes:

- **Coverage below 70% on new code** → add tests for the changed lines, re-push.
- **New bugs / vulnerabilities** → check the SonarQube dashboard for the issue location and fix it.
- **Security hotspots** → review on the dashboard and mark as safe or fix in code.

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
