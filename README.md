<p align="center">
  <img src="assets/logo.png" alt="oschema logo" width="600">
</p>

<p align="center">
  An open, spec-driven ingestion engine that normalizes data from webhooks and external APIs into a unified schema.
</p>

## What is oschema?

oschema sits between your data sources (Shopify, Stripe, GitHub, etc.) and your internal systems. Instead of writing custom integration code for each source, you write a simple YAML spec that tells oschema how to map incoming payloads into a unified event format.

**Key features:**
- **Spec-driven** — Add new integrations via YAML, no code changes
- **Unified schema** — All events normalized to a single format
- **Deduplication** — Prevents duplicate event processing via Redis
- **Durable queue** — Redis Streams with retry + exponential backoff
- **Dead letter** — Failed events captured for inspection and replay
- **CLI** — Manage specs, run the server, replay events

## Architecture

```
                    ┌─────────────────────────────────────────────┐
                    │                  oschema                     │
                    │                                             │
  Webhook ──POST──▶│  /ingest/{source}                           │
                    │       │                                     │
                    │       ▼                                     │
                    │  ┌──────────┐   ┌──────────┐               │
                    │  │  Dedupe  │──▶│  Spec    │               │
                    │  │(Redis NX)│   │ Registry │               │
                    │  └──────────┘   └──────────┘               │
                    │       │              │                      │
                    │       ▼              ▼                      │
                    │  ┌──────────────────────┐                  │
                    │  │   Mapping Engine      │                  │
                    │  │  (dot-notation YAML)  │                  │
                    │  └──────────────────────┘                  │
                    │       │                                     │
                    │       ▼                                     │
                    │  ┌──────────┐   ┌──────────┐               │
                    │  │  Queue   │──▶│  Worker  │               │
                    │  │(Redis    │   │  Pool    │               │
                    │  │ Streams) │   │          │               │
                    │  └──────────┘   └──────────┘               │
                    │                      │                      │
                    │                      ▼                      │
                    │              ┌──────────────┐              │
                    │              │  Event Store  │              │
                    │              │(Redis Streams)│              │
                    │              └──────────────┘              │
                    └─────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- Go 1.22+
- Redis 7+

### Install & Run

```bash
# Clone
git clone https://github.com/O-Schema/oschema.git
cd oschema

# Build
go build -o oschema ./cmd/server/

# Start Redis (if not running)
redis-server &

# Run the server
./oschema serve
```

The server starts on port 8080 with 10 built-in adapter specs.

### Built-in Integrations

| Source | Type Detection | Key Events |
|--------|---------------|------------|
| **Shopify** | `X-Shopify-Topic` header | `order.created`, `order.updated`, `product.created` |
| **Stripe** | `type` body field | `payment.charge_succeeded`, `subscription.created`, `invoice.paid` |
| **GitHub** | `X-GitHub-Event` header | `repo.push`, `repo.pull_request`, `repo.issue`, `repo.release` |
| **Slack** | `event.type` body field | `chat.message`, `chat.mention`, `chat.reaction_added` |
| **Jira** | `webhookEvent` body field | `issue.created`, `issue.updated`, `issue.comment_created` |
| **Linear** | `action` body field | `issue.created`, `issue.updated`, `issue.deleted` |
| **PagerDuty** | `event.event_type` body field | `incident.triggered`, `incident.acknowledged`, `incident.resolved` |
| **SendGrid** | `event` body field | `email.delivered`, `email.bounced`, `email.opened`, `email.clicked` |
| **Discord** | `type` body field | `interaction.application_command`, `interaction.message_component` |
| **Twilio** | `SmsStatus` body field | `sms.received`, `sms.delivered`, `sms.failed` |

### Send a Test Webhook

```bash
curl -X POST http://localhost:8080/ingest/shopify \
  -H "Content-Type: application/json" \
  -H "X-Shopify-Topic: orders/create" \
  -d '{
    "id": 12345,
    "created_at": "2024-07-01T12:00:00Z",
    "total_price": "150.00",
    "customer": {
      "email": "customer@example.com"
    },
    "line_items": [
      {"title": "Widget", "quantity": 2}
    ]
  }'
```

Response:
```json
{
  "status": "accepted",
  "id": "a1b2c3d4-e5f6-..."
}
```

The payload is normalized into:
```json
{
  "id": "a1b2c3d4-e5f6-...",
  "source": "shopify",
  "version": "2024-07",
  "type": "order.created",
  "external_id": "12345",
  "timestamp": "2024-07-01T12:00:00Z",
  "data": {
    "order_id": 12345,
    "total": "150.00",
    "customer_email": "customer@example.com",
    "line_items": [{"title": "Widget", "quantity": 2}]
  },
  "raw": { "..." }
}
```

## Adding a New Integration

Create a YAML spec file — no code changes needed.

### 1. Create the Spec File

Create `specs/stripe_2024-01.yml`:

```yaml
source: stripe
version: "2024-01"

# HTTP header containing the event type
type_header: "Stripe-Event-Type"

# Map source event types to your canonical types
type_mapping:
  charge.succeeded: payment.completed
  charge.failed: payment.failed
  customer.created: customer.created
  invoice.paid: invoice.paid

# Field extraction (dot-notation into the raw JSON payload)
fields:
  external_id: id
  timestamp: created
  data:
    amount: data.object.amount
    currency: data.object.currency
    customer_id: data.object.customer
    receipt_email: data.object.receipt_email
```

### 2. Run with Custom Specs Directory

```bash
./oschema serve --specs-dir ./specs
```

### 3. Send Events

```bash
curl -X POST http://localhost:8080/ingest/stripe \
  -H "Content-Type: application/json" \
  -H "Stripe-Event-Type: charge.succeeded" \
  -d '{
    "id": "evt_abc123",
    "created": "2024-01-15T10:30:00Z",
    "data": {
      "object": {
        "amount": 5000,
        "currency": "usd",
        "customer": "cus_xyz",
        "receipt_email": "buyer@example.com"
      }
    }
  }'
```

### Spec Reference

| Field | Description |
|-------|-------------|
| `source` | Source name (matches URL path `/ingest/{source}`) |
| `version` | Spec version string |
| `type_header` | HTTP header name containing the source event type |
| `type_mapping` | Map of source event type → canonical event type |
| `fields.external_id` | Dot-notation path to the unique ID in the payload |
| `fields.timestamp` | Dot-notation path to the event timestamp (RFC3339) |
| `fields.data` | Map of output field name → dot-notation path in payload |

### Dot-Notation

Fields are extracted using dot-notation paths into the JSON payload:

| Path | Payload | Extracted Value |
|------|---------|-----------------|
| `id` | `{"id": "123"}` | `"123"` |
| `customer.email` | `{"customer": {"email": "a@b.com"}}` | `"a@b.com"` |
| `data.object.amount` | `{"data": {"object": {"amount": 50}}}` | `50` |
| `tags` | `{"tags": ["a", "b"]}` | `["a", "b"]` |
| `missing.field` | `{"id": "123"}` | `null` |

## CLI Reference

### `oschema serve`

Start the ingestion server.

```bash
oschema serve [flags]
```

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--port` | `OSCHEMA_PORT` | `8080` | Server port |
| `--redis-url` | `OSCHEMA_REDIS_URL` | `redis://localhost:6379` | Redis connection URL |
| `--specs-dir` | `OSCHEMA_SPECS_DIR` | *(embedded)* | Additional specs directory |
| `--workers` | `OSCHEMA_WORKERS` | `4` | Number of queue workers |
| `--max-retries` | `OSCHEMA_MAX_RETRIES` | `5` | Max retry attempts |
| `--dedupe-ttl` | `OSCHEMA_DEDUPE_TTL` | `24h` | Deduplication window |

### `oschema specs list`

List all loaded adapter specs.

```bash
oschema specs list [--specs-dir ./specs]
```

Output:
```
SOURCE               VERSION         TYPE HEADER
------               -------         -----------
shopify              2024-07         X-Shopify-Topic
stripe               2024-01         Stripe-Event-Type
```

### `oschema replay`

Replay stored events from a source.

```bash
oschema replay --source shopify [--limit 100] [--redis-url redis://localhost:6379]
```

## API Reference

### `POST /ingest/{source}`

Ingest a webhook payload.

**Path parameters:**
- `source` — Adapter name (must match a loaded spec)

**Headers:**
- `Content-Type: application/json` (required)
- `X-Spec-Version: 2024-07` (optional — override spec version)
- Source-specific type header (e.g., `X-Shopify-Topic`) as defined in the spec

**Query parameters:**
- `version` — Alternative to `X-Spec-Version` header

**Responses:**

| Code | Description |
|------|-------------|
| `202 Accepted` | Event accepted and queued for processing |
| `200 OK` | Duplicate event (already processed) |
| `400 Bad Request` | Invalid JSON body |
| `404 Not Found` | Unknown source or version |
| `503 Service Unavailable` | Redis unavailable |

### `GET /health`

Health check endpoint. Returns `{"status":"ok"}`.

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
  "data": {},
  "raw": {}
}
```

| Field | Description |
|-------|-------------|
| `id` | Server-generated UUID v4 |
| `source` | Adapter name (from URL path) |
| `version` | Spec version used for normalization |
| `type` | Canonical event type (mapped via spec, or pass-through if unmapped) |
| `external_id` | Source-side unique ID (extracted via spec) |
| `timestamp` | Event timestamp in RFC3339 (extracted via spec, falls back to ingestion time) |
| `data` | Normalized payload fields (mapped via spec) |
| `raw` | Original unmodified payload |

## Data Flow

```
1. POST /ingest/shopify arrives
2. Body parsed as JSON
3. Spec resolved by source + version (header/query/latest)
4. Payload normalized using spec's mapping engine
5. Deduplication check via Redis SET NX (24h TTL)
   - If duplicate → 200 OK, discard
6. Event enqueued to Redis Stream (oschema:queue)
7. 202 Accepted returned with event ID
8. Worker dequeues via consumer group (XREADGROUP)
9. Event stored in Redis Stream (oschema:events:{source}) + hash index
10. On failure: retry with exponential backoff (1s → 2s → 4s → 8s → 16s)
11. After max retries (default 5): moved to dead letter stream (oschema:deadletter)
```

## Project Structure

```
cmd/server/main.go           — Application entrypoint
internal/
  cli/                       — Cobra CLI commands (serve, specs, replay)
  ingestion/                 — HTTP handler for POST /ingest/{source}
  adapters/                  — YAML spec loading, parsing, and resolution
  normalization/             — Mapping engine (dot-notation field extraction)
  store/                     — Event storage (Redis Streams + in-memory)
  queue/                     — Job queue + worker pool (Redis Streams)
  dedupe/                    — Deduplication (Redis SET NX)
pkg/
  event/                     — Shared Event type definition
configs/specs/               — Embedded default YAML adapter specs
```

## Development

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test ./... -v

# Build
go build -o oschema ./cmd/server/

# Run with custom specs
./oschema serve --specs-dir ./my-specs --port 9090

# List loaded specs
./oschema specs list
```

## Redis Streams

oschema uses the following Redis keys:

| Key Pattern | Type | Purpose |
|-------------|------|---------|
| `oschema:queue` | Stream | Processing queue (consumer group: `oschema-workers`) |
| `oschema:events:{source}` | Stream | Stored events per source |
| `oschema:event_index` | Hash | Event ID → JSON lookup index |
| `oschema:deadletter` | Stream | Failed events after max retries |
| `dedupe:{source}:{external_id}` | String | Deduplication keys (TTL-based) |

## License

MIT
