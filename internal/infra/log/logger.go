package log

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger
var consoleLogger *zap.Logger // for (ERROR and SUCCESS)
var fileLogger *zap.Logger    // for file
var initOnce sync.Once
var initError error

func init() {
	initOnce.Do(func() {
		initError = initializeLoggers()
	})
	if initError != nil {
		// Fallback to basic logging if initialization fails
		fmt.Fprintf(os.Stderr, "Failed to initialize loggers: %v\n", initError)
		Logger = zap.NewNop()
		consoleLogger = zap.NewNop()
		fileLogger = zap.NewNop()
	}
}

func initializeLoggers() error {
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Configure for file INFO, DEBUG, WARN, ERROR)
	// Use encoder time,
	fileConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05"),
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   nil,
	}

	fileEncoder := &customFileEncoder{Encoder: zapcore.NewConsoleEncoder(fileConfig)}
	fileCore := zapcore.NewCore(
		fileEncoder,
		zapcore.AddSync(getLogFileWriter(filepath.Join(logsDir, "app.log"))),
		zapcore.DebugLevel,
	)

	fileLogger = zap.New(fileCore)

	// Configure for ERROR and SUCCESS)
	var err error
	consoleConfig := zap.NewDevelopmentConfig()
	consoleConfig.EncoderConfig.EncodeLevel = customLevelEncoder
	consoleConfig.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("15:04:05")
	consoleConfig.EncoderConfig.EncodeCaller = nil                // caller
	consoleConfig.Development = false                             // stack trace for
	consoleConfig.DisableStacktrace = true                        // stack trace
	consoleConfig.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel) // INFO (SUCCESS) and ERROR

	consoleLogger, err = consoleConfig.Build()
	if err != nil {
		return fmt.Errorf("failed to build console logger: %w", err)
	}

	// fileLogger in file)
	Logger = fileLogger
	return nil
}

// GenerateRequestID ID for
func GenerateRequestID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// RequestLogger request-id and
func RequestLogger(requestID string) *zap.Logger {
	return Logger.With(
		zap.String("request_id", requestID),
	)
}

// LogRequest HTTP in file)
func LogRequest(requestID, method, endpoint string, fields ...zap.Field) {
	allFields := append([]zap.Field{
		zap.String("request_id", requestID),
		zap.String("method", method),
		zap.String("endpoint", endpoint),
	}, fields...)
	Logger.Info("HTTP request", allFields...)
}

// LogResponse HTTP
func LogResponse(requestID string, statusCode int, durationMs int64, fields ...zap.Field) {
	allFields := append([]zap.Field{
		zap.String("request_id", requestID),
		zap.Int("status_code", statusCode),
		zap.Int64("duration_ms", durationMs),
	}, fields...)

	if statusCode >= 200 && statusCode < 300 {
		// - in file
		Logger.Info("HTTP response", allFields...)
	} else {
		// error - in file and
		Logger.Error("HTTP response", allFields...)
		// in -
		endpointStr := fieldsToString(fields)
		if endpointStr != "" {
			consoleLogger.Error(fmt.Sprintf("✗ HTTP request failed [%d] %s", statusCode, endpointStr))
		} else {
			consoleLogger.Error(fmt.Sprintf("✗ HTTP request failed [%d]", statusCode))
		}
	}
}

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
)

func customLevelEncoder(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	switch level {
	case zapcore.DebugLevel:
		enc.AppendString(colorCyan + "DEBUG" + colorReset)
	case zapcore.InfoLevel:
		enc.AppendString(colorGreen + "SUCCESS" + colorReset) // INFO in = SUCCESS
	case zapcore.WarnLevel:
		enc.AppendString(colorYellow + "WARN" + colorReset)
	case zapcore.ErrorLevel:
		enc.AppendString(colorRed + "ERROR" + colorReset)
	case zapcore.FatalLevel:
		enc.AppendString(colorRed + "FATAL" + colorReset)
	case zapcore.PanicLevel:
		enc.AppendString(colorRed + "PANIC" + colorReset)
	default:
		enc.AppendString(colorWhite + level.String() + colorReset)
	}
}

// LogInfo in file)
func LogInfo(message string, fields ...zap.Field) {
	Logger.Info(message, fields...)
}

// LogSuccess (in file and
func LogSuccess(message string, fields ...zap.Field) {
	durationMs := extractDuration(fields)

	Logger.Info(message, fields...)

	if durationMs > 0 {
		consoleLogger.Info(fmt.Sprintf("✓ %s (%dms)", message, durationMs))
	} else {
		consoleLogger.Info("✓ " + message)
	}
}

// LogError error (in file and
func LogError(message string, fields ...zap.Field) {
	durationMs := extractDuration(fields)

	Logger.Error(message, fields...)

	if durationMs > 0 {
		consoleLogger.Error(fmt.Sprintf("✗ %s (%dms)", message, durationMs))
	} else {
		consoleLogger.Error("✗ " + message)
	}
}

// LogWarn in file)
func LogWarn(message string, fields ...zap.Field) {
	Logger.Warn(message, fields...)
}

// LogDebug in file)
func LogDebug(message string, fields ...zap.Field) {
	Logger.Debug(message, fields...)
}

// LogJSON JSON API in in file)
func LogJSON(data []byte, label string) {
	var prettyJSON interface{}
	if err := json.Unmarshal(data, &prettyJSON); err == nil {
		// If JSON, format
		formatted, err := json.MarshalIndent(prettyJSON, "", "  ")
		if err == nil {
			Logger.Info(label)
			Logger.Sugar().Infof("\n%s\n", string(formatted))
		} else {
			Logger.Info(label, zap.String("response", string(data)))
		}
	} else {
		// If JSON,
		Logger.Info(label, zap.String("response", string(data)))
	}
}

// extractDuration duration_ms from zap
func extractDuration(fields []zap.Field) int64 {
	for _, field := range fields {
		if field.Key == "duration_ms" {
			// zap.Field int64 in Integer
			if field.Type == zapcore.Int64Type {
				return field.Integer
			}
		}
	}
	return 0
}

// fieldsToString zap in for
func fieldsToString(fields []zap.Field) string {
	if len(fields) == 0 {
		return ""
	}
	for _, field := range fields {
		if field.Key == "endpoint" {
			return field.String
		}
	}
	return ""
}

const (
	// MaxLogFileSize - file (50
	MaxLogFileSize = 50 * 1024 * 1024 // 50 in
)

type rotatingLogWriter struct {
	file *os.File
	path string
	mu   sync.Mutex
}

func (w *rotatingLogWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	info, err := w.file.Stat()
	if err == nil && info.Size() > MaxLogFileSize {
		w.file.Close()

		// file 0
		w.file, err = os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return 0, fmt.Errorf("failed to truncate log file: %w", err)
		}
	}

	return w.file.Write(p)
}

func (w *rotatingLogWriter) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Sync()
}

// getLogFileWriter writer for file append and
func getLogFileWriter(path string) zapcore.WriteSyncer {
	// file in append
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// Fallback to stderr if file can't be opened
		fmt.Fprintf(os.Stderr, "failed to open log file %s: %v, falling back to stderr\n", path, err)
		return zapcore.AddSync(os.Stderr)
	}

	info, err := file.Stat()
	if err == nil && info.Size() > MaxLogFileSize {
		file.Close()
		file, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			// Fallback to stderr if truncation fails
			fmt.Fprintf(os.Stderr, "failed to truncate log file %s: %v, falling back to stderr\n", path, err)
			return zapcore.AddSync(os.Stderr)
		}
	}

	writer := &rotatingLogWriter{
		file: file,
		path: path,
	}
	return zapcore.AddSync(writer)
}

// customFileEncoder time,
type customFileEncoder struct {
	zapcore.Encoder
}

func (e *customFileEncoder) Clone() zapcore.Encoder {
	return &customFileEncoder{
		Encoder: e.Encoder.Clone(),
	}
}

func (e *customFileEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	buf := buffer.NewPool().Get()

	// 1. time
	timeStr := entry.Time.Format("2006-01-02 15:04:05")
	buf.AppendString(timeStr)
	buf.AppendString("     ") // 5 and

	// 2. (INFO, ERROR and ..)
	levelStr := entry.Level.CapitalString()
	buf.AppendString(levelStr)
	buf.AppendString(" ") // 1 and

	if entry.Message != "" {
		buf.AppendString(entry.Message)
	}

	// 4. (if
	if len(fields) > 0 {
		buf.AppendString("\t")
		// Format JSON
		fieldMap := make(map[string]interface{})
		for _, field := range fields {
			switch field.Type {
			case zapcore.StringType:
				fieldMap[field.Key] = field.String
			case zapcore.Int64Type:
				fieldMap[field.Key] = field.Integer
			case zapcore.Int32Type:
				fieldMap[field.Key] = field.Integer
			case zapcore.BoolType:
				fieldMap[field.Key] = field.Integer == 1
			case zapcore.Float64Type:
				fieldMap[field.Key] = field.Interface
			case zapcore.ErrorType:
				if field.Interface != nil {
					if err, ok := field.Interface.(error); ok {
						fieldMap[field.Key] = err.Error()
					}
				}
			default:
				// for use Interface or Integer
				if field.Interface != nil {
					fieldMap[field.Key] = field.Interface
				} else {
					fieldMap[field.Key] = field.Integer
				}
			}
		}

		jsonData, err := json.Marshal(fieldMap)
		if err == nil {
			buf.AppendString(string(jsonData))
		}
	}

	buf.AppendString("\n")
	return buf, nil
}
