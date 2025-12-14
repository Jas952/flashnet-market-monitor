package system_works

// Package system_works contains for tokens
// on saved_holders.json

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
)

// SavedHoldersData - for in address:N
type SavedHoldersData struct {
	Holders map[string]string `json:"holders"` // address -> count tokens
}

type DynamicHoldersData struct {
	LastCheckDate string                     `json:"lastCheckDate"` // date in YYYY-MM-DD
	Changes       map[string][]BalanceChange `json:"changes"`       // address ->
	DailyCounts   map[string]int             `json:"dailyCounts"`   // address -> count (lastCheckDate)
}

// BalanceChange -
// - action wallet
type BalanceChange struct {
	Amount float64 `json:"amount"` // count tokens
	Delta  float64 `json:"delta"`  // /)
	Action string  `json:"action"` // "invested" or "sold" or "liquidated"
	Value  float64 `json:"value"`  // amount in BTC
	Date   string  `json:"date"`   // date in YYYY-MM-DD
}

// LoadSavedHolders from file saved_holders.json
// for (ASTY, SOON, BITTY)
func LoadSavedHolders(ticker string) (*SavedHoldersData, error) {
	// Check, ticker
	if !IsTickerAllowed(ticker) {
		return nil, fmt.Errorf("ticker %s is not in allowed list (ASTY, SOON, BITTY)", ticker)
	}

	filename := filepath.Join("data_out", "holders_module", ticker, "saved_holders.json")

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return &SavedHoldersData{
				Holders: make(map[string]string),
			}, nil
		}
		return nil, fmt.Errorf("failed to read saved holders file: %w", err)
	}

	// Check, file or
	dataStr := strings.TrimSpace(string(data))
	if dataStr == "" || dataStr == "{}" || dataStr == "null" {
		// file or - return
		return &SavedHoldersData{
			Holders: make(map[string]string),
		}, nil
	}

	var holdersData SavedHoldersData
	if err := json.Unmarshal(data, &holdersData); err != nil {
		return convertOldHoldersFormat(data, ticker)
	}

	if holdersData.Holders == nil {
		holdersData.Holders = make(map[string]string)
	}

	return &holdersData, nil
}

// convertOldHoldersFormat holders in saved_holders
func convertOldHoldersFormat(data []byte, ticker string) (*SavedHoldersData, error) {
	var oldData TokenHoldersData
	if err := json.Unmarshal(data, &oldData); err != nil {
		return nil, fmt.Errorf("failed to parse holders file: %w", err)
	}

	newData := &SavedHoldersData{
		Holders: make(map[string]string),
	}

	for pubkey, holder := range oldData.Holders {
		// Use pubkey address (in
		// Save balance
		newData.Holders[pubkey] = holder.Balance
	}

	if err := SaveSavedHolders(ticker, newData); err != nil {
		LogWarn("Failed to save converted holders data", zap.String("ticker", ticker), zap.Error(err))
	}

	return newData, nil
}

// SaveSavedHolders in file saved_holders.json
// for (ASTY, SOON, BITTY)
func SaveSavedHolders(ticker string, data *SavedHoldersData) error {
	// Check, ticker
	if !IsTickerAllowed(ticker) {
		return fmt.Errorf("ticker %s is not in allowed list (ASTY, SOON, BITTY)", ticker)
	}

	holdersDir := filepath.Join("data_out", "holders_module", ticker)
	// Check, folder create
	if _, err := os.Stat(holdersDir); os.IsNotExist(err) {
		return fmt.Errorf("holders directory does not exist for ticker %s (only ASTY, SOON, BITTY are allowed)", ticker)
	}

	filename := filepath.Join(holdersDir, "saved_holders.json")

	dataBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal saved holders data: %w", err)
	}

	if err := os.WriteFile(filename, dataBytes, 0644); err != nil {
		return fmt.Errorf("failed to write saved holders file: %w", err)
	}

	return nil
}

// LoadDynamicHolders from file dynamic_holders.json
// for (ASTY, SOON, BITTY)
func LoadDynamicHolders(ticker string) (*DynamicHoldersData, error) {
	// Check, ticker
	if !IsTickerAllowed(ticker) {
		return nil, fmt.Errorf("ticker %s is not in allowed list (ASTY, SOON, BITTY)", ticker)
	}

	filename := filepath.Join("data_out", "holders_module", ticker, "dynamic_holders.json")

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return &DynamicHoldersData{
				LastCheckDate: "",
				Changes:       make(map[string][]BalanceChange),
				DailyCounts:   make(map[string]int),
			}, nil
		}
		LogError("Failed to read dynamic holders file", zap.String("ticker", ticker), zap.String("filename", filename), zap.Error(err))
		return nil, fmt.Errorf("failed to read dynamic holders file: %w", err)
	}

	// Check, file or
	dataStr := strings.TrimSpace(string(data))
	if dataStr == "" || dataStr == "{}" || dataStr == "null" {
		// file or - return
		return &DynamicHoldersData{
			LastCheckDate: "",
			Changes:       make(map[string][]BalanceChange),
			DailyCounts:   make(map[string]int),
		}, nil
	}

	var dynamicData DynamicHoldersData
	if err := json.Unmarshal(data, &dynamicData); err != nil {
		LogError("Failed to parse dynamic holders JSON", zap.String("ticker", ticker), zap.String("filename", filename), zap.String("data", string(data)), zap.Error(err))
		var oldFormat map[string]interface{}
		if err2 := json.Unmarshal(data, &oldFormat); err2 == nil {
			dynamicData = DynamicHoldersData{
				LastCheckDate: "",
				Changes:       make(map[string][]BalanceChange),
				DailyCounts:   make(map[string]int),
			}
			if lastCheck, ok := oldFormat["lastCheckDate"].(string); ok {
				dynamicData.LastCheckDate = lastCheck
			}
			if changes, ok := oldFormat["changes"].(map[string]interface{}); ok {
				dynamicData.Changes = make(map[string][]BalanceChange)
				for addr, changeList := range changes {
					if list, ok := changeList.([]interface{}); ok {
						for _, item := range list {
							if changeMap, ok := item.(map[string]interface{}); ok {
								change := BalanceChange{}
								if amount, ok := changeMap["amount"].(float64); ok {
									change.Amount = amount
								}
								if delta, ok := changeMap["delta"].(float64); ok {
									change.Delta = delta
								}
								if action, ok := changeMap["action"].(string); ok {
									change.Action = action
								}
								// If value use 0 (for
								if value, ok := changeMap["value"].(float64); ok {
									change.Value = value
								} else {
									change.Value = 0 // for value
								}
								// If date in use
								if date, ok := changeMap["date"].(string); ok {
									change.Date = date
								} else {
									// If use or lastCheckDate
									if dynamicData.LastCheckDate != "" {
										change.Date = dynamicData.LastCheckDate
									} else {
										change.Date = time.Now().Format("2006-01-02")
									}
								}
								dynamicData.Changes[addr] = append(dynamicData.Changes[addr], change)
							}
						}
					}
				}
			}
			if err := SaveDynamicHolders(ticker, &dynamicData); err != nil {
				LogWarn("Failed to save converted dynamic holders", zap.String("ticker", ticker), zap.Error(err))
			}
			return &dynamicData, nil
		}
		return nil, fmt.Errorf("failed to parse dynamic holders JSON: %w", err)
	}

	if dynamicData.Changes == nil {
		dynamicData.Changes = make(map[string][]BalanceChange)
	}
	if dynamicData.DailyCounts == nil {
		dynamicData.DailyCounts = make(map[string]int)
	}

	return &dynamicData, nil
}

// SaveDynamicHolders in file dynamic_holders.json
// for (ASTY, SOON, BITTY)
func SaveDynamicHolders(ticker string, data *DynamicHoldersData) error {
	// Check, ticker
	if !IsTickerAllowed(ticker) {
		return fmt.Errorf("ticker %s is not in allowed list (ASTY, SOON, BITTY)", ticker)
	}

	holdersDir := filepath.Join("data_out", "holders_module", ticker)
	// Check, folder create
	if _, err := os.Stat(holdersDir); os.IsNotExist(err) {
		return fmt.Errorf("holders directory does not exist for ticker %s (only ASTY, SOON, BITTY are allowed)", ticker)
	}

	filename := filepath.Join(holdersDir, "dynamic_holders.json")

	dataBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal dynamic holders data: %w", err)
	}

	if err := os.WriteFile(filename, dataBytes, 0644); err != nil {
		return fmt.Errorf("failed to write dynamic holders file: %w", err)
	}

	return nil
}

func GetAllowedTickers() []string {
	return []string{"ASTY", "SOON", "BITTY"}
}

func IsTickerAllowed(ticker string) bool {
	allowed := GetAllowedTickers()
	for _, allowedTicker := range allowed {
		if strings.EqualFold(ticker, allowedTicker) {
			return true
		}
	}
	return false
}

// AddHolderToSaved in saved_holders.json
// address - or address wallet
// tokenAmount - count tokens
// for (ASTY, SOON, BITTY)
func AddHolderToSaved(ticker string, address string, tokenAmount string) error {
	if ticker == "" || address == "" || tokenAmount == "" {
		return fmt.Errorf("ticker, address and tokenAmount are required")
	}

	// Check, ticker for
	if !IsTickerAllowed(ticker) {
		LogDebug("Ticker not in allowed list, skipping holder save", zap.String("ticker", ticker))
		return nil // error,
	}

	savedData, err := LoadSavedHolders(ticker)
	if err != nil {
		return fmt.Errorf("failed to load saved holders: %w", err)
	}

	savedData.Holders[address] = tokenAmount

	// Save
	if err := SaveSavedHolders(ticker, savedData); err != nil {
		return fmt.Errorf("failed to save saved holders: %w", err)
	}

	LogDebug("Added holder to saved_holders", zap.String("ticker", ticker), zap.String("address", address), zap.String("amount", tokenAmount))
	return nil
}

// UpdateDynamicHoldersFromSwap dynamic_holders.json on swap
// - action wallet
func UpdateDynamicHoldersFromSwap(ticker string, swapperPublicKey string, currentAmount float64, previousAmount float64, action string, btcValue float64) error {
	if ticker == "" || swapperPublicKey == "" || action == "" {
		return fmt.Errorf("ticker, swapperPublicKey and action are required")
	}

	// Check, ticker
	if !IsTickerAllowed(ticker) {
		return fmt.Errorf("ticker %s is not in allowed list (ASTY, SOON, BITTY)", ticker)
	}

	// Load
	dynamicData, err := LoadDynamicHolders(ticker)
	if err != nil {
		return fmt.Errorf("failed to load dynamic holders: %w", err)
	}

	// Update
	currentDate := time.Now().Format("2006-01-02")

	if dynamicData.LastCheckDate != currentDate {
		dynamicData.DailyCounts = make(map[string]int)
	}
	dynamicData.LastCheckDate = currentDate

	// Initialize for addresses, if
	if dynamicData.Changes[swapperPublicKey] == nil {
		dynamicData.Changes[swapperPublicKey] = make([]BalanceChange, 0)
	}

	delta := currentAmount - previousAmount
	if action == "liquidated" {
		// delta = -previousAmount balance
		delta = -previousAmount
	}

	if dynamicData.DailyCounts == nil {
		dynamicData.DailyCounts = make(map[string]int)
	}
	dynamicData.DailyCounts[swapperPublicKey]++

	// in ->
	// - action wallet
	dynamicData.Changes[swapperPublicKey] = append(dynamicData.Changes[swapperPublicKey], BalanceChange{
		Amount: currentAmount,
		Delta:  delta,
		Action: action,
		Value:  btcValue,    // amount in BTC
		Date:   currentDate, // date in YYYY-MM-DD
	})

	// Save dynamic_holders.json
	if err := SaveDynamicHolders(ticker, dynamicData); err != nil {
		return fmt.Errorf("failed to save dynamic holders: %w", err)
	}

	LogDebug("Updated dynamic holders from swap",
		zap.String("ticker", ticker),
		zap.String("swapperPublicKey", swapperPublicKey),
		zap.Float64("btcValue", btcValue),
		zap.Float64("amount", currentAmount),
		zap.Float64("delta", delta),
		zap.String("action", action),
		zap.Int("dailyCount", dynamicData.DailyCounts[swapperPublicKey]))

	return nil
}

// GetTokenBalanceFromWallet balance token wallet by
// Uses API https://api.luminex.io/spark/address/{swapperPublicKey} (see wallet_balance.go)
// publicKey - wallet (swapperPublicKey)
// ticker - ticker token for in wallet
// balance in (raw balance) and count tokens decimals)
func GetTokenBalanceFromWallet(publicKey string, ticker string) (string, float64, error) {
	if publicKey == "" || ticker == "" {
		return "", 0, fmt.Errorf("public key and ticker are required")
	}

	// Get balance wallet API from wallet_balance.go
	// API endpoint: https://api.luminex.io/spark/address/{swapperPublicKey}
	balanceResp, err := GetWalletTokensBalance(publicKey)
	if err != nil {
		LogDebug("Failed to get wallet balance from API", zap.String("publicKey", publicKey), zap.String("ticker", ticker), zap.Error(err))
		return "", 0, fmt.Errorf("failed to get wallet balance from API https://api.luminex.io/spark/address/%s: %w", publicKey, err)
	}

	// token by ticker in wallet in GetWalletTokenHolding)
	for _, token := range balanceResp.Tokens {
		if token.Ticker == ticker {
			// Parse balance
			var balanceValue float64
			fmt.Sscanf(token.Balance, "%f", &balanceValue)

			// on 10^decimals for tokens
			decimalsMultiplier := 1.0
			for i := 0; i < token.Decimals; i++ {
				decimalsMultiplier *= 10
			}
			tokenAmount := balanceValue / decimalsMultiplier

			LogDebug("Found token balance", zap.String("publicKey", publicKey), zap.String("ticker", ticker), zap.Float64("amount", tokenAmount), zap.String("rawBalance", token.Balance))
			return token.Balance, tokenAmount, nil
		}
	}

	// token wallet - balance 0
	LogDebug("Token not found in wallet balance", zap.String("publicKey", publicKey), zap.String("ticker", ticker), zap.Int("tokensCount", len(balanceResp.Tokens)))
	return "0", 0, nil
}

// GetTokenAddressFromPoolLpPublicKey address token by poolLpPublicKey
func GetTokenAddressFromPoolLpPublicKey(poolLpPublicKey string, swap Swap) string {
	if swap.AssetOutAddress == NativeTokenAddress {
		// If get BTC, token (assetInAddress - token)
		return swap.AssetInAddress
	} else if swap.AssetInAddress == NativeTokenAddress {
		// If BTC, get token (assetOutAddress - token)
		return swap.AssetOutAddress
	}
	// - use
	if swap.PoolAssetBAddress == NativeTokenAddress {
		return swap.PoolAssetAAddress
	}
	return swap.PoolAssetBAddress
}

// GetTokenAddressFromTokenIdentifier address token by tokenIdentifier
// If get address from API or use address
func GetTokenAddressFromTokenIdentifier(tokenIdentifier string) string {
	// in tokenIdentifier token
	// If addresses from API
	return tokenIdentifier
}

// CheckHoldersBalance balance for token
// balance and dynamic_holders.json
// API from wallet_balance.go for by
// forceCheck - if true, if (for
func CheckHoldersBalance(ticker string, tokenAddress string) error {
	return CheckHoldersBalanceWithForce(ticker, tokenAddress, false)
}

// CheckHoldersBalanceWithForce balance for token
// forceCheck - if true, if
func CheckHoldersBalanceWithForce(ticker string, tokenAddress string, forceCheck bool) error {
	if ticker == "" {
		return fmt.Errorf("ticker is required")
	}

	// Load
	savedData, err := LoadSavedHolders(ticker)
	if err != nil {
		LogError("Failed to load saved holders", zap.String("ticker", ticker), zap.Error(err))
		return fmt.Errorf("failed to load saved holders: %w", err)
	}

	// If check
	if savedData == nil || len(savedData.Holders) == 0 {
		LogDebug("No saved holders found for ticker, skipping check", zap.String("ticker", ticker))
		return nil
	}

	// Load
	dynamicData, err := LoadDynamicHolders(ticker)
	if err != nil {
		LogError("Failed to load dynamic holders", zap.String("ticker", ticker), zap.Error(err))
		return fmt.Errorf("failed to load dynamic holders: %w", err)
	}

	// Get
	currentDate := time.Now().Format("2006-01-02")

	if dynamicData.LastCheckDate != currentDate {
		dynamicData.DailyCounts = make(map[string]int)
		// Update if
		dynamicData.LastCheckDate = currentDate
		// Save if
		if err := SaveDynamicHolders(ticker, dynamicData); err != nil {
			LogWarn("Failed to save lastCheckDate update", zap.String("ticker", ticker), zap.Error(err))
		}
	}

	// If forceCheck = true
	if !forceCheck && dynamicData.LastCheckDate == currentDate {
		LogDebug("Holders balance already checked today, skipping", zap.String("ticker", ticker), zap.String("lastCheckDate", dynamicData.LastCheckDate))
		// lastCheckDate if date
		return nil
	}

	// Update if
	if dynamicData.LastCheckDate != currentDate {
		dynamicData.LastCheckDate = currentDate
		dynamicData.DailyCounts = make(map[string]int)
	}

	LogInfo("Checking holders balance", zap.String("ticker", ticker), zap.Int("holdersCount", len(savedData.Holders)))

	// Check balance from saved_holders.json
	// addresses saveHolderFromSwap swap'
	// and and swap'
	const minBalanceThreshold = 10.0 // balance for (10 tokens)
	hasChanges := false
	changesDetected := 0
	liquidatedCount := 0
	for swapperPublicKey, savedBalanceStr := range savedData.Holders {
		// Parse balance from saved_holders.json
		var savedAmount float64
		if _, err := fmt.Sscanf(savedBalanceStr, "%f", &savedAmount); err != nil {
			LogWarn("Failed to parse saved balance", zap.String("swapperPublicKey", swapperPublicKey), zap.String("savedBalanceStr", savedBalanceStr), zap.Error(err))
			continue
		}

		// Get balance API from wallet_balance.go
		// API: https://api.luminex.io/spark/address/{swapperPublicKey}
		// Use swapperPublicKey and ticker for token
		_, currentAmount, err := GetTokenBalanceFromWallet(swapperPublicKey, ticker)
		if err != nil {
			LogWarn("Failed to get wallet balance", zap.String("swapperPublicKey", swapperPublicKey), zap.String("ticker", ticker), zap.Error(err))
			continue
		}

		// balance from saved_holders.json
		// Use for float (0.0001)
		const epsilon = 0.0001
		balanceDiff := currentAmount - savedAmount

		// If balance -
		// addresses saveHolderFromSwap swap'
		// and and swap'
		if balanceDiff > epsilon || balanceDiff < -epsilon {
			hasChanges = true
			changesDetected++

			var action string

			if currentAmount == 0 {
				// - balance 0
				action = "liquidated"
				liquidatedCount++
				// Remove from saved_holders
				delete(savedData.Holders, swapperPublicKey)
			} else if currentAmount < minBalanceThreshold {
				// balance 10 tokens - remove from saved_holders
				action = "liquidated"
				liquidatedCount++
				delete(savedData.Holders, swapperPublicKey)
			} else if currentAmount > savedAmount {
				// balance -
				action = "invested"
			} else {
				// balance -
				action = "sold"
			}

			// Add in dynamic_holders
			if dynamicData.Changes[swapperPublicKey] == nil {
				dynamicData.Changes[swapperPublicKey] = make([]BalanceChange, 0)
			}

			delta := currentAmount - savedAmount
			if action == "liquidated" {
				// delta = -savedAmount balance
				delta = -savedAmount
			}

			if dynamicData.DailyCounts == nil {
				dynamicData.DailyCounts = make(map[string]int)
			}
			dynamicData.DailyCounts[swapperPublicKey]++

			// Add in ->
			// - action wallet
			// in swap, Value = 0
			dynamicData.Changes[swapperPublicKey] = append(dynamicData.Changes[swapperPublicKey], BalanceChange{
				Amount: currentAmount,
				Delta:  delta,
				Action: action,
				Value:  0,           // in swap, Value = 0
				Date:   currentDate, // date in YYYY-MM-DD
			})

			// Update saved_holders (if balance >= 10 tokens)
			if currentAmount >= minBalanceThreshold {
				savedData.Holders[swapperPublicKey] = fmt.Sprintf("%.8f", currentAmount)
			}

			LogInfo("Holder balance changed",
				zap.String("ticker", ticker),
				zap.String("swapperPublicKey", swapperPublicKey),
				zap.Float64("oldBalance", savedAmount),
				zap.Float64("newBalance", currentAmount),
				zap.String("action", action),
				zap.String("source", "periodic_check"))
		}
	}

	// Update if
	if dynamicData.LastCheckDate != currentDate {
		dynamicData.LastCheckDate = currentDate
		dynamicData.DailyCounts = make(map[string]int)
	}

	// Save if lastCheckDate)
	if hasChanges {
		if err := SaveSavedHolders(ticker, savedData); err != nil {
			LogError("Failed to save saved holders after check", zap.String("ticker", ticker), zap.Error(err))
			return fmt.Errorf("failed to save saved holders: %w", err)
		}
	}
	if err := SaveDynamicHolders(ticker, dynamicData); err != nil {
		LogError("Failed to save dynamic holders after check", zap.String("ticker", ticker), zap.Error(err))
		return fmt.Errorf("failed to save dynamic holders: %w", err)
	}

	if hasChanges {
		LogInfo("Holders balance check completed with changes",
			zap.String("ticker", ticker),
			zap.Int("changesDetected", changesDetected),
			zap.Int("liquidatedCount", liquidatedCount))
	} else {
		LogInfo("Holders balance check completed - no changes detected", zap.String("ticker", ticker))
	}

	return nil
}

// GetTickerFromTokenAddress ticker token by from id_tokens.json
func GetTickerFromTokenAddress(tokenAddress string) (string, error) {
	tokenIDsFile := "data_out/holders_module/id_tokens.json"
	tokenIDs, err := LoadTokenIdentifiers(tokenIDsFile)
	if err != nil {
		return "", fmt.Errorf("failed to load token identifiers: %w", err)
	}

	// ticker by token
	// in id_tokens.json tokenIdentifier -> ticker
	// tokenIdentifier by tokenAddress
	// for in holders_module
	for tokenIdentifier, ticker := range tokenIDs {
		// Check, tokenIdentifier tokenAddress
		// in tokenIdentifier tokenAddress
		if tokenIdentifier == tokenAddress {
			return ticker, nil
		}
	}

	return "", fmt.Errorf("ticker not found for token address: %s", tokenAddress)
}

// GetTickerFromPoolLpPublicKey ticker token by poolLpPublicKey
func GetTickerFromPoolLpPublicKey(poolLpPublicKey string) (string, error) {
	tokenMetadata := GetTokenMetadata(poolLpPublicKey)
	if tokenMetadata != nil && tokenMetadata.Ticker != "" {
		return tokenMetadata.Ticker, nil
	}

	return "", fmt.Errorf("ticker not found for poolLpPublicKey: %s", poolLpPublicKey)
}

// FormatTokenAmountForSaved count tokens for in saved_holders.json
// count tokens from swap and for
func FormatTokenAmountForSaved(amountStr string, decimals int) string {
	// Parse count
	var amountValue float64
	fmt.Sscanf(amountStr, "%f", &amountValue)

	// on 10^decimals for tokens
	decimalsMultiplier := 1.0
	for i := 0; i < decimals; i++ {
		decimalsMultiplier *= 10
	}
	tokenAmount := amountValue / decimalsMultiplier

	// Format
	return fmt.Sprintf("%.8f", tokenAmount)
}

// GetTokenDecimalsFromSwap decimals token from swap
func GetTokenDecimalsFromSwap(swap Swap, poolLpPublicKey string, ticker string) int {
	return GetTokenDecimals(poolLpPublicKey, swap, ticker)
}
