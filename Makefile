.PHONY: all build clean test coverage deps fmt fmt-check lint vet tidy tidy-check verify

BINARY_NAME=mcp-gopls
GO=go
GOFLAGS=-v
LDFLAGS=-ldflags="-s -w"
GO_FILES := $(shell find . -name '*.go' -not -path "./vendor/*" -not -path "./.git/*")

all: deps build

build:
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_NAME) ./cmd/...

clean:
	$(GO) clean
	rm -f $(BINARY_NAME)

test:
	$(GO) test -v ./...

coverage:
	$(GO) test ./... -coverprofile=coverage.out
	$(GO) tool cover -func=coverage.out > coverage.txt
	@echo "Coverage profiles written to coverage.out and coverage.txt"

deps:
	$(GO) mod download
	$(GO) mod tidy

fmt:
	gofmt -w $(GO_FILES)
	goimports -w $(GO_FILES)

fmt-check:
	@files=$$(gofmt -l $(GO_FILES)); if [ -n "$$files" ]; then \
		echo "gofmt would reformat:"; \
		echo "$$files"; \
		exit 1; \
	fi
	@imports=$$(goimports -l $(GO_FILES)); if [ -n "$$imports" ]; then \
		echo "goimports would reformat:"; \
		echo "$$imports"; \
		exit 1; \
	fi

lint:
	golangci-lint run ./...

vet:
	$(GO) vet ./...

tidy:
	$(GO) mod tidy

tidy-check:
	bash scripts/tidy-check.sh

verify: fmt-check vet lint tidy-check test