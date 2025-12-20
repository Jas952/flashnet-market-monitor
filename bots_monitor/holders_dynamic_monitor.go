package bots_monitor

// Package bot contains tokens
// on swap'

import (
	"spark-wallet/internal/features/holders"
	log "spark-wallet/internal/infra/log"
	"time"

	"go.uber.org/zap"
)

// RunHoldersDynamicMonitor
// on swap' (saveHolderFromSwap)
func RunHoldersDynamicMonitor() {
	log.LogInfo("Starting Holders Dynamic Monitor...")

	// Load tokens
	tokenIDsFile := "data_out/holders_module/id_tokens.json"
	tokenIDs, err := holders.LoadTokenIdentifiers(tokenIDsFile)
	if err != nil {
		log.LogError("Failed to load token identifiers", zap.Error(err))
		return
	}

	if len(tokenIDs) == 0 {
		log.LogWarn("No token identifiers found - holders dynamic monitor will not run",
			zap.String("file", tokenIDsFile),
			zap.String("note", "Create this file with token identifiers to enable holders monitoring"))
		return
	}

	log.LogInfo("Loaded token identifiers", zap.Int("count", len(tokenIDs)))

	// Check (ASTY, SOON, BITTY)
	// forceCheck = true, if
	log.LogInfo("Performing initial check of all holders (force check on startup)...")
	for tokenIdentifier, ticker := range tokenIDs {
		// Check, ticker for
		if !holders.IsTickerAllowed(ticker) {
			log.LogDebug("Ticker not in allowed list, skipping", zap.String("ticker", ticker))
			continue
		}

		// Check balance forceCheck = true
		// tokenIdentifier in CheckHoldersBalance, for
		if err := holders.CheckHoldersBalanceWithForce(ticker, tokenIdentifier, true); err != nil {
			log.LogError("Failed to check holders balance", zap.String("ticker", ticker), zap.Error(err))
			continue
		}
	}

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	log.LogSuccess("Holders dynamic monitor is running",
		zap.String("status", "active"),
		zap.String("checkInterval", "24h"),
		zap.String("note", "Works parallel with swap-based tracking"))

	for {
		select {
		case <-ticker.C:
			// Check (ASTY, SOON, BITTY)
			log.LogInfo("Running daily holders balance check...")
			for tokenIdentifier, ticker := range tokenIDs {
				// Check, ticker for
				if !holders.IsTickerAllowed(ticker) {
					log.LogDebug("Ticker not in allowed list, skipping", zap.String("ticker", ticker))
					continue
				}

				log.LogDebug("Checking holders balance for token", zap.String("ticker", ticker), zap.String("tokenIdentifier", tokenIdentifier))

				// Check balance
				// tokenIdentifier in CheckHoldersBalance, for
				if err := holders.CheckHoldersBalance(ticker, tokenIdentifier); err != nil {
					log.LogError("Failed to check holders balance", zap.String("ticker", ticker), zap.Error(err))
					continue
				}
			}
			log.LogInfo("Daily holders balance check completed")
		}
	}
}
