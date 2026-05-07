# Reward Service — Enterprise Enhancements

## What Was Added

### 1. Circuit Breaker (`internal/resilience/circuit_breaker.go`)
Protects outbound NATS calls from cascading failures.

- **States**: CLOSED → OPEN → HALF-OPEN → CLOSED
- **Auto-recovery**: After `Timeout` (30s), allows one probe request
- **Prometheus integration**: State changes reported as metrics
- **Admin endpoint**: `GET /api/v1/admin/circuit-breakers` shows live stats

```
CLOSED  → normal traffic
OPEN    → fast-fail, returns ErrCircuitOpen immediately
HALF-OPEN → one test request, transitions back on success/failure
```

---

### 2. Retry with Exponential Backoff + Jitter (`internal/resilience/retry.go`)
Wraps any operation with configurable retries.

- Exponential backoff with configurable multiplier
- Jitter factor prevents thundering-herd on retry storms
- Composable with circuit breaker via `WithCircuitBreaker()`

---

### 3. Outbox Pattern (`internal/outbox/outbox.go`)
**Guarantees at-least-once delivery** for NATS publishes.

Problem it solves: If the DB write succeeds but NATS publish fails, points are
earned but never forwarded to user-reward-service.

Solution:
1. Business data + outbox message saved in the **same DB transaction**
2. Background `Relay` polls `reward_outbox` every 2 seconds
3. Failed messages are retried up to 5 times, then moved to `dead` status

```
Earn Event arrives
      │
      ▼
[DB Transaction]
  INSERT earn_transaction   ← business record
  INSERT reward_outbox      ← delivery guarantee
      │
      ▼
[Outbox Relay] (background)
  SELECT ... FOR UPDATE SKIP LOCKED
  NATS Publish
  UPDATE outbox SET status='sent'
```

---

### 4. Idempotency Middleware (`internal/middleware/idempotency.go`)
Prevents duplicate processing of retried HTTP requests.

- Clients send `Idempotency-Key: <uuid>` header
- Same key within 24h returns cached response
- Response includes `Idempotency-Replayed: true` header

---

### 5. Rate Limiter (`internal/middleware/rate_limiter.go`)
Token bucket algorithm, per-user or per-IP.

- Default: 10 burst, 2 req/sec refill
- Returns `429 Too Many Requests` with `Retry-After` header
- Auto-cleans inactive buckets after 30 minutes

---

### 6. Structured Logging with Trace IDs (`internal/observability/logger.go`)
JSON-structured logs with automatic context propagation.

- Every request gets a `trace_id` (from upstream or auto-generated)
- Trace ID propagated via `X-Trace-ID` header to downstream services
- All service methods log with trace context
- `StartOp` / `End` pattern for operation timing

```json
{"time":"2025-01-01T00:00:00Z","level":"INFO","msg":"earn processed",
 "service":"reward-service","trace_id":"abc-123","earn_id":"xyz","points":50}
```

---

### 7. Prometheus Metrics (`internal/observability/metrics.go`)
Full observability via `/metrics` endpoint.

| Metric | Type | Labels |
|--------|------|--------|
| `reward_earn_processed_total` | Counter | source |
| `reward_earn_points_total` | Counter | source |
| `reward_redeem_requests_total` | Counter | status |
| `reward_http_request_duration_seconds` | Histogram | method, path |
| `reward_nats_publish_duration_seconds` | Histogram | subject |
| `reward_db_query_duration_seconds` | Histogram | operation |
| `reward_circuit_breaker_state` | Gauge | name |
| `reward_duplicate_earn_total` | Counter | — |

---

### 8. Health Checks (`internal/health/checker.go`)
Kubernetes-compatible health endpoints.

| Endpoint | Purpose |
|----------|---------|
| `GET /live` | Liveness probe — is the process alive? |
| `GET /ready` | Readiness probe — can it serve traffic? |
| `GET /health` | Full dependency health report |

```json
{
  "status": "healthy",
  "service": "reward-service",
  "checks": {
    "postgres": {"status": "healthy", "latency_ms": "2ms"},
    "nats":     {"status": "healthy", "latency_ms": "1ms"}
  }
}
```

---

### 9. Graceful Shutdown (`internal/lifecycle/shutdown.go`)
LIFO shutdown hooks — components shut down in reverse init order.

- 15-second global shutdown timeout
- HTTP server drains in-flight requests before closing
- NATS connection drains before closing
- DB connections closed last

---

### 10. Enhanced Service Logic
- **Double-idempotency**: Check `ref_id` (DB level) + outbox pattern
- **Max redeem guard**: Rejects single request > 1,000,000 points
- **Source validation**: Only allows `trip | order | campaign`
- **Structured audit trail**: Every earn/redeem logged with full context

---

## File Structure

```
internal/
├── middleware/
│   ├── idempotency.go        # HTTP idempotency (Idempotency-Key header)
│   └── rate_limiter.go       # Token bucket rate limiting
├── resilience/
│   ├── circuit_breaker.go    # Circuit breaker pattern
│   └── retry.go              # Exponential backoff + jitter
├── observability/
│   ├── logger.go             # Structured logging + trace IDs
│   └── metrics.go            # Prometheus metrics
├── health/
│   └── checker.go            # /health /ready /live endpoints
├── outbox/
│   └── outbox.go             # Transactional outbox pattern
├── lifecycle/
│   └── shutdown.go           # Graceful shutdown manager
└── service_logic/service/calculation/
    └── enterprise_reward_service.go  # Enhanced service with all patterns
cmd/
└── main.go                   # Updated wiring
migrations/
└── 002_enterprise_enhancements.sql  # DB changes needed
```

## Deployment Checklist

- [ ] Run `migrations/002_enterprise_enhancements.sql`
- [ ] Set `SERVICE_VERSION` env var for log versioning
- [ ] Configure Prometheus scrape: `GET /metrics`
- [ ] Configure k8s liveness: `GET /live`
- [ ] Configure k8s readiness: `GET /ready`
- [ ] Clients should send `Idempotency-Key` header on POST /redeem
