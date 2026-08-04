SHELL := /bin/sh

.DEFAULT_GOAL := help

BIN_DIR := bin
LB_BIN := $(BIN_DIR)/lb
BACKEND_BIN := $(BIN_DIR)/backend
DEVCTL := ./scripts/devctl.sh
DEVCTL_TEST := ./scripts/devctl_test.sh
CONFIG ?= examples/basic/config.yaml

.PHONY: help build run demo restart stop reload status logs \
	fmt fmt-check vet test test-race shell-test bench check clean

help: ## Show available commands
	@echo "go-load-balancer development commands:"
	@echo
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*## "}; {printf "  %-12s %s\n", $$1, $$2}'

build: ## Build the load balancer and demo backend binaries
	@mkdir -p $(BIN_DIR)
	go build -o $(LB_BIN) ./cmd/lb
	go build -o $(BACKEND_BIN) ./examples/basic/backend

run: build ## Run the load balancer in the foreground
	$(LB_BIN) -config $(CONFIG)

demo: build ## Start backend A, backend B, and the load balancer
	CONFIG_FILE="$(abspath $(CONFIG))" $(DEVCTL) start

restart: stop demo ## Restart the complete demo environment

stop: ## Stop the load balancer and demo backends
	$(DEVCTL) stop

reload: ## Reload the load balancer configuration without restarting
	$(DEVCTL) reload

status: ## Show demo service status
	@$(DEVCTL) status; code=$$?; \
	[ "$$code" -eq 0 ] || [ "$$code" -eq 1 ]

logs: ## Follow load balancer and backend logs
	@mkdir -p .run/logs
	@touch .run/logs/lb.log \
		.run/logs/backend-a.log \
		.run/logs/backend-b.log
	tail -f .run/logs/lb.log \
		.run/logs/backend-a.log \
		.run/logs/backend-b.log

fmt: ## Format all Go files
	gofmt -w $$(find . -name '*.go' -not -path './.git/*')

fmt-check: ## Check Go formatting without changing files
	@files=$$(gofmt -l $$(find . -name '*.go' -not -path './.git/*')); \
	if [ -n "$$files" ]; then \
		echo "The following Go files need formatting:"; \
		echo "$$files"; \
		exit 1; \
	fi

vet: ## Run Go static analysis
	go vet ./...

test: ## Run all Go tests without cached results
	go test -count=1 ./...

test-race: ## Run all Go tests with the race detector
	go test -race -count=1 ./...

shell-test: ## Test the development process manager
	$(DEVCTL_TEST)

bench: ## Run backend-selection benchmarks
	go test -bench=. -benchmem ./internal/balancer/

check: fmt-check vet test shell-test ## Run all commit checks

clean: stop ## Stop services and remove generated files
	rm -rf $(BIN_DIR) .run
