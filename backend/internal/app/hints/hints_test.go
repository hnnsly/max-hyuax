package hints_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"dommax/internal/app"
	"dommax/internal/app/hints"
	"dommax/internal/domain/rules"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// fakeLLM отвечает заданным кодом или ошибкой; wait имитирует долгий ответ модели.
type fakeLLM struct {
	code  string
	err   error
	wait  time.Duration
	calls int
	got   []string
}

func (f *fakeLLM) Category(ctx context.Context, _ string, options []rules.Rule) (string, error) {
	f.calls++
	for _, r := range options {
		f.got = append(f.got, r.Code)
	}
	if f.wait > 0 {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(f.wait):
		}
	}
	return f.code, f.err
}

func TestLLMHintWinsWhenValid(t *testing.T) {
	llm := &fakeLLM{code: "leak"}
	h, ok, err := hints.NewService(llm, time.Second, quiet).Suggest(t.Context(), "с потолка капает на пятом этаже")
	if err != nil || !ok || h.Rule.Code != "leak" || h.Source != hints.SourceLLM {
		t.Fatalf("hint = %+v, ok = %v, err = %v", h, ok, err)
	}
	if len(llm.got) != len(rules.Categories()) {
		t.Errorf("LLM got %v, want the whole closed list", llm.got)
	}
}

func TestFallsBackToRules(t *testing.T) {
	text := "не горит свет в подъезде"
	for name, llm := range map[string]*fakeLLM{
		"LLM недоступна":        {err: errors.New("connection refused")},
		"выдуманная категория":  {code: "teleport"},
		"пустой ответ":          {code: ""},
		"таймаут":               {code: "lift", wait: time.Second},
		"модель не узнала тему": {code: "other"},
	} {
		t.Run(name, func(t *testing.T) {
			h, ok, err := hints.NewService(llm, 20*time.Millisecond, quiet).Suggest(t.Context(), text)
			if err != nil || !ok || h.Rule.Code != "lighting" || h.Source != hints.SourceRules {
				t.Fatalf("hint = %+v, ok = %v, err = %v", h, ok, err)
			}
		})
	}
}

func TestWithoutLLMUsesRules(t *testing.T) {
	h, ok, err := hints.NewService(nil, 0, quiet).Suggest(t.Context(), "Лифт застрял между этажами")
	if err != nil || !ok || h.Rule.Code != "lift" || h.Source != hints.SourceRules {
		t.Fatalf("hint = %+v, ok = %v, err = %v", h, ok, err)
	}
}

func TestNothingRecognized(t *testing.T) {
	_, ok, err := hints.NewService(nil, 0, quiet).Suggest(t.Context(), "добрый день")
	if err != nil || ok {
		t.Fatalf("ok = %v, err = %v, want no hint", ok, err)
	}
	// Модель честно сказала «другое», правила тоже молчат: подсказываем «другое».
	h, ok, err := hints.NewService(&fakeLLM{code: "other"}, time.Second, quiet).Suggest(t.Context(), "скамейка сломана")
	if err != nil || !ok || h.Rule.Code != "other" || h.Source != hints.SourceLLM {
		t.Fatalf("hint = %+v, ok = %v, err = %v", h, ok, err)
	}
}

func TestRejectsBadText(t *testing.T) {
	llm := &fakeLLM{code: "lift"}
	svc := hints.NewService(llm, time.Second, quiet)
	for name, text := range map[string]string{
		"пусто":          "   ",
		"не UTF-8":       "\xea\xe0\xef\xe0\xe5\xf2",
		"слишком длинно": strings.Repeat("я", hints.MaxRunes+1),
	} {
		if _, _, err := svc.Suggest(t.Context(), text); !errors.Is(err, app.ErrInvalidInput) {
			t.Errorf("%s: err = %v, want invalid input", name, err)
		}
	}
	if llm.calls != 0 {
		t.Errorf("LLM called %d times for invalid text", llm.calls)
	}
}

// Прогрев загружает модель в память при старте, чтобы первая подсказка не упёрлась в таймаут.
func TestWarmUpLoadsModelOnce(t *testing.T) {
	llm := &fakeLLM{code: "lift"}
	hints.NewService(llm, time.Millisecond, quiet).WarmUp(t.Context())
	if llm.calls != 1 {
		t.Fatalf("calls = %d, want 1", llm.calls)
	}
	hints.NewService(nil, 0, quiet).WarmUp(t.Context()) // без модели ничего не делает
	// Модель ещё не готова: попытки повторяются, пока контекст не отменят.
	failing := &fakeLLM{err: errors.New("model is still downloading")}
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	hints.NewService(failing, time.Millisecond, quiet).WarmUp(ctx)
	if failing.calls < 1 || ctx.Err() == nil {
		t.Fatalf("calls = %d, ctx err = %v: warm-up must retry until cancelled", failing.calls, ctx.Err())
	}
}
