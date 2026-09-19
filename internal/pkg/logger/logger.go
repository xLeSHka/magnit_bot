package logger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"magnit_bot/internal/pkg/config"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	*zap.SugaredLogger
	LogsPath   string
	Name       string
	FileWriter *os.File
}

var (
	Log *Logger
)

func New(config *config.Config, lc fx.Lifecycle) (*Logger, error) {
	var l Logger
	l.Name = "main"
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if config.LogsDir == "" {
		l.LogsPath = wd
	} else {
		l.LogsPath = filepath.Join(wd, config.LogsDir)
	}
	err = os.MkdirAll(l.LogsPath, os.ModePerm)
	if err != nil {
		return nil, err
	}
	encoderConfig := zapcore.EncoderConfig{
		MessageKey:  "message",
		LevelKey:    "level",
		TimeKey:     "timestamp",
		NameKey:     "logger",
		CallerKey:   "caller",
		EncodeLevel: zapcore.CapitalColorLevelEncoder,
		EncodeTime: func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.In(time.FixedZone("GMT+0", 3*60*60)).Format(config.LogFormat))
		},
		EncodeCaller:   zapcore.ShortCallerEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
	}

	if config.TimeLocation != nil {
		encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.In(config.TimeLocation).Format(config.LogFormat))
		}
	}

	var level zapcore.Level
	if config.Debug {
		level = zap.DebugLevel
	} else {
		level = zap.InfoLevel
	}
	consoleEncoderConfig := encoderConfig
	consoleEncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	consoleEncoder := zapcore.NewConsoleEncoder(consoleEncoderConfig)

	fileEncoderConfig := encoderConfig
	fileEncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	fileEncoder := zapcore.NewJSONEncoder(fileEncoderConfig)

	var cores []zapcore.Core

	consoleCore := zapcore.NewCore(consoleEncoder, zapcore.Lock(os.Stdout), level)
	cores = append(cores, consoleCore)

	if config.LogToFile {
		// ДОБАВЛЕНО: .In(config.TimeLocation)
		logPath := filepath.Join(l.LogsPath, fmt.Sprintf("%s.log", time.Now().In(config.TimeLocation).Format(config.LogFormat)))
		fileWriter, errOpen := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
		if errOpen != nil {
			return nil, errOpen
		}

		l.FileWriter = fileWriter

		fileCore := zapcore.NewCore(fileEncoder, zapcore.AddSync(fileWriter), level)
		cores = append(cores, fileCore)
	}

	combinedCore := zapcore.NewTee(cores...)

	log := zap.New(combinedCore, zap.AddCaller())
	l.SugaredLogger = log.Sugar()
	Log = &l

	lc.Append(fx.Hook{
		OnStop: func(c context.Context) error {
			if Log != nil && Log.FileWriter != nil {
				return Log.FileWriter.Close()
			}
			return nil
		},
	})
	return &l, nil
}
func (l *Logger) Named(name string) (*Logger, error) {
	return &Logger{
		SugaredLogger: l.SugaredLogger.Named(name),
		LogsPath:      Log.LogsPath,
		Name:          name,
	}, nil
}
