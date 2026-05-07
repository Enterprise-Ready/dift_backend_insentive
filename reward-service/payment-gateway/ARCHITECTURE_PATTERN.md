# Payment Gateway Strict Enterprise Pattern

This service follows the same strict pattern used by the latest refactored services:

- `cmd/server/main.go` is only the entrypoint.
- `internal/app` owns bootstrap, wiring, lifecycle and graceful shutdown.
- `internal/integration` owns low-level infrastructure clients such as Postgres and Redis.
- `internal/adapter/inbound/http` owns HTTP routing, request decoding and payment-gateway-specific middleware composition.
- `internal/adapter/outbound/persistence/postgres` owns repository implementations.
- `internal/service_logic/service` owns business use cases and provider orchestration.
- `pkg/*` owns service-specific wrappers around shared engines or domain-specific helpers.

## Middleware rule

Shared middleware wrappers are imported directly from:

`github.com/PlatformCore/libpackage/middleware/...`

The service does not duplicate generic middleware implementation. Local middleware remains only for payment-gateway-specific concerns such as API key auth, idempotency header extraction, webhook signature checks and merchant rate-limit composition.

## Engine rule

Shared engine/core packages are adapted through service-specific `pkg/` packages when they must be aligned with payment business logic, for example:

- `pkg/paymentengine`
- `pkg/paymentguard`
- `pkg/metrics`
- `pkg/health`
- `pkg/apperror`

## Safety

The original source tree was preserved under `docs/legacy-source-backup` so no original logic is permanently lost during refactor.
