package bot

// Package bot contains Telegram

import (
	"fmt"
	"os"
	"strings"
	"time"

	"spark-wallet/internal/clients_api/flashnet"
	"spark-wallet/internal/clients_api/luminex"
	"spark-wallet/internal/features/holders"
	"spark-wallet/internal/features/tg_charts"
	storage "spark-wallet/internal/infra/fs"
	log "spark-wallet/internal/infra/log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

// RunCommandHandler for Telegram
// filteredChatID - ID (filtered_chat_id)
// client - Flashnet API for first buy
func RunCommandHandler(bot *tgbotapi.BotAPI, filteredChatID string, client *flashnet.Client) {
	if bot == nil {
		log.LogWarn("Bot is nil, command handler not started")
		return
	}

	if filteredChatID == "" {
		log.LogWarn("Filtered chat ID is empty, command handler not started")
		return
	}

	log.LogInfo("Starting command handler", zap.String("filteredChatID", filteredChatID))

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		// Check, from
		chatID := update.Message.Chat.ID
		chatIDStr := formatChatID(chatID)
		// and (on
		expectedChatID := parseChatIDBig(filteredChatID)
		if chatID != expectedChatID && chatIDStr != filteredChatID {
			continue
		}

		if update.Message.IsCommand() {
			command := update.Message.Command()
			args := update.Message.CommandArguments()

			log.LogDebug("Received command",
				zap.String("command", command),
				zap.String("args", args),
				zap.String("chatID", chatIDStr),
				zap.String("username", update.Message.From.UserName))

			// /flashadd {token}
			// /flashadd SOON or /flashadd@botname SOON
			if command == "flashadd" {
				// Parse "SOON" -> ticker
				ticker := strings.TrimSpace(args)
				if ticker == "" {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID,
						"Usage: /flashadd {ticker}\n\nExample: /flashadd SOON")
					msg.ReplyToMessageID = update.Message.MessageID
					bot.Send(msg)
				} else {
					handleAddTokenCommand(bot, update.Message, ticker)
				}
			}

			// /flashdel {token}
			// /flashdel SOON or /flashdel@botname SOON
			if command == "flashdel" {
				// Parse "SOON" -> ticker
				ticker := strings.TrimSpace(args)
				if ticker == "" {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID,
						"Usage: /flashdel {ticker}\n\nExample: /flashdel SOON")
					msg.ReplyToMessageID = update.Message.MessageID
					bot.Send(msg)
				} else {
					handleDeleteTokenCommand(bot, update.Message, ticker)
				}
			}

			// /flash {ticker} {date}
			// /flash SOON 0812 or /flash@botname SOON 0812
			if command == "flash" {
				// Parse "SOON 0812" -> ticker and date
				parts := strings.Fields(args)
				if len(parts) < 2 {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID,
						"Usage: /flash {ticker} {date}\n\nExample: /flash SOON 0812\n\nDate format: DDMM (e.g., 0812 for December 8)")
					msg.ReplyToMessageID = update.Message.MessageID
					bot.Send(msg)
				} else {
					ticker := strings.TrimSpace(parts[0])
					dateStr := strings.TrimSpace(parts[1])
					handleFlashReportCommand(bot, update.Message, ticker, dateStr, client)
				}
			}

			// /flow {ticker} {date}
			// /flow SOON 0912 or /flow@botname SOON 0912
			if command == "flow" {
				// Parse "SOON 0912" -> ticker and date
				parts := strings.Fields(args)
				if len(parts) < 2 {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID,
						"Usage: /flow {ticker} {date}\n\nExample: /flow SOON 0912\n\nDate format: DDMM (e.g., 0912 for December 9)")
					msg.ReplyToMessageID = update.Message.MessageID
					bot.Send(msg)
				} else {
					ticker := strings.TrimSpace(parts[0])
					dateStr := strings.TrimSpace(parts[1])
					handleFlowReportCommand(bot, update.Message, ticker, dateStr)
				}
			}

			// /stats or /charts
			// /stats, /charts or /stats@botname, /charts@botname
			if command == "stats" || command == "charts" {
				handleStatsCommand(bot, update.Message)
			}

			// /helps
			// /helps or /helps@botname
			if command == "helps" {
				handleHelpCommand(bot, update.Message)
			}

			// /spark
			// /spark or /spark@botname
			if command == "spark" {
				handleSparkCommand(bot, update.Message)
			}
		}
	}
}

// handleHelpCommand /helps
func handleHelpCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	helpText := "" +
		"Commands:\n" +
		"• <code>/flashadd {ticker}</code> - добавляет токен в big sales\n" +
		"• <code>/flashdel {ticker}</code> - удаляет токен из big sales\n" +
		"• <code>/flash {ticker} {date}</code> - движение холдеров в токене\n" +
		"• <code>/flow {ticker} {date}</code> - отчет о коэффициенте покупок/продаж\n" +
		"• <code>/stats</code> - общая статистика по рынку spark\n" +
		"• <code>/spark</code> - график резервов btc в spark\n" +
		"\n" +
		"<a href=\"https:// t.me/+5jHhbz8ZlDIyNWZi\">Big sales</a> / flashnet"

	// Try multiple paths for asty1.jpeg (same as spark.png in stats_chart.go)
	photoPaths := []string{
		"etc/telegram/asty1.jpeg",
		"./etc/telegram/asty1.jpeg",
		"../etc/telegram/asty1.jpeg",
		"../../etc/telegram/asty1.jpeg",
	}

	var photoPath string
	var photoFound bool
	for _, path := range photoPaths {
		if _, err := os.Stat(path); err == nil {
			photoPath = path
			photoFound = true
			log.LogDebug("Found asty1.jpeg", zap.String("path", photoPath))
			break
		}
	}

	if !photoFound {
		log.LogError("Failed to find asty1.jpeg in any expected location",
			zap.Strings("tried_paths", photoPaths))
		// Send text message instead of photo
		msg := tgbotapi.NewMessage(message.Chat.ID, helpText)
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyToMessageID = message.MessageID
		_, err := bot.Send(msg)
		if err != nil {
			log.LogError("Failed to send /helps text message", zap.Error(err))
		}
		return
	}

	// + help in caption
	photo := tgbotapi.NewPhoto(message.Chat.ID, tgbotapi.FilePath(photoPath))
	photo.Caption = helpText
	photo.ParseMode = tgbotapi.ModeHTML
	photo.ReplyToMessageID = message.MessageID
	_, err := bot.Send(photo)
	if err != nil {
		log.LogError("Failed to send /helps photo message",
			zap.String("photoPath", photoPath),
			zap.Error(err))
		// Fallback: send text message if photo fails
		msg := tgbotapi.NewMessage(message.Chat.ID, helpText)
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyToMessageID = message.MessageID
		bot.Send(msg)
		return
	}

	log.LogInfo("Help message sent",
		zap.String("chatID", formatChatID(message.Chat.ID)),
		zap.String("username", message.From.UserName))
}

// handleAddTokenCommand /flashadd {token}
func handleAddTokenCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, ticker string) {

	// poolLpPublicKey by ticker in saved_ticket.json
	poolLpPublicKey, err := storage.FindPoolLpPublicKeyByTicker(ticker)
	if err != nil {
		log.LogWarn("Failed to find token by ticker",
			zap.String("ticker", ticker),
			zap.Error(err))

		// - ticker
		msg := tgbotapi.NewMessage(message.Chat.ID,
			fmt.Sprintf("Ticker {%s} cannot be added at this time", ticker))
		msg.ReplyToMessageID = message.MessageID
		_, err := bot.Send(msg)
		if err != nil {
			log.LogError("Failed to send error message", zap.Error(err))
		}
		return
	}

	// Check, token in
	existingTokens, err := storage.LoadFilteredTokens()
	if err == nil {
		for _, existingToken := range existingTokens {
			if strings.TrimSpace(existingToken) == poolLpPublicKey {
				msg := tgbotapi.NewMessage(message.Chat.ID,
					fmt.Sprintf("Ticker {%s} already exists in the list", ticker))
				msg.ReplyToMessageID = message.MessageID
				_, err := bot.Send(msg)
				if err != nil {
					log.LogError("Failed to send message", zap.Error(err))
				}
				log.LogDebug("Token already in filtered list",
					zap.String("ticker", ticker),
					zap.String("poolLpPublicKey", poolLpPublicKey))
				return
			}
		}
	}

	err = storage.AddFilteredToken(poolLpPublicKey)
	if err != nil {
		log.LogError("Failed to add filtered token",
			zap.String("ticker", ticker),
			zap.String("poolLpPublicKey", poolLpPublicKey),
			zap.Error(err))

		msg := tgbotapi.NewMessage(message.Chat.ID,
			"An error occurred, please try again later")
		msg.ReplyToMessageID = message.MessageID
		_, err := bot.Send(msg)
		if err != nil {
			log.LogError("Failed to send error message", zap.Error(err))
		}
		return
	}

	// Check, token
	verifyTokens, err := storage.LoadFilteredTokens()
	if err == nil {
		found := false
		for _, token := range verifyTokens {
			if strings.TrimSpace(token) == poolLpPublicKey {
				found = true
				break
			}
		}
		if !found {
			log.LogError("Token was not saved to file after AddFilteredToken",
				zap.String("ticker", ticker),
				zap.String("poolLpPublicKey", poolLpPublicKey))
			msg := tgbotapi.NewMessage(message.Chat.ID,
				"An error occurred, please try again later")
			msg.ReplyToMessageID = message.MessageID
			bot.Send(msg)
			return
		}
	}

	msg := tgbotapi.NewMessage(message.Chat.ID,
		fmt.Sprintf("Ticker {%s} successfully added to the list", ticker))
	msg.ReplyToMessageID = message.MessageID
	_, err = bot.Send(msg)
	if err != nil {
		log.LogError("Failed to send success message", zap.Error(err))
	}

	log.LogInfo("Token added to filtered list via command",
		zap.String("ticker", ticker),
		zap.String("poolLpPublicKey", poolLpPublicKey),
		zap.String("chatID", formatChatID(message.Chat.ID)),
		zap.String("username", message.From.UserName))
}

// handleDeleteTokenCommand /flashdel {token}
func handleDeleteTokenCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, ticker string) {
	// poolLpPublicKey by ticker in saved_ticket.json
	poolLpPublicKey, err := storage.FindPoolLpPublicKeyByTicker(ticker)
	if err != nil {
		log.LogWarn("Failed to find token by ticker",
			zap.String("ticker", ticker),
			zap.Error(err))

		// - ticker
		msg := tgbotapi.NewMessage(message.Chat.ID,
			fmt.Sprintf("Ticker {%s} cannot be removed at this time", ticker))
		msg.ReplyToMessageID = message.MessageID
		_, err := bot.Send(msg)
		if err != nil {
			log.LogError("Failed to send error message", zap.Error(err))
		}
		return
	}

	err = storage.RemoveFilteredToken(poolLpPublicKey)
	if err != nil {
		if err.Error() == "token not found in list" {
			msg := tgbotapi.NewMessage(message.Chat.ID,
				fmt.Sprintf("Ticker {%s} is not in the list", ticker))
			msg.ReplyToMessageID = message.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.LogError("Failed to send message", zap.Error(err))
			}
			log.LogDebug("Token not found in filtered list",
				zap.String("ticker", ticker),
				zap.String("poolLpPublicKey", poolLpPublicKey))
		} else {
			log.LogError("Failed to remove filtered token",
				zap.String("ticker", ticker),
				zap.String("poolLpPublicKey", poolLpPublicKey),
				zap.Error(err))

			msg := tgbotapi.NewMessage(message.Chat.ID,
				"An error occurred, please try again later")
			msg.ReplyToMessageID = message.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.LogError("Failed to send error message", zap.Error(err))
			}
		}
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID,
		fmt.Sprintf("Ticker {%s} successfully removed from the list", ticker))
	msg.ReplyToMessageID = message.MessageID
	_, err = bot.Send(msg)
	if err != nil {
		log.LogError("Failed to send success message", zap.Error(err))
	}

	log.LogInfo("Token removed from filtered list via command",
		zap.String("ticker", ticker),
		zap.String("poolLpPublicKey", poolLpPublicKey),
		zap.String("chatID", formatChatID(message.Chat.ID)),
		zap.String("username", message.From.UserName))
}

// handleFlashReportCommand /flash {ticker} {date}
func handleFlashReportCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, ticker string, dateStr string, client *flashnet.Client) {
	// Generate
	report, err := holders.GenerateHoldersReport(ticker, dateStr, client)
	if err != nil {
		log.LogError("Failed to generate holders report",
			zap.String("ticker", ticker),
			zap.String("dateStr", dateStr),
			zap.Error(err))

		msg := tgbotapi.NewMessage(message.Chat.ID,
			fmt.Sprintf("Failed to generate report: %s", err.Error()))
		msg.ReplyToMessageID = message.MessageID
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, report)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true
	msg.ReplyToMessageID = message.MessageID
	_, err = bot.Send(msg)
	if err != nil {
		log.LogError("Failed to send report message", zap.Error(err))
	}

	log.LogInfo("Holders report generated and sent",
		zap.String("ticker", ticker),
		zap.String("dateStr", dateStr),
		zap.String("chatID", formatChatID(message.Chat.ID)),
		zap.String("username", message.From.UserName))
}

// handleFlowReportCommand /flow {ticker} {date}
func handleFlowReportCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, ticker string, dateStr string) {
	// Generate
	report, err := holders.GenerateFlowReport(ticker, dateStr)
	if err != nil {
		log.LogError("Failed to generate flow report",
			zap.String("ticker", ticker),
			zap.String("dateStr", dateStr),
			zap.Error(err))

		msg := tgbotapi.NewMessage(message.Chat.ID,
			fmt.Sprintf("Failed to generate flow report: %s", err.Error()))
		msg.ReplyToMessageID = message.MessageID
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, report)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyToMessageID = message.MessageID
	_, err = bot.Send(msg)
	if err != nil {
		log.LogError("Failed to send flow report", zap.Error(err))
		return
	}

	log.LogInfo("Flow report sent via command",
		zap.String("ticker", ticker),
		zap.String("dateStr", dateStr),
		zap.String("chatID", formatChatID(message.Chat.ID)),
		zap.String("username", message.From.UserName))
}

// handleStatsCommand /stats
func handleStatsCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	// Get from API
	stats, err := luminex.GetStats()
	if err != nil {
		log.LogError("Failed to get stats",
			zap.Error(err))

		msg := tgbotapi.NewMessage(message.Chat.ID,
			fmt.Sprintf("Failed to get stats: %s", err.Error()))
		bot.Send(msg)
		return
	}

	// Check, (check = true)
	// If - save check = true, check = false
	checked, err := luminex.IsStatsCheckedToday()
	if err != nil {
		log.LogWarn("Failed to check if stats were checked today", zap.Error(err))
		// in save check = false
		checked = true // in true, save false
	}

	// If (check = true), save check = false
	// If save check = true
	checkFlag := !checked

	// Save data in stats.json
	if err := luminex.SaveStatsData(stats, checkFlag); err != nil {
		log.LogWarn("Failed to save stats data", zap.Error(err))
	}

	// tokens)
	// If error tokens -
	statsMessage, err := formatStatsMessage(stats)
	if err != nil {
		log.LogError("Failed to format stats message", zap.Error(err))
		return // if error
	}

	chartPath, err := tg_charts.GenerateVolumeChart()
	if err != nil {
		log.LogWarn("Failed to generate volume chart", zap.Error(err))
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL("Trade on Luminex", "https://luminex.io/"),
			),
		)

		msg := tgbotapi.NewMessage(message.Chat.ID, statsMessage)
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = keyboard
		_, err = bot.Send(msg)
		if err != nil {
			log.LogError("Failed to send stats message", zap.Error(err))
			return
		}
	} else {
		// Check, file
		if _, err := os.Stat(chartPath); os.IsNotExist(err) {
			log.LogError("Chart file does not exist", zap.String("chartPath", chartPath), zap.Error(err))
			keyboard := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonURL("Trade on Luminex", "https://luminex.io/"),
				),
			)
			msg := tgbotapi.NewMessage(message.Chat.ID, statsMessage)
			msg.ParseMode = tgbotapi.ModeHTML
			msg.ReplyMarkup = keyboard
			bot.Send(msg)
			return
		}

		log.LogInfo("Sending stats chart", zap.String("chartPath", chartPath))

		photo := tgbotapi.NewPhoto(message.Chat.ID, tgbotapi.FilePath(chartPath))
		photo.Caption = statsMessage
		photo.ParseMode = tgbotapi.ModeHTML

		// create "Trade on Luminex"
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL("Trade on Luminex", "https://luminex.io/"),
			),
		)
		photo.ReplyMarkup = keyboard

		_, err = bot.Send(photo)
		if err != nil {
			log.LogError("Failed to send stats chart", zap.String("chartPath", chartPath), zap.Error(err))
			msg := tgbotapi.NewMessage(message.Chat.ID, statsMessage)
			msg.ParseMode = tgbotapi.ModeHTML
			msg.ReplyMarkup = keyboard
			bot.Send(msg)
			return
		}
	}

	log.LogInfo("Stats sent via command",
		zap.String("chatID", formatChatID(message.Chat.ID)),
		zap.String("username", message.From.UserName))
}

// handleSparkCommand /spark
func handleSparkCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	// Get BTC from API
	btcReserve, err := luminex.GetBTCSparkReserve()
	if err != nil {
		log.LogError("Failed to get BTC spark reserve",
			zap.Error(err))

		msg := tgbotapi.NewMessage(message.Chat.ID,
			fmt.Sprintf("Failed to get BTC reserve: %s", err.Error()))
		bot.Send(msg)
		return
	}

	// Check, (check = true)
	// If - save check = true, check = false
	checked, err := storage.IsBTCSparkCheckedToday()
	if err != nil {
		log.LogWarn("Failed to check if BTC spark was checked today", zap.Error(err))
		// in save check = false
		checked = true // in true, save false
	}

	// If (check = true), save check = false
	// If save check = true
	checkFlag := !checked

	// Save data in btc_spark.json
	if err := storage.SaveBTCSparkData(btcReserve, checkFlag); err != nil {
		log.LogWarn("Failed to save BTC spark data", zap.Error(err))
	}

	// BTC
	sparkMessage := formatSparkMessage(btcReserve)

	// Generate
	chartPath, err := tg_charts.GenerateBTCSparkChart()
	if err != nil {
		log.LogWarn("Failed to generate BTC spark chart", zap.Error(err))
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL("Trade on Luminex", "https://luminex.io/"),
			),
		)

		msg := tgbotapi.NewMessage(message.Chat.ID, sparkMessage)
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = keyboard
		_, err = bot.Send(msg)
		if err != nil {
			log.LogError("Failed to send spark message", zap.Error(err))
			return
		}
		return
	}

	// Check, file and
	fileInfo, err := os.Stat(chartPath)
	if err != nil || os.IsNotExist(err) {
		log.LogError("Chart file does not exist", zap.String("chartPath", chartPath), zap.Error(err))
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL("Trade on Luminex", "https://luminex.io/"),
			),
		)
		msg := tgbotapi.NewMessage(message.Chat.ID, sparkMessage)
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = keyboard
		bot.Send(msg)
		return
	}

	if fileInfo.Size() == 0 {
		log.LogError("Chart file is empty", zap.String("chartPath", chartPath))
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL("Trade on Luminex", "https://luminex.io/"),
			),
		)
		msg := tgbotapi.NewMessage(message.Chat.ID, sparkMessage)
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = keyboard
		bot.Send(msg)
		return
	}

	log.LogInfo("Sending BTC spark chart",
		zap.String("chartPath", chartPath),
		zap.Int64("fileSize", fileInfo.Size()))

	photo := tgbotapi.NewPhoto(message.Chat.ID, tgbotapi.FilePath(chartPath))

	// create "Trade on Luminex"
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("Trade on Luminex", "https://luminex.io/"),
		),
	)
	photo.ReplyMarkup = keyboard

	_, err = bot.Send(photo)
	if err != nil {
		log.LogError("Failed to send BTC spark chart", zap.String("chartPath", chartPath), zap.Error(err))
		msg := tgbotapi.NewMessage(message.Chat.ID, sparkMessage)
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = keyboard
		bot.Send(msg)
		return
	}

	log.LogInfo("BTC spark sent via command",
		zap.String("chatID", formatChatID(message.Chat.ID)),
		zap.String("username", message.From.UserName),
		zap.Float64("btcReserve", btcReserve))
}

// formatSparkMessage BTC
func formatSparkMessage(btcReserve float64) string {
	// Get in moscow time
	moscowLocation, _ := time.LoadLocation("Europe/Moscow")
	currentTime := time.Now().In(moscowLocation)
	dateStr := currentTime.Format("02 Jan")

	message := fmt.Sprintf("BTC Reserve on %s:\n\n", dateStr)
	message += fmt.Sprintf("<blockquote>BTC Reserve: <code>%.2f btc</code></blockquote>", btcReserve)

	return message
}

func formatStatsMessage(stats *luminex.StatsResponse) (string, error) {
	// Get in moscow time
	moscowLocation, _ := time.LoadLocation("Europe/Moscow")
	currentTime := time.Now().In(moscowLocation)
	dateStr := currentTime.Format("02 Jan")

	// Format (FormatUSDValue M/K
	tvlFormatted := luminex.FormatUSDValue(stats.TotalTVLUSD)
	volumeFormatted := luminex.FormatUSDValue(stats.TotalVolume24HUSD)

	message := fmt.Sprintf("Stats on %s:\n\n", dateStr)

	// Get -5 tokens - if error, return error
	topTokens, err := luminex.GetTopTokens(5)
	if err != nil {
		return "", fmt.Errorf("failed to get top tokens: %w", err)
	}

	var lines []string

	// Add
	lines = append(lines, fmt.Sprintf("TVL: <code>$%s</code>", tvlFormatted))
	lines = append(lines, fmt.Sprintf("Volume 24h: <code>$%s</code>", volumeFormatted))
	lines = append(lines, "")

	if len(topTokens) > 0 {
		lines = append(lines, "Top 5 tokens for 24 hours:")
		for i, token := range topTokens {
			marketCapFormatted := luminex.FormatUSDValue(token.MarketCapUSD)
			volumeFormatted := luminex.FormatUSDValue(token.Volume24HUSD)
			priceChangeStr := fmt.Sprintf("%.2f", token.PriceChange24H)
			if token.PriceChange24H > 0 {
				priceChangeStr = "+" + priceChangeStr
			}

			// – Volume –
			// – Price Change: -
			lines = append(lines, fmt.Sprintf("%d. <b>%s</b> (<code>$%s</code>):", i+1, token.Ticker, marketCapFormatted))
			lines = append(lines, fmt.Sprintf("– Volume – <code>$%s</code>", volumeFormatted))
			lines = append(lines, fmt.Sprintf("– Price Change: <i>%s%%</i>", priceChangeStr))

			if i < len(topTokens)-1 {
				lines = append(lines, "")
			}
		}
	}

	maxLength := 0
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Remove HTML for
		plainLine := strings.ReplaceAll(line, "<code>", "")
		plainLine = strings.ReplaceAll(plainLine, "</code>", "")
		plainLine = strings.ReplaceAll(plainLine, "<b>", "")
		plainLine = strings.ReplaceAll(plainLine, "</b>", "")
		plainLine = strings.ReplaceAll(plainLine, "<i>", "")
		plainLine = strings.ReplaceAll(plainLine, "</i>", "")

		if len(plainLine) > maxLength {
			maxLength = len(plainLine)
		}
	}

	// for for 2326px)
	// Add
	targetLength := maxLength + 50

	message += "<blockquote>"
	for _, line := range lines {
		if line == "" {
			message += "\n"
			continue
		}

		// Remove HTML for
		plainLine := strings.ReplaceAll(line, "<code>", "")
		plainLine = strings.ReplaceAll(plainLine, "</code>", "")
		plainLine = strings.ReplaceAll(plainLine, "<b>", "")
		plainLine = strings.ReplaceAll(plainLine, "</b>", "")
		plainLine = strings.ReplaceAll(plainLine, "<i>", "")
		plainLine = strings.ReplaceAll(plainLine, "</i>", "")

		spacesNeeded := targetLength - len(plainLine)
		if spacesNeeded > 0 {
			message += line + strings.Repeat(" ", spacesNeeded) + ".\n"
		} else {
			message += line + " .\n"
		}
	}
	message += "</blockquote>"

	// Check (Telegram for caption - 1024
	// If save
	if len(message) > 1024 {
		log.LogWarn("Stats message too long, reducing spaces",
			zap.Int("currentLength", len(message)),
			zap.Int("maxLength", 1024))

		targetLength = maxLength + 20

		message = fmt.Sprintf("Stats on %s:\n\n", dateStr)
		message += "<blockquote>"
		for _, line := range lines {
			if line == "" {
				message += "\n"
				continue
			}

			plainLine := strings.ReplaceAll(line, "<code>", "")
			plainLine = strings.ReplaceAll(plainLine, "</code>", "")
			plainLine = strings.ReplaceAll(plainLine, "<b>", "")
			plainLine = strings.ReplaceAll(plainLine, "</b>", "")
			plainLine = strings.ReplaceAll(plainLine, "<i>", "")
			plainLine = strings.ReplaceAll(plainLine, "</i>", "")

			spacesNeeded := targetLength - len(plainLine)
			if spacesNeeded > 0 {
				message += line + strings.Repeat(" ", spacesNeeded) + ".\n"
			} else {
				message += line + " .\n"
			}
		}
		message += "</blockquote>"
	}

	return message, nil
}

// formatChatID chat ID in (for
func formatChatID(chatID int64) string {
	return fmt.Sprintf("%d", chatID)
}
