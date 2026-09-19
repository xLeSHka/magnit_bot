package tgconn

import (
	"context"
	"fmt"

	"magnit_bot/internal/pkg/logger"

	"go.uber.org/fx"
	tele "gopkg.in/telebot.v3"
	"gopkg.in/telebot.v3/layout"
)

func New(log *logger.Logger, lc fx.Lifecycle) (*tele.Bot, *layout.Layout, error) {
	if log == nil {
		return nil, nil, fmt.Errorf("logger dependency is nil")
	}
	lt, err := layout.New("telegram.yml")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load layout: %w", err)
	}

	settings := lt.Settings()
	botLogger, err := log.Named("bot")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get name logger: %w", err)
	}
	settings.OnError = func(err error, c tele.Context) {
		if c.Callback() == nil {
			botLogger.Errorf("telegram err: %v, telegramID: %v", err, c.Sender().ID)
		} else {
			botLogger.Errorf("telegram err: %v, telegramID: %v, unique: %s", err, c.Sender().ID, c.Callback().Unique)
		}
	}
	bot, err := tele.NewBot(settings)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create telegram bot: %w", err)
	}
	if cmds := lt.Commands(); cmds != nil {
		if err = bot.SetCommands(cmds); err != nil {
			return nil, nil, fmt.Errorf("failed to set commands: %w", err)
		}
	}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				bot.Start()
			}()
			botLogger.Info("starting telegram bot")
			return nil
		},
		OnStop: nil,
	})
	return bot, lt, nil
}
