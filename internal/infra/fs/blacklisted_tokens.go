package fs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	logging "spark-wallet/internal/infra/log"

	"go.uber.org/zap"
)

const (
	BlacklistedTokensFile = "data_out/blacklisted_tokens.json"
)

type BlacklistedTokensData struct {
	Tokens []string `json:"tokens"`
}

func LoadBlacklistedTokens() ([]string, error) {
	filePath := BlacklistedTokensFile

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		logging.LogDebug("Blacklisted tokens file does not exist, returning empty list", zap.String("file", filePath))
		return []string{}, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read blacklisted tokens file: %w", err)
	}

	if len(data) == 0 || strings.TrimSpace(string(data)) == "" || strings.TrimSpace(string(data)) == "{}" {
		logging.LogDebug("Blacklisted tokens file is empty, returning empty list", zap.String("file", filePath))
		return []string{}, nil
	}

	var tokensData BlacklistedTokensData
	if err := json.Unmarshal(data, &tokensData); err != nil {
		return nil, fmt.Errorf("failed to parse blacklisted tokens JSON: %w", err)
	}

	logging.LogDebug("Loaded blacklisted tokens from file",
		zap.String("file", filePath),
		zap.Int("count", len(tokensData.Tokens)))

	return tokensData.Tokens, nil
}

func SaveBlacklistedTokens(tokens []string) error {
	filePath := BlacklistedTokensFile

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	tokensData := BlacklistedTokensData{
		Tokens: tokens,
	}

	data, err := json.MarshalIndent(tokensData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal blacklisted tokens JSON: %w", err)
	}

	tempFilePath := filePath + ".tmp"
	if err := os.WriteFile(tempFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary blacklisted tokens file: %w", err)
	}

	if err := os.Rename(tempFilePath, filePath); err != nil {
		os.Remove(tempFilePath)
		return fmt.Errorf("failed to rename temporary file to blacklisted tokens file: %w", err)
	}

	logging.LogInfo("Saved blacklisted tokens to file",
		zap.String("file", filePath),
		zap.Int("count", len(tokens)))

	return nil
}

func AddBlacklistedToken(poolLpPublicKey string) error {
	if poolLpPublicKey == "" {
		return fmt.Errorf("poolLpPublicKey cannot be empty")
	}

	tokens, err := LoadBlacklistedTokens()
	if err != nil {
		return fmt.Errorf("failed to load blacklisted tokens: %w", err)
	}

	for _, token := range tokens {
		if strings.TrimSpace(token) == poolLpPublicKey {
			logging.LogDebug("Token already in blacklisted list", zap.String("poolLpPublicKey", poolLpPublicKey))
			return nil
		}
	}

	tokens = append(tokens, poolLpPublicKey)

	if err := SaveBlacklistedTokens(tokens); err != nil {
		return fmt.Errorf("failed to save blacklisted tokens: %w", err)
	}

	verifyTokens, err := LoadBlacklistedTokens()
	if err != nil {
		logging.LogWarn("Failed to verify saved tokens", zap.Error(err))
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

	logging.LogInfo("Added token to blacklisted list",
		zap.String("poolLpPublicKey", poolLpPublicKey),
		zap.Int("totalCount", len(tokens)))

	return nil
}

func RemoveBlacklistedToken(poolLpPublicKey string) error {
	if poolLpPublicKey == "" {
		return fmt.Errorf("poolLpPublicKey cannot be empty")
	}

	tokens, err := LoadBlacklistedTokens()
	if err != nil {
		return fmt.Errorf("failed to load blacklisted tokens: %w", err)
	}

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

	if err := SaveBlacklistedTokens(updatedTokens); err != nil {
		return fmt.Errorf("failed to save blacklisted tokens: %w", err)
	}

	logging.LogInfo("Removed token from blacklisted list",
		zap.String("poolLpPublicKey", poolLpPublicKey),
		zap.Int("totalCount", len(updatedTokens)))

	return nil
}

func IsTokenBlacklisted(poolLpPublicKey string, blacklistedTokens []string) bool {
	if poolLpPublicKey == "" || len(blacklistedTokens) == 0 {
		return false
	}
	for _, token := range blacklistedTokens {
		if strings.TrimSpace(token) == poolLpPublicKey {
			return true
		}
	}
	return false
}
