package system_works

// Package system_works contains for

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

type HolderReportEntry struct {
	Address      string  // address (publicKey)
	AddressShort string  // 3 addresses
	Username     string  // (username) or
	SparkAddress string  // Spark address for
	FirstBuy     string  // date or
	Balance      float64 // balance tokens
	Percentage   float64 // total_supply
	Action       string  // "invested", "sold", "liquidated"
	DailyCount   int     // count
	Value        float64 // amount in BTC
}

func GenerateHoldersReport(ticker string, dateStr string, client *Client) (string, error) {
	if ticker == "" || dateStr == "" {
		return "", fmt.Errorf("ticker and date are required")
	}

	// Check, ticker
	if !IsTickerAllowed(ticker) {
		return "", fmt.Errorf("ticker %s is not in allowed list (ASTY, SOON, BITTY)", ticker)
	}

	// Parse from DDMM in YYYY-MM-DD
	parsedDate, err := parseDateDDMM(dateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse date %s: %w", dateStr, err)
	}
	dateFormatted := parsedDate.Format("2006-01-02")

	LogInfo("Generating holders report",
		zap.String("ticker", ticker),
		zap.String("dateStr", dateStr),
		zap.String("dateFormatted", dateFormatted))

	// Load
	dynamicData, err := LoadDynamicHolders(ticker)
	if err != nil {
		return "", fmt.Errorf("failed to load dynamic holders: %w", err)
	}

	savedData, err := LoadSavedHolders(ticker)
	if err != nil {
		return "", fmt.Errorf("failed to load saved holders: %w", err)
	}

	// Get poolLpPublicKey for total_supply
	poolLpPublicKey, err := FindPoolLpPublicKeyByTicker(ticker)
	if err != nil {
		return "", fmt.Errorf("failed to find poolLpPublicKey for ticker %s: %w", ticker, err)
	}

	// Get total_supply from API Luminex
	totalSupplyStr, decimals, err := GetPoolTotalSupply(poolLpPublicKey)
	if err != nil {
		LogWarn("Failed to get total_supply from API, using default", zap.Error(err))
		// Use by default,
		totalSupplyStr = "1000000000000000000" // 1e18
		decimals = 8
	}

	totalSupplyFloat, err := parseTokenAmount(totalSupplyStr, decimals)
	if err != nil {
		return "", fmt.Errorf("failed to parse total_supply: %w", err)
	}

	addressesForDate := make(map[string]bool)
	for address, changes := range dynamicData.Changes {
		for _, change := range changes {
			if change.Date == dateFormatted {
				addressesForDate[address] = true
				break
			}
		}
	}

	if len(addressesForDate) == 0 {
		return fmt.Sprintf("Report for %s:\n\nNo data for the specified date", dateFormatted), nil
	}

	var reportEntries []HolderReportEntry
	for address := range addressesForDate {
		// Get
		// in -
		var lastChange BalanceChange
		var found bool
		for i := len(dynamicData.Changes[address]) - 1; i >= 0; i-- {
			change := dynamicData.Changes[address][i]
			if change.Date == dateFormatted {
				lastChange = change
				found = true
				break
			}
		}

		if !found {
			continue
		}

		// Get balance from saved_holders or from
		var currentBalance float64
		if balanceStr, exists := savedData.Holders[address]; exists {
			// Parse balance from saved_holders
			if parsedBalance, err := strconv.ParseFloat(balanceStr, 64); err == nil {
				currentBalance = parsedBalance
			} else {
				currentBalance = lastChange.Amount
			}
		} else {
			currentBalance = lastChange.Amount
		}

		// Calculate total_supply
		percentage := 0.0
		if totalSupplyFloat > 0 {
			percentage = (currentBalance / totalSupplyFloat) * 100
		}

		dailyCount := 0
		for _, change := range dynamicData.Changes[address] {
			if change.Date == dateFormatted {
				dailyCount++
			}
		}

		// Get
		firstBuyDate := ""
		if client != nil {
			firstBuy, err := GetFirstBuySwap(client, address, poolLpPublicKey)
			if err == nil && firstBuy != "" {
				firstBuyDate = firstBuy
			}
		}

		// Get 3 addresses
		addressShort := ""
		if len(address) >= 3 {
			addressShort = address[len(address)-3:]
		} else {
			addressShort = address
		}

		// Get username and sparkAddress for creating clickable link
		username := GetWalletUsername(address)
		sparkAddress := address // default: use publicKey
		balanceResp, err := GetWalletBalance(address)
		if err == nil && balanceResp != nil {
			if balanceResp.SparkAddress != "" {
				sparkAddress = balanceResp.SparkAddress
			}
		}

		reportEntries = append(reportEntries, HolderReportEntry{
			Address:      address,
			AddressShort: addressShort,
			Username:     username,
			SparkAddress: sparkAddress,
			FirstBuy:     firstBuyDate,
			Balance:      currentBalance,
			Percentage:   percentage,
			Action:       lastChange.Action,
			DailyCount:   dailyCount,
			Value:        lastChange.Value, // amount in BTC
		})
	}

	var report strings.Builder
	report.WriteString(fmt.Sprintf("Report for %s (%s):\n\n", dateFormatted, ticker))

	// HTML
	report.WriteString("<blockquote>\n")

	for _, entry := range reportEntries {
		var emoji string
		switch entry.Action {
		case "invested":
			emoji = "🟢"
		case "sold":
			emoji = "🟠"
		case "liquidated":
			emoji = "🔴"
		default:
			emoji = "⚪"
		}

		// Get for (username or "wallet")
		displayName := "wallet"
		if entry.Username != "" {
			displayName = entry.Username
		}

		walletLink := fmt.Sprintf("https://luminex.io/spark/address/%s", entry.SparkAddress)

		// Format balance (6 for + + K/M)
		balanceStr := formatBalanceAligned(entry.Balance)

		// for K - |, for M - |
		if strings.HasSuffix(balanceStr, "K") {
			balanceStr = balanceStr + "  " // for K
		} else if strings.HasSuffix(balanceStr, "M") {
			balanceStr = balanceStr + " " // for M
		}

		// Format (DD MMM
		firstBuyStr := formatFirstBuyDate(entry.FirstBuy)

		// Format value BTC in Telegram)
		valueStr := formatBTCValue(entry.Value)
		// in <code> for in Telegram
		if valueStr != "{}" {
			valueStr = fmt.Sprintf("<code>%s</code>", valueStr)
		}

		// in Telegram)
		actionStr := ""
		switch entry.Action {
		case "invested":
			if entry.DailyCount > 0 {
				actionStr = fmt.Sprintf("BUY ×%d", entry.DailyCount)
			} else {
				actionStr = "BUY ×1"
			}
		case "sold":
			if entry.DailyCount > 0 {
				actionStr = fmt.Sprintf("SELL ×%d", entry.DailyCount)
			} else {
				actionStr = "SELL ×1"
			}
		case "liquidated":
			actionStr = "LIQUIDATED"
		default:
			actionStr = strings.ToUpper(entry.Action)
		}
		// in <b> for in Telegram
		actionStr = fmt.Sprintf("<b>%s</b>", actionStr)

		// 🟢 wallet (92c)
		//         Balance: 817.03K  | First buy: 08 Dec | Value: {} | Action: BUY ×1
		report.WriteString(fmt.Sprintf("%s <a href=\"%s\">%s</a> (%s)\n",
			emoji,
			walletLink,
			displayName,
			entry.AddressShort))
		report.WriteString(fmt.Sprintf("Balance: %s | First buy: %s | Value: %s | Action: %s\n\n",
			balanceStr,
			firstBuyStr,
			valueStr,
			actionStr))
	}

	// HTML
	report.WriteString("</blockquote>")

	return report.String(), nil
}

// parseDateDDMM from DDMM in time.Time
func parseDateDDMM(dateStr string) (time.Time, error) {
	if len(dateStr) != 4 {
		return time.Time{}, fmt.Errorf("date must be in DDMM format (4 digits)")
	}

	day, err := strconv.Atoi(dateStr[:2])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid day: %w", err)
	}

	month, err := strconv.Atoi(dateStr[2:])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid month: %w", err)
	}

	if day < 1 || day > 31 {
		return time.Time{}, fmt.Errorf("day must be between 1 and 31")
	}
	if month < 1 || month > 12 {
		return time.Time{}, fmt.Errorf("month must be between 1 and 12")
	}

	// Use
	now := time.Now()
	year := now.Year()

	// If date in use
	if month > int(now.Month()) || (month == int(now.Month()) && day > now.Day()) {
		year--
	}

	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC), nil
}

// parseTokenAmount count tokens from decimals
func parseTokenAmount(amountStr string, decimals int) (float64, error) {
	// Use big.Float for
	amountBig, ok := new(big.Float).SetString(amountStr)
	if !ok {
		return 0, fmt.Errorf("failed to parse amount: %s", amountStr)
	}

	// on 10^decimals
	divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))
	result := new(big.Float).Quo(amountBig, divisor)

	// in float64
	resultFloat, _ := result.Float64()
	return resultFloat, nil
}

func formatBalance(balance float64) string {
	if balance >= 1000000 {
		return fmt.Sprintf("%.2fM", balance/1000000)
	} else if balance >= 1000 {
		return fmt.Sprintf("%.2fK", balance/1000)
	} else {
		return fmt.Sprintf("%.2f", balance)
	}
}

// formatBalanceAligned balance (6 for + + K/M)
// If 080.23K)
// 6 and 2 + K/M = 7-8
func formatBalanceAligned(balance float64) string {
	var value float64
	var suffix string

	if balance >= 1000000 {
		value = balance / 1000000
		suffix = "M"
	} else {
		// for 1000) use K
		value = balance / 1000
		suffix = "K"
	}

	// Format 2
	formatted := fmt.Sprintf("%.2f", value)

	parts := strings.Split(formatted, ".")
	if len(parts) != 2 {
		// If ".00"
		parts = []string{formatted, "00"}
	}

	integerPart := parts[0]
	decimalPart := parts[1]

	// 6 and 2
	// 817.03 (6 080.23 (6 1234.56 (7 -
	// XXX.XX XXX - 3 XX - (2
	// 6 (3 + + 2

	// If 3 3
	if len(integerPart) > 3 {
		integerPart = integerPart[len(integerPart)-3:]
	}

	if len(integerPart) < 3 {
		integerPart = strings.Repeat("0", 3-len(integerPart)) + integerPart
	}

	return integerPart + "." + decimalPart + suffix
}

// formatFirstBuyDate in DD MMM "08 Dec")
func formatFirstBuyDate(firstBuyDate string) string {
	if firstBuyDate == "" {
		return "N/A"
	}

	// Parse from "2006-01-02 15:04" or "2006-01-02"
	var t time.Time
	var err error

	if strings.Contains(firstBuyDate, " ") {
		// "2006-01-02 15:04"
		t, err = time.Parse("2006-01-02 15:04", firstBuyDate)
	} else {
		// date "2006-01-02"
		t, err = time.Parse("2006-01-02", firstBuyDate)
	}

	if err != nil {
		// If return or "N/A"
		return "N/A"
	}

	// Format in DD MMM "08 Dec")
	day := fmt.Sprintf("%02d", t.Day())
	month := t.Format("Jan")

	return day + " " + month
}

// formatBTCValue value BTC for in
func formatBTCValue(btcValue float64) string {
	if btcValue == 0 {
		return "{}"
	}

	// Format BTC in formatBTCWithoutTrailingZeros)
	formatted := fmt.Sprintf("%.8f", btcValue)
	formatted = strings.TrimRight(formatted, "0")
	// If in remove and
	formatted = strings.TrimRight(formatted, ".")

	return formatted
}
