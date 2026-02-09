# GoliveKit Documentation

Welcome to the GoliveKit documentation. GoliveKit is a Phoenix LiveView-inspired framework for Go that enables building interactive, real-time web applications without writing JavaScript.

## Quick Links

- [Getting Started](./getting-started.md) - Installation and first steps
- [Architecture](./architecture.md) - How GoliveKit works
- [JavaScript Client](./javascript-client.md) - Browser-side API
- [Testing](./testing.md) - Unit, chaos, and fuzz testing
- [Examples](./examples.md) - Sample applications

## Package Reference (29 packages)

### Core

| Package | Description |
|---------|-------------|
| [core](./packages/core.md) | Component lifecycle, Socket, Assigns, Session |
| [router](./packages/router.md) | HTTP routing, WebSocket upgrade, middleware |
| [transport](./packages/transport.md) | WebSocket, SSE, Long-polling transports |
| [diff](./packages/diff.md) | Hybrid HTML diff engine with slot caching |
| [protocol](./packages/protocol.md) | Wire protocol, Phoenix-compatible codec |

### UI & Forms

| Package | Description |
|---------|-------------|
| [forms](./packages/forms.md) | Ecto-style changesets and validation |
| [islands](./packages/islands.md) | Partial hydration (5 strategies) |
| [streaming](./packages/streaming.md) | SSR with suspense boundaries |
| [a11y](./packages/a11y.md) | Accessibility helpers |

### State & Real-time

| Package | Description |
|---------|-------------|
| [state](./packages/state.md) | State persistence (Memory, Redis) |
| [pubsub](./packages/pubsub.md) | Real-time pub/sub messaging |
| [presence](./packages/presence.md) | User presence tracking |

### DevOps & Monitoring

| Package | Description |
|---------|-------------|
| [logging](./packages/logging.md) | Structured logging (slog) |
| [metrics](./packages/metrics.md) | Internal metrics collection |
| [tracing](./packages/tracing.md) | OpenTelemetry integration |
| [shutdown](./packages/shutdown.md) | Graceful shutdown handler |
| [health](./packages/health.md) | Kubernetes health checks |
| [observability](./packages/observability.md) | Prometheus-style metrics |

### Security

| Package | Description |
|---------|-------------|
| [security](./packages/security.md) | CSRF, XSS prevention, sanitization |
| [limits](./packages/limits.md) | Rate limiting, backpressure |
| [audit](./packages/audit.md) | Security event logging |
| [recovery](./packages/recovery.md) | State recovery for reconnections |

### Utilities

| Package | Description |
|---------|-------------|
| [retry](./packages/retry.md) | Exponential backoff with jitter |
| [pool](./packages/pool.md) | Buffer pools, RingBuffer |
| [i18n](./packages/i18n.md) | Internationalization |
| [uploads](./packages/uploads.md) | File uploads (multipart, chunked, S3/GCS) |
| [testing](./packages/testing.md) | Component testing utilities |
| [js](./packages/js.md) | JavaScript commands |

### Extensions

| Package | Description |
|---------|-------------|
| [plugin](./packages/plugin.md) | Plugin system with 15+ hooks |

## Plugins

| Plugin | Description |
|--------|-------------|
| [Auth Plugin](./plugins/auth.md) | Authentication with RBAC |

## Live Demo

Visit [golivekit.cloud](https://golivekit.cloud) to see GoliveKit in action.

## Contributing

We welcome contributions! See [CONTRIBUTING.md](../CONTRIBUTING.md) for guidelines.
