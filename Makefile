BINARY := openshift-etcd-backup-explorer
BACKUP ?= ./backup
ADDR   ?= 127.0.0.1:8080

.PHONY: all ui build run demo run-demo dev test test-ci fmt vet lint clean help

all: build

## help: list the targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'

## ui: build the React app into the folder the Go binary embeds
ui:
	cd web && npm ci && npm run build

## build: build the UI and then the single binary that serves it
build: ui
	go build -o $(BINARY) ./cmd/$(BINARY)

## run: build everything and serve $(BACKUP)
run: build
	./$(BINARY) serve -backup $(BACKUP) -addr $(ADDR) -open

## demo: write a synthetic backup to ./demo-backup, no real cluster needed
demo:
	go run ./cmd/$(BINARY) demo -out ./demo-backup

## run-demo: build, generate a synthetic backup and serve it
run-demo: build
	./$(BINARY) demo -out ./demo-backup
	./$(BINARY) serve -backup ./demo-backup -addr $(ADDR) -open

## dev: run the Go API and the Vite dev server side by side
dev:
	@echo "API on :8080, UI on :5173 (the dev server proxies /api)"
	@trap 'kill 0' EXIT; \
	go run ./cmd/$(BINARY) serve -backup $(BACKUP) -addr 127.0.0.1:8080 & \
	cd web && npm run dev & \
	wait

## test: run the Go tests and type check the UI
test:
	go test ./...
	cd web && npx tsc -b

## test-ci: run the tests the way CI does, against the generated fixture only
test-ci:
	OEBE_FIXTURE=demo go test -race ./...

## fmt: format the Go sources
fmt:
	gofmt -w ./cmd ./internal

## vet: run go vet
vet:
	go vet ./...

## lint: run golangci-lint if it is installed
lint:
	@command -v golangci-lint >/dev/null || { echo "golangci-lint is not installed: https://golangci-lint.run/welcome/install/"; exit 1; }
	golangci-lint run

## clean: remove build output, keeping the folder go:embed needs
clean:
	rm -f $(BINARY)
	rm -rf dist demo-backup web/node_modules internal/webui/dist
	mkdir -p internal/webui/dist
	touch internal/webui/dist/.gitkeep
