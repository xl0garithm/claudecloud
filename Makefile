.PHONY: build test dev generate image-build clean vet lint

# Go
GO := go
GOFLAGS := -v
BINARY := bin/cloudcode

build:
	$(GO) build $(GOFLAGS) -o $(BINARY) ./cmd/cloudcode

test:
	$(GO) test ./... -short -count=1

test-all:
	$(GO) test ./... -count=1

vet:
	$(GO) vet ./...

lint: vet
	@echo "Lint passed"

generate:
	$(GO) generate ./...

# Docker
image-build:
	docker build -t claude-instance -f docker/Dockerfile.instance docker/

# Dev stack
dev: image-build
	docker compose up --build

dev-down:
	docker compose down

clean:
	rm -rf $(BINARY)
	$(GO) clean -testcache
