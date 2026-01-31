package logger

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	L       *zap.Logger
	S       *zap.SugaredLogger
	logFile *os.File
)

// Init initializes the global logger
// Logs are written to ~/.config/qedit/qedit.log
func Init(debug bool) error {
	logPath, err := getLogPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}

	// Open log file (truncate on each run for now)
	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}

	// Configure encoder
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Set log level
	level := zapcore.InfoLevel
	if debug {
		level = zapcore.DebugLevel
	}

	// Create core that writes to file
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(logFile),
		level,
	)

	// Create logger with caller info
	L = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	S = L.Sugar()

	S.Info("logger initialized", "path", logPath, "debug", debug)
	return nil
}

// Close flushes and closes the logger
func Close() {
	if L != nil {
		_ = L.Sync()
	}
	if logFile != nil {
		_ = logFile.Close()
	}
}

// getLogPath returns the path to the log file
func getLogPath() (string, error) {
	if v := os.Getenv("QEDIT_LOG_FILE"); v != "" {
		return v, nil
	}

	// Use config directory
	if v := os.Getenv("QEDIT_CONFIG_HOME"); v != "" {
		return filepath.Join(v, "qedit.log"), nil
	}
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return filepath.Join(v, "qedit", "qedit.log"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "qedit", "qedit.log"), nil
}

// Convenience functions for common logging patterns

func Debug(msg string, keysAndValues ...interface{}) {
	if S != nil {
		S.Debugw(msg, keysAndValues...)
	}
}

func Info(msg string, keysAndValues ...interface{}) {
	if S != nil {
		S.Infow(msg, keysAndValues...)
	}
}

func Warn(msg string, keysAndValues ...interface{}) {
	if S != nil {
		S.Warnw(msg, keysAndValues...)
	}
}

func Error(msg string, keysAndValues ...interface{}) {
	if S != nil {
		S.Errorw(msg, keysAndValues...)
	}
}

func Fatal(msg string, keysAndValues ...interface{}) {
	if S != nil {
		S.Fatalw(msg, keysAndValues...)
	}
}

// Panic logs and panics - useful for catching unexpected states
func Panic(msg string, keysAndValues ...interface{}) {
	if S != nil {
		S.Panicw(msg, keysAndValues...)
	}
}
