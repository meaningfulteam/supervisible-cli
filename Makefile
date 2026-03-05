BINARY=supervisible

.PHONY: build test fmt tidy run

build:
	go build -o bin/$(BINARY) ./cmd/supervisible

test:
	go test ./...

fmt:
	gofmt -w ./cmd ./internal

tidy:
	go mod tidy

run:
	go run ./cmd/supervisible
