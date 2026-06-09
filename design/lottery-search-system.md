# Lottery Search System — Design Proposal

> Design a system to search 1 million 6-digit lottery tickets by wildcard pattern and distribute results to concurrent users without duplication.

---

## 1. Requirements Summary

| # | Requirement |
|---|---|
| Data | 1 million tickets, each a 6-digit number (`000000`–`999999`) |
| Search | 6-character pattern with digits and `*` wildcards (e.g. `1****5`, `****23`) |
| Distribution | Same ticket must never be assigned to two users simultaneously |
| Performance | Sub-millisecond search on 1M+ records |

---

## 2. Recommended Storage — Redis + PostgreSQL

### Redis (primary)
- Native bitmap operations (`BITOP AND`) run server-side with zero data-transfer overhead
- Single-threaded command execution makes every `LPOP` atomic — no separate locking needed for ticket assignment
- Persistence via RDB + AOF when durability is required
- Scales horizontally via Redis Cluster if the dataset grows

### PostgreSQL (secondary)
- Durable ownership records: who received which ticket, when, and for which pattern
- Audit trail and analytics queries

---

## 3. Data Structures & Algorithm

### Bitmap index

Pre-compute **60 bitmaps** — one per (position, digit) pair:

```
bitmap[pos][digit]  →  bitset of 1,000,000 bits
```

For ticket `"123456"`, bits are set in:
`bitmap[0][1]`, `bitmap[1][2]`, `bitmap[2][3]`, `bitmap[3][4]`, `bitmap[4][5]`, `bitmap[5][6]`

**Memory**: 60 × 125 KB = **7.5 MB** — fits entirely in L3 cache.

### Pattern matching

For pattern `"1****5"`:
1. Parse constrained positions: `pos[0] = 1`, `pos[5] = 5`
2. AND the two bitmaps: `bitmap[0][1] AND bitmap[5][5]`
3. The resulting bitset contains every matching ticket ID

**Time complexity**: O(N/64) per AND — ~15,625 64-bit word operations for 1M tickets → **< 1 ms**.

**Pattern cache**: Results for previously-seen patterns are cached as bitsets. An LRU cache of 10,000 patterns costs < 1.5 GB. In practice patterns are sparse, so memory usage is much lower.

### Redis key layout

```
bitmap:{pos}:{digit}     →  BITMAP (1M bits per key)
queue:{pattern}          →  LIST of ticket IDs, consumed via LPOP
lock:{pattern}           →  STRING (SET NX EX 5), prevents duplicate queue population
reserved:{ticket_id}     →  STRING with TTL, marks ticket as pending claim
```

---

## 4. Concurrency Strategy — Atomic Queue Consumption

### Problem
Two users submit the same pattern at the same time. A naïve scan returns the same list to both → duplicate assignment.

### Solution

**First request for a pattern:**
1. Acquire `lock:{pattern}` with `SET NX EX 5` (5-second TTL)
2. Run bitmap AND → list of matching ticket IDs
3. `RPUSH queue:{pattern} [ids...]` to populate the queue
4. Release lock
5. `LPOP queue:{pattern}` → returns one ticket atomically to this user

**All subsequent requests:**
- `LPOP queue:{pattern}` → returns the next available ticket, never a duplicate

**If no tickets remain:** return "no tickets available"

**Ticket return (timeout):**
- On `LPOP`, set `reserved:{ticket_id}` with a 30-second TTL
- If user doesn't confirm within TTL, a background worker `RPUSH`es the ticket back to `queue:{pattern}`

```
User A  LPOP queue:"1****5"  →  "100005"  ✓ unique
User B  LPOP queue:"1****5"  →  "100015"  ✓ unique
User C  LPOP queue:"1****5"  →  "100025"  ✓ unique
```

Redis guarantees `LPOP` is atomic — two clients cannot receive the same element.

---

## 5. Ticket Lifecycle

```
┌───────────┐   LPOP (atomic)   ┌──────────────┐   User confirms   ┌──────────┐
│ Available │ ────────────────► │   Reserved   │ ────────────────► │ Assigned │
│ in queue  │                   │  TTL = 30s   │                   │  in PG   │
└───────────┘                   └──────────────┘                   └──────────┘
      ▲                                │
      │          TTL expired           │
      └────────────────────────────────┘
             background worker RPUSH
```

---

## 6. Performance Analysis

| Operation | Complexity | Latency |
|---|---|---|
| Bitmap AND (pattern match) | O(N/64) ~15K ops | **< 1 ms** |
| Cache hit (seen pattern) | O(1) | **< 0.1 ms** |
| LPOP (ticket assignment) | O(1) | **< 0.2 ms** |
| Index build at startup | O(N×6) ~6M ops | **< 1 s** |
| Full index memory | 60 × 125 KB | **7.5 MB** |

Redis handles > 100,000 ops/sec on modest hardware. Concurrent `LPOP` requests for the same pattern are serialised by Redis's single-threaded model — no locking overhead on the hot path.

---

## 7. Design Decisions & Trade-offs

| Decision | Reasoning |
|---|---|
| Redis bitmaps over SQL LIKE | SQL `LIKE '1%5'` requires a full table scan on 1M rows. Bitmaps are O(N/64) and CPU-cache friendly. |
| Per-pattern queues over real-time scan | Separates search cost (one-time) from assignment cost (O(1) per user). |
| Redis over Elasticsearch | Elasticsearch handles wildcard full-text well but adds operational complexity. For fixed-length numeric patterns, bitmaps are simpler and faster. |
| Queue per pattern over global lock | A global lock serialises all users. Per-pattern queues allow full parallelism across different patterns. |
| Reservation TTL over permanent hold | Prevents ticket starvation when users abandon sessions before confirming. |
| PostgreSQL for audit | Redis is ephemeral by default. Ownership records need durability — PostgreSQL is the right tool. |
