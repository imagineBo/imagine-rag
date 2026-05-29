# ============================================================
# imagine-rag — developer shortcuts
# Run `make help` to see all available targets.
# ============================================================

.PHONY: help build vet lint clean

## help: print this help message
help:
	@echo "Usage:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## build: compile the library and example
build:
	go build ./...
	cd example && go build ./...

## vet: run go vet
vet:
	go vet ./...

## lint: run golangci-lint (install: https://golangci-lint.run/usage/install/)
lint:
	golangci-lint run ./...

## clean: remove build artifacts
clean:
	cd example && rm -f example example.exe
