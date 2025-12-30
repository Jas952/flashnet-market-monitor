package holders

// Package system_works contains for tokens
// on saved_holders.json

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"spark-wallet/internal/clients_api/flashnet"
	logging "spark-wallet/internal/infra/log"

	"go.uber.org/zap"
)

// Holder is a minimal legacy holder record used only for converting old saved_holders.json format.
type Holder struct {
	Balance string `json:"balance"`
}

// TokenHoldersData is a legacy format used for converting old saved_holders.json format.
type TokenHoldersData struct {
	TokenIdentifier string            `json:"tokenIdentifier"`
	Ticker          string            `json:"ticker"`
	LastUpdated     string            `json:"lastUpdated"`
	Holders         map[string]Holder `json:"holders"`
	TotalCount      int               `json:"totalCount"`
}

// LoadTokenIdentifiers loads tokenIdentifier -> ticker map from JSON file.
// Returns empty map if file doesn't exist (not an error).
func LoadTokenIdentifiers(filename string) (map[string]string, error) {
	type tokenIDsFile struct {
		IDTokens map[string]string `json:"id_tokens"`
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, fmt.Errorf("failed to read token identifiers file: %w", err)
	}

	var file tokenIDsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("failed to parse token identifiers JSON: %w", err)
	}

	if file.IDTokens == nil {
		return make(map[string]string), nil
	}

	return file.IDTokens, nil
}

// WalletBalanceResponse is a minimal response for /spark/address/{pubkey}.
// We keep only fields used by this holders module.
type WalletBalanceResponse struct {
	Tokens []WalletToken `json:"tokens"`
}

// WalletToken is a minimal token record used by holders checks.
type WalletToken struct {
	TokenIdentifier string `json:"tokenIdentifier"`
	TokenAddress    string `json:"tokenAddress"`
	Ticker          string `json:"ticker"`
	Decimals        int    `json:"decimals"`
	Balance         string `json:"balance"`
}

const luminexAddressAPIBaseURL = "https://api.luminex.io/spark/address"

// GetWalletTokensBalance fetches wallet tokens data from Luminex.
func GetWalletTokensBalance(publicKey string) (*WalletBalanceResponse, error) {
	if publicKey == "" {
		return nil, fmt.Errorf("public key is empty")
	}

	url := fmt.Sprintf("%s/%s", luminexAddressAPIBaseURL, publicKey)
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Cloudflare-friendly headers (same approach as old system_works implementation).
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://luminex.io/")
	req.Header.Set("Origin", "https://luminex.io")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from Luminex API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("luminex API returned status %d", resp.StatusCode)
	}

	var balanceResp WalletBalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&balanceResp); err != nil {
		return nil, fmt.Errorf("failed to decode Luminex API response: %w", err)
	}
	return &balanceResp, nil
}

// TokenMetadata is a minimal token metadata record used by holders module.
type TokenMetadata struct {
	Name   string `json:"name"`
	Ticker string `json:"ticker"`
}

const tokenCacheFile = "data_out/saved_ticket.json"

type savedTicketsFile struct {
	Tickets map[string]string `json:"tickets"` // poolLpPublicKey -> "TICKER:Name"
}

// GetTokenMetadata returns token metadata (ticker/name) from local cache file.
// This keeps holders module working without depending on internal/luminex token_metadata yet.
func GetTokenMetadata(poolLpPublicKey string) *TokenMetadata {
	if poolLpPublicKey == "" {
		return nil
	}

	data, err := os.ReadFile(tokenCacheFile)
	if err != nil {
		return nil
	}

	var saved savedTicketsFile
	if err := json.Unmarshal(data, &saved); err != nil {
		return nil
	}
	if saved.Tickets == nil {
		return nil
	}

	v, ok := saved.Tickets[poolLpPublicKey]
	if !ok || strings.TrimSpace(v) == "" {
		return nil
	}

	parts := strings.SplitN(v, ":", 2)
	md := &TokenMetadata{Ticker: strings.TrimSpace(parts[0])}
	if len(parts) == 2 {
		md.Name = strings.TrimSpace(parts[1])
	}
	return md
}

const luminexPoolAPIBaseURL = "https://api.luminex.io/spark/pool"

type luminexPoolResponse struct {
	AssetAAddress  string                    `json:"assetAAddress"`
	AssetBAddress  string                    `json:"assetBAddress"`
	TokenAMetadata luminexTokenMetadataBrief `json:"tokenAMetadata"`
	TokenBMetadata luminexTokenMetadataBrief `json:"tokenBMetadata"`
}

type luminexTokenMetadataBrief struct {
	Ticker   string `json:"ticker"`
	Decimals int    `json:"decimals"`
}

// GetTokenDecimals returns token decimals (best-effort). Defaults to 8 on any failure.
func GetTokenDecimals(poolLpPublicKey string, swap flashnet.Swap, ticker string) int {
	if poolLpPublicKey == "" {
		return 8
	}

	url := fmt.Sprintf("%s/%s", luminexPoolAPIBaseURL, poolLpPublicKey)
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logging.LogDebug("Failed to create request for token decimals", zap.Error(err))
		return 8
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://luminex.io/")
	req.Header.Set("Origin", "https://luminex.io")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		logging.LogDebug("Failed to fetch token decimals from Luminex API", zap.Error(err))
		return 8
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logging.LogDebug("Luminex pool API returned non-OK status for token decimals", zap.Int("status", resp.StatusCode))
		return 8
	}

	var poolResp luminexPoolResponse
	if err := json.NewDecoder(resp.Body).Decode(&poolResp); err != nil {
		logging.LogDebug("Failed to decode Luminex pool API response for token decimals", zap.Error(err))
		return 8
	}

	// Determine the "token side" (not BTC) using swap info.
	var tokenAddress string
	if swap.AssetOutAddress == flashnet.NativeTokenAddress {
		tokenAddress = swap.AssetInAddress
	} else if swap.AssetInAddress == flashnet.NativeTokenAddress {
		tokenAddress = swap.AssetOutAddress
	} else {
		if swap.PoolAssetBAddress == flashnet.NativeTokenAddress {
			tokenAddress = swap.PoolAssetAAddress
		} else {
			tokenAddress = swap.PoolAssetBAddress
		}
	}

	// Best-effort selection; we don't have asset addresses in swap, so rely on ticker if provided.
	if ticker != "" {
		if strings.EqualFold(poolResp.TokenAMetadata.Ticker, ticker) && poolResp.TokenAMetadata.Decimals > 0 {
			return poolResp.TokenAMetadata.Decimals
		}
		if strings.EqualFold(poolResp.TokenBMetadata.Ticker, ticker) && poolResp.TokenBMetadata.Decimals > 0 {
			return poolResp.TokenBMetadata.Decimals
		}
	}

	// Fallback: if tokenAddress known, try to pick opposite side of BTC by comparing pool assets.
	if tokenAddress != "" {
		if poolResp.AssetAAddress == tokenAddress && poolResp.TokenAMetadata.Decimals > 0 {
			return poolResp.TokenAMetadata.Decimals
		}
		if poolResp.AssetBAddress == tokenAddress && poolResp.TokenBMetadata.Decimals > 0 {
			return poolResp.TokenBMetadata.Decimals
		}
	}

	// Last resort: pick non-zero.
	if poolResp.TokenAMetadata.Decimals > 0 {
		return poolResp.TokenAMetadata.Decimals
	}
	if poolResp.TokenBMetadata.Decimals > 0 {
		return poolResp.TokenBMetadata.Decimals
	}
	return 8
}

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
		logging.LogWarn("Failed to save converted holders data", zap.String("ticker", ticker), zap.Error(err))
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
		logging.LogError("Failed to read dynamic holders file", zap.String("ticker", ticker), zap.String("filename", filename), zap.Error(err))
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
		logging.LogError("Failed to parse dynamic holders JSON", zap.String("ticker", ticker), zap.String("filename", filename), zap.String("data", string(data)), zap.Error(err))
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
				logging.LogWarn("Failed to save converted dynamic holders", zap.String("ticker", ticker), zap.Error(err))
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
		logging.LogDebug("Ticker not in allowed list, skipping holder save", zap.String("ticker", ticker))
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

	logging.LogDebug("Added holder to saved_holders", zap.String("ticker", ticker), zap.String("address", address), zap.String("amount", tokenAmount))
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

	logging.LogDebug("Updated dynamic holders from swap",
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
		logging.LogDebug("Failed to get wallet balance from API", zap.String("publicKey", publicKey), zap.String("ticker", ticker), zap.Error(err))
		return "", 0, fmt.Errorf("failed to get wallet balance from API https://api.luminex.io/spark/address/%s: %w", publicKey, err)
	}

	// token by ticker in wallet in GetWalletTokenHolding)
	for _, token := range balanceResp.Tokens {
		if token.Ticker == ticker {
			// Parse balance
			var balanceValue float64
			n, err := fmt.Sscanf(token.Balance, "%f", &balanceValue)
			if err != nil || n != 1 {
				logging.LogWarn("Failed to parse token balance",
					zap.String("ticker", ticker),
					zap.String("balance", token.Balance),
					zap.Error(err))
				continue
			}

			// on 10^decimals for tokens
			decimalsMultiplier := 1.0
			for i := 0; i < token.Decimals; i++ {
				decimalsMultiplier *= 10
			}
			tokenAmount := balanceValue / decimalsMultiplier

			logging.LogDebug("Found token balance", zap.String("publicKey", publicKey), zap.String("ticker", ticker), zap.Float64("amount", tokenAmount), zap.String("rawBalance", token.Balance))
			return token.Balance, tokenAmount, nil
		}
	}

	// token wallet - balance 0
	logging.LogDebug("Token not found in wallet balance", zap.String("publicKey", publicKey), zap.String("ticker", ticker), zap.Int("tokensCount", len(balanceResp.Tokens)))
	return "0", 0, nil
}

// GetTokenAddressFromPoolLpPublicKey address token by poolLpPublicKey
func GetTokenAddressFromPoolLpPublicKey(poolLpPublicKey string, swap flashnet.Swap) string {
	if swap.AssetOutAddress == flashnet.NativeTokenAddress {
		// If get BTC, token (assetInAddress - token)
		return swap.AssetInAddress
	} else if swap.AssetInAddress == flashnet.NativeTokenAddress {
		// If BTC, get token (assetOutAddress - token)
		return swap.AssetOutAddress
	}
	// - use
	if swap.PoolAssetBAddress == flashnet.NativeTokenAddress {
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
		logging.LogError("Failed to load saved holders", zap.String("ticker", ticker), zap.Error(err))
		return fmt.Errorf("failed to load saved holders: %w", err)
	}

	// If check
	if savedData == nil || len(savedData.Holders) == 0 {
		logging.LogDebug("No saved holders found for ticker, skipping check", zap.String("ticker", ticker))
		return nil
	}

	// Load
	dynamicData, err := LoadDynamicHolders(ticker)
	if err != nil {
		logging.LogError("Failed to load dynamic holders", zap.String("ticker", ticker), zap.Error(err))
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
			logging.LogWarn("Failed to save lastCheckDate update", zap.String("ticker", ticker), zap.Error(err))
		}
	}

	// If forceCheck = true
	if !forceCheck && dynamicData.LastCheckDate == currentDate {
		logging.LogDebug("Holders balance already checked today, skipping", zap.String("ticker", ticker), zap.String("lastCheckDate", dynamicData.LastCheckDate))
		// lastCheckDate if date
		return nil
	}

	// Update if
	if dynamicData.LastCheckDate != currentDate {
		dynamicData.LastCheckDate = currentDate
		dynamicData.DailyCounts = make(map[string]int)
	}

	logging.LogInfo("Checking holders balance", zap.String("ticker", ticker), zap.Int("holdersCount", len(savedData.Holders)))

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
		n, err := fmt.Sscanf(savedBalanceStr, "%f", &savedAmount)
		if err != nil || n != 1 {
			logging.LogWarn("Failed to parse saved balance", zap.String("swapperPublicKey", swapperPublicKey), zap.String("savedBalanceStr", savedBalanceStr), zap.Error(err))
			continue
		}

		// Get balance API from wallet_balance.go
		// API: https://api.luminex.io/spark/address/{swapperPublicKey}
		// Use swapperPublicKey and ticker for token
		_, currentAmount, err := GetTokenBalanceFromWallet(swapperPublicKey, ticker)
		if err != nil {
			logging.LogWarn("Failed to get wallet balance", zap.String("swapperPublicKey", swapperPublicKey), zap.String("ticker", ticker), zap.Error(err))
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

			logging.LogInfo("Holder balance changed",
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
			logging.LogError("Failed to save saved holders after check", zap.String("ticker", ticker), zap.Error(err))
			return fmt.Errorf("failed to save saved holders: %w", err)
		}
	}
	if err := SaveDynamicHolders(ticker, dynamicData); err != nil {
		logging.LogError("Failed to save dynamic holders after check", zap.String("ticker", ticker), zap.Error(err))
		return fmt.Errorf("failed to save dynamic holders: %w", err)
	}

	if hasChanges {
		logging.LogInfo("Holders balance check completed with changes",
			zap.String("ticker", ticker),
			zap.Int("changesDetected", changesDetected),
			zap.Int("liquidatedCount", liquidatedCount))
	} else {
		logging.LogInfo("Holders balance check completed - no changes detected", zap.String("ticker", ticker))
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
	n, err := fmt.Sscanf(amountStr, "%f", &amountValue)
	if err != nil || n != 1 {
		logging.LogWarn("Failed to parse token amount in FormatTokenAmountForSaved",
			zap.String("amountStr", amountStr),
			zap.Error(err))
		return "0"
	}

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
func GetTokenDecimalsFromSwap(swap flashnet.Swap, poolLpPublicKey string, ticker string) int {
	return GetTokenDecimals(poolLpPublicKey, swap, ticker)
}
