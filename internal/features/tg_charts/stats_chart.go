package tg_charts

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"time"

	"spark-wallet/internal/clients_api/luminex"
	logging "spark-wallet/internal/infra/log"

	"github.com/fogleman/gg"
	"go.uber.org/zap"
)

const (
	chartWidth  = 2326
	chartHeight = 1334

	logoX     = 200.0 // X
	logoY     = 30.0  // Y - for
	logoScale = 0.3   // (1.0 = 0.5 = in 2 2.0 = in 2

	dailyVolumeX      = 1320.0 // X for
	dailyVolumeY      = 160.0  // Y for
	dailyVolumeValueX = 1320.0 // X for
	dailyVolumeValueY = 240.0  // Y for
	dailyVolumeWidth  = 250.0

	avgVolumeX       = 1720.0 // X for
	avgVolumeY       = 160.0  // Y for Y for
	avgVolumeValueX  = 1720.0 // X for
	avgVolumeValueY  = 240.0  // Y for
	avgVolumeWidth   = 250.0
	avgVolumeOffsetX = 0.0 // by X (0 = on

	chartAreaLeft   = 300.0
	chartAreaRight  = 2000.0
	chartAreaTop    = 400.0 // for
	chartAreaBottom = 1200.0

	barWidth   = 200.0
	barSpacing = 60.0

	// X- (Mon..Sun).
	bar1X = 300.0  // 1 (Monday) - chartAreaLeft
	bar2X = 550.0  // 2 (Tuesday) - bar1X + barWidth + spacing
	bar3X = 800.0  // 3 (Wednesday)
	bar4X = 1050.0 // 4 (Thursday)
	bar5X = 1300.0 // 5 (Friday)
	bar6X = 1550.0 // 6 (Saturday)
	bar7X = 1800.0 // 7 (Sunday) -

	gridLinesCount = 3 // 3
	gridLineStartX = 200.0
	gridLineEndX   = 2100.0

	yAxisStep = 50000.0 // 50

	mainFontSize         = 35.0
	barValueFontSize     = 35.0
	dateFontSize         = 28.0
	dailyVolumeLabelSize = 35.0 // "Daily Volume"
	dailyVolumeValueSize = 70.0 // (in 2 32.0 * 2)
	avgVolumeLabelSize   = 35.0 // "Average Daily Volume"
	avgVolumeValueSize   = 70.0 // (in 2 32.0 * 2)

	barValueOffsetY = 40.0
	dateOffsetY     = 40.0
)

// GenerateVolumeChart 24 on from stats.json
func GenerateVolumeChart() (string, error) {
	statsData, err := luminex.LoadStatsData()
	if err != nil {
		return "", fmt.Errorf("failed to load stats data: %w", err)
	}

	if len(statsData.Entries) == 0 {
		return "", fmt.Errorf("no stats data available")
	}

	var totalVolumeSum float64
	for _, entry := range statsData.Entries {
		totalVolumeSum += entry.TotalVolume24HUSD
	}
	avgDailyVolume := totalVolumeSum / float64(len(statsData.Entries))

	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7 // = 7
	}
	daysFromMonday := weekday - 1
	lastMonday := now.AddDate(0, 0, -daysFromMonday).Truncate(24 * time.Hour)

	// create for (7 Monday)
	var volumes []float64
	var dateLabels []string

	for i := 0; i < 7; i++ {
		date := lastMonday.AddDate(0, 0, i)
		dateStr := date.Format("2006-01-02")
		dateLabel := date.Format("Mon")

		// for in stats.json
		var volume float64
		for _, entry := range statsData.Entries {
			if entry.Date == dateStr {
				volume = entry.TotalVolume24HUSD
				break
			}
		}

		volumes = append(volumes, volume)
		dateLabels = append(dateLabels, dateLabel)
	}

	// Get 24h from
	var currentVolume24H float64
	if len(statsData.Entries) > 0 {
		currentVolume24H = statsData.Entries[len(statsData.Entries)-1].TotalVolume24HUSD
	}

	dc := gg.NewContext(chartWidth, chartHeight)

	dc.SetColor(color.Black)
	dc.Clear()

	// Load spark.png (from etc/telegram)
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
				logging.LogInfo("Loaded spark.png logo", zap.String("path", logoPath))
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

		// DrawImage
		dc.DrawImage(logoImg, int(logoX), int(logoY))
	} else {
		logging.LogWarn("Failed to load spark.png logo - file not found in any expected location", zap.Strings("tried_paths", logoPaths))
	}

	// Load Inter or fallback
	// Inter - for for
	fontPaths := []string{
		// Inter - folder (if in
		"etc/fonts/InterVariable.ttf",
		"etc/fonts/Inter-Regular.ttf",
		"etc/fonts/Inter-Regular.otf",
		"./etc/fonts/InterVariable.ttf",
		"./etc/fonts/Inter-Regular.ttf",
		"./etc/fonts/Inter-Regular.otf",
		// Inter - on macOS TTF, gg OTF
		"~/Library/Fonts/InterVariable.ttf", // Variable font (TTF) -
		"~/Library/Fonts/Inter-Regular.ttf",
		"/Library/Fonts/InterVariable.ttf",
		"/Library/Fonts/Inter-Regular.ttf",
		"/System/Library/Fonts/Supplemental/InterVariable.ttf",
		"/System/Library/Fonts/Supplemental/Inter-Regular.ttf",
		"/usr/share/fonts/truetype/inter/InterVariable.ttf",
		"/usr/share/fonts/truetype/inter/Inter-Regular.ttf",
		"/usr/local/share/fonts/InterVariable.ttf",
		"/usr/local/share/fonts/Inter-Regular.ttf",
		"/System/Library/Fonts/SFNS.ttf",               // San Francisco
		"/System/Library/Fonts/HelveticaNeue.ttc",      // Helvetica Neue
		"/System/Library/Fonts/Helvetica.ttc",          // Helvetica
		"/System/Library/Fonts/Supplemental/Arial.ttf", // Arial
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
				logging.LogInfo("Successfully loaded Inter font",
					zap.String("path", expandedPath),
					zap.Int64("size", fileInfo.Size()))
				break
			} else {
				logging.LogWarn("Font file exists but failed to load",
					zap.String("path", expandedPath),
					zap.Error(err))
			}
		}
	}
	if !fontLoaded {
		// If use
		logging.LogWarn("Failed to load Inter font from any path, using default system font",
			zap.Int("paths_checked", len(fontPaths)))
	}

	// Daily Volume - value
	if fontLoaded {
		dc.LoadFontFace(loadedFontPath, dailyVolumeLabelSize)
	}

	// "Daily Volume"
	// by (use Y
	dc.SetColor(color.White)
	dailyVolumeLabel := "Daily Volume"
	dc.DrawString(dailyVolumeLabel, dailyVolumeX, dailyVolumeY)

	// value on - in 2 by
	if fontLoaded {
		dc.LoadFontFace(loadedFontPath, dailyVolumeValueSize)
	}
	dailyVolumeValue := fmt.Sprintf("$%s", luminex.FormatUSDValue(currentVolume24H))
	dc.SetColor(color.RGBA{0, 255, 0, 255})
	dc.DrawString(dailyVolumeValue, dailyVolumeValueX, dailyVolumeValueY)

	// Average Daily Volume - value
	dc.SetColor(color.White)
	if fontLoaded {
		dc.LoadFontFace(loadedFontPath, avgVolumeLabelSize)
	}
	avgVolumeLabel := "Average Daily Volume"
	// Use avgVolumeX for X
	dc.DrawString(avgVolumeLabel, avgVolumeX, avgVolumeY)

	// value on - in 2 by
	if fontLoaded {
		dc.LoadFontFace(loadedFontPath, avgVolumeValueSize)
	}
	avgVolumeValue := fmt.Sprintf("$%s", luminex.FormatUSDValue(avgDailyVolume))
	dc.SetColor(color.White)
	dc.DrawString(avgVolumeValue, avgVolumeValueX, avgVolumeValueY)

	// Return
	if fontLoaded {
		dc.LoadFontFace(loadedFontPath, mainFontSize)
	}

	maxVolume := 0.0
	for _, v := range volumes {
		if v > maxVolume {
			maxVolume = v
		}
	}
	if maxVolume == 0 {
		maxVolume = 1.0 // on
	}

	maxVolumeY := yAxisStep
	if maxVolume > 0 {
		steps := int(maxVolume/yAxisStep) + 1
		if maxVolume > float64(steps-1)*yAxisStep {
			steps++
		}
		maxVolumeY = float64(steps) * yAxisStep
	}

	dc.SetColor(color.White)
	dc.SetLineWidth(1)
	chartAreaHeight := chartAreaBottom - chartAreaTop

	// count maxVolumeY
	numSteps := int(maxVolumeY / yAxisStep)
	if numSteps > gridLinesCount+1 {
		numSteps = gridLinesCount + 1
	}

	for i := 0; i <= numSteps; i++ {
		volumeValue := float64(i) * yAxisStep
		// value in Y
		// chartAreaBottom 0, chartAreaTop maxVolumeY
		y := chartAreaBottom - (volumeValue/maxVolumeY)*chartAreaHeight
		if y >= chartAreaTop && y <= chartAreaBottom {
			dc.DrawLine(gridLineStartX, y, gridLineEndX, y)
			dc.Stroke()
		}
	}

	barPositionsX := []float64{bar1X, bar2X, bar3X, bar4X, bar5X, bar6X, bar7X}

	dc.SetColor(color.RGBA{128, 128, 128, 255})

	for i, vol := range volumes {
		barX := barPositionsX[i]
		// Use maxVolumeY for Y)
		barHeight := (vol / maxVolumeY) * chartAreaHeight
		barY := chartAreaBottom - barHeight

		dc.DrawRectangle(barX, barY, barWidth, barHeight)
		dc.Fill()

		// Add - if > 0
		if vol > 0 {
			dc.SetColor(color.White)
			volumeText := luminex.FormatUSDValue(vol)
			if fontLoaded {
				dc.LoadFontFace(loadedFontPath, barValueFontSize)
			}
			textWidth, _ := dc.MeasureString(volumeText)
			textX := barX + (barWidth-textWidth)/2
			textY := barY - barValueOffsetY
			dc.DrawString(volumeText, textX, textY)
		}
		// Return
		if fontLoaded {
			dc.LoadFontFace(loadedFontPath, fontSize)
		}

		// Add
		dateText := dateLabels[i]
		if fontLoaded {
			dc.LoadFontFace(loadedFontPath, dateFontSize)
		}
		dateTextWidth, _ := dc.MeasureString(dateText)
		dateTextX := barX + (barWidth-dateTextWidth)/2
		dateTextY := chartAreaBottom + dateOffsetY
		dc.DrawString(dateText, dateTextX, dateTextY)
		// Return
		if fontLoaded {
			dc.LoadFontFace(loadedFontPath, fontSize)
		}

		dc.SetColor(color.RGBA{128, 128, 128, 255})
	}

	chartsDir := filepath.Join("etc", "charts")
	if err := os.MkdirAll(chartsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create charts directory: %w", err)
	}

	// Save
	filename := filepath.Join(chartsDir, "volume_chart.png")
	if err := dc.SavePNG(filename); err != nil {
		return "", fmt.Errorf("failed to save chart: %w", err)
	}

	// Check, file and
	fileInfo, err := os.Stat(filename)
	if err != nil {
		return "", fmt.Errorf("failed to stat chart file: %w", err)
	}
	if fileInfo.Size() == 0 {
		os.Remove(filename)
		logging.LogError("Chart file is empty after rendering", zap.String("filename", filename))
		return "", fmt.Errorf("chart file is empty after rendering")
	}

	logging.LogInfo("Volume chart generated successfully",
		zap.String("filename", filename),
		zap.Int64("fileSize", fileInfo.Size()),
		zap.Int("barsCount", len(volumes)))

	return filename, nil
}
