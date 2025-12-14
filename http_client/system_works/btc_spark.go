package system_works

// Package system_works contains for BTC Spark

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
)

const (
	// LuminexSparkAddressAPIBaseURL - URL API Luminex for addresses Spark
	LuminexSparkAddressAPIBaseURL = "https://api.luminex.io/spark/address"
	// SparkPublicKey - Spark addresses for BTC
	SparkPublicKey   = "023e33e2920326f64ea31058d44777442d97d7d5cbfcf54e3060bc1695e5261c93"
	BTCSparkDataFile = "data_out/telegram_out/btc_spark.json"
)

// BTCSparkAddressResponse - API Luminex for addresses Spark
type BTCSparkAddressResponse struct {
	SparkAddress string `json:"sparkAddress"`
	PublicKey    string `json:"publicKey"`
	Balance      struct {
		BtcSoftBalanceSats int64   `json:"btcSoftBalanceSats"`
		BtcHardBalanceSats int64   `json:"btcHardBalanceSats"`
		BtcValueUsdHard    float64 `json:"btcValueUsdHard"`
		BtcValueUsdSoft    float64 `json:"btcValueUsdSoft"`
		TotalTokenValueUsd float64 `json:"totalTokenValueUsd"`
	} `json:"balance"`
	TotalValueUsd    float64       `json:"totalValueUsd"`
	TransactionCount int           `json:"transactionCount"`
	TokenCount       int           `json:"tokenCount"`
	Tokens           []interface{} `json:"tokens"`
}

type BTCSparkDataEntry struct {
	Timestamp  string  `json:"timestamp"`   // in RFC3339 "2006-01-02T15:04:05Z07:00"
	Date       string  `json:"date"`        // date in "2006-01-02" (for
	BtcReserve float64 `json:"btc_reserve"` // BTC 2
	Check      bool    `json:"check"`       // (true if /)
}

// BTCSparkData - for BTC in
type BTCSparkData struct {
	Entries []BTCSparkDataEntry `json:"entries"`
}

// GetBTCSparkReserve BTC from API Luminex
func GetBTCSparkReserve() (float64, error) {
	// URL
	url := fmt.Sprintf("%s/%s", LuminexSparkAddressAPIBaseURL, SparkPublicKey)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// create Cloudflare)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://luminex.io/")
	req.Header.Set("Origin", "https://luminex.io")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch from Luminex Spark Address API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("luminex Spark Address API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var addressResp BTCSparkAddressResponse
	if err := json.NewDecoder(resp.Body).Decode(&addressResp); err != nil {
		return 0, fmt.Errorf("failed to decode Luminex Spark Address API response: %w", err)
	}

	// satoshi in BTC and 2
	btcReserve := float64(addressResp.Balance.BtcSoftBalanceSats) / 1e8
	btcReserve = float64(int(btcReserve*100+0.5)) / 100

	return btcReserve, nil
}

// LoadBTCSparkData data BTC from file
func LoadBTCSparkData() (*BTCSparkData, error) {
	filePath := BTCSparkDataFile

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return &BTCSparkData{Entries: []BTCSparkDataEntry{}}, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read BTC spark data file: %w", err)
	}

	// Check, file
	if len(data) == 0 {
		return &BTCSparkData{Entries: []BTCSparkDataEntry{}}, nil
	}

	// Parse JSON
	var btcSparkData BTCSparkData
	if err := json.Unmarshal(data, &btcSparkData); err != nil {
		return nil, fmt.Errorf("failed to parse BTC spark data JSON: %w", err)
	}

	return &btcSparkData, nil
}

// SaveBTCSparkData data BTC in file btc_spark.json
// check - (true if /)
func SaveBTCSparkData(btcReserve float64, check bool) error {
	filePath := BTCSparkDataFile

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Get
	now := time.Now()
	timestamp := now.Format(time.RFC3339)
	today := now.Format("2006-01-02")

	existingData, err := LoadBTCSparkData()
	if err != nil {
		existingData = &BTCSparkData{Entries: []BTCSparkDataEntry{}}
	}

	// Add -
	existingData.Entries = append(existingData.Entries, BTCSparkDataEntry{
		Timestamp:  timestamp,
		Date:       today,
		BtcReserve: btcReserve,
		Check:      check,
	})

	data, err := json.MarshalIndent(existingData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal BTC spark data JSON: %w", err)
	}

	tempFilePath := filePath + ".tmp"
	if err := os.WriteFile(tempFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary BTC spark data file: %w", err)
	}

	if err := os.Rename(tempFilePath, filePath); err != nil {
		os.Remove(tempFilePath)
		return fmt.Errorf("failed to rename temporary file to BTC spark data file: %w", err)
	}

	LogInfo("Saved BTC spark data",
		zap.String("file", filePath),
		zap.String("date", today),
		zap.Float64("btcReserve", btcReserve),
		zap.Bool("check", check))

	return nil
}

// IsBTCSparkCheckedToday BTC
func IsBTCSparkCheckedToday() (bool, error) {
	data, err := LoadBTCSparkData()
	if err != nil {
		return false, err
	}

	today := time.Now().Format("2006-01-02")

	for _, entry := range data.Entries {
		if entry.Date == today {
			return entry.Check, nil
		}
	}

	return false, nil
}
