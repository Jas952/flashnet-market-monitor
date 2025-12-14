.PHONY: help auth-token sign run-big-sales run-bot run-holders clean

help: ## Show available commands
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

auth-token: ## Get challenge, sign it, and fetch JWT token (full auth flow)
	@go run cmd/auth/get_challenge/main.go
	@cd spark-cli && node sign-challenge.mjs
	@go run cmd/auth/verify_token/main.go

sign: ## Sign challenge (used by auth-token)
	@cd spark-cli && node sign-challenge.mjs

run-big-sales: ## Run Big Sales monitor only (no Telegram), with auto token refresh
	@go run cmd/big_sales/main.go

run-bot: auth-token ## Run full bot with Telegram (auth + Telegram notifications)
	@go run cmd/bot/main.go

run-holders: ## Run holders dynamics monitor
	@go run cmd/holders/main.go

clean: ## Remove build artifacts
	@rm -rf bin
	@rm -f spark-wallet
