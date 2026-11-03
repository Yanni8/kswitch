BINARY  := kswitch
MODULE  := github.com/Yanni8/kswitch
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -X $(MODULE)/internal/version.version=$(VERSION) \
           -X $(MODULE)/internal/version.commit=$(COMMIT) \
           -X $(MODULE)/internal/version.date=$(DATE)

.PHONY: all build run test fmt vet tidy install clean

all: build

## build: compile the binary into ./bin
build:
	go build -ldflags '$(LDFLAGS)' -o bin/$(BINARY) ./cmd/$(BINARY)

## run: build and run the binary (ARGS="..." to pass arguments)
run: build
	./bin/$(BINARY) $(ARGS)

## test: run all tests
test:
	go test ./...

## fmt: format all Go files
fmt:
	gofmt -s -w .

## vet: run go vet
vet:
	go vet ./...

## tidy: sync go.mod / go.sum
tidy:
	go mod tidy

## install: install into GOPATH/bin with version info
install:
	go install -ldflags '$(LDFLAGS)' ./cmd/$(BINARY)

## clean: remove build artifacts
clean:
	rm -rf bin
