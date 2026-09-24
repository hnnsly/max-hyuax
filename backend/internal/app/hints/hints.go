// Пакет hints — подсказка категории по тексту жителя. LLM, если подключена, только
// предлагает код из закрытого справочника; при сбое, таймауте или выдумке работают
// ключевые слова. Ответственного и срок всегда задают правила по выбранной категории.
package hints

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"dommax/internal/app"
	"dommax/internal/domain/rules"
)

// MaxRunes — сколько символов текста принимается для подсказки.
const MaxRunes = 1000

// LLM выбирает код категории из переданного списка.
type LLM interface {
	Category(ctx context.Context, text string, options []rules.Rule) (string, error)
}

type Source string

const (
	SourceLLM   Source = "llm"
	SourceRules Source = "rules"
)

type Hint struct {
	Rule   rules.Rule
	Source Source
}

type Service struct {
	llm         LLM // nil — LLM не подключена
	timeout     time.Duration
	log         *slog.Logger
	warmUpPause time.Duration
}

func NewService(llm LLM, timeout time.Duration, log *slog.Logger) *Service {
	return &Service{llm: llm, timeout: timeout, log: log, warmUpPause: warmUpRetry}
}

// Suggest подсказывает категорию; ok=false — ни модель, ни правила её не узнали.
func (s *Service) Suggest(ctx context.Context, text string) (Hint, bool, error) {
	text = strings.TrimSpace(text)
	if text == "" || !utf8.ValidString(text) || utf8.RuneCountInString(text) > MaxRunes {
		return Hint{}, false, fmt.Errorf("%w: text must be valid UTF-8, 1..%d characters", app.ErrInvalidInput, MaxRunes)
	}
	var fromLLM *rules.Rule
	if s.llm != nil {
		fromLLM = s.askLLM(ctx, text)
	}
	if fromLLM != nil && fromLLM.Code != rules.CodeOther {
		return Hint{Rule: *fromLLM, Source: SourceLLM}, true, nil
	}
	if r, ok := rules.Classify(text); ok {
		return Hint{Rule: r, Source: SourceRules}, true, nil
	}
	if fromLLM != nil {
		return Hint{Rule: *fromLLM, Source: SourceLLM}, true, nil
	}
	return Hint{}, false, nil
}

// askLLM возвращает категорию модели или nil. В лог — только причина, без текста жителя.
func (s *Service) askLLM(ctx context.Context, text string) *rules.Rule {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	code, err := s.llm.Category(ctx, text, rules.Categories())
	if err != nil {
		s.log.WarnContext(ctx, "llm hint failed, using rules", "err", err)
		return nil
	}
	r, err := rules.Lookup(code)
	if err != nil {
		s.log.WarnContext(ctx, "llm returned unknown category, using rules", "code", code)
		return nil
	}
	return &r
}

const (
	// warmUpTimeout — сколько ждать одну загрузку модели в память (на CPU это десятки секунд).
	warmUpTimeout = 3 * time.Minute
	// warmUpRetry — пауза между попытками: модель может ещё скачиваться (ollama-pull).
	warmUpRetry = 20 * time.Second
	// warmUpAttempts — после стольких неудач (около 10 минут) прогрев сдаётся: адрес или модель, видимо, неверны.
	warmUpAttempts = 30
)

// WarmUp загружает модель заранее, иначе первая подсказка после старта не уложится в таймаут.
// Повторяет попытки, пока не получится, пока ctx не отменён или пока не кончатся попытки;
// всё это время работают ключевые слова.
func (s *Service) WarmUp(ctx context.Context) {
	if s.llm == nil {
		return
	}
	var err error
	for range warmUpAttempts {
		start := time.Now()
		attempt, cancel := context.WithTimeout(ctx, warmUpTimeout)
		_, err = s.llm.Category(attempt, "лифт не работает", rules.Categories())
		cancel()
		if err == nil {
			s.log.InfoContext(ctx, "llm warmed up", "took", time.Since(start).Round(time.Millisecond).String())
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(s.warmUpPause):
		}
	}
	s.log.WarnContext(ctx, "llm warm-up gave up, hints use keywords until the model answers", "attempts", warmUpAttempts, "err", err)
}
