package bot

import (
	"fmt"
	"spark-wallet/http_client/system_works"
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
func FormatHotTokenMessage(poolData *system_works.LuminexFullPoolResponse) string {
	// token BTC)
	var tokenMeta *system_works.LuminexFullTokenMetadata
	var marketcap float64

	if poolData.AssetBAddress == system_works.NativeTokenAddress {
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
	marketcapStr := system_works.FormatUSDValue(marketcap)

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

// RunHotTokenMonitor tokens
// bot - Telegram for
// client - for Flashnet API
// filteredChatID - ID for
// swapsCount - count for
func RunHotTokenMonitor(bot *tgbotapi.BotAPI, client *system_works.Client, filteredChatID string, swapsCount int, minAddresses int, checkInterval int) {
	system_works.LogInfo("Starting Hot Token Monitor...",
		zap.String("filteredChatID", filteredChatID),
		zap.Int("swapsCount", swapsCount),
		zap.Int("minAddresses", minAddresses),
		zap.Int("checkInterval", checkInterval))

	sentNotifications := make(map[string]time.Time)
	notificationCooldown := 1 * time.Hour // in

	ticker := time.NewTicker(time.Duration(checkInterval) * time.Second)
	defer ticker.Stop()

	// Load tokens
	filteredTokens, err := system_works.LoadFilteredTokens()
	if err != nil {
		system_works.LogError("Failed to load filtered tokens", zap.Error(err))
		return
	}

	system_works.LogInfo("Loaded filtered tokens for hot token monitor",
		zap.Int("count", len(filteredTokens)))

	checkHotTokens(bot, client, filteredChatID, swapsCount, minAddresses, filteredTokens, sentNotifications, notificationCooldown)

	for range ticker.C {
		reloadedTokens, err := system_works.LoadFilteredTokens()
		if err != nil {
			system_works.LogWarn("Failed to reload filtered tokens, using previous list", zap.Error(err))
		} else {
			filteredTokens = reloadedTokens
			if len(filteredTokens) > 0 {
				system_works.LogDebug("Reloaded filtered tokens", zap.Int("count", len(filteredTokens)))
			}
		}
		checkHotTokens(bot, client, filteredChatID, swapsCount, minAddresses, filteredTokens, sentNotifications, notificationCooldown)
	}
}

// checkHotTokens for tokens
func checkHotTokens(bot *tgbotapi.BotAPI, client *system_works.Client, filteredChatID string,
	swapsCount int, minAddresses int, filteredTokens []string,
	sentNotifications map[string]time.Time, cooldown time.Duration) {

	for _, poolLpPublicKey := range filteredTokens {
		isHot, uniqueCount, err := system_works.CheckHotTokenConditions(client, poolLpPublicKey, swapsCount, minAddresses)
		if err != nil {
			system_works.LogWarn("Failed to check hot token conditions",
				zap.String("poolLpPublicKey", poolLpPublicKey),
				zap.Error(err))
			continue
		}

		if !isHot {
			continue
		}

		// Check, for token
		lastSent, exists := sentNotifications[poolLpPublicKey]
		if exists && time.Since(lastSent) < cooldown {
			system_works.LogDebug("Hot token notification skipped (cooldown)",
				zap.String("poolLpPublicKey", poolLpPublicKey),
				zap.Time("lastSent", lastSent))
			continue
		}

		poolData, err := system_works.GetFullPoolData(poolLpPublicKey)
		if err != nil {
			system_works.LogWarn("Failed to get full pool data",
				zap.String("poolLpPublicKey", poolLpPublicKey),
				zap.Error(err))
			continue
		}

		// Format
		message := FormatHotTokenMessage(poolData)
		if message == "" {
			system_works.LogWarn("Failed to format hot token message",
				zap.String("poolLpPublicKey", poolLpPublicKey))
			continue
		}

		chatID := parseChatIDBig(filteredChatID)

		msg := tgbotapi.NewMessage(chatID, message)
		msg.ParseMode = tgbotapi.ModeHTML
		msg.DisableWebPagePreview = true

		if _, err := bot.Send(msg); err != nil {
			system_works.LogError("Failed to send hot token notification",
				zap.String("poolLpPublicKey", poolLpPublicKey),
				zap.Error(err))
			continue
		}

		sentNotifications[poolLpPublicKey] = time.Now()

		system_works.LogInfo("Hot token notification sent",
			zap.String("poolLpPublicKey", poolLpPublicKey),
			zap.Int("uniqueAddresses", uniqueCount),
			zap.String("chatID", filteredChatID))
	}
}

// SendTestHotTokenNotification
func SendTestHotTokenNotification(bot *tgbotapi.BotAPI, filteredChatID string, poolLpPublicKey string) error {
	poolData, err := system_works.GetFullPoolData(poolLpPublicKey)
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

	system_works.LogInfo("Test hot token notification sent",
		zap.String("poolLpPublicKey", poolLpPublicKey),
		zap.String("chatID", filteredChatID))

	return nil
}
