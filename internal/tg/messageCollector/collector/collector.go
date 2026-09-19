package collector

import (
	tele "gopkg.in/telebot.v3"
	"gopkg.in/telebot.v3/layout"
)

type MessageCollector struct {
	messages []*tele.Message
	layout   *layout.Layout
}

func New(Layout *layout.Layout) *MessageCollector {
	return &MessageCollector{
		layout:   Layout,
		messages: make([]*tele.Message, 0),
	}
}
func (mc *MessageCollector) Collect(msg *tele.Message) {
	for _, m := range mc.messages {
		if m.ID == msg.ID {
			return
		}
	}
	mc.messages = append(mc.messages, msg)
}
func (mc *MessageCollector) GetMessages() []*tele.Message {
	return mc.messages
}
func (mc *MessageCollector) Send(c tele.Context, what interface{}, opts ...interface{}) error {
	message, err := c.Bot().Send(c.Chat(), what, opts...)
	if err != nil {
		return err
	}
	mc.Collect(message)
	return nil
}

type ClearOptions struct {
	IgnoreErrors    bool
	ExcludeLast     bool
	ExcludePrevLast bool
	ExcludeFirst    bool
}

func (mc *MessageCollector) Clear(c tele.Context, opts ClearOptions) error {
	for i, message := range mc.messages {
		if opts.ExcludeLast && i == len(mc.messages)-1 {
			continue
		}
		if opts.ExcludeFirst && i == 0 {
			continue
		}
		if opts.ExcludePrevLast && i == len(mc.messages)-2 {
			continue
		}
		err := c.Bot().Delete(message)
		if err != nil && !opts.IgnoreErrors {
			return err
		}
	}
	mc.messages = make([]*tele.Message, 0)
	return nil
}
