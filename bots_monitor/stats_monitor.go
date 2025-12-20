package bot

// Package bot contains and in Telegram

import (
	"fmt"
	"os"
	"strings"
	"time"

	"spark-wallet/internal/clients_api/luminex"
	"spark-wallet/internal/features/tg_charts"
	log "spark-wallet/internal/infra/log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

// RunStatsMonitor in time by
// bot - Telegram for
// filteredChatID - ID for
// sendTime - time in "HH:MM" "10:00")
func RunStatsMonitor(bot *tgbotapi.BotAPI, filteredChatID string, sendTime string) {
	if bot == nil {
		log.LogWarn("Bot is nil, stats monitor not started")
		return
	}

	if filteredChatID == "" {
		log.LogWarn("Filtered chat ID is empty, stats monitor not started")
		return
	}

	log.LogInfo("Starting Stats Monitor...", zap.String("filteredChatID", filteredChatID))

	// Load
	moscowLocation, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		log.LogError("Failed to load Moscow timezone, using UTC", zap.Error(err))
		moscowLocation = time.UTC
	}

	sendStats := func(check bool) {
		// Get from API
		stats, err := luminex.GetStats()
		if err != nil {
			log.LogError("Failed to get stats", zap.Error(err))
			return
		}

		// Save data in stats.json check
		if err := luminex.SaveStatsData(stats, check); err != nil {
			log.LogError("Failed to save stats data", zap.Error(err))
		}

		statsMessage, err := formatStatsMessage(stats)
		if err != nil {
			log.LogError("Failed to format stats message", zap.Error(err))
			return // if error
		}

		// create "Trade on Luminex"
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL("Trade on Luminex", "https://luminex.io/"),
			),
		)

		chartPath, err := tg_charts.GenerateVolumeChart()
		if err != nil {
			log.LogWarn("Failed to generate volume chart", zap.Error(err))
			msg := tgbotapi.NewMessage(parseChatIDBig(filteredChatID), statsMessage)
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
				log.LogError("Failed to send stats chart", zap.Error(err))
				msg := tgbotapi.NewMessage(parseChatIDBig(filteredChatID), statsMessage)
				msg.ParseMode = tgbotapi.ModeHTML
				msg.ReplyMarkup = keyboard
				bot.Send(msg)
				return
			}
		}

		log.LogInfo("Stats sent successfully",
			zap.String("chatID", filteredChatID),
			zap.Bool("check", check),
			zap.Float64("tvl", stats.TotalTVLUSD),
			zap.Float64("volume24h", stats.TotalVolume24HUSD))
	}

	// Parse time "HH:MM")
	timeParts := strings.Split(sendTime, ":")
	if len(timeParts) != 2 {
		log.LogWarn("Invalid send time format, using default 10:00", zap.String("sendTime", sendTime))
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
	log.LogInfo("Stats monitor scheduled",
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

	log.LogInfo("Stats monitor started successfully",
		zap.String("sendTime", sendTime),
		zap.Time("nextSend", nextSend),
		zap.Duration("delay", delay))
}

// CheckAndSendStatsOnStartup and if -
// bot - Telegram for
// filteredChatID - ID for
func CheckAndSendStatsOnStartup(bot *tgbotapi.BotAPI, filteredChatID string) {
	if bot == nil {
		log.LogWarn("Bot is nil, skipping stats check on startup")
		return
	}

	if filteredChatID == "" {
		log.LogWarn("Filtered chat ID is empty, skipping stats check on startup")
		return
	}

	// Check,
	checked, err := luminex.IsStatsCheckedToday()
	if err != nil {
		log.LogWarn("Failed to check if stats were checked today", zap.Error(err))
	}

	if checked {
		log.LogInfo("Stats already checked today, skipping startup send")
		return
	}

	log.LogInfo("Stats not checked today, sending on startup")

	// Get from API
	stats, err := luminex.GetStats()
	if err != nil {
		log.LogError("Failed to get stats on startup", zap.Error(err))
		return
	}

	// Save data in stats.json check = true
	if err := luminex.SaveStatsData(stats, true); err != nil {
		log.LogError("Failed to save stats data on startup", zap.Error(err))
	}

	statsMessage, err := formatStatsMessage(stats)
	if err != nil {
		log.LogError("Failed to format stats message on startup", zap.Error(err))
		return // if error
	}

	// create "Trade on Luminex"
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("Trade on Luminex", "https://luminex.io/"),
		),
	)

	chartPath, err := tg_charts.GenerateVolumeChart()
	if err != nil {
		log.LogWarn("Failed to generate volume chart on startup", zap.Error(err))
		msg := tgbotapi.NewMessage(parseChatIDBig(filteredChatID), statsMessage)
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = keyboard
		_, err = bot.Send(msg)
		if err != nil {
			log.LogError("Failed to send stats message on startup", zap.Error(err))
			return
		}
	} else {
		// Check, file
		if _, err := os.Stat(chartPath); os.IsNotExist(err) {
			log.LogError("Chart file does not exist on startup", zap.String("chartPath", chartPath), zap.Error(err))
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
			log.LogError("Failed to send stats chart on startup", zap.Error(err))
			msg := tgbotapi.NewMessage(parseChatIDBig(filteredChatID), statsMessage)
			msg.ParseMode = tgbotapi.ModeHTML
			msg.ReplyMarkup = keyboard
			bot.Send(msg)
			return
		}
	}

	log.LogInfo("Stats sent successfully on startup",
		zap.String("chatID", filteredChatID),
		zap.Float64("tvl", stats.TotalTVLUSD),
		zap.Float64("volume24h", stats.TotalVolume24HUSD))
}
