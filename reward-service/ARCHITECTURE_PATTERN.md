# Reward Service Architecture Pattern

This service follows the same pattern as the refactored travel/user-coupon services.

## Layers

- `cmd/main.go` — entrypoint only.
- `internal/app` — bootstrap/wiring for config, infra, adapters, handlers, services, and runtime.
- `internal/integration` — pure infra/client/server setup such as Postgres, NATS, HTTP server.
- `internal/adapter/inbound` — HTTP/event/admin consumers that receive input and call handler/ports.
- `internal/adapter/outbound` — Postgres repositories and NATS producers that implement outbound ports.
- `internal/service_logic` — handler + business service flow.
- `internal/interface` — ports/interfaces connecting layers.
- `pkg` — service-specific engine package adapters: metrics, health, retry/resilience, logger, apperror, rewardguard, enginebundle.

## Shared library usage

- Middleware wrapper packages are imported only from `github.com/PlatformCore/libpackage/middleware/...`, mainly from HTTP middleware composition.
- Engine-core packages are adapted under `pkg/` so service logic imports local service-oriented packages, not scattered external packages.

## Integration rule

`integration` remains separate as infra. It is wired together with adapters in `internal/app`.
