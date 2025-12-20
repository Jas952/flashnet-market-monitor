package bots_monitor

// AMM swaps monitor (Big Sales/Buys) + Telegram message formatting.

import (
	"context"
	"fmt"
	"html"
	"os/exec"
	"path/filepath"
	"spark-wallet/internal/clients_api/flashnet"
	"spark-wallet/internal/clients_api/luminex"
	"spark-wallet/internal/features/holders"
	storage "spark-wallet/internal/infra/fs"
	log "spark-wallet/internal/infra/log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

// Constants for SOON token photos
const (
	// SOONPoolLpPublicKey - poolLpPublicKey of SOON token
	SOONPoolLpPublicKey = "021cda97a28df127f41e480ebede196f6f7d46dd6754feab7c228d8273dce6d39e"
	// SOONBuyPhotoURL - photo URL for SOON buy
	SOONBuyPhotoURL = "https://i.ibb.co/VsXVSdx/soongreen.jpg"
	// SOONSellPhotoURL - photo URL for SOON sell
	SOONSellPhotoURL = "https://i.ibb.co/hRN5G3qn/Gemini-Generated-Image-rzyanmrzyanmrzya.png"
)

// formatSwapMessageBig assembles swap message text for Telegram.
func formatSwapMessageBig(swap flashnet.Swap) string {
	swapType := swap.GetSwapType()

	var typeEmoji, typeLabel string
	switch swapType {
	case flashnet.SwapTypeBuy:
		typeEmoji = "🟢"
		typeLabel = "ПОКУПКА"
	case flashnet.SwapTypeSell:
		typeEmoji = "🔴"
		typeLabel = "ПРОДАЖА"
	default:
		typeEmoji = "🔄"
		typeLabel = "ОБМЕН"
	}

	message := fmt.Sprintf("%s %s (%s)\n\n", typeEmoji, typeLabel, swapType)

	// Format amounts based on operation type
	if swapType == flashnet.SwapTypeBuy {
		// Buy: give BTC, receive token
		btcAmount := formatBTCAmountBig(swap.AmountIn)
		message += fmt.Sprintf("💰 Отдали: %s BTC\n", btcAmount)
		message += fmt.Sprintf("📦 Получили: %s токенов\n", swap.AmountOut)
	} else if swapType == flashnet.SwapTypeSell {
		// Sell: give token, receive BTC
		btcAmount := formatBTCAmountBig(swap.AmountOut)
		message += fmt.Sprintf("📦 Отдали: %s токенов\n", swap.AmountIn)
		message += fmt.Sprintf("💰 Получили: %s BTC\n", btcAmount)
	} else {
		// Token-to-token swap
		message += fmt.Sprintf("Amount In: %s\n", swap.AmountIn)
		message += fmt.Sprintf("Amount Out: %s\n", swap.AmountOut)
	}

	message += fmt.Sprintf("Price: %s\n", swap.Price)
	message += fmt.Sprintf("Time: %s\n", swap.CreatedAt)
	message += fmt.Sprintf("Pool Type: %s\n", swap.PoolType)
	message += fmt.Sprintf("Swapper: %s\n", swap.SwapperPublicKey)
	message += fmt.Sprintf("Pool LP: %s\n", swap.PoolLpPublicKey)

	if swap.FeePaid != "" {
		message += fmt.Sprintf("Fee: %s\n", swap.FeePaid)
	}

	return message
}

// formatBTCAmountBig formats BTC amount from minimal units (satoshi) to readable format
func formatBTCAmountBig(satoshiStr string) string {
	var satoshi float64
	fmt.Sscanf(satoshiStr, "%f", &satoshi)
	btc := satoshi / 1e8
	return formatBTCWithoutTrailingZeros(btc)
}

// formatBTCWithoutTrailingZeros formats BTC number without trailing zeros
func formatBTCWithoutTrailingZeros(btc float64) string {
	formatted := fmt.Sprintf("%.8f", btc)
	// Remove trailing zeros after decimal point
	formatted = strings.TrimRight(formatted, "0")
	// If only decimal point remains after removing zeros, remove it too
	formatted = strings.TrimRight(formatted, ".")
	return formatted
}

// getBTCAmountFromSwap returns BTC amount from swap based on operation type
func getBTCAmountFromSwap(swap flashnet.Swap) float64 {
	swapType := swap.GetSwapType()
	var satoshiStr string

	if swapType == flashnet.SwapTypeBuy {
		// Buy: amountIn is BTC
		satoshiStr = swap.AmountIn
	} else if swapType == flashnet.SwapTypeSell {
		// Sell: amountOut is BTC
		satoshiStr = swap.AmountOut
	} else {
		// Token-to-token swap - don't send
		log.LogDebug("Swap is not buy/sell, returning 0 BTC value",
			zap.String("swapType", string(swapType)),
			zap.String("assetInAddress", swap.AssetInAddress),
			zap.String("assetOutAddress", swap.AssetOutAddress),
			zap.String("poolLpPublicKey", swap.PoolLpPublicKey))
		return 0
	}

	// Check that string is not empty
	if satoshiStr == "" {
		log.LogWarn("Empty AmountIn/AmountOut for buy/sell swap, returning 0 BTC value",
			zap.String("swapType", string(swapType)),
			zap.String("amountIn", swap.AmountIn),
			zap.String("amountOut", swap.AmountOut),
			zap.String("poolLpPublicKey", swap.PoolLpPublicKey))
		return 0
	}

	var satoshi float64
	n, err := fmt.Sscanf(satoshiStr, "%f", &satoshi)
	if err != nil || n != 1 {
		log.LogWarn("Failed to parse AmountIn/AmountOut, returning 0 BTC value",
			zap.String("swapType", string(swapType)),
			zap.String("satoshiStr", satoshiStr),
			zap.String("amountIn", swap.AmountIn),
			zap.String("amountOut", swap.AmountOut),
			zap.Error(err))
		return 0
	}

	btcValue := satoshi / 1e8
	log.LogDebug("Calculated BTC value from swap",
		zap.String("swapType", string(swapType)),
		zap.String("satoshiStr", satoshiStr),
		zap.Float64("satoshi", satoshi),
		zap.Float64("btcValue", btcValue))
	return btcValue
}

// shouldSendSwap checks if swap should be sent to Telegram (amount >= minBTCAmount)
func shouldSendSwap(swap flashnet.Swap, minBTCAmount float64) bool {
	btcAmount := getBTCAmountFromSwap(swap)
	return btcAmount >= minBTCAmount
}

// isFilteredToken checks if token is filtered (in the list)
func isFilteredToken(poolLpPublicKey string, filteredTokensList []string) bool {
	if poolLpPublicKey == "" || len(filteredTokensList) == 0 {
		return false
	}
	for _, token := range filteredTokensList {
		if strings.TrimSpace(token) == poolLpPublicKey {
			return true
		}
	}
	return false
}

// ParseFilteredTokens parses comma-separated token string into slice
// Exported function for use in other packages
func ParseFilteredTokens(tokensStr string) []string {
	if tokensStr == "" {
		return []string{}
	}
	tokens := strings.Split(tokensStr, ",")
	result := make([]string, 0, len(tokens))
	for _, token := range tokens {
		trimmed := strings.TrimSpace(token)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// getTokenAmountFromSwap and count tokens from swap
func getTokenAmountFromSwap(swap flashnet.Swap, amountStr string, tokenMetadata *luminex.TokenMetadata) string {
	if amountStr == "" {
		return ""
	}

	var amountValue float64
	fmt.Sscanf(amountStr, "%f", &amountValue)

	decimals := 8 // Default value
	if tokenMetadata != nil && tokenMetadata.Ticker != "" {
		decimals = luminex.GetTokenDecimals(swap.PoolLpPublicKey, swap, tokenMetadata.Ticker)
	}

	// on 10^decimals
	decimalsMultiplier := 1.0
	for i := 0; i < decimals; i++ {
		decimalsMultiplier *= 10
	}
	tokenAmount := amountValue / decimalsMultiplier

	// Format count tokens (use formatTokenAmount from system_works)
	return formatTokenAmountLocal(tokenAmount)
}

// saveHolderFromSwap address and dynamic_holders.json on swap
// swap get balance token API,
// from saved_holders.json and update file
func saveHolderFromSwap(swap flashnet.Swap) {
	ticker, err := holders.GetTickerFromPoolLpPublicKey(swap.PoolLpPublicKey)
	if err != nil {
		log.LogDebug("Failed to get ticker from poolLpPublicKey", zap.String("poolLpPublicKey", swap.PoolLpPublicKey), zap.Error(err))
		return
	}

	if ticker == "" {
		log.LogDebug("Ticker is empty, skipping holder save", zap.String("poolLpPublicKey", swap.PoolLpPublicKey))
		return
	}

	// Check, ticker for ASTY, SOON, BITTY)
	if !holders.IsTickerAllowed(ticker) {
		log.LogDebug("Ticker not in allowed list, skipping holder save", zap.String("ticker", ticker))
		return
	}

	// Get balance token API
	// API: https://api.luminex.io/spark/address/{swapperPublicKey}
	_, currentAmount, err := holders.GetTokenBalanceFromWallet(swap.SwapperPublicKey, ticker)
	if err != nil {
		log.LogDebug("Failed to get current token balance from API", zap.String("address", swap.SwapperPublicKey), zap.String("ticker", ticker), zap.Error(err))
		return
	}

	savedData, err := holders.LoadSavedHolders(ticker)
	if err != nil {
		log.LogWarn("Failed to load saved holders", zap.String("ticker", ticker), zap.Error(err))
		return
	}

	// Get balance from saved_holders.json (if address
	var previousAmount float64
	previousBalanceStr, exists := savedData.Holders[swap.SwapperPublicKey]
	if exists {
		fmt.Sscanf(previousBalanceStr, "%f", &previousAmount)
	}

	currentAmountStr := fmt.Sprintf("%.8f", currentAmount)

	var action string
	const minBalanceThreshold = 10.0 // balance for (10 tokens)

	if !exists {
		// address -
		// on swap'
		swapType := swap.GetSwapType()

		if currentAmount == 0 {
			// balance 0 -
			action = "liquidated"
		} else if currentAmount < minBalanceThreshold {
			// balance 10 tokens - save
			log.LogDebug("New address has balance below threshold, skipping", zap.String("ticker", ticker), zap.String("address", swap.SwapperPublicKey), zap.Float64("amount", currentAmount))
			return
		} else {
			// on swap'
			if swapType == flashnet.SwapTypeBuy {
				action = "invested"
			} else if swapType == flashnet.SwapTypeSell {
				action = "sold"
			} else {
				// Swap - by default invested if balance > 0
				action = "invested"
			}
		}
	} else {
		// address -
		const epsilon = 0.0001
		balanceDiff := currentAmount - previousAmount

		if currentAmount == 0 {
			action = "liquidated"
		} else if balanceDiff > epsilon {
			action = "invested"
		} else if balanceDiff < -epsilon {
			action = "sold"
		} else {
			// balance -
			log.LogDebug("Balance unchanged, skipping update", zap.String("ticker", ticker), zap.String("address", swap.SwapperPublicKey), zap.Float64("amount", currentAmount))
			return
		}
	}

	// Update saved_holders.json
	// Remove address if balance 10 tokens or 0
	if currentAmount >= minBalanceThreshold {
		// Save balance (or update
		savedData.Holders[swap.SwapperPublicKey] = currentAmountStr
	} else {
		// balance 10 tokens or 0 - remove from saved_holders.json
		delete(savedData.Holders, swap.SwapperPublicKey)
	}

	// Save saved_holders.json
	if err := holders.SaveSavedHolders(ticker, savedData); err != nil {
		log.LogWarn("Failed to save saved holders", zap.String("ticker", ticker), zap.Error(err))
		return
	}

	// Calculate amount in BTC
	btcValue := getBTCAmountFromSwap(swap)

	log.LogDebug("Calculated BTC value from swap",
		zap.String("ticker", ticker),
		zap.String("swapperPublicKey", swap.SwapperPublicKey),
		zap.String("swapType", string(swap.GetSwapType())),
		zap.String("amountIn", swap.AmountIn),
		zap.String("amountOut", swap.AmountOut),
		zap.Float64("btcValue", btcValue))

	// Update dynamic_holders.json
	// previousAmount for Delta and btcValue for
	if err := holders.UpdateDynamicHoldersFromSwap(ticker, swap.SwapperPublicKey, currentAmount, previousAmount, action, btcValue); err != nil {
		log.LogWarn("Failed to update dynamic holders", zap.String("ticker", ticker), zap.String("address", swap.SwapperPublicKey), zap.Error(err))
		return
	}

	// Update flow data for invested and sold, for liquidated)
	if action == "invested" || action == "sold" {
		if err := holders.UpdateFlowFromSwap(ticker, action, btcValue); err != nil {
			log.LogWarn("Failed to update flow from swap", zap.String("ticker", ticker), zap.String("action", action), zap.Error(err))
		}
	}

	log.LogDebug("Updated holder from swap",
		zap.String("ticker", ticker),
		zap.String("address", swap.SwapperPublicKey),
		zap.Float64("previousAmount", previousAmount),
		zap.Float64("currentAmount", currentAmount),
		zap.String("action", action))
}

// formatTokenAmountLocal count tokens in (1.1M, 2.2K and ..)
func formatTokenAmountLocal(amount float64) string {
	if amount == 0 {
		return "0"
	}

	if amount >= 1e9 {
		value := amount / 1e9
		formatted := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", value), "0"), ".")
		return fmt.Sprintf("%sB", formatted)
	} else if amount >= 1e6 {
		value := amount / 1e6
		formatted := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", value), "0"), ".")
		return fmt.Sprintf("%sM", formatted)
	} else if amount >= 1e3 {
		value := amount / 1e3
		formatted := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", value), "0"), ".")
		return fmt.Sprintf("%sK", formatted)
	} else {
		// - 2 or if
		if amount == float64(int64(amount)) {
			return fmt.Sprintf("%.0f", amount)
		}
		formatted := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", amount), "0"), ".")
		return formatted
	}
}

func formatMarketCap(marketcap float64) string {
	if marketcap == 0 {
		return ""
	}

	if marketcap >= 1e9 {
		value := marketcap / 1e9
		formatted := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", value), "0"), ".")
		return fmt.Sprintf("$%sB", formatted)
	} else if marketcap >= 1e6 {
		value := marketcap / 1e6
		formatted := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", value), "0"), ".")
		return fmt.Sprintf("$%sM", formatted)
	} else if marketcap >= 1e3 {
		value := marketcap / 1e3
		formatted := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", value), "0"), ".")
		return fmt.Sprintf("$%sK", formatted)
	} else {
		formatted := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", marketcap), "0"), ".")
		return fmt.Sprintf("$%s", formatted)
	}
}

// formatSwapMessageForTelegram formats swap message for Telegram.
func formatSwapMessageForTelegram(client *flashnet.Client, swap flashnet.Swap) (string, string) {
	swapType := swap.GetSwapType()
	btcAmount := getBTCAmountFromSwap(swap)
	btcAmountStr := formatBTCWithoutTrailingZeros(btcAmount)

	// Get token from Luminex API
	tokenMetadata := luminex.GetTokenMetadata(swap.PoolLpPublicKey)

	// Get from Luminex API for
	marketcap := luminex.GetPoolMarketCap(swap.PoolLpPublicKey, swap)
	marketcapStr := formatMarketCap(marketcap)

	var emoji, action string
	if swapType == flashnet.SwapTypeBuy {
		emoji = "🟢"
		action = "Buy"
	} else if swapType == flashnet.SwapTypeSell {
		emoji = "🔴"
		action = "Sell"
	} else {
		message := formatSwapMessageBig(swap)
		tradeLink := fmt.Sprintf("https://luminex.io/spark/trade/%s", swap.PoolLpPublicKey)
		return message, tradeLink
	}

	// on token (for
	tradeLink := fmt.Sprintf("https://luminex.io/spark/trade/%s", swap.PoolLpPublicKey)

	// Get balance wallet and
	var walletInfo string
	balanceResp, err := luminex.GetWalletBalance(swap.SwapperPublicKey)

	// Get (username) for "wallet"
	username := luminex.GetWalletUsername(swap.SwapperPublicKey)
	displayName := "wallet" // Default value
	if username != "" {
		displayName = username
	}

	// Get token walletInfo)
	firstBuyDate := ""
	if client != nil {
		firstBuyDateStr, err := flashnet.GetFirstBuySwap(client, swap.SwapperPublicKey, swap.PoolLpPublicKey)
		if err != nil {
			log.LogDebug("Failed to get first buy swap",
				zap.String("swapperPublicKey", swap.SwapperPublicKey),
				zap.String("poolLpPublicKey", swap.PoolLpPublicKey),
				zap.Error(err))
		} else if firstBuyDateStr != "" {
			firstBuyDate = fmt.Sprintf("First buy - %s\n", firstBuyDateStr)
			log.LogDebug("First buy date added to message",
				zap.String("swapperPublicKey", swap.SwapperPublicKey),
				zap.String("poolLpPublicKey", swap.PoolLpPublicKey),
				zap.String("firstBuyDate", firstBuyDateStr))
		} else {
			log.LogDebug("First buy date is empty",
				zap.String("swapperPublicKey", swap.SwapperPublicKey),
				zap.String("poolLpPublicKey", swap.PoolLpPublicKey))
		}
	}

	var marketcapInfo string
	if marketcapStr != "" {
		marketcapInfo = fmt.Sprintf("Market cap - %s\n", marketcapStr)
	}

	// Get holding token wallet
	var holdingInfo string
	tokenTicker := ""
	if tokenMetadata != nil && tokenMetadata.Ticker != "" {
		tokenTicker = tokenMetadata.Ticker
		holdingAmount, holdingValue := luminex.GetWalletTokenHolding(swap.SwapperPublicKey, swap.PoolLpPublicKey, swap, tokenTicker)
		if holdingAmount != "null" {
			if holdingValue != "" {
				holdingInfo = fmt.Sprintf("Holding right now - %s (%s)\n", holdingAmount, holdingValue)
			} else {
				holdingInfo = fmt.Sprintf("Holding right now - %s\n", holdingAmount)
			}
		} else {
			holdingInfo = "Holding right now - null\n"
		}
	}

	if err == nil && balanceResp != nil {
		// Use SparkAddress if use
		sparkAddress := balanceResp.SparkAddress
		if sparkAddress == "" {
			sparkAddress = swap.SwapperPublicKey
		}
		balanceBTCFloat := float64(balanceResp.Balance.BtcHardBalanceSats) / 1e8
		balanceBTC := formatBTCWithoutTrailingZeros(balanceBTCFloat)
		walletLink := fmt.Sprintf("https://luminex.io/spark/address/%s", sparkAddress)

		// Get 3 wallet (swapperPublicKey)
		walletSuffix := ""
		if len(swap.SwapperPublicKey) >= 3 {
			walletSuffix = swap.SwapperPublicKey[len(swap.SwapperPublicKey)-3:]
		}

		// displayName for HTML
		displayNameEscaped := html.EscapeString(displayName)

		// Use HTML for in Telegram
		// Buyer wallet, Holding Current net balance
		// Add 3 in
		// Add First buy Buyer wallet
		walletInfo = fmt.Sprintf("\n<blockquote>%sBuyer wallet - <a href=\"%s\">%s</a> (%s)\n%s%sCurrent net balance - %s btc</blockquote>", marketcapInfo, walletLink, displayNameEscaped, walletSuffix, firstBuyDate, holdingInfo, balanceBTC)
	} else {
		// If get balance, or
		walletSuffix := ""
		if len(swap.SwapperPublicKey) >= 3 {
			walletSuffix = swap.SwapperPublicKey[len(swap.SwapperPublicKey)-3:]
		}

		walletInfo = fmt.Sprintf("\n<blockquote>%sBuyer wallet - ", marketcapInfo)
		if username != "" {
			walletInfo += fmt.Sprintf("%s (%s)\n%s%s</blockquote>", username, walletSuffix, firstBuyDate, holdingInfo)
		} else {
			walletInfo += fmt.Sprintf("%s (%s)\n%s%s</blockquote>", swap.SwapperPublicKey, walletSuffix, firstBuyDate, holdingInfo)
		}
	}

	var tokenNameHTML string
	if tokenMetadata != nil && tokenMetadata.Name != "" && tokenMetadata.Ticker != "" {
		tokenNameHTML = fmt.Sprintf("%s {%s}", tokenMetadata.Name, tokenMetadata.Ticker)
	} else {
		tokenNameHTML = swap.PoolLpPublicKey
	}

	// Get count tokens from swap
	var tokenAmountStr string
	if swapType == flashnet.SwapTypeBuy {
		// AmountOut - count tokens BTC)
		tokenAmountStr = getTokenAmountFromSwap(swap, swap.AmountOut, tokenMetadata)
	} else if swapType == flashnet.SwapTypeSell {
		// AmountIn - count tokens BTC)
		tokenAmountStr = getTokenAmountFromSwap(swap, swap.AmountIn, tokenMetadata)
	}

	// tokens (if
	var tokenAmountDisplay string
	if tokenAmountStr != "" {
		tokenAmountDisplay = fmt.Sprintf(" (%s)", tokenAmountStr)
	}

	message := fmt.Sprintf("%s %s %s - %s btc%s%s", emoji, action, tokenNameHTML, btcAmountStr, tokenAmountDisplay, walletInfo)

	return message, tradeLink
}

// findNewSwapsBig swaps
// newSwaps - swaps API)
func findNewSwapsBig(oldSwaps, newSwaps []flashnet.Swap) []flashnet.Swap {
	if len(oldSwaps) == 0 {
		return newSwaps
	}

	// Create (map) for
	oldSwapMap := make(map[string]bool)
	for _, swap := range oldSwaps {
		oldSwapMap[swap.ID] = true
	}

	var newSwapsList []flashnet.Swap
	for _, swap := range newSwaps {
		if !oldSwapMap[swap.ID] {
			newSwapsList = append(newSwapsList, swap)
		}
	}

	return newSwapsList
}

// parseChatIDBig Chat ID from ID for
// in Telegram ID -1003190218710)
func parseChatIDBig(chatIDStr string) int64 {
	var chatID int64
	fmt.Sscanf(chatIDStr, "%d", &chatID)
	return chatID
}

// RunBigSalesBuysMonitor in AMM and
// bot - Telegram for nil for
// client - for Flashnet API
// chatID - ID in Telegram,
// minBTCAmount - amount in BTC for
// filteredBot - for in nil)
// filteredTokensList - tokens for
// filteredMinBTCAmount - amount for
func RunBigSalesBuysMonitor(bot *tgbotapi.BotAPI, client *flashnet.Client, chatID string, minBTCAmount float64, filteredBot *tgbotapi.BotAPI, filteredChatID string, filteredTokensList []string, filteredMinBTCAmount float64) {
	log.LogInfo("Starting Big Sales/Buys Monitor...",
		zap.Bool("hasMainBot", bot != nil),
		zap.String("mainChatID", chatID),
		zap.Bool("hasFilteredBot", filteredBot != nil),
		zap.String("filteredChatID", filteredChatID),
		zap.Int("filteredTokensCount", len(filteredTokensList)),
		zap.Float64("filteredMinBTCAmount", filteredMinBTCAmount))

	// Create for 5
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Create for token 30
	tokenCheckTicker := time.NewTicker(30 * time.Minute)
	defer tokenCheckTicker.Stop()

	// Create for tokens 30
	var reloadTokensTicker *time.Ticker
	var reloadTokensChan <-chan time.Time
	if filteredChatID != "" {
		reloadTokensTicker = time.NewTicker(30 * time.Second)
		reloadTokensChan = reloadTokensTicker.C
		defer reloadTokensTicker.Stop()
		log.LogInfo("Filtered tokens reload enabled", zap.Int("initialTokensCount", len(filteredTokensList)))
	} else {
		log.LogWarn("Filtered chat ID is empty - filtered tokens monitoring disabled")
		// Create a nil channel that never receives
		reloadTokensChan = nil
	}

	checkAndRefreshToken(client)

	for {
		select {
		case <-reloadTokensChan:
			if reloadTokensTicker != nil && filteredChatID != "" {
				newTokensList, err := storage.LoadFilteredTokens()
				if err != nil {
					log.LogWarn("Failed to reload filtered tokens, using cached list", zap.Error(err))
				} else {
					filteredTokensList = newTokensList
					log.LogInfo("Reloaded filtered tokens from file", zap.Int("count", len(filteredTokensList)))
				}
			}
		case <-tokenCheckTicker.C:
			checkAndRefreshToken(client)
		case <-ticker.C:
			// 100 swaps from AMM
			ctx := context.Background()
			limit := 100
			swapsResp, err := client.GetSwaps(ctx, flashnet.GetSwapsOptions{
				Limit: &limit, // 100 swaps
			})
			if err != nil {
				log.LogError("Failed to get swaps", zap.Error(err))
				continue
			}

			// Load from file for
			oldSwapsResp, _ := storage.LoadSwapsResponse("big_sales_module/100_swaps.json")
			var oldSwaps []flashnet.Swap
			if oldSwapsResp != nil {
				oldSwaps = oldSwapsResp.Swaps
			}

			// Save in file big_sales_module/100_swaps.json
			err = storage.SaveSwapsResponse("big_sales_module/100_swaps.json", swapsResp)
			if err != nil {
				log.LogWarn("Failed to save swaps response", zap.Error(err))
			} else {
				log.LogInfo("Saved swaps to big_sales_module/100_swaps.json", zap.Int("count", len(swapsResp.Swaps)), zap.Int("totalAvailable", swapsResp.TotalCount))
			}

			newSwaps := findNewSwapsBig(oldSwaps, swapsResp.Swaps)

			if len(newSwaps) > 0 {
				log.LogInfo("Found new swaps", zap.Int("count", len(newSwaps)))

				for _, swap := range newSwaps {
					// in (for tokens)
					if bot != nil && chatID != "" {
						if shouldSendSwap(swap, minBTCAmount) {
							message, tradeLink := formatSwapMessageForTelegram(client, swap)

							// Create and in Telegram HTML (for
							msg := tgbotapi.NewMessage(parseChatIDBig(chatID), message)
							msg.ParseMode = tgbotapi.ModeHTML
							msg.DisableWebPagePreview = true
							keyboard := tgbotapi.NewInlineKeyboardMarkup(
								tgbotapi.NewInlineKeyboardRow(
									tgbotapi.NewInlineKeyboardButtonURL("Trade on Luminex", tradeLink),
								),
							)
							msg.ReplyMarkup = keyboard
							_, err := bot.Send(msg)
							if err != nil {
								log.LogError("Failed to send message", zap.Error(err))
							} else {
								log.LogInfo("Sent swap notification", zap.String("swapID", swap.ID))
								// Save address in saved_holders.json
								saveHolderFromSwap(swap)
							}
						}
					}

					// in (for tokens)
					if filteredBot != nil && filteredChatID != "" && len(filteredTokensList) > 0 {
						// Check, token
						isFiltered := isFilteredToken(swap.PoolLpPublicKey, filteredTokensList)
						log.LogDebug("Checking swap for filtered tokens",
							zap.String("swapID", swap.ID),
							zap.String("poolLpPublicKey", swap.PoolLpPublicKey),
							zap.Bool("isFiltered", isFiltered),
							zap.Int("filteredTokensCount", len(filteredTokensList)))

						if isFiltered {
							btcAmount := getBTCAmountFromSwap(swap)
							shouldSend := shouldSendSwap(swap, filteredMinBTCAmount)
							log.LogDebug("Filtered token swap check",
								zap.String("swapID", swap.ID),
								zap.Float64("btcAmount", btcAmount),
								zap.Float64("minBTCAmount", filteredMinBTCAmount),
								zap.Bool("shouldSend", shouldSend))

							if shouldSend {
								message, tradeLink := formatSwapMessageForTelegram(client, swap)

								// Check, SOON
								isSOON := swap.PoolLpPublicKey == SOONPoolLpPublicKey
								swapType := swap.GetSwapType()

								keyboard := tgbotapi.NewInlineKeyboardMarkup(
									tgbotapi.NewInlineKeyboardRow(
										tgbotapi.NewInlineKeyboardButtonURL("Trade on Luminex", tradeLink),
									),
								)

								var err error
								if isSOON {
									var photoURL string
									if swapType == flashnet.SwapTypeBuy {
										photoURL = SOONBuyPhotoURL
									} else if swapType == flashnet.SwapTypeSell {
										photoURL = SOONSellPhotoURL
									} else {
										photoURL = ""
									}

									if photoURL != "" {
										photoMsg := tgbotapi.NewPhoto(parseChatIDBig(filteredChatID), tgbotapi.FileURL(photoURL))
										photoMsg.Caption = message
										photoMsg.ParseMode = tgbotapi.ModeHTML
										photoMsg.ReplyMarkup = keyboard
										_, err = filteredBot.Send(photoMsg)
									} else {
										// If buy/sell,
										msg := tgbotapi.NewMessage(parseChatIDBig(filteredChatID), message)
										msg.ParseMode = tgbotapi.ModeHTML
										msg.DisableWebPagePreview = true
										msg.ReplyMarkup = keyboard
										_, err = filteredBot.Send(msg)
									}
								} else {
									msg := tgbotapi.NewMessage(parseChatIDBig(filteredChatID), message)
									msg.ParseMode = tgbotapi.ModeHTML
									msg.DisableWebPagePreview = true
									msg.ReplyMarkup = keyboard
									_, err = filteredBot.Send(msg)
								}

								if err != nil {
									log.LogError("Failed to send filtered token message", zap.Error(err), zap.String("chatID", filteredChatID), zap.Bool("isSOON", isSOON))
								} else {
									log.LogInfo("Sent filtered token notification", zap.String("swapID", swap.ID), zap.String("poolLpPublicKey", swap.PoolLpPublicKey), zap.Bool("isSOON", isSOON), zap.String("swapType", string(swapType)))
									// Save address in saved_holders.json
									saveHolderFromSwap(swap)
								}
							}
						}
					}
				}
			}
		}
	}
}

func checkAndRefreshToken(client *flashnet.Client) {
	dataDir := "data_in"

	// Check, token
	tokenFile, err := flashnet.LoadTokenFromFile(dataDir)
	if err == nil && tokenFile.AccessToken != "" {
		expiresAt, err := flashnet.GetTokenExpirationTime(tokenFile.AccessToken)
		if err == nil && expiresAt > time.Now().Unix() {
			// token use
			client.SetJWT(tokenFile.AccessToken)
			return
		}
	}

	// token or -
	publicKey := tokenFile.PublicKey
	if publicKey == "" {
		log.LogWarn("Cannot refresh token: public key not found")
		return
	}

	log.LogInfo("Token expired or invalid, refreshing...")
	ctx := context.Background()

	// Get challenge
	_, err = client.GetChallengeAndSave(ctx, dataDir, publicKey)
	if err != nil {
		log.LogError("Failed to get challenge for token refresh", zap.Error(err))
		return
	}

	signChallengePath := filepath.Join("spark-cli", "sign-challenge.mjs")
	cmd := exec.Command("node", signChallengePath)
	cmd.Dir = "."
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.LogError("Failed to sign challenge for token refresh",
			zap.Error(err),
			zap.String("output", string(output)))
		return
	}

	// time on file
	time.Sleep(500 * time.Millisecond)

	sigFile, err := flashnet.LoadSignatureFromFile(dataDir)
	if err != nil || sigFile.Signature == "" {
		log.LogError("Signature file not found after signing")
		return
	}

	_, err = client.VerifySignatureAndSave(ctx, dataDir, sigFile.PublicKey, sigFile.Signature)
	if err != nil {
		log.LogError("Failed to verify signature for token refresh", zap.Error(err))
		return
	}

	log.LogSuccess("Token refreshed successfully")
}

// RunFilteredTokensMonitor for tokens and in
// bot - Telegram for nil for
// client - for Flashnet API
// chatID - ID in Telegram for tokens
// filteredTokensList - poolLpPublicKey tokens for
// minBTCAmount - amount in BTC for
func RunFilteredTokensMonitor(bot *tgbotapi.BotAPI, client *flashnet.Client, chatID string, filteredTokensList []string, minBTCAmount float64) {
	log.LogInfo("Starting Filtered Tokens Monitor...", zap.Int("filteredTokensCount", len(filteredTokensList)))

	// Create for 5
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Create for token 30
	tokenCheckTicker := time.NewTicker(30 * time.Minute)
	defer tokenCheckTicker.Stop()

	checkAndRefreshToken(client)

	for {
		select {
		case <-tokenCheckTicker.C:
			checkAndRefreshToken(client)
		case <-ticker.C:
			// 100 swaps from AMM
			ctx := context.Background()
			limit := 100
			swapsResp, err := client.GetSwaps(ctx, flashnet.GetSwapsOptions{
				Limit: &limit, // 100 swaps
			})
			if err != nil {
				log.LogError("Failed to get swaps", zap.Error(err))
				continue
			}

			// Load from file for
			oldSwapsResp, _ := storage.LoadSwapsResponse("big_sales_module/100_swaps.json")
			var oldSwaps []flashnet.Swap
			if oldSwapsResp != nil {
				oldSwaps = oldSwapsResp.Swaps
			}

			newSwaps := findNewSwapsBig(oldSwaps, swapsResp.Swaps)

			if len(newSwaps) > 0 {
				log.LogInfo("Found new swaps for filtered monitor", zap.Int("count", len(newSwaps)))

				if bot != nil && chatID != "" && len(filteredTokensList) > 0 {
					for _, swap := range newSwaps {
						// Check, token
						if !isFilteredToken(swap.PoolLpPublicKey, filteredTokensList) {
							continue
						}

						if !shouldSendSwap(swap, minBTCAmount) {
							continue
						}

						message, tradeLink := formatSwapMessageForTelegram(client, swap)

						msg := tgbotapi.NewMessage(parseChatIDBig(chatID), message)
						msg.ParseMode = tgbotapi.ModeHTML
						msg.DisableWebPagePreview = true
						keyboard := tgbotapi.NewInlineKeyboardMarkup(
							tgbotapi.NewInlineKeyboardRow(
								tgbotapi.NewInlineKeyboardButtonURL("Trade on Luminex", tradeLink),
							),
						)
						msg.ReplyMarkup = keyboard
						_, err := bot.Send(msg)
						if err != nil {
							log.LogError("Failed to send filtered token message", zap.Error(err), zap.String("chatID", chatID))
						} else {
							log.LogInfo("Sent filtered token notification", zap.String("swapID", swap.ID), zap.String("poolLpPublicKey", swap.PoolLpPublicKey))
							// Save address in saved_holders.json
							saveHolderFromSwap(swap)
						}
					}
				}
			}
		}
	}
}
