# 1) Goal & Scope

* **Goal:** Operate a planet-scale crawler/extractor using the Part-1 Go service as the per-URL worker.
* **Scope:** Ingest monthly URL lists (files/MySQL), deduplicate, schedule politely, fetch, parse, classify, store **raw HTML + metadata**, expose APIs, and provide strong **observability, SLOs, cost controls**.

---

# 2) Architecture (components from your diagram)

* **Client → API GW → Auth service:** Public entry. Auth (JWT/API key), quotas, per-tenant rate limits, request shaping.
  (Add an L4/L7 **Load Balancer** in front of API GW for HA.)
* **URL management service:** Accepts jobs (`POST /api/v1/jobs` with URLs or file handles), tracks job state, exposes `/status`.
* **Scheduler:** Enforces **robots.txt / crawl-delay** and **per-host backoff**; schedules next attempts (recrawl windows) and spreads load.
* **URL frontier:** Canonicalize + deduplicate; assigns **priority**; enqueues to **Kafka** (or Redpanda) by **domain hash**.
* **Kafka topics:** Primary frontier, plus multiple **priority topics** (see §7).
* **URL processor job (workers):**

  * Resolve DNS (cache), **download HTML**, parse, classify, extract topics, discover **child links** (if enabled), and persist both **raw** and **metadata**.
  * Emits child URLs back to appropriate Kafka topic(s) if you decide to spider.
* **Stores:**

  * **Content storage:** Raw HTML (gz) in object store (e.g., MinIO/S3).
  * **DB (warehouse):** Columnar analytics store (e.g., ClickHouse) with unified schema.
  * **DB (serving):** PostgreSQL/CockroachDB for hot/low-latency reads.
* **Observability:** Distributed tracing, logs, metrics dashboards and alerts (Prometheus + Grafana + Loki/ELK).

---

# 3) End-to-end flows

## A) Job submission (batch from file/MySQL)

1. **Client** uploads CSV/NDJSON or references a **MySQL partition** (e.g., `crawl_urls_2025_07`).
2. **API GW** authenticates → **URL management** creates a `job_id`, persists job meta.
3. **URL frontier** reads in chunks (e.g., 1–5M lines), **canonicalizes** URLs, **dedups** using a Bloom filter + Redis/ClickHouse monthly “seen” table, and assigns **initial priority**.
4. Frontier publishes messages to **Kafka** with partition key = **domain hash** for politeness and locality.
5. **Scheduler** may push host-level delays back to frontier for later re-enqueuing.

## B) Per-URL processing

1. **URL processor** consumes from Kafka.
2. **DNS resolver** (local cache, e.g., `nscd`/in-proc LRU) resolves host.
3. **Downloader** applies timeouts (connect 5s, read 20s), headers, **gzip**, size cap (e.g., 5 MB), **robots.txt & allowlist** checks, **per-domain concurrency** (token bucket).
4. On success:

   * Store **raw HTML (gz)** to object storage; generate `storage_uri`.
   * **Parse** (title, meta desc, OG tags, headings, main text, lang, word_count, outgoing links count).
   * **Classify** (product/news/blog/other) + **top topics**.
   * **Write metadata** to ClickHouse; **upsert latest** to Serving DB.
   * Optionally enqueue **child URLs** (respecting robots/nofollow) to proper priority topic(s).
5. On failure: apply **retry** policy (exp backoff); after N attempts → **DLQ** with reason, keep minimal trace.

## C) Public querying & status

* **GET /status?job_id=…** from URL management (reads from DB).
* **POST /crawl** (strict rate-limit) → direct fetch (good for demo).
* **POST /crawl/batch** → enqueue + return `job_id`, results land in Serving DB.
* All public endpoints behind **LB → API GW** with authentication and quotas.

---

# 4) Priority & politeness (from your box “How priority can be handled”)

* Maintain **3 priority Kafka topics** (e.g., P0, P1, P2).

  * **P0:** paid/critical tenants, regulatory domains, recrawl SLA breaches.
  * **P1:** default monthly input.
  * **P2:** exploratory/child links, low-rank domains.
* **Priority Selector service** (or logic in frontier) routes **seed URLs** to proper topic based on tenant/host policy, historical error rates, and freshness targets.
* Worker pools subscribe with **weighted consumption** (e.g., 60% P0, 30% P1, 10% P2).
* **Politeness**: per-host token bucket (e.g., 1–2 concurrent; X req/sec), **crawl-delay** compliance, and **circuit breaker** on 429/5xx.

---

# 5) Unified data model

### 5.1 ClickHouse (warehouse) – partitioned, columnar, analytics at scale

```sql
CREATE TABLE page_metadata (
  url_id           String,                  -- hash(normalized_url)
  normalized_url   String,
  final_url        String,
  host             String,
  domain_bucket    UInt16,                  -- murmur3(host) % 1024
  fetch_ts         DateTime64(3),
  http_status      UInt16,
  content_type     String,
  content_len      UInt32,
  content_sha256   FixedString(32),
  lang             LowCardinality(String),
  title            String,
  description      String,
  og               JSON,
  h1               String,
  h2               Array(String),
  headings         Array(String),
  word_count       UInt32,
  links_out        UInt32,
  class_label      LowCardinality(String),  -- product/news/blog/other
  topics           Array(String),
  storage_uri      String,                  -- s3://minio/raw/...
  yyyymm           UInt32,                  -- partition month
  ingest_source    LowCardinality(String),  -- file|mysql
  attempt          UInt8,
  error            String                   -- reason if failed
) ENGINE = MergeTree
PARTITION BY toYYYYMM(fetch_ts)
ORDER BY (domain_bucket, fetch_ts, url_id);
```

### 5.2 Serving DB (PostgreSQL/CockroachDB) – fast API reads

```sql
CREATE TABLE page_latest (
  url_id         TEXT PRIMARY KEY,
  normalized_url TEXT UNIQUE,
  final_url      TEXT,
  fetch_ts       TIMESTAMPTZ,
  http_status    INT,
  title          TEXT,
  description    TEXT,
  lang           TEXT,
  class_label    TEXT,
  topics         JSONB,         -- ["toaster","kitchen","cuisinart"]
  word_count     INT,
  storage_uri    TEXT
);
CREATE INDEX ON page_latest (fetch_ts DESC);
CREATE INDEX ON page_latest (class_label);
```

**Idempotency key:** `(normalized_url, yyyymm)`; upsert on conflict.
**Canonicalization:** scheme/host case-fold, default ports removed, query params sorted, tracker params dropped, punycode normalized.

---

# 6) SLOs / SLAs

### Pipeline SLOs

* **Freshness:** 99% of monthly URLs processed within **24 h** from ingestion start.
* **Throughput:** sustain **≥50k URLs/min** end-to-end (steady state).
* **Success rate:** ≥97% 2xx/304 (robots/blocked excluded).
* **Extractor correctness:** ≥99% rows valid (non-empty core fields or valid lang).
* **Duplicates:** ≤0.1% duplicated metadata rows per month.

### Public API SLA (behind LB + API GW)

* **Availability:** 99.9% monthly.
* **Latency:** p95 ≤ **300 ms**, p99 ≤ **800 ms** for read paths.
* **Ingress limits:** per-tenant QPS + daily quotas; default per-IP throttles; request size caps.

---

# 7) Observability (metrics, logs, tracing)

**Metrics (Prometheus/Grafana)**

* **API/LB/GW:** req/s, p95/99, 4xx/5xx, **429 rate**, upstream errors, active backends.
* **Kafka:** producer/consumer rate, **consumer lag**, rebalances.
* **Frontier:** dedup hit-rate, enqueue failures, priority distribution.
* **Fetchers:** success%, 429/403/5xx distribution, TLS/timeout errors, bytes/sec, DNS latency.
* **Parser/Classifier:** p95 time, parse failures, empty-content%.
* **Warehouse:** insert throughput, merges, disk/part count, query latency.
* **Serving DB:** QPS, read p95, locks, replication lag (Cockroach).
* **DLQ:** backlog size, top failure reasons.
* **Cost telemetry:** $/day, $/1M URLs (infra labels, S3 size, egress).

**Logs & tracing**

* **JSON logs** with correlation IDs (`trace_id`, `job_id`, `url_id`).
* **Distributed tracing** (OpenTelemetry) for sample paths (API → worker → stores).
* **Redaction**: no full HTML in logs; log only hashes/URIs + lightweight snippets for QA.

**Alerts**

* Gateway 5xx > 1% (5m), Kafka lag > 15m, Fetch success < 90% (10m),
  ClickHouse insert error > 0.1%, Serving p95 > 500 ms (10m), DLQ growth > 0.5%/day, Cost > budget.

---

# 8) Capacity & cost notes

* **Fetcher throughput math:** If a single core ≈ 20 fetch/s, to reach **50k URLs/min (~833/s)** you need ≈ **42 active cores** for fetch, plus headroom for parse/classify and politeness limits. In practice, run **hundreds of small workers** across shards/regions.
* **Storage:** Avg gz HTML ~**120 KB** → **120 TB per 1B pages** (raw). Metadata row ~**1–2 KB** → **1–2 TB per 1B**. Apply lifecycle: hot → infrequent access → archive.
* **Cost levers:** spot/preemptible compute for stateless workers; **ETag/If-Modified-Since** on recrawls; HTML-only; size caps; ClickHouse ZSTD; partition TTL; MinIO/S3 lifecycle.

---

# 9) Failure handling

* **Retries with backoff** for 408/429/5xx/TLS; cap attempts; **jitter** to avoid thundering herds.
* **DLQ** on permanent 4xx (non-robots) and repeated transient failures.
* **Poison pill** detector (same domain repeatedly failing) triggers **circuit breaker**.
* **Idempotent writes** (upserts) ensure reprocessing is safe.

---

# 10) APIs (public surface)

* `POST /api/v1/jobs` → create job from URLs list or file handle (returns `job_id`).
* `GET /api/v1/jobs/{job_id}` → progress: queued/processing/completed/failed + counts.
* `POST /api/v1/crawl` → **single URL** (strictly rate-limited).
* `POST /api/v1/crawl/batch` → enqueue batch; returns `job_id`.
* `POST /api/v1/crawl/upload` → CSV/NDJSON; streams accepted items; enqueues.
* **Auth:** API key/JWT at **API GW**; per-tenant quota and QPS limits.

---

# 11) Next steps / PoC rollout

**Week 1–2**

* Stand up **Kafka**, **MinIO**, **ClickHouse**, **Postgres/Cockroach**, **Prometheus/Grafana/Loki**.
* Implement **frontier** (canonicalize/dedup), **priority routing**, and **workers** wired to Part-1 Go crawler.
* Ingest **10–50M URLs** (one month), validate schema and dashboards.

**Week 3–4**

* Add **API GW + LB**, auth, quotas.
* Implement **scheduler** with robots/crawl-delay, per-host token buckets, circuit breakers.
* Add **DLQ dashboards**, daily run report, and cost panel.

**Week 5+**

* **Autoscale** by Kafka lag & CPU/net; multi-region worker pools.
* Lifecycle policies; archive raw; backfill prior months.
* Upgrade topics extraction (TF-IDF/KeyBERT), ML classifier; add search (OpenSearch) if needed.

---

## TL;DR

Your diagram maps cleanly to a **queue-backed, priority-aware** pipeline with **API Gateway + Load Balancer** for high traffic. We:

* Canonicalize & deduplicate at the **URL frontier**,
* Enforce **politeness** and **priorities** with Kafka + scheduler,
* Store **raw HTML** (MinIO) and **metadata** (ClickHouse + Postgres/Cockroach),
* Expose a controlled **public API** via **LB → API GW**,
* Define **SLO/SLA**, **metrics/alerts**, and a concrete **roadmap** to billions-scale.

If you want, I can turn this into a polished PDF/README plus a **Mermaid diagram** that includes **LB + API GW + priority topics** (validated to render).
