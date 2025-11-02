Understood. Here is the **clean, symbol-free version** of the Part-3 write-up.

---

# Part 3 — Proof of Concept (PoC) Engineering Plan

## 1. Objective of the PoC

Build a functional, measurable version of the system that demonstrates:

| Area          | What must be proven in PoC                                                      |
| ------------- | ------------------------------------------------------------------------------- |
| Reliability   | System retries failures, handles timeouts, supports DLQ, and does not drop data |
| Performance   | System can sustain at least 5K URLs/min in controlled run                       |
| Scalability   | Queue-based architecture supports horizontal workers with no shared bottleneck  |
| Cost          | Cost per 1M URLs can be estimated and optimized                                 |
| Data quality  | Metadata extraction, parsing, and classification produce accurate results       |
| Observability | Metrics dashboards, logs, and error tracking exist and are usable               |

Target PoC dataset size: 5–10 million URLs from mixed domains such as:
`amazon.com`, `walmart.com`, `bestbuy.com`, `rei.com`, `cnn.com`, news and blog URLs.

---

## 2. PoC Scope

| Component                              | Included in PoC | Deferred to Production Phase |
| -------------------------------------- | --------------- | ---------------------------- |
| URL ingestion (file + MySQL)           | Yes             | No                           |
| Kafka frontier queue                   | Yes             | No                           |
| Fetcher workers                        | Yes             | No                           |
| robots.txt + rate limit                | Partial         | Full enforcement later       |
| Object storage (MinIO or S3)           | Yes             | No                           |
| Metadata warehouse (ClickHouse)        | Yes             | No                           |
| Public API (`/crawl/batch`, `/status`) | Yes             | No                           |
| API Gateway + Load Balancer            | Optional stub   | Full version later           |
| Classification model                   | Rule-based only | ML model later               |
| Multi-region worker pool               | No              | Later phase                  |
| Multi-priority topics                  | No              | Later phase                  |
| Scheduled re-crawl system              | No              | Later phase                  |

---

## 3. Execution Plan

### Phase 0 — Infrastructure Setup (2–3 days)

* Bring up local stack: Kafka, ClickHouse, MinIO, Postgres (Docker Compose or K8s).
* Prepare sample URL datasets.
* Deploy Prometheus + Grafana, basic log aggregation.

### Phase 1 — Core Pipeline Build (Week 1)

| Task                                   | Estimated Time |
| -------------------------------------- | -------------- |
| Ingest URLs from CSV/NDJSON and MySQL  | 1 day          |
| Canonicalization + de-dup logic        | 1 day          |
| Publish to Kafka frontier              | 1 day          |
| Fetch worker with retry, gzip, timeout | 2 days         |
| Store raw HTML in MinIO                | 0.5 day        |
| Parse metadata, extract simple topics  | 1 day          |
| Write metadata to ClickHouse           | 0.5 day        |
| DLQ + retry policy                     | 1 day          |

### Phase 2 — API + Observability (Week 2)

| Task                                       | Estimated Time |
| ------------------------------------------ | -------------- |
| Implement `/crawl/batch` and `/status` API | 1.5 days       |
| Add metrics counters (Prometheus)          | 0.5 day        |
| Add structured JSON logging                | 0.5 day        |
| Build Grafana dashboard                    | 1 day          |
| Run PoC on 500K–1M URLs                    | 1 day          |
| Summarize results                          | 0.5 day        |

### Phase 3 — PoC Validation (Week 3)

* Run full 5–10M URL test.
* Measure throughput, error rate, storage footprint, cost per 1M URLs.
* Collect improvement backlog for production rollout.

---

## 4. Potential Blockers

| Blocker                                | Type         | Mitigation Plan                                       |
| -------------------------------------- | ------------ | ----------------------------------------------------- |
| Site rate-limiting or bot detection    | External     | Use robots.txt, UA header rotation, cap parallelism   |
| Kafka partition pressure               | Technical    | Use compression + protobuf + increase partition count |
| Storage growth too fast                | Cost         | Enforce HTML size limits and compression              |
| Memory blowup from deep link discovery | Logic        | Apply per-page link budget                            |
| Parsing accuracy issues                | Data quality | Validate against sample domains and adjust rules      |
| Poor observability signal-to-noise     | Ops          | Sampling logs/traces, metric filtering                |

---

## 5. PoC Success Criteria

| Category               | Evaluation Target                                          |
| ---------------------- | ---------------------------------------------------------- |
| Throughput             | At least 5000 URLs per minute sustained                    |
| Success rate           | 94 percent or higher (robots and blocked sites excluded)   |
| Raw HTML compression   | At least 5x reduction vs uncompressed                      |
| Metadata query latency | Under 300 ms at p95 from ClickHouse                        |
| Cost benchmark         | Under 6 USD per 1M URLs in PoC scale                       |
| Error handling         | DLQ captures all failed cases and supports retry           |
| API contract           | `/crawl/batch` and `/status` function correctly            |
| Dashboard              | Live metrics: throughput, failure rate, worker utilization |

---

## 6. Timeline Summary

| Phase                   | Duration |
| ----------------------- | -------- |
| Infra bootstrap         | 3 days   |
| Core pipeline           | 5 days   |
| API + observability     | 5 days   |
| PoC validation + report | 3 days   |
| Total estimate          | ~3 weeks |

---

## 7. How the PoC Will Be Measured

1. Run at least 1M URLs through the system.
2. Collect:

   * Success rate and error classification
   * Average fetch latency
   * Total HTML bytes before and after compression
   * ClickHouse row ingestion rate
   * Worker CPU and bandwidth usage
3. Produce PoC report including:

   ```
   URLs processed: 1,000,000
   Successful: 948,000
   Failed: 52,000 (breakdown: robots=19k, TLS=4k, 404=22k, timeout=7k)
   Avg fetch time: 410ms
   Raw HTML bytes: 92GB
   Compressed HTML stored: 14.6GB
   Approx cost: $5.7 per 1M URLs
   ```

---

## 8. Release Plan for Full System After PoC

Before production launch:

1. Add API Gateway, auth, and rate limits.
2. Add multi-priority Kafka topics.
3. Add multi-region fetch workers.
4. Deploy ClickHouse on production-grade storage.
5. Add runbook, dashboards, and SLO alerting.
6. Introduce cost guardrails and monthly reports.
7. Shadow traffic testing → partial rollout → full rollout.

---

## 9. Documentation Required Before Release

| Document                 | Purpose                                    |
| ------------------------ | ------------------------------------------ |
| `POC_RESULTS.md`         | Final report from PoC run                  |
| `SYSTEM_DESIGN.md`       | Updated architecture with PoC learnings    |
| `DATA_SCHEMA.md`         | Unified schema documentation               |
| `RUNBOOK.md`             | Incident response and troubleshooting      |
| `RELEASE_CHECKLIST.md`   | Steps to go from staging to production     |
| `OBSERVABILITY_GUIDE.md` | Dashboards, alerts, and metric definitions |
| `COST_MODEL.md`          | Cost per million URLs projections          |

---

## 10. Key Principle

The PoC is not required to prove billion-scale capacity.
The PoC must prove that the architecture can scale to billions.

