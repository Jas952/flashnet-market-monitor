package system_works

// BTC reserve (Spark) for Telegram.

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"time"

	"github.com/fogleman/gg"
	"go.uber.org/zap"
)

// GenerateBTCSparkChart BTC reserve by btc_spark.json.
func GenerateBTCSparkChart() (string, error) {
	btcSparkData, err := LoadBTCSparkData()
	if err != nil {
		return "", fmt.Errorf("failed to load BTC spark data: %w", err)
	}

	if len(btcSparkData.Entries) == 0 {
		return "", fmt.Errorf("no BTC spark data available")
	}

	var currentBTCReserve float64
	if len(btcSparkData.Entries) > 0 {
		currentBTCReserve = btcSparkData.Entries[len(btcSparkData.Entries)-1].BtcReserve
	}

	var points []struct {
		Timestamp time.Time
		Reserve   float64
		DateLabel string
	}

	for _, entry := range btcSparkData.Entries {
		var timestamp time.Time
		var err error

		if entry.Timestamp != "" {
			timestamp, err = time.Parse(time.RFC3339, entry.Timestamp)
			if err != nil {
				timestamp, err = time.Parse("2006-01-02", entry.Date)
				if err != nil {
					continue
				}
			}
		} else {
			timestamp, err = time.Parse("2006-01-02", entry.Date)
			if err != nil {
				continue
			}
		}

		dateLabel := timestamp.Format("02.01")

		points = append(points, struct {
			Timestamp time.Time
			Reserve   float64
			DateLabel string
		}{
			Timestamp: timestamp,
			Reserve:   entry.BtcReserve,
			DateLabel: dateLabel,
		})
	}

	if len(points) == 0 {
		return "", fmt.Errorf("no valid BTC spark data points available")
	}

	// by time (on
	for i := 0; i < len(points)-1; i++ {
		for j := i + 1; j < len(points); j++ {
			if points[i].Timestamp.After(points[j].Timestamp) {
				points[i], points[j] = points[j], points[i]
			}
		}
	}

	// create from stats_chart.go.
	dc := gg.NewContext(chartWidth, chartHeight)

	dc.SetColor(color.Black)
	dc.Clear()

	// Load spark.png (etc/telegram).
	sparkLogoPath := filepath.Join("etc", "telegram", "spark.png")
	logoPaths := []string{
		sparkLogoPath,
		filepath.Join(".", "etc", "telegram", "spark.png"),
		filepath.Join("..", "etc", "telegram", "spark.png"),
		filepath.Join("..", "..", "etc", "telegram", "spark.png"),
	}

	var logoImg image.Image
	var logoLoaded bool
	for _, logoPath := range logoPaths {
		if _, err := os.Stat(logoPath); err == nil {
			img, err := gg.LoadImage(logoPath)
			if err == nil {
				logoImg = img
				logoLoaded = true
				LogInfo("Loaded spark.png logo for BTC chart", zap.String("path", logoPath))
				break
			}
		}
	}

	if logoLoaded {
		if logoScale != 1.0 {
			originalWidth := float64(logoImg.Bounds().Dx())
			originalHeight := float64(logoImg.Bounds().Dy())
			newWidth := originalWidth * logoScale
			newHeight := originalHeight * logoScale

			scaledCtx := gg.NewContext(int(newWidth), int(newHeight))
			scaledCtx.Scale(logoScale, logoScale)
			scaledCtx.DrawImage(logoImg, 0, 0)
			logoImg = scaledCtx.Image()
		}

		dc.DrawImage(logoImg, int(logoX), int(logoY))
	} else {
		LogWarn("Failed to load spark.png logo for BTC chart", zap.Strings("tried_paths", logoPaths))
	}

	// Load Inter (use and in stats_chart.go)
	fontPaths := []string{
		"etc/fonts/InterVariable.ttf",
		"etc/fonts/Inter-Regular.ttf",
		"etc/fonts/Inter-Regular.otf",
		"./etc/fonts/InterVariable.ttf",
		"./etc/fonts/Inter-Regular.ttf",
		"./etc/fonts/Inter-Regular.otf",
		"~/Library/Fonts/InterVariable.ttf",
		"~/Library/Fonts/Inter-Regular.ttf",
		"/Library/Fonts/InterVariable.ttf",
		"/Library/Fonts/Inter-Regular.ttf",
		"/System/Library/Fonts/Supplemental/InterVariable.ttf",
		"/System/Library/Fonts/Supplemental/Inter-Regular.ttf",
		"/usr/share/fonts/truetype/inter/InterVariable.ttf",
		"/usr/share/fonts/truetype/inter/Inter-Regular.ttf",
		"/usr/local/share/fonts/InterVariable.ttf",
		"/usr/local/share/fonts/Inter-Regular.ttf",
		"/System/Library/Fonts/SFNS.ttf",
		"/System/Library/Fonts/HelveticaNeue.ttc",
		"/System/Library/Fonts/Helvetica.ttc",
		"/System/Library/Fonts/Supplemental/Arial.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
	}
	fontSize := mainFontSize
	fontLoaded := false
	var loadedFontPath string

	expandPath := func(path string) string {
		if len(path) > 0 && path[0] == '~' {
			homeDir, err := os.UserHomeDir()
			if err == nil {
				return filepath.Join(homeDir, path[1:])
			}
		}
		return path
	}

	for _, fontPath := range fontPaths {
		expandedPath := expandPath(fontPath)
		if fileInfo, err := os.Stat(expandedPath); err == nil {
			if err := dc.LoadFontFace(expandedPath, fontSize); err == nil {
				fontLoaded = true
				loadedFontPath = expandedPath
				LogInfo("Successfully loaded Inter font for BTC chart",
					zap.String("path", expandedPath),
					zap.Int64("size", fileInfo.Size()))
				break
			}
		}
	}
	if !fontLoaded {
		LogWarn("Failed to load Inter font for BTC chart, using default system font",
			zap.Int("paths_checked", len(fontPaths)))
	}

	// BTC Reserve - value
	if fontLoaded {
		dc.LoadFontFace(loadedFontPath, avgVolumeLabelSize)
	}

	dc.SetColor(color.White)
	btcReserveLabel := "BTC Reserve"
	// Use and for Average Daily Volume
	dc.DrawString(btcReserveLabel, avgVolumeX, avgVolumeY)

	// value on - in 2
	if fontLoaded {
		dc.LoadFontFace(loadedFontPath, avgVolumeValueSize)
	}
	btcReserveValue := fmt.Sprintf("%.2f btc", currentBTCReserve)
	dc.SetColor(color.White)
	dc.DrawString(btcReserveValue, avgVolumeValueX, avgVolumeValueY)

	// Return
	if fontLoaded {
		dc.LoadFontFace(loadedFontPath, mainFontSize)
	}

	maxReserve := 0.0
	minReserve := 0.0
	hasData := false
	for _, point := range points {
		if point.Reserve > 0 {
			if !hasData {
				minReserve = point.Reserve
				maxReserve = point.Reserve
				hasData = true
			} else {
				if point.Reserve > maxReserve {
					maxReserve = point.Reserve
				}
				if point.Reserve < minReserve {
					minReserve = point.Reserve
				}
			}
		}
	}

	if !hasData {
		maxReserve = 100.0 // Default value
		minReserve = 0.0
	}

	// Use for Y: 1 BTC
	btcStep := 1.0

	// minReserve (1 BTC)
	minReserveY := float64(int(minReserve/btcStep)) * btcStep
	// maxReserve (1 BTC)
	maxReserveY := float64(int(maxReserve/btcStep)+1) * btcStep

	reserveRange := maxReserveY - minReserveY
	if reserveRange < btcStep*2 {
		minReserveY -= btcStep
		maxReserveY += btcStep
	} else {
		minReserveY -= btcStep * 0.5
		maxReserveY += btcStep * 0.5
	}

	if minReserveY < 0 {
		minReserveY = 0
	}

	chartAreaHeight := chartAreaBottom - chartAreaTop

	dc.SetColor(color.White)
	dc.SetLineWidth(2)
	dc.SetDash() // for

	dc.DrawLine(chartAreaLeft, chartAreaBottom, chartAreaRight, chartAreaBottom)
	dc.Stroke()

	dc.DrawLine(chartAreaLeft, chartAreaTop, chartAreaLeft, chartAreaBottom)
	dc.Stroke()

	dc.SetLineWidth(1)
	dc.SetDash(10, 5) // for

	// Calculate count for 1 BTC
	reserveRangeY := maxReserveY - minReserveY
	numSteps := int(reserveRangeY / btcStep)

	for i := 0; i <= numSteps; i++ {
		reserveValue := minReserveY + float64(i)*btcStep
		// value in Y
		y := chartAreaBottom - ((reserveValue-minReserveY)/(maxReserveY-minReserveY))*chartAreaHeight
		if y >= chartAreaTop && y <= chartAreaBottom {
			dc.DrawLine(chartAreaLeft, y, chartAreaRight, y)
			dc.Stroke()

			// Add on Y
			dc.SetColor(color.White)
			dc.SetLineWidth(2)
			dc.SetDash() // for
			tickLength := 8.0
			dc.DrawLine(chartAreaLeft-tickLength, y, chartAreaLeft, y)
			dc.Stroke()

			// Add BTC
			dc.SetColor(color.White)
			if fontLoaded {
				dc.LoadFontFace(loadedFontPath, dateFontSize) // Use for
			}
			// Format value BTC 1 BTC)
			btcLabel := fmt.Sprintf("%.0f", reserveValue)
			labelWidth, _ := dc.MeasureString(btcLabel)
			labelX := chartAreaLeft - labelWidth - 10.0 // Y
			labelY := y
			dc.DrawString(btcLabel, labelX, labelY)

			dc.SetDash(10, 5)
		}
	}

	dc.SetDash()

	// Calculate time for by X
	var minTime, maxTime time.Time
	if len(points) > 0 {
		minTime = points[0].Timestamp
		maxTime = points[len(points)-1].Timestamp
	}

	// If all points at same time, add small range
	timeRange := maxTime.Sub(minTime)
	if timeRange == 0 {
		timeRange = 24 * time.Hour // default: 24 hours
		maxTime = minTime.Add(timeRange)
	}

	dc.SetDash(10, 5)
	dc.SetColor(color.White)
	dc.SetLineWidth(1)
	chartAreaWidth := chartAreaRight - chartAreaLeft

	numVerticalLines := 4
	for i := 0; i <= numVerticalLines; i++ {
		// Calculate X for
		x := chartAreaLeft + (float64(i)/float64(numVerticalLines))*chartAreaWidth

		dc.DrawLine(x, chartAreaTop, x, chartAreaBottom)
		dc.Stroke()
	}

	dc.SetDash()

	dc.SetColor(color.RGBA{0, 255, 0, 255})
	dc.SetLineWidth(3)
	dc.SetDash()

	// Calculate for on time
	var chartPoints []struct {
		X, Y      float64
		Reserve   float64
		DateLabel string
	}

	for _, point := range points {
		if point.Reserve > 0 {
			// Calculate X on time time minTime and maxTime)
			timeOffset := point.Timestamp.Sub(minTime)
			timeRatio := float64(timeOffset) / float64(timeRange)
			barX := chartAreaLeft + timeRatio*chartAreaWidth

			// value in Y
			reserveY := chartAreaBottom - ((point.Reserve-minReserveY)/(maxReserveY-minReserveY))*chartAreaHeight

			chartPoints = append(chartPoints, struct {
				X, Y      float64
				Reserve   float64
				DateLabel string
			}{
				X:         barX,
				Y:         reserveY,
				Reserve:   point.Reserve,
				DateLabel: point.DateLabel,
			})
		}
	}

	if len(chartPoints) > 1 {
		for i := 0; i < len(chartPoints)-1; i++ {
			dc.DrawLine(chartPoints[i].X, chartPoints[i].Y, chartPoints[i+1].X, chartPoints[i+1].Y)
			dc.Stroke()
		}
	}

	// on -
	dc.SetColor(color.RGBA{0, 255, 0, 255})
	for _, point := range chartPoints {
		dc.DrawCircle(point.X, point.Y, 3) // 5 3
		dc.Fill()
	}

	// Add X)
	dc.SetColor(color.White)
	if fontLoaded {
		dc.LoadFontFace(loadedFontPath, dateFontSize)
	}

	datePositions := make(map[string]float64)
	for _, point := range chartPoints {
		if _, exists := datePositions[point.DateLabel]; !exists {
			datePositions[point.DateLabel] = point.X
		}
	}

	// and on X
	for dateLabel, xPos := range datePositions {
		// Add on X
		dc.SetColor(color.White)
		dc.SetLineWidth(2)
		dc.SetDash() // for
		tickLength := 8.0
		dc.DrawLine(xPos, chartAreaBottom, xPos, chartAreaBottom+tickLength)
		dc.Stroke()

		dateTextWidth, _ := dc.MeasureString(dateLabel)
		dateTextX := xPos - dateTextWidth/2 // by
		dateTextY := chartAreaBottom + dateOffsetY
		dc.DrawString(dateLabel, dateTextX, dateTextY)
	}

	chartsDir := filepath.Join("etc", "charts")
	if err := os.MkdirAll(chartsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create charts directory: %w", err)
	}

	// Save
	filename := filepath.Join(chartsDir, "btc_spark_chart.png")
	if err := dc.SavePNG(filename); err != nil {
		return "", fmt.Errorf("failed to save BTC spark chart: %w", err)
	}

	// Check, file and
	fileInfo, err := os.Stat(filename)
	if err != nil {
		return "", fmt.Errorf("failed to stat BTC spark chart file: %w", err)
	}
	if fileInfo.Size() == 0 {
		os.Remove(filename)
		LogError("BTC spark chart file is empty after rendering", zap.String("filename", filename))
		return "", fmt.Errorf("BTC spark chart file is empty after rendering")
	}

	LogInfo("BTC spark chart generated successfully",
		zap.String("filename", filename),
		zap.Int64("fileSize", fileInfo.Size()),
		zap.Int("pointsCount", len(chartPoints)))

	return filename, nil
}
