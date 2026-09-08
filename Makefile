BINARY := bin/signaldock
PACKAGE := ./cmd/signaldock
IMAGE := signaldock

VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD)

LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT)

.PHONY: build run test check clean docker-build docker-run up down

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

docker-build:
	docker build --build-arg VERSION="$(VERSION)" --build-arg COMMIT="$(COMMIT)" -t "$(IMAGE):$(VERSION)" .

docker-run:
	docker run --rm -p 8080:8080 "$(IMAGE):$(VERSION)"

up:
	docker compose up -d --build

down:
	docker compose down