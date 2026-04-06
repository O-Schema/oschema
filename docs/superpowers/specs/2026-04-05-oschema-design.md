# oschema Design Spec

**Date:** 2026-04-05
**Status:** Draft

## Overview

oschema is an open, spec-driven ingestion engine that normalizes data from webhooks and external APIs into a unified schema. It supports push (webhooks) and pull (API polling) ingestion, with versioned YAML-based adapter specs that define how to map source-specific payloads into a canonical event format.

## Goals

- Normalize heterogeneous webhook/API payloads into a single event schema
- Add new integrations via YAML specs — no code changes required
- Deduplicate events reliably
- Retry failed processing with exponential backoff
- Provide a CLI for operating the server, managing specs, and replaying events

## Non-Goals (for initial release)

- Authentication/authorization on the ingest endpoint
- Multi-tenant isolation
- Horizontal scaling / clustering
- API polling adapters (interface only, no implementation yet)

## Architecture

```
cmd/server/main.go          — entrypoint, wires dependencies
internal/
  ingestion/                — HTTP handlers (POST /ingest/{source})
  adapters/                 — YAML spec loader + registry
  normalization/            — mapping engine (dot-notation field extraction)
  store/                    — EventStore interface + Redis Streams + in-memory fallback
  queue/                    — Redis-backed job queue with retry + backoff
  dedupe/                   — Redis SET NX deduplication with TTL
pkg/
  event/                    — shared Event type definition
configs/specs/              — embedded default YAML specs (overridable at runtime)
```

## Technology Choices

| Component | Choice | Rationale |
|-----------|--------|-----------|
| HTTP | `net/http` + Go 1.26 `ServeMux` | Zero deps, built-in `{param}` routing |
| Event store | Redis Streams | High throughput, consumer groups, natural event log |
| Job queue | Redis Streams | Same infra as store, durable retries |
| Deduplication | Redis `SET NX` with TTL | Atomic, fast, auto-expiring |
| CLI | Cobra | Industry standard, subcommand support |
| Spec format | YAML only | Human-readable, widely adopted |
| Spec loading | `go:embed` + `--specs-dir` override | Zero-config defaults, runtime flexibility |

## Unified Event Schema

Every ingested event is normalized into this structure:

```json
{
  "id": "uuid-v4",
  "source": "shopify",
  "version": "2024-07",
  "type": "order.created",
  "external_id": "12345",
  "timestamp": "2024-07-01T12:00:00Z",
  "data": {
    "order_id": "12345",
    "total": "150.00",
    "customer_email": "user@example.com"
  },
  "raw": { ... }
}
```

Fields:
- `id` — server-generated UUID
- `source` — adapter name (from URL path)
- `version` — spec version (from header or query param)
- `type` — normalized event type (mapped from source-specific type)
- `external_id` — source-side unique ID (extracted via spec)
- `timestamp` — event timestamp (extracted via spec, falls back to ingestion time)
- `data` — normalized payload fields (mapped via spec)
- `raw` — original unmodified payload

## Adapter Spec Format (YAML)

```yaml
source: shopify
version: "2024-07"

# Where to find the source event type
type_header: "X-Shopify-Topic"

# Map source event types to canonical types
type_mapping:
  orders/create: order.created
  orders/updated: order.updated
  products/create: product.created

# Field extraction using dot-notation paths into the raw payload
fields:
  external_id: id
  timestamp: created_at
  data:
    order_id: id
    total: total_price
    customer_email: customer.email
    line_items: line_items
```

### Spec Resolution

1. Source is determined from the URL path: `POST /ingest/{source}`
2. Version is determined by (in order): `X-Spec-Version` header, `?version=` query param, latest available version
3. Spec file is loaded from: runtime `--specs-dir` first, then embedded defaults

### Mapping Engine

- Dot-notation traversal: `customer.email` navigates `{"customer": {"email": "val"}}`
- Missing fields produce `null` in output (not an error)
- Type coercion: values are preserved as-is (string, number, bool, object, array)

## Data Flow

```
1. POST /ingest/shopify arrives
2. Parse body as JSON
3. Dedupe check: Redis SET NX on key "dedupe:{source}:{external_id}" with 24h TTL
   - If key exists → return 200 (already processed), discard
4. Resolve spec: source="shopify", version from header/query/latest
5. Normalize: apply spec mapping engine to raw payload → unified event
6. Enqueue: XADD to Redis Stream "oschema:queue"
7. Return 202 Accepted with event ID
8. Worker goroutine: XREADGROUP from "oschema:queue"
   - Store event: XADD to "oschema:events:{source}" (raw + normalized)
   - ACK the queue message
   - On failure: increment attempt count, re-enqueue with backoff delay
   - After max attempts (default 5): move to "oschema:deadletter" stream
```

## Queue & Retry

- Redis Stream: `oschema:queue` with consumer group `oschema-workers`
- Worker pool: configurable concurrency (default 4 goroutines)
- Retry backoff: exponential — 1s, 2s, 4s, 8s, 16s (base * 2^attempt)
- Max retries: 5 (configurable)
- Dead letter: `oschema:deadletter` stream for inspection/replay

## CLI Commands (Cobra)

```
oschema serve [--port 8080] [--specs-dir ./specs] [--redis-url redis://localhost:6379]
oschema specs list [--specs-dir ./specs]
oschema replay --source shopify [--from 2024-01-01] [--to 2024-01-31]
```

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/redis/go-redis/v9` | Redis client |
| `gopkg.in/yaml.v3` | YAML spec parsing |
| `github.com/google/uuid` | Event ID generation |

## Error Handling

- Invalid JSON body → 400 Bad Request
- Unknown source (no spec found) → 404 Not Found
- Spec version not found → 404 with available versions in response
- Redis unavailable → 503 Service Unavailable with retry-after header
- Normalization failure → event stored in dead letter with error context

## Configuration

Server config via environment variables and/or CLI flags:

| Variable | Flag | Default |
|----------|------|---------|
| `OSCHEMA_PORT` | `--port` | `8080` |
| `OSCHEMA_REDIS_URL` | `--redis-url` | `redis://localhost:6379` |
| `OSCHEMA_SPECS_DIR` | `--specs-dir` | (uses embedded) |
| `OSCHEMA_WORKERS` | `--workers` | `4` |
| `OSCHEMA_MAX_RETRIES` | `--max-retries` | `5` |
| `OSCHEMA_DEDUPE_TTL` | `--dedupe-ttl` | `24h` |

## Testing Strategy

- Unit tests for mapping engine (dot-notation extraction, type mapping)
- Unit tests for spec loading and resolution
- Integration tests for the full ingest flow (using miniredis or test containers)
- Example spec for Shopify included in `configs/specs/`
