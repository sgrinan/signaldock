BINARY := bin/signaldock
PACKAGE := ./cmd/signaldock

VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD)

LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT)

.PHONY: build run test check clean

build:
	mkdir -p bin
	go build -trimpath -ldflags "$(LDFLAGS)" -o "$(BINARY)" "$(PACKAGE)"

run:
	go run "$(PACKAGE)"

test:
	go test ./...

check:
	gofmt -w .
	go vet ./...
	go test -race ./...

clean:
	rm -rf bin