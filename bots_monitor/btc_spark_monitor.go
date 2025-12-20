package bots_monitor

// bot — Telegram- and

import (
	"fmt"
	"os"
	"strings"
	"time"

	"spark-wallet/internal/clients_api/luminex"
	"spark-wallet/internal/features/tg_charts"
	storage "spark-wallet/internal/infra/fs"
	log "spark-wallet/internal/infra/log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

// RunBTCSparkMonitor by BTC Spark in time (MSK).
func RunBTCSparkMonitor(bot *tgbotapi.BotAPI, filteredChatID string, sendTime string) {
	if bot == nil {
		log.LogWarn("Bot is nil, BTC spark monitor not started")
		return
	}

	if filteredChatID == "" {
		log.LogWarn("Filtered chat ID is empty, BTC spark monitor not started")
		return
	}

	log.LogInfo("Starting BTC Spark Monitor...", zap.String("filteredChatID", filteredChatID))

	// Load
	moscowLocation, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		log.LogError("Failed to load Moscow timezone, using UTC", zap.Error(err))
		moscowLocation = time.UTC
	}

	sendBTCReserve := func(check bool) {
		// Get BTC from API
		btcReserve, err := luminex.GetBTCSparkReserve()
		if err != nil {
			log.LogError("Failed to get BTC spark reserve", zap.Error(err))
			return
		}

		// Save data in btc_spark.json check
		if err := storage.SaveBTCSparkData(btcReserve, check); err != nil {
			log.LogError("Failed to save BTC spark data", zap.Error(err))
		}

		// BTC
		sparkMessage := formatSparkMessage(btcReserve)

		// create "Trade on Luminex"
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL("Trade on Luminex", "https://luminex.io/"),
			),
		)

		chartPath, err := tg_charts.GenerateBTCSparkChart()
		if err != nil {
			log.LogWarn("Failed to generate BTC spark chart", zap.Error(err))
			msg := tgbotapi.NewMessage(parseChatIDBig(filteredChatID), sparkMessage)
			msg.ParseMode = tgbotapi.ModeHTML
			msg.ReplyMarkup = keyboard
			_, err = bot.Send(msg)
			if err != nil {
				log.LogError("Failed to send BTC spark message", zap.Error(err))
				return
			}
		} else {
			// Check, file
			if _, err := os.Stat(chartPath); os.IsNotExist(err) {
				log.LogError("Chart file does not exist", zap.String("chartPath", chartPath), zap.Error(err))
				msg := tgbotapi.NewMessage(parseChatIDBig(filteredChatID), sparkMessage)
				msg.ParseMode = tgbotapi.ModeHTML
				msg.ReplyMarkup = keyboard
				bot.Send(msg)
				return
			}

			photo := tgbotapi.NewPhoto(parseChatIDBig(filteredChatID), tgbotapi.FilePath(chartPath))
			photo.Caption = sparkMessage
			photo.ParseMode = tgbotapi.ModeHTML
			photo.ReplyMarkup = keyboard

			_, err = bot.Send(photo)
			if err != nil {
				log.LogError("Failed to send BTC spark chart", zap.Error(err))
				msg := tgbotapi.NewMessage(parseChatIDBig(filteredChatID), sparkMessage)
				msg.ParseMode = tgbotapi.ModeHTML
				msg.ReplyMarkup = keyboard
				bot.Send(msg)
				return
			}
		}

		log.LogInfo("BTC spark reserve sent successfully",
			zap.String("chatID", filteredChatID),
			zap.Bool("check", check),
			zap.Float64("btcReserve", btcReserve))
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
	nextSend := time.Date(now.Year(), now.Month(), now.Day(), now.Hour()+1, 0, 0, 0, moscowLocation)

	if now.After(nextSend) || now.Equal(nextSend) {
		nextSend = nextSend.Add(1 * time.Hour)
	}

	// Calculate
	delay := nextSend.Sub(now)
	log.LogInfo("BTC spark monitor scheduled",
		zap.Time("nextSend", nextSend),
		zap.Duration("delay", delay))

	firstTimer := time.NewTimer(delay)
	go func() {
		<-firstTimer.C
		sendBTCReserve(true) // check = true for

		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			sendBTCReserve(true) // check = true for
		}
	}()

	log.LogInfo("BTC spark monitor started successfully",
		zap.String("sendTime", sendTime),
		zap.Time("nextSend", nextSend),
		zap.Duration("delay", delay))
}

// CheckAndSendBTCSparkOnStartup BTC and if -
// bot - Telegram for
// filteredChatID - ID for
func CheckAndSendBTCSparkOnStartup(bot *tgbotapi.BotAPI, filteredChatID string) {
	if bot == nil {
		log.LogWarn("Bot is nil, skipping BTC spark check on startup")
		return
	}

	if filteredChatID == "" {
		log.LogWarn("Filtered chat ID is empty, skipping BTC spark check on startup")
		return
	}

	// Check,
	checked, err := storage.IsBTCSparkCheckedToday()
	if err != nil {
		log.LogWarn("Failed to check if BTC spark was checked today", zap.Error(err))
	}

	if checked {
		log.LogInfo("BTC spark already checked today, skipping startup send")
		return
	}

	log.LogInfo("BTC spark not checked today, sending on startup")

	// Get BTC from API
	btcReserve, err := luminex.GetBTCSparkReserve()
	if err != nil {
		log.LogError("Failed to get BTC spark reserve on startup", zap.Error(err))
		return
	}

	// Save data in btc_spark.json check = true
	if err := storage.SaveBTCSparkData(btcReserve, true); err != nil {
		log.LogError("Failed to save BTC spark data on startup", zap.Error(err))
	}

	// BTC
	sparkMessage := formatSparkMessage(btcReserve)

	// create "Trade on Luminex"
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("Trade on Luminex", "https://luminex.io/"),
		),
	)

	chartPath, err := tg_charts.GenerateBTCSparkChart()
	if err != nil {
		log.LogWarn("Failed to generate BTC spark chart on startup", zap.Error(err))
		msg := tgbotapi.NewMessage(parseChatIDBig(filteredChatID), sparkMessage)
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = keyboard
		_, err = bot.Send(msg)
		if err != nil {
			log.LogError("Failed to send BTC spark message on startup", zap.Error(err))
			return
		}
	} else {
		// Check, file
		if _, err := os.Stat(chartPath); os.IsNotExist(err) {
			log.LogError("Chart file does not exist on startup", zap.String("chartPath", chartPath), zap.Error(err))
			msg := tgbotapi.NewMessage(parseChatIDBig(filteredChatID), sparkMessage)
			msg.ParseMode = tgbotapi.ModeHTML
			msg.ReplyMarkup = keyboard
			bot.Send(msg)
			return
		}

		photo := tgbotapi.NewPhoto(parseChatIDBig(filteredChatID), tgbotapi.FilePath(chartPath))
		photo.Caption = sparkMessage
		photo.ParseMode = tgbotapi.ModeHTML
		photo.ReplyMarkup = keyboard

		_, err = bot.Send(photo)
		if err != nil {
			log.LogError("Failed to send BTC spark chart on startup", zap.Error(err))
			msg := tgbotapi.NewMessage(parseChatIDBig(filteredChatID), sparkMessage)
			msg.ParseMode = tgbotapi.ModeHTML
			msg.ReplyMarkup = keyboard
			bot.Send(msg)
			return
		}
	}

	log.LogInfo("BTC spark reserve sent successfully on startup",
		zap.String("chatID", filteredChatID),
		zap.Float64("btcReserve", btcReserve))
}
