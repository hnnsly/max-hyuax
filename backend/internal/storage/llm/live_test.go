//go:build live

// Проверка на настоящем Ollama: OLLAMA_URL=http://localhost:11434 go test -tags live ./internal/storage/llm
// Модель — OLLAMA_MODEL (по умолчанию qwen3:4b), она должна быть скачана заранее.
package llm_test

import (
	"cmp"
	"net/http"
	"os"
	"testing"
	"time"

	"dommax/internal/domain/rules"
	"dommax/internal/storage/llm"
)

func TestLiveOllamaClassifies(t *testing.T) {
	base := os.Getenv("OLLAMA_URL")
	if base == "" {
		t.Skip("OLLAMA_URL is not set")
	}
	o := llm.NewOllama(base, cmp.Or(os.Getenv("OLLAMA_MODEL"), "qwen3:4b"), &http.Client{Timeout: 2 * time.Minute})
	// Фразы, где ключевых слов справочника нет или они уводят не туда («светится» — не про свет):
	// здесь как раз нужна модель.
	cases := map[string]string{
		"Вода льётся по стене на пятом этаже":          "leak",
		"Подъёмник не едет, кнопка вызова не светится": "lift",
		"На площадке кромешная тьма, ничего не видно":  "lighting",
		"Магнитный ключ не открывает вход в подъезд":   "door",
	}
	hits := 0
	for text, want := range cases {
		start := time.Now()
		got, err := o.Category(t.Context(), text, rules.Categories())
		if err != nil {
			t.Fatalf("%q: %v", text, err)
		}
		t.Logf("%q → %s (%s, want %s)", text, got, time.Since(start).Round(time.Millisecond), want)
		if got == want {
			hits++
		}
	}
	// Модель маленькая и может ошибиться; ответ всё равно из списка, житель подтверждает сам.
	if hits < 3 {
		t.Errorf("model matched %d of %d phrases", hits, len(cases))
	}
}
