package system_works

// Package system_works contains for

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

const (
	// FilteredTokensFile -
	FilteredTokensFile = "data_out/filtered_tokens.json"
)

// FilteredTokensData - for tokens
type FilteredTokensData struct {
	Tokens []string `json:"tokens"` // poolLpPublicKey tokens for
}

// LoadFilteredTokens tokens from file
func LoadFilteredTokens() ([]string, error) {
	filePath := FilteredTokensFile

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		LogDebug("Filtered tokens file does not exist, returning empty list", zap.String("file", filePath))
		return []string{}, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read filtered tokens file: %w", err)
	}

	// Check, file
	if len(data) == 0 || strings.TrimSpace(string(data)) == "" || strings.TrimSpace(string(data)) == "{}" {
		LogDebug("Filtered tokens file is empty, returning empty list", zap.String("file", filePath))
		return []string{}, nil
	}

	// Parse JSON
	var tokensData FilteredTokensData
	if err := json.Unmarshal(data, &tokensData); err != nil {
		return nil, fmt.Errorf("failed to parse filtered tokens JSON: %w", err)
	}

	LogDebug("Loaded filtered tokens from file",
		zap.String("file", filePath),
		zap.Int("count", len(tokensData.Tokens)))

	return tokensData.Tokens, nil
}

// SaveFilteredTokens tokens in file
func SaveFilteredTokens(tokens []string) error {
	filePath := FilteredTokensFile

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	tokensData := FilteredTokensData{
		Tokens: tokens,
	}

	data, err := json.MarshalIndent(tokensData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal filtered tokens JSON: %w", err)
	}

	// Use file,
	tempFilePath := filePath + ".tmp"
	if err := os.WriteFile(tempFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary filtered tokens file: %w", err)
	}

	// on Unix-
	if err := os.Rename(tempFilePath, filePath); err != nil {
		os.Remove(tempFilePath)
		return fmt.Errorf("failed to rename temporary file to filtered tokens file: %w", err)
	}

	LogInfo("Saved filtered tokens to file",
		zap.String("file", filePath),
		zap.Int("count", len(tokens)))

	return nil
}

// AddFilteredToken token in tokens (if
func AddFilteredToken(poolLpPublicKey string) error {
	if poolLpPublicKey == "" {
		return fmt.Errorf("poolLpPublicKey cannot be empty")
	}

	// Load
	tokens, err := LoadFilteredTokens()
	if err != nil {
		return fmt.Errorf("failed to load filtered tokens: %w", err)
	}

	// Check, token
	for _, token := range tokens {
		if strings.TrimSpace(token) == poolLpPublicKey {
			LogDebug("Token already in filtered list", zap.String("poolLpPublicKey", poolLpPublicKey))
			return nil // token
		}
	}

	tokens = append(tokens, poolLpPublicKey)

	// Save
	if err := SaveFilteredTokens(tokens); err != nil {
		return fmt.Errorf("failed to save filtered tokens: %w", err)
	}

	// Check, file
	verifyTokens, err := LoadFilteredTokens()
	if err != nil {
		LogWarn("Failed to verify saved tokens", zap.Error(err))
	} else {
		found := false
		for _, token := range verifyTokens {
			if strings.TrimSpace(token) == poolLpPublicKey {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("token was not found in file after save")
		}
	}

	LogInfo("Added token to filtered list",
		zap.String("poolLpPublicKey", poolLpPublicKey),
		zap.Int("totalCount", len(tokens)))

	return nil
}

// RemoveFilteredToken token from tokens
func RemoveFilteredToken(poolLpPublicKey string) error {
	if poolLpPublicKey == "" {
		return fmt.Errorf("poolLpPublicKey cannot be empty")
	}

	// Load
	tokens, err := LoadFilteredTokens()
	if err != nil {
		return fmt.Errorf("failed to load filtered tokens: %w", err)
	}

	// token in and remove
	found := false
	var updatedTokens []string
	for _, token := range tokens {
		if strings.TrimSpace(token) != poolLpPublicKey {
			updatedTokens = append(updatedTokens, token)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("token not found in list")
	}

	// Save
	if err := SaveFilteredTokens(updatedTokens); err != nil {
		return fmt.Errorf("failed to save filtered tokens: %w", err)
	}

	LogInfo("Removed token from filtered list",
		zap.String("poolLpPublicKey", poolLpPublicKey),
		zap.Int("totalCount", len(updatedTokens)))

	return nil
}

// MigrateTokensFromEnv from env in file (if file
// tokensFromEnv - tokens from env
func MigrateTokensFromEnv(tokensFromEnv string) bool {
	if tokensFromEnv == "" {
		return false
	}

	existingTokens, err := LoadFilteredTokens()
	if err == nil && len(existingTokens) > 0 {
		LogDebug("Filtered tokens file already contains tokens, skipping migration", zap.Int("count", len(existingTokens)))
		return false
	}

	// Parse from env "token1,token2,token3")
	tokens := strings.Split(tokensFromEnv, ",")
	var validTokens []string
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token != "" {
			validTokens = append(validTokens, token)
		}
	}

	if len(validTokens) == 0 {
		return false
	}

	if err := SaveFilteredTokens(validTokens); err != nil {
		LogWarn("Failed to migrate tokens from env to file", zap.Error(err))
		return false
	}

	LogInfo("Migrated filtered tokens from env to file",
		zap.Int("count", len(validTokens)),
		zap.Strings("tokens", validTokens))
	return true
}

// FindPoolLpPublicKeyByTicker poolLpPublicKey by ticker in saved_ticket.json
func FindPoolLpPublicKeyByTicker(ticker string) (string, error) {
	if ticker == "" {
		return "", fmt.Errorf("ticker cannot be empty")
	}

	// Load saved_ticket.json
	filePath := "data_out/saved_ticket.json"
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read saved_ticket.json: %w", err)
	}

	// Parse JSON
	var ticketsData struct {
		Tickets map[string]string `json:"tickets"` // poolLpPublicKey -> "TICKER:Name"
	}
	if err := json.Unmarshal(data, &ticketsData); err != nil {
		return "", fmt.Errorf("failed to parse saved_ticket.json: %w", err)
	}

	// token by ticker
	tickerUpper := strings.ToUpper(strings.TrimSpace(ticker))
	for poolLpPublicKey, ticketValue := range ticketsData.Tickets {
		// ticketValue "TICKER:Name", TICKER
		parts := strings.Split(ticketValue, ":")
		if len(parts) > 0 {
			ticketTicker := strings.ToUpper(strings.TrimSpace(parts[0]))
			if ticketTicker == tickerUpper {
				LogDebug("Found poolLpPublicKey by ticker",
					zap.String("ticker", ticker),
					zap.String("poolLpPublicKey", poolLpPublicKey),
					zap.String("ticketValue", ticketValue))
				return poolLpPublicKey, nil
			}
		}
	}

	return "", fmt.Errorf("ticker '%s' not found in saved_ticket.json", ticker)
}
