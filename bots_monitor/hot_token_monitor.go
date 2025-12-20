package bots_monitor

import (
	"fmt"
	"spark-wallet/internal/clients_api/flashnet"
	"spark-wallet/internal/clients_api/luminex"
	"spark-wallet/internal/features/hot_token"
	"spark-wallet/internal/infra/log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

// btkn1xegehq3k8gh8hctgvgdegwver5y5f9qt84750reh7xvm49rg0a7qwqh0dl -> btkn1xeg...h0dl
func FormatTokenAddress(address string) string {
	if len(address) <= 12 {
		return address
	}
	return address[:8] + "..." + address[len(address)-4:]
}

// FormatHotTokenMessage
func FormatHotTokenMessage(poolData *hot_token.LuminexFullPoolResponse) string {
	// token BTC)
	var tokenMeta *hot_token.LuminexFullTokenMetadata
	var marketcap float64

	if poolData.AssetBAddress == flashnet.NativeTokenAddress {
		// assetBAddress BTC, assetAAddress token
		tokenMeta = &poolData.TokenAMetadata
		marketcap = poolData.Extra.MarketCapUsd
	} else {
		// assetAAddress BTC, assetBAddress token
		tokenMeta = &poolData.TokenBMetadata
		marketcap = poolData.Extra.MarketCapUsd
	}

	if tokenMeta == nil {
		return ""
	}

	// Format
	marketcapStr := luminex.FormatUSDValue(marketcap)

	// address token
	tokenAddressShort := FormatTokenAddress(tokenMeta.TokenAddress)

	var message strings.Builder
	message.WriteString(fmt.Sprintf("❗️<b>hot</b> rn: {%s} - %s\n", tokenMeta.Ticker, marketcapStr))
	message.WriteString("<blockquote>")

	// Token: address token
	message.WriteString(fmt.Sprintf("Token: %s\n", tokenAddressShort))

	// Luminex: on Luminex
	message.WriteString(fmt.Sprintf("Luminex: <a href=\"%s\">link</a>\n", "https://luminex.io/"))

	// Website: if URL - if - null
	if tokenMeta.WebsiteURL != nil && *tokenMeta.WebsiteURL != "" {
		message.WriteString(fmt.Sprintf("Website: <a href=\"%s\">link</a>\n", *tokenMeta.WebsiteURL))
	} else {
		message.WriteString("Website: null\n")
	}

	// TA: on in Twitter
	twitterSearchURL := fmt.Sprintf("https://x.com/search?q=%s", tokenMeta.TokenAddress)
	message.WriteString(fmt.Sprintf("TA: <a href=\"%s\">link</a>\n", twitterSearchURL))

	// X: Twitter URL if null
	if tokenMeta.TwitterURL != nil && *tokenMeta.TwitterURL != "" {
		message.WriteString(fmt.Sprintf("X: <a href=\"%s\">link</a>", *tokenMeta.TwitterURL))
	} else {
		message.WriteString("X: null")
	}

	message.WriteString("</blockquote>")

	return message.String()
}

// RunHotTokenMonitor checks ALL tokens from recent swaps and sends notifications
// bot - Telegram bot for sending notifications
// client - Flashnet API client
// filteredChatID - Chat ID for sending notifications (FILTERED_CHAT_ID)
// swapsCount - Minimum number of swaps required for hot token
// minAddresses - Minimum number of unique addresses required
// checkInterval - Interval between checks in seconds
func RunHotTokenMonitor(bot *tgbotapi.BotAPI, client *flashnet.Client, filteredChatID string, swapsCount int, minAddresses int, checkInterval int) {
	log.LogInfo("Starting Hot Token Monitor...",
		zap.String("filteredChatID", filteredChatID),
		zap.Int("swapsCount", swapsCount),
		zap.Int("minAddresses", minAddresses),
		zap.Int("checkInterval", checkInterval),
		zap.String("note", "Checking ALL tokens from recent swaps"))

	sentNotifications := make(map[string]time.Time)
	notificationCooldown := 1 * time.Hour // Cooldown between notifications for the same token

	ticker := time.NewTicker(time.Duration(checkInterval) * time.Second)
	defer ticker.Stop()

	// Initial check
	checkHotTokens(bot, client, filteredChatID, swapsCount, minAddresses, sentNotifications, notificationCooldown)

	// Periodic checks
	for range ticker.C {
		checkHotTokens(bot, client, filteredChatID, swapsCount, minAddresses, sentNotifications, notificationCooldown)
	}
}

// checkHotTokens checks ALL tokens from recent swaps and sends notifications for hot tokens
func checkHotTokens(bot *tgbotapi.BotAPI, client *flashnet.Client, filteredChatID string,
	swapsCount int, minAddresses int,
	sentNotifications map[string]time.Time, cooldown time.Duration) {

	// Get all unique pools from recent swaps AND the swaps themselves
	// This way we only make ONE API request instead of one per pool
	uniquePools, swaps, err := hot_token.GetAllUniquePoolsFromSwaps(client, swapsCount)
	if err != nil {
		log.LogWarn("Failed to get unique pools from swaps", zap.Error(err))
		return
	}

	if len(uniquePools) == 0 {
		log.LogDebug("No pools found in recent swaps")
		return
	}

	log.LogDebug("Checking hot token conditions",
		zap.Int("totalPools", len(uniquePools)),
		zap.Int("totalSwaps", len(swaps)),
		zap.Int("swapsCount", swapsCount),
		zap.Int("minAddresses", minAddresses))

	// Check each pool for hot token conditions using already fetched swaps
	// This avoids making additional API requests for each pool
	for _, poolLpPublicKey := range uniquePools {
		isHot, uniqueCount := hot_token.CheckHotTokenConditionsFromSwaps(swaps, poolLpPublicKey, swapsCount, minAddresses)

		if !isHot {
			continue
		}

		// Check cooldown for this token
		lastSent, exists := sentNotifications[poolLpPublicKey]
		if exists && time.Since(lastSent) < cooldown {
			log.LogDebug("Hot token notification skipped (cooldown)",
				zap.String("poolLpPublicKey", poolLpPublicKey),
				zap.Time("lastSent", lastSent))
			continue
		}

		poolData, err := hot_token.GetFullPoolData(poolLpPublicKey)
		if err != nil {
			log.LogWarn("Failed to get full pool data",
				zap.String("poolLpPublicKey", poolLpPublicKey),
				zap.Error(err))
			continue
		}

		// Format message
		message := FormatHotTokenMessage(poolData)
		if message == "" {
			log.LogWarn("Failed to format hot token message",
				zap.String("poolLpPublicKey", poolLpPublicKey))
			continue
		}

		chatID := parseChatIDBig(filteredChatID)

		msg := tgbotapi.NewMessage(chatID, message)
		msg.ParseMode = tgbotapi.ModeHTML
		msg.DisableWebPagePreview = true

		if _, err := bot.Send(msg); err != nil {
			log.LogError("Failed to send hot token notification",
				zap.String("poolLpPublicKey", poolLpPublicKey),
				zap.Error(err))
			continue
		}

		sentNotifications[poolLpPublicKey] = time.Now()

		log.LogInfo("Hot token notification sent",
			zap.String("poolLpPublicKey", poolLpPublicKey),
			zap.Int("uniqueAddresses", uniqueCount),
			zap.String("chatID", filteredChatID))
	}
}

// SendTestHotTokenNotification
func SendTestHotTokenNotification(bot *tgbotapi.BotAPI, filteredChatID string, poolLpPublicKey string) error {
	poolData, err := hot_token.GetFullPoolData(poolLpPublicKey)
	if err != nil {
		return fmt.Errorf("failed to get full pool data: %w", err)
	}

	// Format
	message := FormatHotTokenMessage(poolData)
	if message == "" {
		return fmt.Errorf("failed to format hot token message")
	}

	chatID := parseChatIDBig(filteredChatID)

	msg := tgbotapi.NewMessage(chatID, message)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true

	if _, err := bot.Send(msg); err != nil {
		return fmt.Errorf("failed to send test hot token notification: %w", err)
	}

	log.LogInfo("Test hot token notification sent",
		zap.String("poolLpPublicKey", poolLpPublicKey),
		zap.String("chatID", filteredChatID))

	return nil
}
