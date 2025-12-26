.PHONY: help auth-token sign run-big-sales run-bot run-holders stop clean build test test-flashnet test-luminex test-integration test-integration-flashnet test-integration-luminex all_test

help:
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

auth-token:
	@go run cmd/main.go auth full

sign:
	@cd spark-cli && node sign-challenge.mjs

run-bot: auth-token
	@go run cmd/main.go bot

run-big-sales:
	@go run cmd/main.go big-sales

run-holders:
	@go run cmd/main.go holders

stop:
	@echo "Looking for running bot processes..."
	@PIDS=$$(pgrep -f "go run.*cmd/main.go bot\|flashnet-api.*bot" 2>/dev/null || true); \
	if [ -z "$$PIDS" ]; then \
		echo "No running bot processes found"; \
		exit 0; \
	fi; \
	echo "Found bot processes: $$PIDS"; \
	for PID in $$PIDS; do \
		echo "Sending SIGTERM to process $$PID..."; \
		kill -SIGTERM $$PID 2>/dev/null || true; \
	done; \
	echo "Graceful shutdown signal sent. Waiting for processes to stop..."; \
	for i in 1 2 3 4 5; do \
		REMAINING=$$(pgrep -f "go run.*cmd/main.go bot\|flashnet-api.*bot" 2>/dev/null || true); \
		if [ -z "$$REMAINING" ]; then \
			echo "All bot processes stopped successfully"; \
			exit 0; \
		fi; \
		sleep 1; \
	done; \
	REMAINING=$$(pgrep -f "go run.*cmd/main.go bot\|flashnet-api.*bot" 2>/dev/null || true); \
	if [ -n "$$REMAINING" ]; then \
		echo "Some processes are still running after 5 seconds: $$REMAINING"; \
		echo "Force stopping with SIGKILL..."; \
		for PID in $$REMAINING; do \
			kill -9 $$PID 2>/dev/null || true; \
		done; \
		sleep 1; \
		FINAL=$$(pgrep -f "go run.*cmd/main.go bot\|flashnet-api.*bot" 2>/dev/null || true); \
		if [ -z "$$FINAL" ]; then \
			echo "All bot processes stopped"; \
		else \
			echo "Warning: Some processes may still be running: $$FINAL"; \
		fi; \
	else \
		echo "All bot processes stopped successfully"; \
	fi

build:
	@mkdir -p bin
	@go build -o bin/flashnet-api cmd/main.go
	@echo "Binary built: bin/flashnet-api"
	@echo "Usage: ./bin/flashnet-api [command]"

clean:
	@rm -rf bin
	@rm -f spark-wallet
	@echo "Cleaned build artifacts"

test:
	@go test ./...

test-integration-flashnet:
	@go test -tags=integration ./internal/tests -run TestIntegration_Flashnet

test-integration-luminex:
	@go test -tags=integration ./internal/tests -run TestIntegration_Luminex

all_test:
	@mkdir -p internal/tests/logs

	@echo "Running Luminex integration tests..." > /dev/null; \
	go test -count=1 -tags=integration ./internal/tests -run TestIntegration_Luminex > internal/tests/logs/integration_luminex.log 2>&1; \
	status_lum=$$?; \
	time_lum=$$(grep -E '^ok[ \t]+' internal/tests/logs/integration_luminex.log 2>/dev/null | grep -oE '[0-9]+\.[0-9]+s' | head -1 || echo ""); \
	if [ $$status_lum -eq 0 ]; then \
		printf '\033[32mok\033[0m  internal/tests/luminex_integration_test.go'; \
		if [ -n "$$time_lum" ]; then printf '  %s\n' "$$time_lum"; else printf '\n'; fi; \
	else \
		printf '\033[31merror\033[0m  internal/tests/luminex_integration_test.go\n'; \
		echo "Check logs: internal/tests/logs/integration_luminex.log"; \
		exit $$status_lum; \
	fi

	@echo "Running Flashnet integration tests..." > /dev/null; \
	go test -count=1 -tags=integration ./internal/tests -run TestIntegration_Flashnet > internal/tests/logs/integration_flashnet.log 2>&1; \
	status_fnet=$$?; \
	time_fnet=$$(grep -E '^ok[ \t]+' internal/tests/logs/integration_flashnet.log 2>/dev/null | grep -oE '[0-9]+\.[0-9]+s' | head -1 || echo ""); \
	if [ $$status_fnet -eq 0 ]; then \
		printf '\033[32mok\033[0m  internal/tests/flashnet_integration_test.go'; \
		if [ -n "$$time_fnet" ]; then printf '  %s\n' "$$time_fnet"; else printf '\n'; fi; \
	else \
		printf '\033[31merror\033[0m  internal/tests/flashnet_integration_test.go\n'; \
		echo "Check logs: internal/tests/logs/integration_flashnet.log"; \
		exit $$status_fnet; \
	fi
