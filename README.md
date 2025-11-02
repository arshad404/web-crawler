
# brightedge-go-crawler

A minimal Go service that:
- Fetches a URL
- Extracts metadata (title, description, og: tags, headings, main text)
- Returns lightweight classification (product/news/blog/other)
- Extracts top topics (keywords) via a simple frequency-based approach
- Exposes HTTP endpoints for single URL and batch crawl
- Ready for Docker deployment

## Endpoints

- `GET /health` – health probe
- `POST /crawl` – crawl a single URL  
  **Body:** `{"url":"https://example.com"}`  
  **Response:** metadata + classification + topics
- `POST /crawl/batch` – crawl multiple URLs concurrently  
  **Body:** `{"urls":["https://example.com","https://cnn.com"]}`  
  **Response:** map of url -> result/error

## Quick start (local)

```bash
# requires Go 1.21+
make deps
make run
# in a new shell:
curl -s localhost:8080/health
curl -s -X POST localhost:8080/crawl -H 'Content-Type: application/json' -d '{"url":"https://www.cnn.com/2025/09/23/tech/google-study-90-percent-tech-jobs-ai"}' | jq .
curl -s -X POST localhost:8080/crawl -H 'Content-Type: application/json' -d '{"url":"https://example.com"}' | jq .
curl -s -X POST localhost:8080/crawl/batch -H 'Content-Type: application/json' -d '{"urls":["https://example.com","https://cnn.com"]}' | jq .
```

## Integration Tests (Optional, external network)
Run the live URL tests (may be flaky due to blocking/network):
```bash
go test ./... -tags=integration -v
```

or 

```bash
make test
```

## Build

```bash
make build   # builds ./bin/server
```

## CSV / NDJSON

- **CLI**: `go run ./cmd/cli --input examples/urls.csv --output examples/output.ndjson`
- **API (multipart)**: `POST /crawl/upload` with `file=@examples/urls.csv` returns NDJSON stream.

### Example files 
See `examples/` folder.
Run the example files from the root
```bash
sh examples/run_example.sh
```

Outout:

```text
2025/11/01 11:17:11 [INFO] server listening on :8080
Wrote examples/output.ndjson
2025/11/01 11:17:14 [INFO] POST /crawl/upload 774.451354ms
Wrote examples/upload_output.ndjson
```

## Docker

```bash
docker build -t brightedge-go-crawler:local .
docker run --rm -p 8080:8080 brightedge-go-crawler:local
```

## Notes & Assumptions

- This is a respectful, single-URL fetcher, not a full web spider: it does **not** follow links.
- Classification is deliberately simple and explainable (rule-based signals). In production,
  you'd enhance it with learned models and site-specific features.
- Topic extraction is frequency-based with a stopword list; switch to TF-IDF or RAKE for better results.
- Timeouts, retries, and size caps keep the service robust for demo purposes.

## Project Structure

```
.
├── cmd
│   └── server
│       └── main.go
├── internal
│   ├── classifier
│   │   └── classifier.go
│   ├── crawler
│   │   └── crawler.go
│   ├── models
│   │   └── types.go
│   └── parser
│       └── parser.go
├── pkg
│   └── logger
│       └── logger.go
├── go.mod
├── Makefile
├── Dockerfile
└── README.md
```

## Example Request

```bash
curl -s -X POST localhost:8080/crawl \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://www.cnn.com/"}' | jq .
```

---


## CSV / NDJSON

- **CLI**: `go run ./cmd/cli --input examples/urls.csv --output examples/output.ndjson`
- **API (multipart)**: `POST /crawl/upload` with `file=@examples/urls.csv` returns NDJSON stream.

### Example files 
See `examples/` folder.
Run the example files from the root
```bash
sh examples/run_example.sh
```

Outout:

```text
2025/11/01 11:17:11 [INFO] server listening on :8080
Wrote examples/output.ndjson
2025/11/01 11:17:14 [INFO] POST /crawl/upload 774.451354ms
Wrote examples/upload_output.ndjson
```

