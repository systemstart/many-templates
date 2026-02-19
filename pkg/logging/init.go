package logging

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
)

const (
	JSON = "json"
	Text = "text"
	Tint = "tint"
)

func Initialize(loggingType string, logLevelName string) error {
	var logLevel slog.Level
	err := logLevel.UnmarshalText([]byte(logLevelName))
	if err != nil {
		return fmt.Errorf("could not parse log level: %v", err)
	}

	var (
		logHandlerOptions = slog.HandlerOptions{
			AddSource: true,
			Level:     logLevel,
		}
		logHandler slog.Handler
	)

	switch loggingType {
	case JSON:
		logHandler = slog.NewJSONHandler(os.Stdout, &logHandlerOptions)
	case Text:
		logHandler = slog.NewTextHandler(os.Stdout, &logHandlerOptions)
	case Tint:
		logHandler = tint.NewHandler(os.Stdout, &tint.Options{
			AddSource: logHandlerOptions.AddSource,
			Level:     logHandlerOptions.Level,
		})
	default:
		return fmt.Errorf("unknown logging type: %s", loggingType)

	}

	slog.SetDefault(slog.New(logHandler))
	slog.Info("logging initialized", "logLevel", logLevel)
	return nil
}
