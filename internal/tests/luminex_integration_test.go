//go:build integration

package tests

import (
	"testing"

	luminex "spark-wallet/internal/clients_api/luminex"
)

func TestIntegration_Luminex_GetStats(t *testing.T) {
	stats, err := luminex.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if stats == nil {
		t.Fatalf("expected stats, got nil")
	}
	if stats.TotalPools <= 0 {
		t.Fatalf("expected TotalPools > 0, got %d", stats.TotalPools)
	}
}

func TestIntegration_Luminex_GetBTCSparkReserve(t *testing.T) {
	reserve, err := luminex.GetBTCSparkReserve()
	if err != nil {
		t.Fatalf("GetBTCSparkReserve failed: %v", err)
	}
	if reserve <= 0 {
		t.Fatalf("expected reserve > 0, got %f", reserve)
	}
}
