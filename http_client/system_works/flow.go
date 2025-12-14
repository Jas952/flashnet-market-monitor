package system_works

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
)

type FlowData struct {
	// Note: All values and logic are taken from holders_module folder and work together with dynamic_holders.json
	Note       string               `json:"_note,omitempty"` // holders_module
	DailyFlows map[string]DailyFlow `json:"dailyFlows"`      // date (YYYY-MM-DD) -> data
}

type DailyFlow struct {
	Date         string  `json:"date"`         // date in YYYY-MM-DD
	BuyCount     int     `json:"buyCount"`     // count (invested actions)
	SellCount    int     `json:"sellCount"`    // count (sold actions)
	BuyValueBTC  float64 `json:"buyValueBTC"`  // amount in BTC
	SellValueBTC float64 `json:"sellValueBTC"` // amount in BTC
}

func LoadFlowData() (*FlowData, error) {
	filename := filepath.Join("data_out", "telegram_out", "flow.json")

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return &FlowData{
			Note:       "All values and logic are taken from holders_module folder and work together with dynamic_holders.json",
			DailyFlows: make(map[string]DailyFlow),
		}, nil
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read flow file: %w", err)
	}

	if len(data) == 0 {
		return &FlowData{
			Note:       "All values and logic are taken from holders_module folder and work together with dynamic_holders.json",
			DailyFlows: make(map[string]DailyFlow),
		}, nil
	}

	var flowData FlowData
	if err := json.Unmarshal(data, &flowData); err != nil {
		if len(data) < 100 {
			return &FlowData{
				Note:       "All values and logic are taken from holders_module folder and work together with dynamic_holders.json",
				DailyFlows: make(map[string]DailyFlow),
			}, nil
		}
		return nil, fmt.Errorf("failed to parse flow JSON: %w", err)
	}

	if flowData.DailyFlows == nil {
		flowData.DailyFlows = make(map[string]DailyFlow)
	}

	if flowData.Note == "" {
		flowData.Note = "All values and logic are taken from holders_module folder and work together with dynamic_holders.json"
	}

	return &flowData, nil
}

func SaveFlowData(flowData *FlowData) error {
	dir := filepath.Join("data_out", "telegram_out")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create telegram_out directory: %w", err)
	}

	filename := filepath.Join(dir, "flow.json")

	tmpFile := filename + ".tmp"
	file, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(flowData); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to encode flow JSON: %w", err)
	}

	file.Close()

	if err := os.Rename(tmpFile, filename); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// UpdateFlowFromSwap updates flow data based on swap from dynamic_holders.json
func UpdateFlowFromSwap(ticker string, action string, value float64) error {
	if !IsTickerAllowed(ticker) {
		return fmt.Errorf("ticker %s is not in allowed list", ticker)
	}

	flowData, err := LoadFlowData()
	if err != nil {
		return fmt.Errorf("failed to load flow data: %w", err)
	}

	currentDate := time.Now().Format("2006-01-02")

	dailyFlow, exists := flowData.DailyFlows[currentDate]
	if !exists {
		dailyFlow = DailyFlow{
			Date:         currentDate,
			BuyCount:     0,
			SellCount:    0,
			BuyValueBTC:  0,
			SellValueBTC: 0,
		}
	}

	if action == "invested" {
		dailyFlow.BuyCount++
		dailyFlow.BuyValueBTC += value
	} else if action == "sold" {
		dailyFlow.SellCount++
		dailyFlow.SellValueBTC += value
	}

	flowData.DailyFlows[currentDate] = dailyFlow

	if err := SaveFlowData(flowData); err != nil {
		return fmt.Errorf("failed to save flow data: %w", err)
	}

	LogDebug("Updated flow from swap",
		zap.String("ticker", ticker),
		zap.String("action", action),
		zap.Float64("value", value),
		zap.String("date", currentDate),
		zap.Int("buyCount", dailyFlow.BuyCount),
		zap.Int("sellCount", dailyFlow.SellCount))

	return nil
}

func GetFlowForDate(date string) (*DailyFlow, error) {
	flowData, err := LoadFlowData()
	if err != nil {
		return nil, fmt.Errorf("failed to load flow data: %w", err)
	}

	dailyFlow, exists := flowData.DailyFlows[date]
	if !exists {
		return &DailyFlow{
			Date:         date,
			BuyCount:     0,
			SellCount:    0,
			BuyValueBTC:  0,
			SellValueBTC: 0,
		}, nil
	}

	return &dailyFlow, nil
}

func RecalculateFlowFromDynamicHolders(ticker string, date string) error {
	if !IsTickerAllowed(ticker) {
		return fmt.Errorf("ticker %s is not in allowed list", ticker)
	}

	dynamicData, err := LoadDynamicHolders(ticker)
	if err != nil {
		return fmt.Errorf("failed to load dynamic holders: %w", err)
	}

	flowData, err := LoadFlowData()
	if err != nil {
		return fmt.Errorf("failed to load flow data: %w", err)
	}

	dailyFlow := DailyFlow{
		Date:         date,
		BuyCount:     0,
		SellCount:    0,
		BuyValueBTC:  0,
		SellValueBTC: 0,
	}

	for _, changes := range dynamicData.Changes {
		for _, change := range changes {
			if change.Date == date {
				if change.Action == "invested" {
					dailyFlow.BuyCount++
					dailyFlow.BuyValueBTC += change.Value
				} else if change.Action == "sold" {
					dailyFlow.SellCount++
					dailyFlow.SellValueBTC += change.Value
				}
			}
		}
	}

	flowData.DailyFlows[date] = dailyFlow

	if err := SaveFlowData(flowData); err != nil {
		return fmt.Errorf("failed to save flow data: %w", err)
	}

	LogDebug("Recalculated flow from dynamic holders",
		zap.String("ticker", ticker),
		zap.String("date", date),
		zap.Int("buyCount", dailyFlow.BuyCount),
		zap.Int("sellCount", dailyFlow.SellCount),
		zap.Float64("buyValueBTC", dailyFlow.BuyValueBTC),
		zap.Float64("sellValueBTC", dailyFlow.SellValueBTC))

	return nil
}

// CalculateFlowFromDynamicHolders calculates flow data from dynamic_holders.json for specified date
func CalculateFlowFromDynamicHolders(ticker string, date string) (*DailyFlow, error) {
	if !IsTickerAllowed(ticker) {
		return nil, fmt.Errorf("ticker %s is not in allowed list", ticker)
	}

	dynamicData, err := LoadDynamicHolders(ticker)
	if err != nil {
		return nil, fmt.Errorf("failed to load dynamic holders: %w", err)
	}

	dailyFlow := DailyFlow{
		Date:         date,
		BuyCount:     0,
		SellCount:    0,
		BuyValueBTC:  0,
		SellValueBTC: 0,
	}

	for address, changes := range dynamicData.Changes {
		for _, change := range changes {
			if change.Date == date {
				if change.Action == "invested" {
					dailyFlow.BuyCount++
					dailyFlow.BuyValueBTC += change.Value
					LogDebug("Found invested action for flow",
						zap.String("ticker", ticker),
						zap.String("date", date),
						zap.String("address", address),
						zap.Float64("value", change.Value))
				} else if change.Action == "sold" {
					dailyFlow.SellCount++
					dailyFlow.SellValueBTC += change.Value
					LogDebug("Found sold action for flow",
						zap.String("ticker", ticker),
						zap.String("date", date),
						zap.String("address", address),
						zap.Float64("value", change.Value))
				}
			}
		}
	}

	LogInfo("Calculated flow from dynamic holders",
		zap.String("ticker", ticker),
		zap.String("date", date),
		zap.Int("buyCount", dailyFlow.BuyCount),
		zap.Int("sellCount", dailyFlow.SellCount),
		zap.Float64("buyValueBTC", dailyFlow.BuyValueBTC),
		zap.Float64("sellValueBTC", dailyFlow.SellValueBTC))

	return &dailyFlow, nil
}
