package middlewares

import (
	"strings"

	"magnit_bot/internal/tg/messageCollector"
	tele "gopkg.in/telebot.v3"
)

func ResetCollectorOnBack(collector *messageCollector.MessageCollector) func(handlerFunc tele.HandlerFunc) tele.HandlerFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			if c.Callback() != nil {
				if strings.Contains(c.Callback().Data, "back") || strings.Contains(c.Callback().Unique, "back") {
					collector.Cancel(c.Sender().ID)
				}
			}
			if c.Message() != nil {
				if strings.HasPrefix(c.Message().Text, "/") {
					collector.Cancel(c.Sender().ID)
				}
			}
			return next(c)
		}
	}
}
