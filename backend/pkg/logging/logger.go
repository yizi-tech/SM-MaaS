package logging

import (
	"fmt"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

var Logger zerolog.Logger

func Init(level string, output string) {
	logLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		logLevel = zerolog.InfoLevel
	}

	var writer zerolog.LevelWriter
	if output == "file" {
		logDir := "logs"
		os.MkdirAll(logDir, 0755)
		logFile := fmt.Sprintf("%s/mass-%s.log", logDir, time.Now().Format("2006-01-02"))
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			writer = zerolog.MultiLevelWriter(zerolog.ConsoleWriter{Out: os.Stdout})
		} else {
			writer = zerolog.MultiLevelWriter(zerolog.ConsoleWriter{Out: os.Stdout}, f)
		}
	} else {
		writer = zerolog.MultiLevelWriter(zerolog.ConsoleWriter{Out: os.Stdout})
	}

	Logger = zerolog.New(writer).
		Level(logLevel).
		With().
		Timestamp().
		Caller().
		Logger()
}

func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		event := Logger.Info()
		if statusCode >= 400 {
			event = Logger.Error()
		}

		event.Str("method", method).
			Str("path", path).
			Str("query", query).
			Int("status", statusCode).
			Str("ip", clientIP).
			Dur("latency", latency).
			Str("user_agent", c.Request.UserAgent()).
			Msg("request completed")
	}
}

func Info(module, action, message string, fields map[string]interface{}) {
	e := Logger.Info().Str("module", module).Str("action", action)
	for k, v := range fields {
		e = e.Interface(k, v)
	}
	e.Msg(message)
}

func Error(module, action, message string, err error, fields map[string]interface{}) {
	e := Logger.Error().Str("module", module).Str("action", action).Err(err)
	for k, v := range fields {
		e = e.Interface(k, v)
	}
	e.Msg(message)
}

func Warn(module, action, message string, fields map[string]interface{}) {
	e := Logger.Warn().Str("module", module).Str("action", action)
	for k, v := range fields {
		e = e.Interface(k, v)
	}
	e.Msg(message)
}