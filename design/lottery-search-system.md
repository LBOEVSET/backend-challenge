# Lottery Search System — Design Proposal

## Overview

Design a system that can search 1 million 6-digit lottery tickets by wildcard pattern (e.g. `1****5`, `****23`) and distribute results to concurrent users without duplication.

---

## 1. Data Structures & Algorithm

### Ticket representation

Each ticket is a 6-digit string: `"000000"` to `"999999"` (1 million entries). The entire space fits in memory.

### Positional bitmap index

Pre-compute **60 bitmaps** — one per (position, digit) combination:

```
bitmap[pos][digit]  →  bitset of 1 M bits
```

For ticket `"123456"`:
- `bitmap[0]['1']` sets bit 123456
- `bitmap[1]['2']` sets bit 123456
- …
- `bitmap[5]['6']` sets bit 123456

**Memory**: 60 × (1,000,000 bits / 8) = 60 × 125 KB = **7.5 MB** — trivially in-memory.

### Pattern matching algorithm

For a query like `"1****5"`:

1. Identify constrained positions: pos 0 = `'1'`, pos 5 = `'5'`
2. AND the corresponding bitmaps: `bitmap[0]['1'] AND bitmap[5]['5']`
3. The resulting bitset contains all matching ticket IDs

**Time complexity**: O(N/64) per AND operation (64-bit words), where N = 1,000,000 → ≈ 15,625 word operations. Sub-millisecond for any pattern.

**Pre-computed pattern cache**: For patterns seen repeatedly, cache the result bitset. An LRU cache of ~10,000 patterns costs < 2 GB (1M bits × 10,000 / 8 = 1.25 GB). In practice patterns are few, so cache size is much smaller.

---

## 2. Recommended Production Database / Storage

### Primary store: **Redis** (with custom bitmap structures)

**Why Redis:**
- Native `BITAND`, `BITCOUNT`, `BITPOS` operations run server-side with no data transfer overhead.
- Single-threaded command execution guarantees atomicity — `LPOP` from a matching-ticket list is atomic by design, preventing duplicate assignments.
- Persistence (RDB + AOF) provides durability when needed.
- Horizontal scaling via Redis Cluster if the dataset grows beyond a single node.

**Complementary: PostgreSQL** for durable ticket ownership records and audit trails (who received which ticket, when).

### Data layout in Redis

```
# Bitmap index
bitmap:{pos}:{digit}     →  BITMAP of 1M bits

# Per-pattern result queue (populated on first query, then consumed atomically)
queue:{pattern}          →  Redis LIST of ticket IDs [ "123456", "100005", … ]

# Ticket ownership log
owner:{ticket_id}        →  Hash { user_id, assigned_at, pattern }
```

---

## 3. Concurrency / Distribution Strategy

### The problem

Two users submit the same pattern simultaneously. Naive search returns the same list to both → duplicate assignments.

### Solution: Atomic queue consumption

**On first request for a pattern:**

1. Evaluate the bitmap query → sorted list of matching ticket IDs.
2. Atomically push all matching IDs into `queue:{pattern}` with `RPUSH` (only if key does not exist — use `SETNX` + `EXPIRE` on a lock key to ensure single population).
3. `LPOP queue:{pattern}` → returns one ticket atomically to this user.

**On subsequent requests for the same pattern:**

- `LPOP queue:{pattern}` → returns the next available ticket, never the same ticket twice.

**If all tickets are consumed:** return "no tickets available" or refill from a reserved pool.

**Ticket return:** If a user does not claim their ticket within a TTL, a background worker uses `RPUSH` to return it to the queue.

```
User A ──LPOP queue:"1****5"──→ "100005"   ✓ unique
User B ──LPOP queue:"1****5"──→ "100015"   ✓ unique
User C ──LPOP queue:"1****5"──→ "100025"   ✓ unique
```

Redis guarantees that `LPOP` is atomic — two clients cannot receive the same element.

### Race condition during initial queue population

Use a **Redis lock** to ensure only one process populates the queue for a given pattern:

```
SET lock:{pattern} 1 NX EX 5   # acquire lock, 5 s TTL
… populate queue …
DEL lock:{pattern}              # release lock
```

Other workers wait or retry. Once the queue exists, all workers use `LPOP` directly with no further locking needed.

---

## 4. Performance Analysis

| Operation | Cost |
|---|---|
| Bitmap AND for one pattern | O(N/64) ≈ 15K ops → **< 1 ms** |
| Cache hit (pre-computed pattern) | O(1) → **< 0.1 ms** |
| Redis LPOP (ticket assignment) | O(1) → **< 0.2 ms** network |
| Full index build (startup) | O(N × 6) ≈ 6M ops → **< 1 s** |

**Throughput**: Redis handles > 100,000 operations/second on modest hardware. Concurrent `LPOP` requests for the same pattern are serialised by Redis's single-threaded model — no locking overhead on the hot path.

**Scalability**: The 7.5 MB bitmap index fits in L3 cache on a modern CPU. For > 1M tickets, Redis Cluster shards the keyspace automatically.

---

## 5. Architecture Diagram

```
Client A ──┐                         ┌─ Redis ─────────────────────────────┐
Client B ──┤──► API Server ──────►  │  bitmap[pos][digit]  (index)        │
Client C ──┘    (Go/gRPC)           │  queue:{pattern}     (FIFO of IDs)  │
                     │              │  owner:{ticket_id}   (assignment log)│
                     └─ PostgreSQL  └─────────────────────────────────────┘
                       (audit log)
```

---

## 6. Design Decisions & Trade-offs

| Decision | Reasoning |
|---|---|
| Redis bitmaps over SQL LIKE | SQL `LIKE '1%5'` requires a full table scan (1M rows). Bitmaps are O(N/64) and CPU-cache friendly. |
| Per-pattern queues over real-time scan | Pre-materialising the queue separates search cost from assignment cost. Assignment is O(1). |
| Redis over Elasticsearch | Elasticsearch handles wildcard full-text well but adds operational complexity. For fixed-length numeric patterns, bitmaps are simpler and faster. |
| Queue per pattern over global lock | A global lock serialises all users. Per-pattern queues allow full parallelism across different patterns. |
| Return-to-queue on timeout | Prevents ticket starvation when users abandon their session before claiming. |
