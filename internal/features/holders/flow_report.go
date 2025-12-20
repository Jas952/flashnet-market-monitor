package holders

// Package system_works contains for and

import (
	"fmt"
	"spark-wallet/internal/clients_api/luminex"
	logging "spark-wallet/internal/infra/log"
	"strings"
	"time"

	"go.uber.org/zap"
)

func GenerateFlowReport(ticker string, dateStr string) (string, error) {
	if !IsTickerAllowed(ticker) {
		return "", fmt.Errorf("ticker %s is not in allowed list", ticker)
	}

	// Parse from DDMM
	parsedDate, err := parseDateFromDDMM(dateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse date: %w", err)
	}

	// Format for (DD Mon)
	dateDisplay := formatDateForFlow(parsedDate)

	// Get poolLpPublicKey for
	poolLpPublicKey, err := luminex.GetPoolLpPublicKeyForTicker(ticker)
	if err != nil {
		return "", fmt.Errorf("failed to get poolLpPublicKey for ticker %s: %w", ticker, err)
	}

	// Get pool from API Luminex
	poolStats, err := luminex.GetPoolStats(poolLpPublicKey)
	if err != nil {
		return "", fmt.Errorf("failed to get pool stats: %w", err)
	}

	// Parse from (API
	var buyVolume, sellVolume float64
	if poolStats.BuyVolume != "" {
		if _, err := fmt.Sscanf(poolStats.BuyVolume, "%f", &buyVolume); err != nil {
			logging.LogWarn("Failed to parse buyVolume", zap.String("buyVolume", poolStats.BuyVolume), zap.Error(err))
			buyVolume = 0
		}
	}
	if poolStats.SellVolume != "" {
		if _, err := fmt.Sscanf(poolStats.SellVolume, "%f", &sellVolume); err != nil {
			logging.LogWarn("Failed to parse sellVolume", zap.String("sellVolume", poolStats.SellVolume), zap.Error(err))
			sellVolume = 0
		}
	}

	// Format BTC
	buyValueStr := formatBTCValueForFlow(buyVolume)
	sellValueStr := formatBTCValueForFlow(sellVolume)

	// Calculate B/S (count / count
	var bsRatio string
	if poolStats.Sells > 0 {
		ratio := float64(poolStats.Buys) / float64(poolStats.Sells)
		bsRatio = fmt.Sprintf("%.2f", ratio)
	} else if poolStats.Buys > 0 {
		bsRatio = "∞" // if
	} else {
		bsRatio = "0.00"
	}

	// Calculate B/S$ (value / value
	var bsValueRatio string
	if sellVolume > 0 {
		ratio := buyVolume / sellVolume
		bsValueRatio = fmt.Sprintf("%.2f", ratio)
	} else if buyVolume > 0 {
		bsValueRatio = "∞" // if
	} else {
		bsValueRatio = "0.00"
	}

	var report strings.Builder
	report.WriteString(fmt.Sprintf("%s for %s:\n\n", strings.ToUpper(ticker), dateDisplay))

	// in (blockquote)
	report.WriteString("<blockquote>")
	report.WriteString(fmt.Sprintf("Buys: %d (%s)\n", poolStats.Buys, buyValueStr))
	report.WriteString(fmt.Sprintf("Sells: %d (%s)\n\n", poolStats.Sells, sellValueStr))

	bsRatioFormatted := bsRatio
	if bsRatio != "∞" {
		bsRatioFormatted = fmt.Sprintf("<code>%s</code>", bsRatio)
	}

	bsValueRatioFormatted := bsValueRatio
	if bsValueRatio != "∞" {
		bsValueRatioFormatted = fmt.Sprintf("<code>%s</code>", bsValueRatio)
	}

	report.WriteString(fmt.Sprintf("– B/S = %s\n", bsRatioFormatted))
	report.WriteString(fmt.Sprintf("– B/S$ = %s", bsValueRatioFormatted))
	report.WriteString("</blockquote>")

	return report.String(), nil
}

// parseDateFromDDMM from DDMM "0912" -> 09
func parseDateFromDDMM(dateStr string) (time.Time, error) {
	if len(dateStr) != 4 {
		return time.Time{}, fmt.Errorf("date must be 4 digits (DDMM format)")
	}

	dayStr := dateStr[:2]
	monthStr := dateStr[2:]

	var day, month int
	if _, err := fmt.Sscanf(dayStr, "%d", &day); err != nil {
		return time.Time{}, fmt.Errorf("invalid day: %s", dayStr)
	}
	if _, err := fmt.Sscanf(monthStr, "%d", &month); err != nil {
		return time.Time{}, fmt.Errorf("invalid month: %s", monthStr)
	}

	if day < 1 || day > 31 {
		return time.Time{}, fmt.Errorf("invalid day: %d", day)
	}
	if month < 1 || month > 12 {
		return time.Time{}, fmt.Errorf("invalid month: %d", month)
	}

	// Use
	now := time.Now()
	year := now.Year()

	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)

	// Check, date 31
	if date.Day() != day || date.Month() != time.Month(month) {
		return time.Time{}, fmt.Errorf("invalid date: %02d/%02d", day, month)
	}

	return date, nil
}

// formatDateForFlow for in (DD Mon)
func formatDateForFlow(date time.Time) string {
	months := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	return fmt.Sprintf("%02d %s", date.Day(), months[date.Month()-1])
}

// formatBTCValueForFlow value BTC for in
func formatBTCValueForFlow(btcValue float64) string {
	if btcValue == 0 {
		return "0"
	}

	// Format BTC 8 and remove
	formatted := fmt.Sprintf("%.8f", btcValue)
	formatted = strings.TrimRight(formatted, "0")
	formatted = strings.TrimRight(formatted, ".")

	return formatted
}
