package checklist

import (
	"context"
	"errors"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	checklistService "magnit_bot/internal/app/service/checklist"
	"magnit_bot/internal/pkg/logger"
	"magnit_bot/internal/tg/messageCollector"

	"go.uber.org/fx"
	tele "gopkg.in/telebot.v3"
	"gopkg.in/telebot.v3/layout"
)

const resultRecipientTelegramID int64 = 1332028019

type Handler struct {
	checklistService *checklistService.Service
	layout           *layout.Layout
	logger           *logger.Logger
	messageCollector *messageCollector.MessageCollector
}

type FxOpts struct {
	fx.In
	ChecklistService *checklistService.Service
	Layout           *layout.Layout
	Logger           *logger.Logger
	MessageCollector *messageCollector.MessageCollector
}

func New(opts FxOpts) *Handler {
	return &Handler{
		checklistService: opts.ChecklistService,
		layout:           opts.Layout,
		logger:           opts.Logger,
		messageCollector: opts.MessageCollector,
	}
}

func (h *Handler) Setup(bot *tele.Bot) {
	bot.Handle("/start", h.start)
	bot.Handle(h.layout.Callback("checklist:restart"), h.start)
	bot.Handle(h.layout.Callback("checklist:cancel"), h.cancel)
}

func (h *Handler) start(c tele.Context) error {
	h.messageCollector.Cancel(c.Sender().ID)
	h.checklistService.Cancel(context.Background(), c.Sender().ID)

	session, err := h.checklistService.Start(context.Background(), c.Sender().ID, c.Sender().Username)
	if err != nil {
		if errors.Is(err, checklistService.ErrAccessDenied) {
			h.logger.Infof("telegram.checklist.start access denied: telegramID: %d, username: %s", c.Sender().ID, c.Sender().Username)

			return c.Send(h.layout.Text(c, "access_denied"))
		}

		h.logger.Errorf("telegram.checklist.start err: %v, telegramID: %d", err, c.Sender().ID)
		return c.Send(h.layout.Text(c, "technical_issues"))
	}

	h.logger.Infof("telegram.checklist.start info: telegramID: %d, username: %s", c.Sender().ID, session.Username)
	_ = c.Send(h.layout.Text(c, "start_text", struct{ Name string }{Name: session.FullName}))
	return h.chooseShop(c)
}

func (h *Handler) chooseShop(c tele.Context) error {
	markup := h.layout.Markup(c, "checklist:shop:menu")
	for {
		if err := c.Send(h.layout.Text(c, "choose_shop_text"), markup); err != nil {
			h.logger.Errorf("telegram.checklist.chooseShop send err: %v, telegramID: %d", err, c.Sender().ID)
		}

		resp, err := h.messageCollector.Get(context.Background(), c.Sender().ID, 0, endpoints(markup)...)
		if err != nil {
			h.logger.Errorf("telegram.checklist.chooseShop collect err: %v, telegramID: %d", err, c.Sender().ID)
			return c.Send(h.layout.Text(c, "technical_issues"))
		}
		if resp.Canceled {
			return nil
		}
		if resp.Callback == nil {
			_ = c.Send(h.layout.Text(c, "input_error"))
			continue
		}

		shop := callbackValue(resp.Callback)
		if shop == "cancel" {
			return h.cancel(c)
		}

		if _, err = h.checklistService.SetShop(context.Background(), c.Sender().ID, shop); err != nil {
			_ = c.Send(h.layout.Text(c, "input_error"))
			continue
		}

		return h.askAddress(c)
	}
}

func (h *Handler) askAddress(c tele.Context) error {
	markup := h.layout.Markup(c, "checklist:cancel:menu")

	for {
		if err := c.Send(h.layout.Text(c, "address_text"), markup); err != nil {
			h.logger.Errorf("telegram.checklist.askAddress send err: %v, telegramID: %d", err, c.Sender().ID)
		}

		resp, err := h.messageCollector.Get(context.Background(), c.Sender().ID, 0, endpoints(markup)...)
		if err != nil {
			h.logger.Errorf("telegram.checklist.askAddress collect err: %v, telegramID: %d", err, c.Sender().ID)
			return c.Send(h.layout.Text(c, "technical_issues"))
		}
		if resp.Canceled {
			return nil
		}
		if resp.Callback != nil && callbackValue(resp.Callback) == "cancel" {
			return h.cancel(c)
		}
		if resp.Message == nil || strings.TrimSpace(resp.Message.Text) == "" {
			_ = c.Send(h.layout.Text(c, "input_error"))
			continue
		}

		if _, err = h.checklistService.SetAddress(context.Background(), c.Sender().ID, resp.Message.Text); err != nil {
			_ = c.Send(h.layout.Text(c, "input_error"))
			continue
		}

		return h.askQuestions(c)
	}
}

func (h *Handler) generalMenu(c tele.Context) error {
	for {
		session, err := h.checklistService.GetSession(context.Background(), c.Sender().ID)
		if err != nil {
			return c.Send(h.layout.Text(c, "technical_issues"))
		}

		var b strings.Builder
		b.WriteString("<b>📋 Общее меню вопросов:</b>\n\n")

		var rows []tele.Row
		var currentRow []tele.Btn

		for i, ans := range session.Answers {
			icon := "➖"
			scoreStr := "не отвечен"

			if ans.HasScore {
				icon = "✅"
				scoreStr = strconv.Itoa(ans.Score)
			}

			commentStr := ""
			if ans.Comment != "" {
				commentStr = " 💬"
			}

			b.WriteString(fmt.Sprintf("%s <b>%d.</b> %s - %s%s\n", icon, ans.Question.Number, html.EscapeString(ans.Question.Text), scoreStr, commentStr))

			btn := tele.Btn{
				Unique: fmt.Sprintf("q_%d", i),
				Data:   strconv.Itoa(i),
				// Кнопки также будут содержать значки: "✅ 1" или "➖ 2"
				Text: fmt.Sprintf("%s %d", icon, ans.Question.Number),
			}
			currentRow = append(currentRow, btn)
			if len(currentRow) == 5 {
				rows = append(rows, currentRow)
				currentRow = nil
			}
		}
		if len(currentRow) > 0 {
			rows = append(rows, currentRow)
		}

		b.WriteString("\n<i>Выберите номер вопроса для ответа или изменения.</i>")

		finishBtn := tele.Btn{
			Unique: "checklist_finish",
			Data:   "finish",
			Text:   "✅ Завершить чеклист",
		}
		cancelBtn := tele.Btn{
			Unique: "checklist_cancel",
			Data:   "cancel",
			Text:   "Отмена",
		}

		rows = append(rows, []tele.Btn{finishBtn})
		rows = append(rows, []tele.Btn{cancelBtn})

		markup := &tele.ReplyMarkup{}
		markup.Inline(rows...)

		text := b.String()
		// Ограничение Telegram на длину одного сообщения
		if len(text) > 4000 {
			text = text[:4000] + "...\n\n<i>Список сокращен, выберите вопрос:</i>"
		}

		if err = c.Send(text, markup); err != nil {
			h.logger.Errorf("telegram.checklist.generalMenu send err: %v", err)
		}

		resp, err := h.messageCollector.Get(context.Background(), c.Sender().ID, 0, endpoints(markup)...)
		if err != nil {
			h.logger.Errorf("telegram.checklist.generalMenu collect err: %v", err)
			return c.Send(h.layout.Text(c, "technical_issues"))
		}
		if resp.Canceled {
			return nil
		}

		if resp.Callback != nil {
			data := callbackValue(resp.Callback)
			if data == "cancel" {
				return h.cancel(c)
			} else if data == "finish" {
				return h.finish(c)
			} else {
				idx, err := strconv.Atoi(data)
				if err == nil {
					_ = h.checklistService.GoToQuestion(context.Background(), c.Sender().ID, idx)
					return h.askQuestions(c)
				}
			}
		} else {
			_ = c.Send(h.layout.Text(c, "input_error"))
		}
	}
}

func (h *Handler) askQuestions(c tele.Context) error {
	markup := h.layout.Markup(c, "checklist:score:menu")

	for {
		ans, current, total, err := h.checklistService.CurrentQuestion(context.Background(), c.Sender().ID)
		if err != nil {
			h.logger.Errorf("telegram.checklist.askQuestions question err: %v, telegramID: %d", err, c.Sender().ID)
			return c.Send(h.layout.Text(c, "technical_issues"))
		}

		if err = c.Send(h.layout.Text(c, "question_text", struct {
			Current  int
			Total    int
			Section  string
			Number   int
			Text     string
			HasScore bool
			Score    int
			Comment  string
		}{
			Current:  current,
			Total:    total,
			Section:  html.EscapeString(ans.Question.Section),
			Number:   ans.Question.Number,
			Text:     html.EscapeString(ans.Question.Text),
			HasScore: ans.HasScore,
			Score:    ans.Score,
			Comment:  html.EscapeString(ans.Comment),
		}), markup); err != nil {
			h.logger.Errorf("telegram.checklist.askQuestions send err: %v, telegramID: %d", err, c.Sender().ID)
		}

		resp, err := h.messageCollector.Get(context.Background(), c.Sender().ID, 0, endpoints(markup)...)
		if err != nil {
			h.logger.Errorf("telegram.checklist.askQuestions collect err: %v, telegramID: %d", err, c.Sender().ID)
			return c.Send(h.layout.Text(c, "technical_issues"))
		}
		if resp.Canceled {
			return nil
		}

		if resp.Callback != nil {
			data := callbackValue(resp.Callback)
			if data == "cancel" {
				return h.cancel(c)
			} else if data == "menu" {
				return h.generalMenu(c)
			} else if data == "next" {
				_ = h.checklistService.NextQuestion(context.Background(), c.Sender().ID)
				continue
			}

			score, err := strconv.Atoi(data)
			if err != nil {
				_ = c.Send(h.layout.Text(c, "score_input_error"))
				continue
			}

			allAnswered, err := h.checklistService.SetScore(context.Background(), c.Sender().ID, score)
			if err != nil {
				_ = c.Send(h.layout.Text(c, "score_input_error"))
				continue
			}
			if allAnswered {
				return h.generalMenu(c)
			}
			continue
		} else if resp.Message != nil && strings.TrimSpace(resp.Message.Text) != "" {
			comment := strings.TrimSpace(resp.Message.Text)
			if len(comment) > 1000 {
				comment = comment[:1000]
			}
			err = h.checklistService.SetComment(context.Background(), c.Sender().ID, comment)
			if err != nil {
				h.logger.Errorf("telegram.checklist.askQuestions comment err: %v", err)
			}
			continue
		} else {
			_ = c.Send(h.layout.Text(c, "input_error"))
		}
	}
}

func (h *Handler) finish(c tele.Context) error {
	result, err := h.checklistService.Finish(context.Background(), c.Sender().ID)
	if err != nil {
		h.logger.Errorf("telegram.checklist.finish err: %v, telegramID: %d", err, c.Sender().ID)
		return c.Send(h.layout.Text(c, "technical_issues"))
	}

	reportPath, reportName, err := createResultSpreadsheet(result)
	if err != nil {
		h.logger.Errorf("telegram.checklist.finish report err: %v, telegramID: %d", err, c.Sender().ID)
		return c.Send(result.Summary(), h.layout.Markup(c, "checklist:finish:menu"))
	}
	defer func() {
		if err = os.Remove(reportPath); err != nil {
			h.logger.Errorf("telegram.checklist.finish cleanup err: %v, path: %s", err, reportPath)
		}
	}()

	if err = c.Send(resultDocument(reportPath, reportName), h.layout.Markup(c, "checklist:finish:menu")); err != nil {
		h.logger.Errorf("telegram.checklist.finish send report err: %v, telegramID: %d", err, c.Sender().ID)
		return err
	}

	if _, err = c.Bot().Send(tele.ChatID(resultRecipientTelegramID), resultDocument(reportPath, reportName)); err != nil {
		h.logger.Errorf("telegram.checklist.finish notify err: %v, telegramID: %d, recipientID: %d", err, c.Sender().ID, resultRecipientTelegramID)
	}

	return nil
}

func (h *Handler) cancel(c tele.Context) error {
	h.messageCollector.Cancel(c.Sender().ID)
	h.checklistService.Cancel(context.Background(), c.Sender().ID)
	return c.Send(h.layout.Text(c, "cancel_text"), h.layout.Markup(c, "checklist:restart:menu"))
}

func endpoints(markup *tele.ReplyMarkup) []tele.CallbackEndpoint {
	if markup == nil {
		return nil
	}

	callbacks := make([]tele.CallbackEndpoint, 0)
	for _, row := range markup.InlineKeyboard {
		for _, button := range row {
			btn := button
			callbacks = append(callbacks, &btn)
		}
	}

	return callbacks
}

func callbackValue(callback *tele.Callback) string {
	if callback == nil {
		return ""
	}

	data := strings.TrimSpace(callback.Data)
	if data == "" {
		return strings.TrimSpace(callback.Unique)
	}

	for _, separator := range []string{"|", "\f"} {
		if strings.Contains(data, separator) {
			parts := strings.Split(data, separator)
			data = strings.TrimSpace(parts[len(parts)-1])
		}
	}

	return strings.TrimSpace(strings.TrimPrefix(data, fmt.Sprintf("%s|", callback.Unique)))
}

func createResultSpreadsheet(result checklistService.Result) (string, string, error) {
	reportName := resultSpreadsheetName(result)
	reportPath := filepath.Join(os.TempDir(), fmt.Sprintf("%d_%s", time.Now().UnixNano(), reportName))

	if err := result.SaveSpreadsheet(reportPath); err != nil {
		return "", "", err
	}

	return reportPath, reportName, nil
}

func resultDocument(path string, name string) *tele.Document {
	return &tele.Document{
		File:     tele.FromDisk(path),
		FileName: name,
		MIME:     "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		Caption:  "Чеклист завершен. Таблица во вложении.",
	}
}

func resultSpreadsheetName(result checklistService.Result) string {
	shop := sanitizeFilenamePart(result.Session.Shop)
	if shop == "" {
		shop = "shop"
	}

	address := sanitizeFilenamePart(result.Session.Address)
	if address == "" {
		address = "address"
	}

	return fmt.Sprintf("%s_%s_%s.xlsx", shop, result.FinishedAt.Format("2006-01-02_15-04"), address)
}

func sanitizeFilenamePart(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var b strings.Builder
	separator := false
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'а' && r <= 'я' || r >= '0' && r <= '9' || r == '-' {
			b.WriteRune(r)
			separator = false
			continue
		}
		if !separator && b.Len() > 0 {
			b.WriteRune('_')
			separator = true
		}
	}

	return strings.Trim(b.String(), "_")
}
