package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Config defines logger construction options.
type Config struct {
	ServiceName string
	Level       string
	Pretty      bool
}

// New creates a zerolog Logger configured with sane defaults for the starter.
func New(cfg Config) zerolog.Logger {
	level := parseLevel(cfg.Level)

	var writer io.Writer = os.Stdout
	if cfg.Pretty {
		console := zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
		console.FormatLevel = func(i interface{}) string {
			return strings.ToUpper(fmt.Sprintf("| %-6s|", i))
		}
		console.FormatMessage = func(i interface{}) string {
			return fmt.Sprintf("%s", i)
		}
		console.FormatFieldName = func(i interface{}) string {
			return fmt.Sprintf("%s:", i)
		}
		writer = console
	}

	logger := zerolog.New(writer).With().Timestamp()
	if cfg.ServiceName != "" {
		logger = logger.Str("service", cfg.ServiceName)
	}
	return logger.Logger().Level(level)
}

func parseLevel(value string) zerolog.Level {
	switch strings.ToLower(value) {
	case "trace":
		return zerolog.TraceLevel
	case "debug":
		return zerolog.DebugLevel
	case "info", "":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	case "panic":
		return zerolog.PanicLevel
	default:
		return zerolog.InfoLevel
	}
}
