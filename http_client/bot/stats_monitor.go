package bot

// Package bot contains and in Telegram

import (
	"fmt"
	"os"
	"strings"
	"time"

	"spark-wallet/http_client/system_works"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

// RunStatsMonitor in time by
// bot - Telegram for
// filteredChatID - ID for
// sendTime - time in "HH:MM" "10:00")
func RunStatsMonitor(bot *tgbotapi.BotAPI, filteredChatID string, sendTime string) {
	if bot == nil {
		system_works.LogWarn("Bot is nil, stats monitor not started")
		return
	}

	if filteredChatID == "" {
		system_works.LogWarn("Filtered chat ID is empty, stats monitor not started")
		return
	}

	system_works.LogInfo("Starting Stats Monitor...", zap.String("filteredChatID", filteredChatID))

	// Load
	moscowLocation, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		system_works.LogError("Failed to load Moscow timezone, using UTC", zap.Error(err))
		moscowLocation = time.UTC
	}

	sendStats := func(check bool) {
		// Get from API
		stats, err := system_works.GetStats()
		if err != nil {
			system_works.LogError("Failed to get stats", zap.Error(err))
			return
		}

		// Save data in stats.json check
		if err := system_works.SaveStatsData(stats, check); err != nil {
			system_works.LogError("Failed to save stats data", zap.Error(err))
		}

		statsMessage, err := formatStatsMessage(stats)
		if err != nil {
			system_works.LogError("Failed to format stats message", zap.Error(err))
			return // if error
		}

		// create "Trade on Luminex"
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL("Trade on Luminex", "https://luminex.io/"),
			),
		)

		chartPath, err := system_works.GenerateVolumeChart()
		if err != nil {
			system_works.LogWarn("Failed to generate volume chart", zap.Error(err))
			msg := tgbotapi.NewMessage(parseChatIDBig(filteredChatID), statsMessage)
			msg.ParseMode = tgbotapi.ModeHTML
			msg.ReplyMarkup = keyboard
			_, err = bot.Send(msg)
			if err != nil {
				system_works.LogError("Failed to send stats message", zap.Error(err))
				return
			}
		} else {
			// Check, file
			if _, err := os.Stat(chartPath); os.IsNotExist(err) {
				system_works.LogError("Chart file does not exist", zap.String("chartPath", chartPath), zap.Error(err))
				msg := tgbotapi.NewMessage(parseChatIDBig(filteredChatID), statsMessage)
				msg.ParseMode = tgbotapi.ModeHTML
				msg.ReplyMarkup = keyboard
				bot.Send(msg)
				return
			}

			photo := tgbotapi.NewPhoto(parseChatIDBig(filteredChatID), tgbotapi.FilePath(chartPath))
			photo.Caption = statsMessage
			photo.ParseMode = tgbotapi.ModeHTML
			photo.ReplyMarkup = keyboard

			_, err = bot.Send(photo)
			if err != nil {
				system_works.LogError("Failed to send stats chart", zap.Error(err))
				msg := tgbotapi.NewMessage(parseChatIDBig(filteredChatID), statsMessage)
				msg.ParseMode = tgbotapi.ModeHTML
				msg.ReplyMarkup = keyboard
				bot.Send(msg)
				return
			}
		}

		system_works.LogInfo("Stats sent successfully",
			zap.String("chatID", filteredChatID),
			zap.Bool("check", check),
			zap.Float64("tvl", stats.TotalTVLUSD),
			zap.Float64("volume24h", stats.TotalVolume24HUSD))
	}

	// Parse time "HH:MM")
	timeParts := strings.Split(sendTime, ":")
	if len(timeParts) != 2 {
		system_works.LogWarn("Invalid send time format, using default 10:00", zap.String("sendTime", sendTime))
		sendTime = "10:00"
		timeParts = []string{"10", "00"}
	}

	var hour, minute int
	fmt.Sscanf(timeParts[0], "%d", &hour)
	fmt.Sscanf(timeParts[1], "%d", &minute)

	now := time.Now().In(moscowLocation)
	nextSend := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, moscowLocation)

	// If 10:00 on
	if now.After(nextSend) || now.Equal(nextSend) {
		nextSend = nextSend.Add(24 * time.Hour)
	}

	// Calculate
	delay := nextSend.Sub(now)
	system_works.LogInfo("Stats monitor scheduled",
		zap.Time("nextSend", nextSend),
		zap.Duration("delay", delay))

	firstTimer := time.NewTimer(delay)
	go func() {
		<-firstTimer.C
		sendStats(true) // check = true for

		// create ticker on 24
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			sendStats(true) // check = true for
		}
	}()

	system_works.LogInfo("Stats monitor started successfully",
		zap.String("sendTime", sendTime),
		zap.Time("nextSend", nextSend),
		zap.Duration("delay", delay))
}

// CheckAndSendStatsOnStartup and if -
// bot - Telegram for
// filteredChatID - ID for
func CheckAndSendStatsOnStartup(bot *tgbotapi.BotAPI, filteredChatID string) {
	if bot == nil {
		system_works.LogWarn("Bot is nil, skipping stats check on startup")
		return
	}

	if filteredChatID == "" {
		system_works.LogWarn("Filtered chat ID is empty, skipping stats check on startup")
		return
	}

	// Check,
	checked, err := system_works.IsStatsCheckedToday()
	if err != nil {
		system_works.LogWarn("Failed to check if stats were checked today", zap.Error(err))
	}

	if checked {
		system_works.LogInfo("Stats already checked today, skipping startup send")
		return
	}

	system_works.LogInfo("Stats not checked today, sending on startup")

	// Get from API
	stats, err := system_works.GetStats()
	if err != nil {
		system_works.LogError("Failed to get stats on startup", zap.Error(err))
		return
	}

	// Save data in stats.json check = true
	if err := system_works.SaveStatsData(stats, true); err != nil {
		system_works.LogError("Failed to save stats data on startup", zap.Error(err))
	}

	statsMessage, err := formatStatsMessage(stats)
	if err != nil {
		system_works.LogError("Failed to format stats message on startup", zap.Error(err))
		return // if error
	}

	// create "Trade on Luminex"
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("Trade on Luminex", "https://luminex.io/"),
		),
	)

	chartPath, err := system_works.GenerateVolumeChart()
	if err != nil {
		system_works.LogWarn("Failed to generate volume chart on startup", zap.Error(err))
		msg := tgbotapi.NewMessage(parseChatIDBig(filteredChatID), statsMessage)
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = keyboard
		_, err = bot.Send(msg)
		if err != nil {
			system_works.LogError("Failed to send stats message on startup", zap.Error(err))
			return
		}
	} else {
		// Check, file
		if _, err := os.Stat(chartPath); os.IsNotExist(err) {
			system_works.LogError("Chart file does not exist on startup", zap.String("chartPath", chartPath), zap.Error(err))
			msg := tgbotapi.NewMessage(parseChatIDBig(filteredChatID), statsMessage)
			msg.ParseMode = tgbotapi.ModeHTML
			msg.ReplyMarkup = keyboard
			bot.Send(msg)
			return
		}

		photo := tgbotapi.NewPhoto(parseChatIDBig(filteredChatID), tgbotapi.FilePath(chartPath))
		photo.Caption = statsMessage
		photo.ParseMode = tgbotapi.ModeHTML
		photo.ReplyMarkup = keyboard

		_, err = bot.Send(photo)
		if err != nil {
			system_works.LogError("Failed to send stats chart on startup", zap.Error(err))
			msg := tgbotapi.NewMessage(parseChatIDBig(filteredChatID), statsMessage)
			msg.ParseMode = tgbotapi.ModeHTML
			msg.ReplyMarkup = keyboard
			bot.Send(msg)
			return
		}
	}

	system_works.LogInfo("Stats sent successfully on startup",
		zap.String("chatID", filteredChatID),
		zap.Float64("tvl", stats.TotalTVLUSD),
		zap.Float64("volume24h", stats.TotalVolume24HUSD))
}
