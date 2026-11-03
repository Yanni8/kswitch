# Copyright 2026 Yannick Müller
# SPDX-License-Identifier: Apache-2.0

BINARY  := kubeswitch
MODULE  := github.com/Yanni8/kubeswitch
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -X $(MODULE)/internal/version.version=$(VERSION) \
           -X $(MODULE)/internal/version.commit=$(COMMIT) \
           -X $(MODULE)/internal/version.date=$(DATE)

# GOOS/GOARCH pairs the release archives are built for.
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

# Copyright header for source files, see "make headers".
COPYRIGHT := Copyright 2026 Yannick Müller
SPDX      := SPDX-License-Identifier: Apache-2.0

.PHONY: all build run test fmt vet tidy install licenses headers dist clean

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

## licenses: copy LICENSE/NOTICE and the licenses of every compiled-in dependency into internal/licenses
licenses:
	rm -rf internal/licenses/third_party
	cp LICENSE NOTICE internal/licenses/
	@{ for os in linux darwin windows; do \
		GOOS=$$os go list -e -deps -f '{{with .Module}}{{if not .Main}}{{.Path}}{{end}}{{end}}' ./cmd/$(BINARY); \
	done; } | sort -u | while read -r mod; do \
		[ -n "$$mod" ] || continue; \
		go mod download "$$mod" >/dev/null 2>&1; \
		dir=$$(go list -m -f '{{.Dir}}' "$$mod"); \
		found=0; \
		for f in "$$dir"/LICENSE* "$$dir"/COPYING* "$$dir"/NOTICE*; do \
			[ -f "$$f" ] || continue; \
			mkdir -p "internal/licenses/third_party/$$mod"; \
			cp "$$f" "internal/licenses/third_party/$$mod/"; \
			chmod 644 "internal/licenses/third_party/$$mod/$$(basename "$$f")"; \
			found=1; \
		done; \
		[ $$found = 1 ] || { echo "no license file found for $$mod in $$dir" >&2; exit 1; }; \
		echo "collected $$mod"; \
	done

## headers: add the copyright header to source files that lack it
headers:
	@{ find . -name '*.go' -not -path './.git/*'; printf '%s\n' ./install.sh ./Makefile .github/workflows/*.yml; } | \
	while read -r f; do \
		head -n 5 "$$f" | grep -q 'SPDX-License-Identifier' && continue; \
		case "$$f" in \
		*.go) printf '// $(COPYRIGHT)\n// $(SPDX)\n\n' | cat - "$$f" > "$$f.tmp" ;; \
		*.sh) { head -n 1 "$$f"; printf '# $(COPYRIGHT)\n# $(SPDX)\n'; tail -n +2 "$$f"; } > "$$f.tmp" ;; \
		*) printf '# $(COPYRIGHT)\n# $(SPDX)\n\n' | cat - "$$f" > "$$f.tmp" ;; \
		esac; \
		mv "$$f.tmp" "$$f"; [ "$${f##*.}" = sh ] && chmod +x "$$f"; echo "added header to $$f"; \
	done; true

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
		cp README.md LICENSE NOTICE dist/$$name/; \
		if [ "$$os" = windows ]; then \
			(cd dist/$$name && zip -q ../$$name.zip $$bin README.md LICENSE NOTICE); \
		else \
			tar -czf dist/$$name.tar.gz -C dist/$$name $$bin README.md LICENSE NOTICE; \
		fi; \
		rm -r dist/$$name; \
	done
	cd dist && sha256sum *.tar.gz *.zip > checksums.txt
	@ls -1 dist

## clean: remove build artifacts
clean:
	rm -rf bin dist
