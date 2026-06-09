# Backend Challenge — Implementation

## Quick Start

### Docker (recommended)

```bash
# Set your MongoDB Atlas URI in docker-compose or env
MONGODB_URI="mongodb+srv://..." docker compose up --build
# API available at http://localhost:8080
```

### Local (requires Go 1.22+)

```bash
export MONGODB_URI="mongodb+srv://..."
export JWT_SECRET="your-secret"
go run ./cmd/server
```

### Tests

```bash
go test ./... -v
```

---

## Architecture — Hexagonal (Ports & Adapters)

```
cmd/server/            ← Entry point: wires everything together
internal/
  domain/              ← Core entity (User) — no external deps
  port/                ← Interface: UserRepository (the port)
  application/         ← Use cases: Register, Login, CRUD
  adapter/
    mongodb/           ← MongoDB adapter (implements port)
    http/              ← Gin HTTP adapter (handler, middleware, router)
proto/user.proto       ← gRPC definition (run `make proto` to generate)
pkg/auth/              ← JWT utilities (HS256)
pkg/hash/              ← bcrypt helpers
test/mock/             ← Testify mock for UserRepository
test/service/          ← Unit tests (no DB required)
design/                ← Lottery Search System design doc
```

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

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | /api/v1/health | — | Health check |
| POST | /api/v1/auth/register | — | Register new user |
| POST | /api/v1/auth/login | — | Login → JWT |
| POST | /api/v1/users | ✓ | Create user |
| GET | /api/v1/users | ✓ | List all users |
| GET | /api/v1/users/:id | ✓ | Get user by ID |
| PUT | /api/v1/users/:id | ✓ | Update name/email |
| DELETE | /api/v1/users/:id | ✓ | Delete user |

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

The `proto/user.proto` file defines `CreateUser` and `GetUser` RPCs.

To generate Go stubs and enable the gRPC server:

```bash
# Install protoc and plugins (once)
brew install protobuf
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate
make proto

# Build with gRPC enabled
go build -tags grpc -o bin/server ./cmd/server
```

---

## Lottery Search System

See [`design/lottery-search-system.md`](design/lottery-search-system.md).

---

# Original Assignment

## Overview

This coding test has two parts.

| Section | Focus | Submission Type |
| --- | --- | --- |
| User Management API | Build a Golang user management API with MongoDB and JWT authentication | Code implementation |
| Lottery Search System | Design a real-world lottery ticket search solution with wildcard matching | Design proposal only (no code) |

## User Management API

### Objective (User Management API)

Build a RESTful API in Golang to manage users, using MongoDB for persistence, JWT for authentication, and clean code practices.

### Requirements (User Management API)

#### 1. User Model

Define a user entity with the following fields:

- `ID` (auto-generated)
- `Name` (string)
- `Email` (string, unique)
- `Password` (hashed)
- `CreatedAt` (timestamp)

#### 2. Authentication

Implement:

- User registration
- User authentication that returns a JWT token

JWT requirements:

- Protect endpoints with JWT
- Validate tokens via middleware
- Sign tokens using HMAC (`HS256`) with a secret key

#### 3. User Operations

Implement the following operations:

- Create a new user
- Fetch a user by ID
- List all users
- Update a user's name or email
- Delete a user

#### 4. MongoDB Integration

- Use the official Go MongoDB driver
- Persist and retrieve user data from MongoDB

#### 5. Middleware

- Implement logging middleware to capture HTTP method, path, and execution time

#### 6. Concurrency Task

- Run a background goroutine every 10 seconds to log the total number of users in the database

#### 7. Testing

- Write unit tests using Go's standard `testing` package
- Mock MongoDB interactions where appropriate

### Bonus (Optional, User Management API)

- **Containerization**: Add Docker and `docker-compose` support for the API and MongoDB
- **Abstraction**: Use Go interfaces to abstract MongoDB operations for better testability
- **Validation**: Implement input validation (for example, required fields and email format)
- **Graceful Shutdown**: Handle system signals using `context.Context`
- **gRPC Support**:
  - Define a `.proto` file for `CreateUser` and `GetUser`
  - Implement a gRPC server (optionally secure with token metadata)
- **Hexagonal Architecture**:
  - Structure the project using ports and adapters
  - Separate domain, application, and infrastructure layers
  - Decouple business logic from frameworks and drivers

### Deliverables (User Management API)

Provide a Git repository containing:

- `README.md` with setup and execution instructions
- A guide explaining how to generate and use JWT tokens
- Sample API requests and responses
- Documentation of assumptions or design decisions

### Evaluation Criteria (User Management API)

- Code quality, structure, and readability
- Correctness and completeness of the REST API
- Security and implementation of JWT
- Proper usage and abstraction of MongoDB
- Test coverage and effective mocking
- Idiomatic Go usage
- Bonus implementations (gRPC, Docker, validation, architecture)

## Lottery Search System

### Objective (Lottery Search System)

Design a real-world solution to search a large dataset of lottery tickets using pattern matching with wildcard support.

> This section is a design exercise. Do not implement code.

### Requirements (Lottery Search System)

#### 1. Data Volume

- Handle a dataset of **1 million** lottery tickets
- Each ticket is a 6-digit number

#### 2. Search Pattern

- Support a 6-character search pattern containing digits and wildcards (`*`)
- Example patterns:

| Pattern | Matches |
| --- | --- |
| `****23` | Numbers ending in `23` |
| `1****5` | Numbers starting with `1` and ending with `5` |
| `123***` | Numbers starting with `123` |

#### 3. Result Distribution

- Constraint: the same search pattern should not return the same ticket to multiple users at the same time
- Propose a distribution mechanism so matching tickets are assigned without duplicate simultaneous selection

#### 4. Performance

- Ensure the search is performant for `1M+` records
- Propose an efficient approach for querying and allocation

#### 5. Real-World Design Proposal (No Code Required)

- Recommend the database/storage technology you would use in production and explain why
- Describe the algorithm and indexing strategy used for wildcard pattern matching
- Explain how you would prevent duplicate simultaneous results for the same pattern (for example, locking, reservation, or atomic allocation)
- No code implementation is required; provide a solution/design only

### Deliverables (Lottery Search System)

Submit a design document only (no code implementation) that includes:

- Proposed solution architecture, data structures, and algorithms
- Recommended production database/storage choice with justification (for example, query performance, concurrency handling, operational simplicity)
- Performance analysis summarizing efficiency and tradeoffs
- Concurrency/distribution strategy explaining how duplicate results are avoided for the same pattern

### Evaluation Criteria (Lottery Search System)

- Feasibility: the solution addresses the stated requirements
- Performance: the search approach is efficient for the target scale
- Correctness: the distribution constraint is handled correctly
- Real-world practicality: the database/storage and concurrency approach are appropriate for production use
- Creativity: thoughtful use of data structures and algorithms

