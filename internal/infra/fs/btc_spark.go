package fs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// BTCSparkDataFile is the path used by the bot to store BTC Spark snapshots.
	BTCSparkDataFile = "data_out/telegram_out/btc_spark.json"
)

// BTCSparkDataEntry is one snapshot record.
type BTCSparkDataEntry struct {
	Timestamp  string  `json:"timestamp"`   // RFC3339
	Date       string  `json:"date"`        // YYYY-MM-DD
	BtcReserve float64 `json:"btc_reserve"` // BTC reserve (2 decimals used by writer)
	Check      bool    `json:"check"`       // true if checked via command/schedule
}

// BTCSparkData is file structure for btc_spark.json.
type BTCSparkData struct {
	Entries []BTCSparkDataEntry `json:"entries"`
}

// LoadBTCSparkData loads BTC spark data from file.
// Returns empty dataset if file doesn't exist (not an error).
func LoadBTCSparkData() (*BTCSparkData, error) {
	filePath := BTCSparkDataFile

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return &BTCSparkData{Entries: []BTCSparkDataEntry{}}, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read BTC spark data file: %w", err)
	}

	if len(data) == 0 {
		return &BTCSparkData{Entries: []BTCSparkDataEntry{}}, nil
	}

	var btcSparkData BTCSparkData
	if err := json.Unmarshal(data, &btcSparkData); err != nil {
		return nil, fmt.Errorf("failed to parse BTC spark data JSON: %w", err)
	}

	if btcSparkData.Entries == nil {
		btcSparkData.Entries = []BTCSparkDataEntry{}
	}

	return &btcSparkData, nil
}

// SaveBTCSparkData appends one snapshot into btc_spark.json.
func SaveBTCSparkData(btcReserve float64, check bool) error {
	filePath := BTCSparkDataFile

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	now := time.Now()
	timestamp := now.Format(time.RFC3339)
	today := now.Format("2006-01-02")

	existingData, err := LoadBTCSparkData()
	if err != nil {
		existingData = &BTCSparkData{Entries: []BTCSparkDataEntry{}}
	}

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
		_ = os.Remove(tempFilePath)
		return fmt.Errorf("failed to rename temporary file to BTC spark data file: %w", err)
	}

	return nil
}

// IsBTCSparkCheckedToday returns check-flag for today's entry (if present).
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
