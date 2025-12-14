package bot

// Package bot contains tokens
// on swap'

import (
	"spark-wallet/http_client/system_works"
	"time"

	"go.uber.org/zap"
)

// RunHoldersDynamicMonitor
// on swap' (saveHolderFromSwap)
func RunHoldersDynamicMonitor() {
	system_works.LogInfo("Starting Holders Dynamic Monitor...")

	// Load tokens
	tokenIDsFile := "data_out/holders_module/id_tokens.json"
	tokenIDs, err := system_works.LoadTokenIdentifiers(tokenIDsFile)
	if err != nil {
		system_works.LogError("Failed to load token identifiers", zap.Error(err))
		return
	}

	if len(tokenIDs) == 0 {
		system_works.LogWarn("No token identifiers found")
		return
	}

	system_works.LogInfo("Loaded token identifiers", zap.Int("count", len(tokenIDs)))

	// Check (ASTY, SOON, BITTY)
	// forceCheck = true, if
	system_works.LogInfo("Performing initial check of all holders (force check on startup)...")
	for tokenIdentifier, ticker := range tokenIDs {
		// Check, ticker for
		if !system_works.IsTickerAllowed(ticker) {
			system_works.LogDebug("Ticker not in allowed list, skipping", zap.String("ticker", ticker))
			continue
		}

		// Check balance forceCheck = true
		// tokenIdentifier in CheckHoldersBalance, for
		if err := system_works.CheckHoldersBalanceWithForce(ticker, tokenIdentifier, true); err != nil {
			system_works.LogError("Failed to check holders balance", zap.String("ticker", ticker), zap.Error(err))
			continue
		}
	}

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	system_works.LogSuccess("Holders dynamic monitor is running",
		zap.String("status", "active"),
		zap.String("checkInterval", "24h"),
		zap.String("note", "Works parallel with swap-based tracking"))

	for {
		select {
		case <-ticker.C:
			// Check (ASTY, SOON, BITTY)
			system_works.LogInfo("Running daily holders balance check...")
			for tokenIdentifier, ticker := range tokenIDs {
				// Check, ticker for
				if !system_works.IsTickerAllowed(ticker) {
					system_works.LogDebug("Ticker not in allowed list, skipping", zap.String("ticker", ticker))
					continue
				}

				system_works.LogDebug("Checking holders balance for token", zap.String("ticker", ticker), zap.String("tokenIdentifier", tokenIdentifier))

				// Check balance
				// tokenIdentifier in CheckHoldersBalance, for
				if err := system_works.CheckHoldersBalance(ticker, tokenIdentifier); err != nil {
					system_works.LogError("Failed to check holders balance", zap.String("ticker", ticker), zap.Error(err))
					continue
				}
			}
			system_works.LogInfo("Daily holders balance check completed")
		}
	}
}
