package setup

import (
	"magnit_bot/internal/pkg/config"
	"magnit_bot/internal/pkg/logger"
	checklistHandler "magnit_bot/internal/tg/handlers/checklist"
	"magnit_bot/internal/tg/handlers/middlewares"
	"magnit_bot/internal/tg/messageCollector"

	"go.uber.org/fx"
	tele "gopkg.in/telebot.v3"
	"gopkg.in/telebot.v3/layout"
	"gopkg.in/telebot.v3/middleware"
)

type Bot struct {
	api              *tele.Bot
	layout           *layout.Layout
	logger           *logger.Logger
	messageCollector *messageCollector.MessageCollector
	checklistHandler *checklistHandler.Handler
}

type FxOpts struct {
	fx.In
	Bot              *tele.Bot
	Layout           *layout.Layout
	Logger           *logger.Logger
	MessageCollector *messageCollector.MessageCollector
	ChecklistHandler *checklistHandler.Handler
	Config           *config.Config
}

func New(opts FxOpts) *Bot {
	botkit := &Bot{
		api:              opts.Bot,
		layout:           opts.Layout,
		logger:           opts.Logger,
		messageCollector: opts.MessageCollector,
		checklistHandler: opts.ChecklistHandler,
	}

	if opts.Config.Debug {
		botkit.api.Use(middleware.Logger())
	}

	botkit.api.Use(botkit.layout.Middleware("ru"))
	botkit.api.Use(middleware.AutoRespond())
	botkit.api.Use(middlewares.ResetCollectorOnBack(botkit.messageCollector))

	botkit.api.Handle(tele.OnText, botkit.messageCollector.MessageCollector())
	botkit.api.Handle(tele.OnCallback, botkit.messageCollector.CallbackCollector())
	botkit.checklistHandler.Setup(botkit.api)

	return botkit
}
