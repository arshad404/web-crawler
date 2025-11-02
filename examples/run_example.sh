
#!/usr/bin/env bash
set -euo pipefail

# Start server in background
(go run ./cmd/server & echo $! > /tmp/brightedge_pid) || true
sleep 1

# CLI example reading CSV and writing NDJSON
go run ./cmd/cli --input ./examples/urls.csv --output ./examples/output.ndjson
echo "Wrote examples/output.ndjson"

# API multipart upload example
curl -s -X POST "http://localhost:8080/crawl/upload"   -F "file=@examples/urls.csv"   -o examples/upload_output.ndjson
echo "Wrote examples/upload_output.ndjson"

# Cleanup server
kill "$(cat /tmp/brightedge_pid)" || true
rm -f /tmp/brightedge_pid
