package agent

import (
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger provides structured logging for agent operations.
type Logger struct {
	zap *zap.Logger
}

// NewLogger creates a new Logger instance that writes to a file.
// If logPath is empty, logging is disabled.
// If development is true, uses development config with readable output.
// Otherwise uses production config with JSON output.
func NewLogger(logPath string, development bool) (*Logger, error) {
	if logPath == "" {
		// No logging
		return &Logger{zap: zap.NewNop()}, nil
	}

	// Open log file
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	// Create encoder config
	var encoderConfig zapcore.EncoderConfig
	if development {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
	} else {
		encoderConfig = zap.NewProductionEncoderConfig()
	}

	// Create core that writes to file
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(logFile),
		zapcore.InfoLevel,
	)

	logger := zap.New(core)

	return &Logger{zap: logger}, nil
}

// Close syncs the logger (should be called on shutdown).
func (l *Logger) Close() error {
	return l.zap.Sync()
}

// ToolExecuted logs a tool execution with details.
func (l *Logger) ToolExecuted(toolName string, duration time.Duration, success bool, err error) {
	if err != nil {
		l.zap.Info("tool executed",
			zap.String("tool", toolName),
			zap.Duration("duration", duration),
			zap.Bool("success", success),
			zap.Error(err),
		)
	} else {
		l.zap.Info("tool executed",
			zap.String("tool", toolName),
			zap.Duration("duration", duration),
			zap.Bool("success", success),
		)
	}
}

// LLMCall logs an LLM API call.
func (l *Logger) LLMCall(model string, promptTokens, completionTokens int, duration time.Duration) {
	l.zap.Info("llm call",
		zap.String("model", model),
		zap.Int("prompt_tokens", promptTokens),
		zap.Int("completion_tokens", completionTokens),
		zap.Int("total_tokens", promptTokens+completionTokens),
		zap.Duration("duration", duration),
	)
}

// AgentIteration logs an agent loop iteration.
func (l *Logger) AgentIteration(iteration int, toolCallCount int) {
	l.zap.Debug("agent iteration",
		zap.Int("iteration", iteration),
		zap.Int("tool_calls", toolCallCount),
	)
}

// Error logs an error.
func (l *Logger) Error(msg string, err error) {
	l.zap.Error(msg, zap.Error(err))
}

// Info logs an info message.
func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.zap.Info(msg, fields...)
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.zap.Debug(msg, fields...)
}
