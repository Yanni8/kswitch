BINARY  := kswitch
MODULE  := github.com/Yanni8/kswitch
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -X $(MODULE)/internal/version.version=$(VERSION) \
           -X $(MODULE)/internal/version.commit=$(COMMIT) \
           -X $(MODULE)/internal/version.date=$(DATE)

# GOOS/GOARCH pairs the release archives are built for.
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

.PHONY: all build run test fmt vet tidy install dist clean

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

## dist: cross-compile release archives for all platforms into ./dist
dist:
	rm -rf dist && mkdir -p dist
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		name=$(BINARY)_$${os}_$${arch}; \
		bin=$(BINARY); [ "$$os" = windows ] && bin=$(BINARY).exe; \
		echo "building $$name"; \
		mkdir -p dist/$$name; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' \
			-o dist/$$name/$$bin ./cmd/$(BINARY) || exit 1; \
		cp README.md dist/$$name/; \
		if [ "$$os" = windows ]; then \
			(cd dist/$$name && zip -q ../$$name.zip $$bin README.md); \
		else \
			tar -czf dist/$$name.tar.gz -C dist/$$name $$bin README.md; \
		fi; \
		rm -r dist/$$name; \
	done
	cd dist && sha256sum *.tar.gz *.zip > checksums.txt
	@ls -1 dist

## clean: remove build artifacts
clean:
	rm -rf bin dist
