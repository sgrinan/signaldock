BINARY := bin/signaldock
PACKAGE := ./cmd/signaldock

IMAGE := signaldock

VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD)

LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT)

TEST_COMPOSE := docker compose -f compose.test.yaml
TEST_DATABASE_URL := postgres://signaldock:test@localhost:5433/signaldock_test?sslmode=disable


.PHONY: build run test check ci fmt fmt-check vet clean docker-build up down


build:
	mkdir -p bin
	go build -trimpath -ldflags "$(LDFLAGS)" -o "$(BINARY)" "$(PACKAGE)"


run:
	go run "$(PACKAGE)"


fmt:
	gofmt -w .


fmt-check:
	test -z "$$(gofmt -l .)"


vet:
	go vet ./...


test:
	@set -e; \
	trap '$(TEST_COMPOSE) down >/dev/null 2>&1' EXIT; \
	$(TEST_COMPOSE) up -d --wait; \
	SIGNALDOCK_TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -race -p 1 ./...


check: fmt vet test


ci: fmt-check vet test build


clean:
	rm -rf bin


docker-build:
	docker build \
		--build-arg VERSION="$(VERSION)" \
		--build-arg COMMIT="$(COMMIT)" \
		-t "$(IMAGE):$(VERSION)" .


up:
	docker compose up -d --build


down:
	docker compose down