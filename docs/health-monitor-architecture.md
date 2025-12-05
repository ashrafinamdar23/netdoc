# Health Monitor Service Architecture

This document outlines an implementable architecture for a health monitoring application that evaluates Kubernetes workloads via Prometheus/Mimir and produces HTML-based status reports.

## Goals
- Support multiple Kubernetes clusters and multiple Prometheus/Mimir endpoints per cluster.
- Store configured checks, their schedules, and evaluation history in a database.
- Enable configurable expressions, activation flags, and frequencies.
- Generate human-friendly HTML status pages and deliver them via email/S3/SFTP (extensible).

## High-Level Components

### 1) Configuration Loader
- Reads a YAML/JSON configuration that defines clusters, data sources, checks, schedules, and reporters.
- Validates and normalizes configuration before persisting to the database.

### 2) Catalog & Persistence Layer
- Database tables (or collections) to store:
  - `clusters`: cluster id, name, kube context, default scrape source.
  - `prom_sources`: id, cluster_id, base_url, auth, tls settings.
  - `checks`: id, cluster_id, prom_source_id, expression, display_name, severity, is_active, schedule_id, tags.
  - `schedules`: id, cron/frequency definition (e.g., `@hourly`, `0 3 * * *`).
  - `results`: id, check_id, ts, status (pass/warn/fail), value, threshold metadata, message.
  - `reports`: id, generated_at, status, delivery_channel, link/reference.
- Suggested database options: PostgreSQL or SQLite for quick bootstrap; schema can be translated to ORM (SQLAlchemy) models.

### 3) Scheduler
- Enqueues check evaluation jobs according to schedules defined in the DB.
- Possible implementations: APScheduler (Python), Celery beat (distributed), or Kubernetes CronJobs emitting queue events.

### 4) Evaluator
- For each scheduled check:
  - Fetch metrics using Prometheus HTTP API (or Mimir-compatible endpoint).
  - Evaluate the configured PromQL expression.
  - Apply optional thresholds/rules (e.g., `warn_if > 0.5`, `fail_if > 0.8`).
  - Persist the raw value and derived status into `results`.
- Supports check toggles via `is_active` flag and tagged grouping.

### 5) Reporter
- Periodically generates HTML reports by combining the latest `results` for each check.
- Supports grouping by cluster, severity, and tags.
- Renders trend charts (sparklines) when historical data is available.
- Delivery plugins:
  - `email`: SMTP configuration, recipients, subject templates.
  - `s3`: bucket/key prefix for uploading HTML.
  - `sftp`: host/path/credentials.
  - Local filesystem export for debugging.

### 6) Status Page (Static or App)
- Either generate static HTML or serve via a lightweight web UI (Flask/FastAPI/Express).
- Provides filters (cluster, severity, tag) and detail views for each check.

## Configuration Model (YAML Example)
```yaml
clusters:
  - id: "prod"
    display_name: "Production"
    default_prometheus: prod-main
  - id: "staging"
    display_name: "Staging"
    default_prometheus: stg-main

prometheus:
  - id: prod-main
    cluster: prod
    url: https://prom-prod.example.com
    auth:
      bearer_token: ${PROM_PROD_TOKEN}
    tls:
      verify: true
  - id: stg-main
    cluster: staging
    url: https://prom-stg.example.com
    auth:
      basic:
        username: prom
        password: ${PROM_STG_PASS}

schedules:
  hourly: "@hourly"
  daily: "0 3 * * *"

checks:
  - id: api-availability
    display_name: API availability
    cluster: prod
    prom_source: prod-main
    expression: 1 - (sum(rate(http_requests_total{status=~"5.."}[5m])) / sum(rate(http_requests_total[5m])))
    thresholds:
      warn_gt: 0.02
      fail_gt: 0.05
    is_active: true
    schedule: hourly
    tags: [service:api, slo]

  - id: cpu-burst
    display_name: CPU saturation
    cluster: staging
    prom_source: stg-main
    expression: max(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_rate{namespace="app"})
    thresholds:
      warn_gt: 0.7
      fail_gt: 0.85
    is_active: true
    schedule: daily
    tags: [capacity]
```

## Data Flow
1. **Bootstrap**: load config file, insert/merge records into `clusters`, `prom_sources`, `schedules`, `checks`.
2. **Scheduling**: scheduler enqueues evaluation jobs based on `schedules` entries.
3. **Evaluation**: evaluator runs PromQL, determines status, stores row in `results`.
4. **Reporting**: reporter queries latest `results` per check, renders HTML, stores `reports` metadata, and triggers delivery plugins.

## Suggested Tech Stack (reference implementation)
- **Language**: Python 3.11+
- **Web/UI**: FastAPI for status endpoints; Jinja2 for HTML templating.
- **Scheduling**: APScheduler (standalone) or Celery beat with Redis broker.
- **DB/ORM**: PostgreSQL + SQLAlchemy; fallback to SQLite for demos.
- **Prometheus Client**: `httpx` + Prometheus HTTP API.
- **Reporting**: Jinja2 templates + Plotly or Chart.js (optional) for trends.
- **Containerization**: Dockerfile + Kubernetes manifests with ConfigMap-based configuration and optional Secrets for credentials.

## Minimal Table Sketch (SQL)
```sql
CREATE TABLE clusters (
  id TEXT PRIMARY KEY,
  display_name TEXT NOT NULL,
  default_prometheus TEXT
);

CREATE TABLE prom_sources (
  id TEXT PRIMARY KEY,
  cluster_id TEXT NOT NULL REFERENCES clusters(id),
  url TEXT NOT NULL,
  auth JSONB,
  tls JSONB
);

CREATE TABLE schedules (
  id TEXT PRIMARY KEY,
  cron TEXT NOT NULL
);

CREATE TABLE checks (
  id TEXT PRIMARY KEY,
  cluster_id TEXT NOT NULL REFERENCES clusters(id),
  prom_source_id TEXT NOT NULL REFERENCES prom_sources(id),
  expression TEXT NOT NULL,
  display_name TEXT NOT NULL,
  severity TEXT DEFAULT 'medium',
  is_active BOOLEAN DEFAULT TRUE,
  schedule_id TEXT NOT NULL REFERENCES schedules(id),
  tags JSONB,
  thresholds JSONB
);

CREATE TABLE results (
  id UUID PRIMARY KEY,
  check_id TEXT NOT NULL REFERENCES checks(id),
  ts TIMESTAMPTZ NOT NULL DEFAULT now(),
  status TEXT NOT NULL,
  value DOUBLE PRECISION,
  message TEXT,
  thresholds JSONB
);

CREATE TABLE reports (
  id UUID PRIMARY KEY,
  generated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  status TEXT NOT NULL,
  channel TEXT NOT NULL,
  location TEXT
);
```

## Extensibility Notes
- Add plugin interfaces for collectors (non-Prometheus sources), evaluators (custom logic), and reporters (delivery transports).
- Use feature flags to gate new checks without deleting configuration.
- Support per-check retries/timeouts for slow Prometheus endpoints.

## Operational Considerations
- Secure secrets (tokens/passwords) via Kubernetes Secrets and environment variable expansion.
- Expose health endpoints for the monitor itself (liveness/readiness).
- Add tracing/metrics for scheduler delays, evaluation latency, and report generation durations.
- Implement role-based access for the status page when exposed outside the cluster.

## Roadmap (phased)
1. **MVP**: config loader, DB schema, PromQL evaluator, HTML report generator (static), email + local export.
2. **Scale-out**: move scheduling to distributed queue, add S3/SFTP exporters, add trend charts.
3. **Harden**: RBAC, multi-tenant isolation, audit logging, synthetic checks (HTTP pings), and SLA dashboards.
