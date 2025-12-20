package fs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"spark-wallet/internal/clients_api/flashnet"
)

const jsonsDir = "data_out"

func ensureJsonsDir() error {
	if err := os.MkdirAll(filepath.Join(jsonsDir, "big_sales_module"), 0755); err != nil {
		return err
	}
	return os.MkdirAll(jsonsDir, 0755)
}

func SaveSwapsResponse(filename string, data *flashnet.SwapsResponse) error {
	if err := ensureJsonsDir(); err != nil {
		return fmt.Errorf("failed to create jsons directory: %w", err)
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal swaps response: %w", err)
	}

	fullPath := filepath.Join(jsonsDir, filename)
	if err := os.WriteFile(fullPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to save swaps response: %w", err)
	}
	return nil
}

func SaveUserSwapsResponse(filename string, data interface{}) error {
	if err := ensureJsonsDir(); err != nil {
		return fmt.Errorf("failed to create jsons directory: %w", err)
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal user swaps response: %w", err)
	}

	fullPath := filepath.Join(jsonsDir, filename)
	if err := os.WriteFile(fullPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to save user swaps response: %w", err)
	}
	return nil
}

// LoadSwapsResponse loads swaps response from JSON file under data_out.
func LoadSwapsResponse(filename string) (*flashnet.SwapsResponse, error) {
	fullPath := filepath.Join(jsonsDir, filename)

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read swaps response file: %w", err)
	}

	var swapsResp flashnet.SwapsResponse
	if err := json.Unmarshal(data, &swapsResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal swaps response: %w", err)
	}

	return &swapsResp, nil
}

func LoadUserSwapsResponse(filename string) (*flashnet.UserSwapsResponse, error) {
	fullPath := filepath.Join(jsonsDir, filename)

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read user swaps response file: %w", err)
	}

	var userSwapsResp flashnet.UserSwapsResponse
	if err := json.Unmarshal(data, &userSwapsResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user swaps response: %w", err)
	}

	return &userSwapsResp, nil
}

func SaveLast10Swaps(filename string, swaps []flashnet.Swap) error {
	maxSwaps := 10
	if len(swaps) < maxSwaps {
		maxSwaps = len(swaps)
	}

	lastSwaps := swaps[:maxSwaps]

	response := flashnet.SwapsResponse{
		Swaps:      lastSwaps,
		TotalCount: len(lastSwaps),
	}

	return SaveSwapsResponse(filename, &response)
}
