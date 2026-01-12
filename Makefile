SHELL := /bin/sh

.PHONY: fmt test lint

fmt:
	gofmt -w ./cmd ./internal

test:
	go test ./...

lint:
	@echo "lint not configured yet"
