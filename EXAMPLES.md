# EXAMPLES

```
make run
curl -s -X POST localhost:8080/crawl -H 'Content-Type: application/json' -d '{"url":"https://example.com"}' | jq .

# CLI CSV -> NDJSON
go run ./cmd/cli --input ./examples/urls.csv --output ./examples/output.ndjson

# API upload (multipart)
curl -s -X POST "http://localhost:8080/crawl/upload" -F "file=@examples/urls.csv" -o examples/upload_output.ndjson
```
