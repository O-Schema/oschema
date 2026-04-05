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

The server starts on port 8080 with 12 built-in adapter specs (10 sources, including 2025 versions for Stripe and GitHub).

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

Create `specs/my-service_2025-01.yml`:

```yaml
source: my-service
version: "2025-01"

# Where to find the event type — pick ONE:
type_header: "X-Event-Type"      # from an HTTP header
# type_field: "event.type"       # OR from a JSON body field (dot-notation)

# Map source event types to your canonical types
type_mapping:
  order.created: order.created
  order.updated: order.updated
  user.signup: user.created

# Field extraction (dot-notation into the raw JSON payload)
fields:
  external_id: id
  timestamp: created_at
  data:
    user_id: user.id
    email: user.email
    amount: order.total
    currency: order.currency
```

### 2. Run with Custom Specs Directory

```bash
./oschema serve --specs-dir ./specs
```

### 3. Send Events

```bash
curl -X POST http://localhost:8080/ingest/my-service \
  -H "Content-Type: application/json" \
  -H "X-Event-Type: order.created" \
  -d '{
    "id": "ord_abc123",
    "created_at": "2025-01-15T10:30:00Z",
    "user": {"id": "usr_xyz", "email": "buyer@example.com"},
    "order": {"total": 5000, "currency": "usd"}
  }'
```

### Spec Reference

| Field | Description |
|-------|-------------|
| `source` | Source name (matches URL path `/ingest/{source}`) |
| `version` | Spec version string |
| `type_header` | HTTP header containing the event type (use this OR `type_field`) |
| `type_field` | JSON body field containing the event type, dot-notation (use this OR `type_header`) |
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

## Production Deployment

### Infrastructure Requirements

```
┌──────────────┐     ┌─────────────────┐     ┌───────────────┐
│   Internet   │────▶│  Load Balancer  │────▶│   oschema     │
│  (webhooks)  │     │  (nginx/Caddy)  │     │  instance(s)  │
└──────────────┘     │  TLS termination│     └───────┬───────┘
                     │  rate limiting   │             │
                     └─────────────────┘             ▼
                                              ┌───────────────┐
                                              │  Redis 7+     │
                                              │  (Streams)    │
                                              └───────────────┘
```

**Minimum requirements:**
- 1 oschema instance (single binary, ~12MB)
- Redis 7+ with persistence enabled (AOF recommended)
- A reverse proxy for TLS termination (nginx, Caddy, or cloud LB)

**Recommended for production:**
- 2+ oschema instances behind a load balancer
- Redis with AOF persistence + RDB snapshots
- Separate Redis instance or cluster for high-throughput workloads
- Monitoring stack (Prometheus + Grafana or similar)

### Docker

```dockerfile
# Dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /oschema ./cmd/server/

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /oschema /usr/local/bin/oschema
EXPOSE 8080
ENTRYPOINT ["oschema"]
CMD ["serve"]
```

```bash
# Build and run
docker build -t oschema .
docker run -p 8080:8080 \
  -e OSCHEMA_REDIS_URL=redis://redis:6379 \
  oschema
```

### Docker Compose

```yaml
# docker-compose.yml
version: "3.9"
services:
  oschema:
    build: .
    ports:
      - "8080:8080"
    environment:
      OSCHEMA_REDIS_URL: redis://redis:6379
      OSCHEMA_WORKERS: "4"
      OSCHEMA_MAX_RETRIES: "5"
      OSCHEMA_DEDUPE_TTL: "24h"
    depends_on:
      redis:
        condition: service_healthy
    restart: unless-stopped
    deploy:
      resources:
        limits:
          memory: 256M
          cpus: "0.5"

  redis:
    image: redis:7-alpine
    command: redis-server --appendonly yes --maxmemory 512mb --maxmemory-policy noeviction
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 3s
      retries: 3
    restart: unless-stopped

volumes:
  redis-data:
```

```bash
docker compose up -d
```

### Kubernetes

```yaml
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: oschema
  labels:
    app: oschema
spec:
  replicas: 2
  selector:
    matchLabels:
      app: oschema
  template:
    metadata:
      labels:
        app: oschema
    spec:
      containers:
        - name: oschema
          image: ghcr.io/o-schema/oschema:latest
          ports:
            - containerPort: 8080
          env:
            - name: OSCHEMA_REDIS_URL
              valueFrom:
                secretKeyRef:
                  name: oschema-secrets
                  key: redis-url
            - name: OSCHEMA_WORKERS
              value: "4"
            - name: OSCHEMA_MAX_RETRIES
              value: "5"
            - name: OSCHEMA_DEDUPE_TTL
              value: "24h"
          resources:
            requests:
              cpu: 100m
              memory: 64Mi
            limits:
              cpu: 500m
              memory: 256Mi
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 3
            periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: oschema
spec:
  selector:
    app: oschema
  ports:
    - port: 80
      targetPort: 8080
  type: ClusterIP
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: oschema
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
    - hosts:
        - webhooks.yourdomain.com
      secretName: oschema-tls
  rules:
    - host: webhooks.yourdomain.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: oschema
                port:
                  number: 80
```

### Reverse Proxy (nginx)

oschema does not handle TLS directly. Use a reverse proxy for TLS termination, rate limiting, and request filtering.

```nginx
# /etc/nginx/conf.d/oschema.conf
upstream oschema {
    server 127.0.0.1:8080;
    server 127.0.0.1:8081;  # second instance
    keepalive 32;
}

server {
    listen 443 ssl http2;
    server_name webhooks.yourdomain.com;

    ssl_certificate     /etc/letsencrypt/live/webhooks.yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/webhooks.yourdomain.com/privkey.pem;

    # Rate limiting: 100 requests/sec per IP
    limit_req_zone $binary_remote_addr zone=webhooks:10m rate=100r/s;

    # Max request body size (webhook payloads)
    client_max_body_size 1m;

    location /ingest/ {
        limit_req zone=webhooks burst=200 nodelay;

        proxy_pass http://oschema;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Timeouts
        proxy_connect_timeout 5s;
        proxy_read_timeout 30s;
        proxy_send_timeout 30s;
    }

    location /health {
        proxy_pass http://oschema;
        access_log off;
    }

    # Block everything else
    location / {
        return 404;
    }
}
```

### Horizontal Scaling

oschema scales horizontally out of the box. Multiple instances share the same Redis Streams consumer group (`oschema-workers`), so events are automatically distributed across workers.

```
                  ┌──────────────┐
                  │  Instance 1  │──┐
  Load Balancer──▶│  4 workers   │  │
                  └──────────────┘  │
                  ┌──────────────┐  │    ┌─────────┐
                  │  Instance 2  │──┼───▶│  Redis   │
                  │  4 workers   │  │    │ Streams  │
                  └──────────────┘  │    └─────────┘
                  ┌──────────────┐  │
                  │  Instance 3  │──┘
                  │  4 workers   │
                  └──────────────┘
```

**How it works:**
- Each instance has its own unique consumer name (`worker-{PID}-{id}`)
- All instances join the same consumer group (`oschema-workers`)
- Redis Streams guarantees each message is delivered to exactly one consumer
- The HTTP endpoint is stateless — any instance can handle any request
- Deduplication is centralized in Redis, so it works across all instances

**Scaling guidelines:**
- Start with 1 instance, 4 workers
- Add instances when queue depth grows (monitor `XLEN oschema:queue`)
- Each instance handles ~1,000-5,000 webhooks/sec depending on payload size
- Workers are the bottleneck, not the HTTP handler — increase `--workers` first

### Redis Configuration

Production Redis should be configured for durability and bounded memory:

```conf
# redis.conf

# Persistence: AOF with fsync every second (good balance of durability/performance)
appendonly yes
appendfsync everysec

# RDB snapshots as backup
save 900 1
save 300 10
save 60 10000

# Memory limit — prevent OOM
maxmemory 2gb
maxmemory-policy noeviction  # IMPORTANT: never evict stream data

# Connection limits
maxclients 10000
timeout 300

# Slow log for debugging
slowlog-log-slower-than 10000
slowlog-max-len 128
```

**Key sizing estimates:**
- Each event in the queue: ~1-5 KB (depends on payload size)
- Each dedupe key: ~100 bytes (expires after TTL)
- Event index hash: ~1-5 KB per event (grows unbounded — plan retention)
- At 10,000 events/day: ~50-250 MB/day in stream + index storage

**Redis memory monitoring:**
```bash
# Check memory usage
redis-cli INFO memory

# Check stream lengths
redis-cli XLEN oschema:queue
redis-cli XLEN oschema:events:shopify
redis-cli XLEN oschema:deadletter

# Check pending messages (unprocessed)
redis-cli XPENDING oschema:queue oschema-workers

# Check dedupe key count
redis-cli DBSIZE
```

### Environment Variables

All configuration can be set via environment variables (recommended for containers):

| Variable | Default | Description |
|----------|---------|-------------|
| `OSCHEMA_PORT` | `8080` | HTTP server port |
| `OSCHEMA_REDIS_URL` | `redis://localhost:6379` | Redis connection string |
| `OSCHEMA_SPECS_DIR` | *(embedded)* | Path to additional YAML specs |
| `OSCHEMA_WORKERS` | `4` | Number of queue worker goroutines |
| `OSCHEMA_MAX_RETRIES` | `5` | Max retry attempts before dead-lettering |
| `OSCHEMA_DEDUPE_TTL` | `24h` | How long to remember processed event IDs |

**Redis URL formats:**
```bash
# Local
OSCHEMA_REDIS_URL=redis://localhost:6379

# With password
OSCHEMA_REDIS_URL=redis://:your-password@redis-host:6379

# With database number
OSCHEMA_REDIS_URL=redis://:password@host:6379/2

# TLS (Elasticache, Upstash, etc.)
OSCHEMA_REDIS_URL=rediss://:password@host:6380
```

### Monitoring & Observability

#### Key Metrics to Track

| Metric | How to Check | Alert Threshold |
|--------|-------------|-----------------|
| Queue depth | `XLEN oschema:queue` | > 10,000 (workers falling behind) |
| Dead letter count | `XLEN oschema:deadletter` | > 0 (events failing permanently) |
| Pending messages | `XPENDING oschema:queue oschema-workers` | > 1,000 (consumers stalled) |
| Dedupe keys | `redis-cli DBSIZE` | Unexpected growth |
| Redis memory | `INFO memory` | > 80% of maxmemory |
| HTTP 5xx rate | nginx/LB metrics | > 1% of requests |
| Response latency | nginx/LB metrics | p99 > 500ms |

#### Health Check Integration

Use the `/health` endpoint with your monitoring stack:

```bash
# Simple check
curl -f http://localhost:8080/health || alert "oschema is down"

# Kubernetes liveness/readiness (see k8s manifest above)

# Docker healthcheck
HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
  CMD wget -qO- http://localhost:8080/health || exit 1
```

#### Log Monitoring

oschema logs to stdout. In production, aggregate logs with your preferred stack:

```bash
# Docker: logs go to stdout automatically
docker logs -f oschema

# Kubernetes: use kubectl or a log aggregator (Loki, ELK, Datadog)
kubectl logs -f deployment/oschema

# Key log patterns to alert on:
# "dedupe error"       — Redis connection issues
# "enqueue error"      — Redis stream write failures
# "process error"      — Worker processing failures
# "dead letter"        — Events exhausted all retries
```

### Webhook Provider Setup

When configuring your webhook providers to point at oschema, use these URL patterns:

| Provider | Webhook URL | Notes |
|----------|------------|-------|
| Shopify | `https://webhooks.yourdomain.com/ingest/shopify` | Set in Shopify Admin → Notifications |
| Stripe | `https://webhooks.yourdomain.com/ingest/stripe` | Set in Stripe Dashboard → Webhooks |
| GitHub | `https://webhooks.yourdomain.com/ingest/github` | Set in repo Settings → Webhooks |
| Slack | `https://webhooks.yourdomain.com/ingest/slack` | Set in Slack App → Event Subscriptions |
| Jira | `https://webhooks.yourdomain.com/ingest/jira` | Set in Jira Settings → System → Webhooks |
| Linear | `https://webhooks.yourdomain.com/ingest/linear` | Set in Linear Settings → API → Webhooks |
| PagerDuty | `https://webhooks.yourdomain.com/ingest/pagerduty` | Set in PagerDuty → Integrations → Generic Webhooks v3 |
| SendGrid | `https://webhooks.yourdomain.com/ingest/sendgrid` | Set in SendGrid → Settings → Mail Settings → Event Webhook |
| Discord | `https://webhooks.yourdomain.com/ingest/discord` | Set in Discord Developer Portal → Interactions Endpoint URL |
| Twilio | `https://webhooks.yourdomain.com/ingest/twilio` | Set in Twilio Console → Phone Numbers → Webhook URL |

### Disaster Recovery

**Redis data loss:** If Redis data is lost, dedupe keys reset (some duplicates may be processed) and unprocessed queue messages are lost. Events already stored in streams are lost unless Redis persistence was enabled.

**Mitigation:**
1. Enable Redis AOF persistence (`appendonly yes`)
2. Take periodic RDB snapshots
3. For critical workloads, use Redis replication with automatic failover (Redis Sentinel or Redis Cluster)
4. Monitor `XLEN oschema:deadletter` — dead-lettered events can be replayed once the issue is fixed

**Replaying dead-lettered events:**
```bash
# View dead letter contents
redis-cli XRANGE oschema:deadletter - + COUNT 10

# Move events back to the processing queue (manual recovery)
# Read each event from deadletter, re-enqueue via the API
```

### Security Checklist

Before exposing oschema to the internet:

- [ ] TLS termination configured (nginx/Caddy/cloud LB)
- [ ] Rate limiting enabled at the reverse proxy level
- [ ] Request body size limited (`client_max_body_size 1m` in nginx)
- [ ] Redis not exposed to the internet (bind to private network)
- [ ] Redis password set (`requirepass` in redis.conf)
- [ ] Only `/ingest/*` and `/health` routes exposed (block everything else)
- [ ] Firewall rules restrict access to expected webhook source IPs where possible
- [ ] Monitor dead letter stream for anomalies
- [ ] Log aggregation and alerting configured

## License

MIT
