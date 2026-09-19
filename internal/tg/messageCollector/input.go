package messageCollector

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"magnit_bot/internal/pkg/logger"
	"magnit_bot/internal/tg/messageCollector/statemachine"

	"go.uber.org/fx"
	tele "gopkg.in/telebot.v3"
	"gopkg.in/telebot.v3/layout"
)

const (
	waiting = iota
)

type Request struct {
	mu        sync.RWMutex
	message   *tele.Message
	callback  *tele.Callback
	completed bool
	canceled  bool
	callbacks []tele.CallbackEndpoint
}
type MessageCollector struct {
	statemachine statemachine.StateMachine
	requests     sync.Map
	layout       *layout.Layout
	logger       *logger.Logger
}
type FxOpts struct {
	fx.In
	Layout *layout.Layout
	Logger *logger.Logger
}

func New(opts FxOpts) *MessageCollector {
	return &MessageCollector{
		statemachine: statemachine.New(),
		layout:       opts.Layout,
		logger:       opts.Logger,
	}
}
func (mc *MessageCollector) MessageCollector() tele.HandlerFunc {
	return func(c tele.Context) error {
		if c.Message() == nil {
			return nil
		}

		userID := c.Sender().ID

		state, err := mc.statemachine.Get(userID)
		if err != nil || state != waiting {
			return nil
		}

		value, _ := mc.requests.LoadOrStore(userID, &Request{})
		req := value.(*Request)

		req.mu.Lock()
		req.message = c.Message()
		req.completed = true
		req.mu.Unlock()

		mc.statemachine.Delete(userID)
		return nil
	}
}
func (mc *MessageCollector) CallbackCollector() tele.HandlerFunc {
	return func(c tele.Context) error {
		userID := c.Sender().ID

		state, err := mc.statemachine.Get(userID)
		if err != nil || state != waiting {
			return nil
		}

		value, _ := mc.requests.LoadOrStore(userID, &Request{})
		req := value.(*Request)

		for _, cb := range req.callbacks {
			var unique string
			if c.Callback().Unique == "" {
				date := strings.Split(c.Callback().Data, "|")
				unique = strings.TrimSpace(date[0])
			} else {
				unique = strings.TrimSpace(c.Callback().Unique)
			}
			if strings.TrimSpace(cb.CallbackUnique()) == unique {
				_ = c.Respond(&tele.CallbackResponse{})
				req.mu.Lock()
				req.callback = c.Callback()
				req.message = c.Message()
				req.completed = true
				req.mu.Unlock()

				mc.statemachine.Delete(userID)
				return nil
			}
		}
		return nil
	}
}
func (mc *MessageCollector) Cancel(userID int64) {
	value, ok := mc.requests.Load(userID)
	if !ok {
		return
	}

	req := value.(*Request)
	req.mu.Lock()
	req.canceled = true
	req.completed = true
	req.mu.Unlock()

	mc.statemachine.Delete(userID)
	mc.requests.Delete(userID)
}

type Response struct {
	Message  *tele.Message
	Callback *tele.Callback
	Canceled bool
}

func (mc *MessageCollector) Get(ctx context.Context, userID int64, timeout time.Duration, callback ...tele.CallbackEndpoint) (Response, error) {
	req := &Request{
		callbacks: callback,
	}
	mc.requests.Store(userID, req)

	if err := mc.statemachine.Set(userID, waiting); err != nil {
		mc.requests.Delete(userID)
		return Response{}, err
	}
	defer func() {
		mc.statemachine.Delete(userID)
		mc.requests.Delete(userID)
	}()
	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			return Response{Canceled: true}, ctx.Err()
		default:
			req.mu.Lock()
			if req.completed {
				canceled := req.canceled
				res := Response{
					Message:  req.message,
					Callback: req.callback,
					Canceled: canceled,
				}

				req.mu.Unlock()
				return res, nil
			}
			req.mu.Unlock()

			if timeout > 0 && time.Since(start) > timeout {
				return Response{}, errors.New("timeout")
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}
