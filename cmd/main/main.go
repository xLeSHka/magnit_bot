package main

import (
	"magnit_bot/internal/app/service/checklist"
	"magnit_bot/internal/connections/tgconn"
	"magnit_bot/internal/pkg/config"
	"magnit_bot/internal/pkg/logger"
	checklistHandler "magnit_bot/internal/tg/handlers/checklist"
	"magnit_bot/internal/tg/messageCollector"
	"magnit_bot/internal/tg/setup"

	"go.uber.org/fx"
)

var Services = fx.Module("services",
	fx.Provide(
		checklist.New,
	),
)

var Telegram = fx.Module("telegram",
	fx.Provide(
		checklistHandler.New,
	),
	fx.Invoke(
		setup.New,
	),
)

var App = fx.Options(
	fx.Provide(
		config.New,
		logger.New,
		tgconn.New,
		messageCollector.New,
	),
	Services,
	Telegram,
)

func main() {
	fx.New(App).Run()
}
