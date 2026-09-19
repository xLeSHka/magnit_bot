package checklist

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"html"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"magnit_bot/internal/pkg/config"
	"magnit_bot/internal/pkg/logger"

	"go.uber.org/fx"
)

var (
	ErrAccessDenied    = errors.New("access denied")
	ErrSessionNotFound = errors.New("session not found")
	ErrInvalidScore    = errors.New("score must be from 0 to 10")
	ErrInvalidShop     = errors.New("invalid shop")
)

type Service struct {
	mu        sync.RWMutex
	users     map[string]User
	sections  []Section
	questions []Question
	sessions  map[int64]*Session
	results   []Result
	logger    *logger.Logger
	timeLoc   *time.Location
}

type FxOpts struct {
	fx.In
	Config *config.Config
	Logger *logger.Logger
}

type User struct {
	Username string
	FullName string
}

type Section struct {
	Title     string
	Questions []Question
}

type Question struct {
	Number  int
	Text    string
	Section string
}

type Answer struct {
	Question Question
	Score    int
	HasScore bool
	Comment  string
}

type Session struct {
	TelegramID int64
	Username   string
	FullName   string
	Shop       string
	Address    string
	Current    int
	Answers    []Answer
	CreatedAt  time.Time
}

type Result struct {
	Session    Session
	FinishedAt time.Time
}

func New(opts FxOpts) (*Service, error) {
	users, sections, questions, err := parseChecklist(opts.Config.ChecklistPath)
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, errors.New("checklist users not found")
	}
	if len(questions) == 0 {
		return nil, errors.New("checklist questions not found")
	}

	return &Service{
		users:     users,
		sections:  sections,
		questions: questions,
		sessions:  make(map[int64]*Session),
		results:   make([]Result, 0),
		logger:    opts.Logger,
		timeLoc:   opts.Config.TimeLocation,
	}, nil
}

func (s *Service) Start(ctx context.Context, telegramID int64, username string) (*Session, error) {
	_ = ctx

	username = normalizeUsername(username)
	user, ok := s.users[username]
	if !ok {
		return nil, ErrAccessDenied
	}

	session := &Session{
		TelegramID: telegramID,
		Username:   username,
		FullName:   user.FullName,
		Answers:    make([]Answer, len(s.questions)),
		CreatedAt:  time.Now().In(s.timeLoc), // Используем локацию!
	}

	for i, q := range s.questions {
		session.Answers[i] = Answer{Question: q}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[telegramID] = session

	return cloneSession(session), nil
}

func (s *Service) SetShop(ctx context.Context, telegramID int64, shop string) (*Session, error) {
	_ = ctx

	shop = strings.TrimSpace(shop)
	if shop != "Магнит" && shop != "Пятерочка" {
		return nil, ErrInvalidShop
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[telegramID]
	if !ok {
		return nil, ErrSessionNotFound
	}
	session.Shop = shop

	return cloneSession(session), nil
}

func (s *Service) SetAddress(ctx context.Context, telegramID int64, address string) (*Session, error) {
	_ = ctx

	address = strings.TrimSpace(address)
	if address == "" {
		return nil, errors.New("address is empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[telegramID]
	if !ok {
		return nil, ErrSessionNotFound
	}
	session.Address = address

	return cloneSession(session), nil
}

func (s *Service) CurrentQuestion(ctx context.Context, telegramID int64) (Answer, int, int, error) {
	_ = ctx

	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[telegramID]
	if !ok {
		return Answer{}, 0, len(s.questions), ErrSessionNotFound
	}
	if session.Current >= len(s.questions) {
		return Answer{}, session.Current, len(s.questions), nil
	}

	return session.Answers[session.Current], session.Current + 1, len(s.questions), nil
}

func (s *Service) NextQuestion(ctx context.Context, telegramID int64) error {
	_ = ctx

	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[telegramID]
	if !ok {
		return ErrSessionNotFound
	}

	next := (session.Current + 1) % len(s.questions)
	for i := 0; i < len(s.questions); i++ {
		idx := (next + i) % len(s.questions)
		if !session.Answers[idx].HasScore {
			session.Current = idx
			return nil
		}
	}

	session.Current = next
	return nil
}

func (s *Service) GoToQuestion(ctx context.Context, telegramID int64, index int) error {
	_ = ctx

	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[telegramID]
	if !ok {
		return ErrSessionNotFound
	}
	if index < 0 || index >= len(s.questions) {
		return errors.New("invalid index")
	}

	session.Current = index
	return nil
}

func (s *Service) SetScore(ctx context.Context, telegramID int64, score int) (bool, error) {
	_ = ctx

	if score < 0 || score > 10 {
		return false, ErrInvalidScore
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[telegramID]
	if !ok {
		return false, ErrSessionNotFound
	}
	if session.Current >= len(s.questions) {
		return true, nil
	}

	session.Answers[session.Current].Score = score
	session.Answers[session.Current].HasScore = true

	allAnswered := true
	next := -1
	for i := 1; i <= len(s.questions); i++ {
		idx := (session.Current + i) % len(s.questions)
		if !session.Answers[idx].HasScore {
			if next == -1 {
				next = idx
			}
			allAnswered = false
		}
	}

	if next != -1 {
		session.Current = next
	}

	return allAnswered, nil
}

func (s *Service) SetComment(ctx context.Context, telegramID int64, comment string) error {
	_ = ctx

	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[telegramID]
	if !ok {
		return ErrSessionNotFound
	}
	if session.Current >= len(s.questions) {
		return nil
	}

	session.Answers[session.Current].Comment = comment
	return nil
}

func (s *Service) GetSession(ctx context.Context, telegramID int64) (*Session, error) {
	_ = ctx

	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[telegramID]
	if !ok {
		return nil, ErrSessionNotFound
	}

	return cloneSession(session), nil
}

func (s *Service) Finish(ctx context.Context, telegramID int64) (Result, error) {
	_ = ctx

	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[telegramID]
	if !ok {
		return Result{}, ErrSessionNotFound
	}

	result := Result{
		Session:    *cloneSession(session),
		FinishedAt: time.Now().In(s.timeLoc), // Используем локацию!
	}
	s.results = append(s.results, result)
	delete(s.sessions, telegramID)

	s.logger.Infof("checklist.Finish info: telegramID: %d, username: %s, shop: %s, address: %s", telegramID, session.Username, session.Shop, session.Address)

	return result, nil
}

func (s *Service) Cancel(ctx context.Context, telegramID int64) {
	_ = ctx

	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, telegramID)
}

func (s *Service) Shops(ctx context.Context) []string {
	_ = ctx
	return []string{"Магнит", "Пятерочка"}
}

func (r Result) AverageScore() float64 {
	var sum int
	var count int
	for _, answer := range r.Session.Answers {
		if answer.HasScore {
			sum += answer.Score
			count++
		}
	}

	if count == 0 {
		return 0
	}
	return float64(sum) / float64(count)
}

func (r Result) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "<b>Чеклист завершен</b>\n\n")
	// ЗДЕСЬ СМЕНИЛИ Проверяющий на Автор
	fmt.Fprintf(&b, "Автор: %s (@%s)\n", html.EscapeString(r.Session.FullName), html.EscapeString(r.Session.Username))
	fmt.Fprintf(&b, "Магазин: %s\n", html.EscapeString(r.Session.Shop))
	fmt.Fprintf(&b, "Адрес: %s\n", html.EscapeString(r.Session.Address))
	fmt.Fprintf(&b, "Средняя оценка: %.1f/10\n\n", r.AverageScore())
	fmt.Fprintf(&b, "<b>Ответы</b>\n")

	for _, answer := range r.Session.Answers {
		if b.Len() > 3500 {
			b.WriteString("...\n(полный отчет в прикрепленном файле)\n")
			break
		}
		scoreStr := "➖"
		if answer.HasScore {
			scoreStr = strconv.Itoa(answer.Score)
		}
		fmt.Fprintf(&b, "%d. %s: %s\n", answer.Question.Number, html.EscapeString(answer.Question.Text), scoreStr)
		if answer.Comment != "" {
			fmt.Fprintf(&b, "   💬 <i>%s</i>\n", html.EscapeString(answer.Comment))
		}
	}

	return b.String()
}

func parseChecklist(path string) (map[string]User, []Section, []Question, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, nil, err
	}
	defer file.Close()

	users := make(map[string]User)
	sectionByTitle := make(map[string]int)
	sections := make([]Section, 0)
	questions := make([]Question, 0)

	userRegexp := regexp.MustCompile(`^@([A-Za-z0-9_]+)\s*-\s*(.+)$`)
	questionRegexp := regexp.MustCompile(`^(\d+)\.\s+(.+)$`)
	currentSection := ""

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if matches := userRegexp.FindStringSubmatch(line); len(matches) == 3 {
			username := normalizeUsername(matches[1])
			users[username] = User{
				Username: username[1:],
				FullName: strings.TrimSpace(matches[2]),
			}
			continue
		}

		if matches := questionRegexp.FindStringSubmatch(line); len(matches) == 3 {
			number, err := strconv.Atoi(matches[1])
			if err != nil {
				return nil, nil, nil, err
			}

			question := Question{
				Number:  number,
				Text:    strings.TrimSpace(matches[2]),
				Section: currentSection,
			}
			questions = append(questions, question)

			if currentSection != "" {
				index, ok := sectionByTitle[currentSection]
				if !ok {
					sections = append(sections, Section{Title: currentSection})
					index = len(sections) - 1
					sectionByTitle[currentSection] = index
				}
				sections[index].Questions = append(sections[index].Questions, question)
			}
			continue
		}

		if isSectionLine(line) {
			currentSection = line
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, nil, err
	}

	sort.SliceStable(questions, func(i, j int) bool {
		return questions[i].Number < questions[j].Number
	})

	return users, sections, questions, nil
}

func isSectionLine(line string) bool {
	if strings.Contains(line, "@") {
		return false
	}

	skip := []string{
		"Авторизация",
		"Если пишет",
		"Выбор магазина",
		"Магнит или пятерочка",
		"Ввести адрес магазина",
	}
	for _, prefix := range skip {
		if strings.HasPrefix(line, prefix) {
			return false
		}
	}

	return !strings.Contains(line, ".")
}

func normalizeUsername(username string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(username)), "@")
}

func cloneSession(session *Session) *Session {
	if session == nil {
		return nil
	}

	cloned := *session
	cloned.Answers = append([]Answer(nil), session.Answers...)
	return &cloned
}
