
BINARY=server

.PHONY: deps run build test fmt lint clean

deps:
	go mod tidy

run:
	go run ./cmd/server

build:
	mkdir -p bin
	go build -o bin/$(BINARY) ./cmd/server

test:
	go test ./... -v

fmt:
	go fmt ./...

clean:
	rm -rf bin


cli:
	go build -o bin/cli ./cmd/cli

example:
	bash ./examples/run_example.sh
