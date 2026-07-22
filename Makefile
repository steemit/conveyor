# Conveyor Go Makefile

BINARY := bin/conveyor
VERSION := $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
LDFLAGS := -X github.com/steemit/conveyor/internal/server.Version=$(VERSION)

.PHONY: build test vet tidy devserver docker-build docker-run clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/conveyor

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

devserver:
	go run ./cmd/conveyor

docker-build:
	docker build --build-arg VERSION=$(VERSION) -t steemit/conveyor:latest .

docker-run:
	docker run -it -p 8080:8080 steemit/conveyor:latest

clean:
	rm -rf bin/
