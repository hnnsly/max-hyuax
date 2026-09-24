// Пакет llm — клиент self-hosted модели в Ollama для подсказки категории (ADR-008).
// Ответ модели ограничен JSON-схемой с закрытым списком кодов справочника.
package llm

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"dommax/internal/domain/rules"
)

// MaxParallel — сколько запросов к модели выполняется одновременно; остальные сразу получают ErrBusy.
const MaxParallel = 2

var ErrBusy = errors.New("llm: busy")

type Ollama struct {
	base  string
	model string
	hc    *http.Client
	slots chan struct{}
}

func NewOllama(baseURL, model string, hc *http.Client) *Ollama {
	return &Ollama{base: strings.TrimRight(baseURL, "/"), model: model, hc: hc, slots: make(chan struct{}, MaxParallel)}
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Category выбирает код категории из options по тексту жителя.
func (o *Ollama) Category(ctx context.Context, text string, options []rules.Rule) (string, error) {
	select {
	case o.slots <- struct{}{}:
		defer func() { <-o.slots }()
	default:
		return "", ErrBusy
	}

	codes := make([]string, len(options))
	var list strings.Builder
	for i, r := range options {
		codes[i] = r.Code
		fmt.Fprintf(&list, "- %s: %s\n", r.Code, r.Title)
	}
	body, err := json.Marshal(map[string]any{
		"model": o.model,
		"messages": []message{
			{Role: "system", Content: "Ты помогаешь жителям многоквартирного дома оформить заявку о поломке общего имущества. " +
				"Выбери одну категорию из списка по описанию жителя. Если ни одна не подходит, выбери other. " +
				"Ответь JSON вида {\"category\": \"<код>\"}.\n\nКатегории:\n" + list.String()},
			{Role: "user", Content: text},
		},
		"stream": false,
		"think":  false,
		"format": map[string]any{
			"type":       "object",
			"properties": map[string]any{"category": map[string]any{"type": "string", "enum": codes}},
			"required":   []string{"category"},
		},
		"options":    map[string]any{"temperature": 0, "num_predict": 32},
		"keep_alive": "30m",
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.base+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := o.hc.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm: ollama status %d", res.StatusCode)
	}
	var out struct {
		Message message `json:"message"`
	}
	if err := json.UnmarshalRead(res.Body, &out); err != nil {
		return "", fmt.Errorf("llm: decode response: %w", err)
	}
	var answer struct {
		Category string `json:"category"`
	}
	if err := json.Unmarshal([]byte(out.Message.Content), &answer); err != nil {
		return "", fmt.Errorf("llm: answer is not JSON: %w", err)
	}
	if !slices.Contains(codes, answer.Category) {
		return "", fmt.Errorf("llm: category %q is not in the list", answer.Category)
	}
	return answer.Category, nil
}
