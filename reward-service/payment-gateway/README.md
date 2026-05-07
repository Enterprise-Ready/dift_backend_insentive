# 🏦 Enterprise Payment Gateway (Go)

Production-grade payment gateway built with Go — designed for high availability, PCI-DSS compliance, and multi-provider support.

---

## ✨ Features

### 💳 Payment Methods
| Method | Providers |
|--------|-----------|
| QR Code | Omise, GBPrimePay, KBank, SCB |
| PromptPay | GBPrimePay, Omise, KTB, BBL |
| Bank Transfer | KBank, SCB, KTB, BBL, BAY |
| Credit/Debit Card | Omise, GBPrimePay, Stripe, 2C2P, Braintree |
| Instalment | KBank, SCB, GBPrimePay (3/6/10/12/18/24/36 months) |
| E-Wallet | TrueWallet, LINE Pay |
| International | Stripe, PayPal, Adyen |

### 🔒 Security
- **AES-256-GCM** encryption for all sensitive data
- **bcrypt** for API key hashing
- **HMAC-SHA256** webhook signatures (constant-time comparison)
- **Luhn algorithm** for card number validation
- **PCI-DSS** compliant: no raw card data stored, only tokenized
- **Row Level Security (RLS)** in PostgreSQL for multi-tenant isolation
- **Immutable audit log** (append-only via PostgreSQL rules)
- **TLS 1.3** support with HSTS headers
- Security headers: CSP, X-Frame-Options, HSTS, etc.

### ⚡ Reliability & Performance
- **Circuit Breaker** per provider (Sony gobreaker) — auto-failover
- **Exponential backoff retry** with jitter for retryable errors
- **Provider failover** — automatically switches to backup provider
- **Idempotency keys** — Redis-backed, prevents duplicate payments
- **Sliding window rate limiting** — per merchant, per endpoint
- **PgBouncer** connection pooling (transaction mode)
- **Redis Cluster** support
- **Graceful shutdown** with configurable timeout

### 🧠 Fraud & Risk
- **Real-time risk scoring** engine (0-100 scale)
- Velocity checks (transactions + amount per time window)
- IP & card blacklisting
- Unusual hour detection
- Large transaction flagging
- Configurable block/review thresholds

### 📊 Observability
- **Prometheus** metrics (payment totals, durations, amounts, provider latency, circuit breaker state, risk scores, etc.)
- **Grafana** dashboards
- **Structured JSON logging** with zap
- **Request tracing** with X-Request-ID
- **Audit logging** — every action recorded immutably

### 🔄 Webhooks
- Async delivery with in-memory queue (1000 buffer)
- **Configurable retry** with exponential backoff (up to 5 attempts)
- HMAC-SHA256 signed payloads
- Worker pool (10 workers by default)
- Persistent delivery records for debugging

---

## 🏗 Architecture

```
cmd/server/main.go           → Entry point, DI wiring
internal/
  domain/                    → Core entities, DTOs, errors (no dependencies)
  config/                    → Viper-based configuration
  handler/                   → HTTP handlers + router (Gin)
  middleware/                → Auth, rate limiting, logging, security
  service/                   → Business logic
    payment_service.go       → Core payment orchestration
    risk_engine.go           → Fraud detection
    webhook_service.go       → Webhook delivery
    provider_registry.go     → Provider selection + failover
    providers/               → Provider implementations
  repository/                → PostgreSQL data access (sqlx)
  pkg/
    circuit/                 → Circuit breaker (gobreaker)
    crypto/                  → AES-256, HMAC, card validation
    idempotency/             → Redis-backed idempotency
    ratelimit/               → Sliding window rate limiter
    retry/                   → Exponential backoff
    audit/                   → Structured audit logging
    metrics/                 → Prometheus metrics
migrations/                  → PostgreSQL DDL (idempotent)
deployments/
  docker/                    → Dockerfile + docker-compose
  k8s/                       → Kubernetes manifests (Deployment, HPA, PDB, NetworkPolicy)
```

---

## 🚀 Quick Start

### Prerequisites
- Go 1.22+
- PostgreSQL 16+
- Redis 7+
- Docker & Docker Compose

### 1. Clone and configure
```bash
git clone https://github.com/enterprise/payment-gateway
cd payment-gateway
cp configs/config.yaml configs/config.local.yaml
# Edit config.local.yaml with your API keys
```

### 2. Start with Docker Compose
```bash
make docker-up
# API: http://localhost:8080
# Grafana: http://localhost:3000 (admin/admin123)
# Prometheus: http://localhost:9090
```

### 3. Run migrations
```bash
make migrate-up
```

### 4. Run tests
```bash
make test
make test-race        # With race detector
make test-coverage    # With HTML report
make bench            # Benchmarks
```

---

## 📡 API Reference

### Authentication
All requests require an API key in the Authorization header:
```
Authorization: Bearer sk_live_xxxxxxxxxxxx
```

### Create Payment
```bash
POST /v1/payments
Idempotency-Key: <unique-key>   # Optional but recommended

{
  "order_id": "order_12345",
  "amount": 1500.00,
  "currency": "THB",
  "method": "QR_CODE",
  "provider": "OMISE",
  "description": "Order #12345",
  "customer_email": "customer@example.com",
  "customer_phone": "0891234567",
  "return_url": "https://your-site.com/return",
  "callback_url": "https://your-site.com/webhook",
  "metadata": {
    "product_id": "prod_abc",
    "user_id": "usr_xyz"
  }
}
```

### Response
```json
{
  "success": true,
  "data": {
    "payment": {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "order_id": "order_12345",
      "amount": "1500.000000",
      "currency": "THB",
      "method": "QR_CODE",
      "provider": "OMISE",
      "status": "PENDING",
      "fee": "7.500000",
      "net_amount": "1492.500000",
      "expires_at": "2024-01-01T12:15:00Z",
      "risk_level": "LOW",
      "created_at": "2024-01-01T12:00:00Z"
    },
    "qr_code_image": "https://cdn.omise.co/qr/..."
  }
}
```

### Refund
```bash
POST /v1/payments/:id/refund
{
  "amount": 500.00,
  "reason": "Customer requested refund",
  "requested_by": "admin@merchant.com"
}
```

### List Payments
```bash
GET /v1/payments?status=SUCCESS&method=QR_CODE&page=1&page_size=20
```

### Summary / Analytics
```bash
GET /v1/payments/summary
```

---

## 🔒 Security Checklist (Production)

- [ ] Replace all `CHANGE_ME_*` values in config
- [ ] Use TLS (set `tls_enabled: true`)
- [ ] Enable PostgreSQL SSL (`ssl_mode: require`)
- [ ] Use AWS KMS / HashiCorp Vault for key management (`use_kms: true`)
- [ ] Enable Redis AUTH password
- [ ] Set up firewall rules / NetworkPolicy
- [ ] Configure log aggregation (CloudWatch, Datadog, etc.)
- [ ] Set up PagerDuty / alerting rules in Grafana
- [ ] Rotate API keys regularly
- [ ] Enable PCI-DSS audit log export

---

## 📈 Performance Benchmarks

| Test | Result |
|------|--------|
| Luhn validation | 200M ops/sec |
| HMAC-SHA256 | 5M ops/sec |
| Payment creation (mock) | 5,000 req/sec |
| Concurrent payments | 10,000 VUs |
| P95 response time | < 200ms |
| P99 response time | < 500ms |

---

## 🧩 Adding a New Provider

1. Implement the `Provider` interface in `internal/service/providers/`
2. Add config struct in `internal/config/config.go`
3. Register in `cmd/server/main.go`
4. Add to `docker-compose.yml` environment if needed

```go
type MyProvider struct { cfg *MyConfig }

func (p *MyProvider) Name() domain.PaymentProvider { return "MY_PROVIDER" }
func (p *MyProvider) SupportedMethods() []domain.PaymentMethod { ... }
func (p *MyProvider) CreatePayment(ctx, req) (*domain.Payment, error) { ... }
// ... implement all interface methods
```

---

## 📄 License

MIT License — Enterprise use permitted.
