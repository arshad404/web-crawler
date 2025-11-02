# Cost Optimization Strategies for URL Crawling System

This document lists practical optimizations to reduce compute, storage, and network cost when operating a crawler that processes billions of URLs per month.

---

## 1. Compress All Payloads (Network and Storage)

| Area | Technique | Expected Savings |
|-------|-----------|------------------|
| HTTP responses | Send `Accept-Encoding: gzip, br` | 60–85% smaller responses |
| Raw HTML storage | Store gzip-compressed payload instead of plain text | 4–8x storage reduction |
| API responses | Enable gzip for client-facing JSON | 3–5x smaller payloads |
| Internal file exports | Use `.ndjson.gz` instead of `.json` | 5–10x smaller exports |

---

## 2. Use Protobuf (or Avro) Instead of JSON for Internal Messages

| Target | Before | After | Savings |
|---------|--------|-------|---------|
| Kafka queue messages | JSON (text) | Protobuf (binary) | 3–5x smaller messages |
| CPU time | Full JSON decode | Binary decode | ~4x faster |
| Broker storage | Logs stored uncompressed | Binary + compression | Reduces retention footprint |

---

## 3. ClickHouse Compression and Column Layout

| Setting | Type | Benefit |
|----------|------|---------|
| `CODEC(ZSTD)` | Strong column compression | Up to 8x smaller storage |
| `LowCardinality(String)` | Dictionary-encoded strings | Lower memory and disk usage |
| `Sparse columns` | For JSON/array fields | Avoids writing empty values |
| `TTL + PARTITION PRUNING` | Drop old data automatically | Reduces compute and SSD space |

---

## 4. Zone-Aware or Region-Aware Routing

| Strategy | Benefit |
|----------|---------|
| Run fetchers close to target domains | Lower latency and fewer timeouts |
| Avoid cross-region egress in cloud | Reduces bandwidth billing |
| Multi-region deployment for global crawl | Reduces packet loss and throttling |

---

## 5. Conditional GET (ETag / If-Modified-Since)

| Case | Action | Savings |
|------|--------|---------|
| Page unchanged | Responds with status 304 and no body | Nearly 100% bandwidth saved |
| Recrawl cycles | Skip HTML download and parsing | Saves compute and storage |

---

## 6. Maximum Size Guardrails

Limit max page size to something like X MB:

if content_length > X-MB -> stop
if streamed gzip > X-MB -> stop

This prevents oversized payloads (videos, PDFs, media dumps) from consuming bandwidth, memory, and disk.

---

## 7. Tiered Storage Lifecycle

| Time since fetch | Storage Class | Cost Level |
|------------------|---------------|------------|
| 0–30 days | S3 Standard / MinIO Hot | High |
| 30–180 days | S3 Infrequent Access / MinIO EC | Medium |
| 180+ days | Glacier / Deep Archive | Low |

Only metadata stays in analytical DB indefinitely. Raw HTML is archived based on access frequency.

---

## 8. Spot or Preemptible Compute for Worker Nodes

Crawl fetchers and parser workers are stateless and retry safe, so they can run on spot instances or Kubernetes spot pools. This reduces compute cost by 70–85 percent compared to on-demand VMs.

---

## 9. Adaptive Crawl Scheduling

| Domain Type | Example Frequency |
|-------------|-------------------|
| News, e-commerce | Daily or hourly |
| Medium-traffic sites | Weekly |
| Static pages / archived docs | Monthly or quarterly |

Reduces waste when content does not change frequently.

---

## 10. Selective Raw HTML Retention

| Page Category | Retention Policy |
|---------------|------------------|
| High importance domains | Keep full HTML forever |
| Medium value | Keep full HTML for 12 months |
| Low value URLs | Store metadata only, no raw HTML |

This avoids storing terabytes of HTML for non-valuable pages.

---

## 11. Batch Inserts for Databases

- Write 10,000–50,000 rows per batch into ClickHouse or Postgres.
- Avoid row-by-row inserts.
- Use `INSERT INTO table FORMAT Parquet` for bulk ingestion.

This minimizes write amplification and improves compression.

---

## 12. Kafka Topic Compression

Producer settings example:

compression.type = zstd

yaml
Copy code

This reduces topic size, speeds up replication, and lowers SSD usage on brokers.

---

## 13. Log and Metrics Downsampling

| Type | Strategy |
|------|----------|
| Logs | Log full data only for failures, sample 1–5% of success |
| Tracing | Sample 1% except for error cases |
| Metrics | Roll up raw 10s metrics into 1m, then 5m for historical storage |

Prevents observability costs from scaling linearly with traffic.

---

## 14. DNS and Robots.txt Caching

| Cache | TTL | Result |
|-------|-----|--------|
| DNS lookup | 1–24h | Lower resolver traffic |
| robots.txt | 6–24h | Lower repeated requests to same host |

Reduces latency and unnecessary lookups.

---

## 15. Link Budgeting to Avoid Crawl Explosion

Limit number of extracted child links per page (for example: 30 max).  
Apply allow/deny patterns for pagination, session IDs, ads, infinite calendars, etc.  
Prevents unbounded queue growth, which directly lowers Kafka cost, compute cost, and storage cost.

---

## Summary Table

| Category | Technique | Savings Range |
|----------|-----------|---------------|
| Storage | Compression, TTL, tiering | 5–10x reduction |
| Compute | Spot workers, adaptive crawl | 50–85% cheaper |
| Network | Payload compression, conditional GET | 3–10x reduction |
| Queue and DB | Protobuf, batch ingest, columnar DB | 3–5x reduction |
| Observability | Sampling and retention policies | 70–90% cheaper |

---