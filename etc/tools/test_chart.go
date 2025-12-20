package main

import (
	"fmt"
	"os"
	"spark-wallet/internal/features/tg_charts"
)

// go run etc/tools/test_chart.go
// in etc/charts/volume_chart.png
func main() {
	fmt.Println("Generating test chart...")

	chartPath, err := tg_charts.GenerateVolumeChart()
	if err != nil {
		fmt.Printf("Error generating chart: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Chart generated successfully: %s\n", chartPath)
	fmt.Println("Open the file to see the result!")
}
