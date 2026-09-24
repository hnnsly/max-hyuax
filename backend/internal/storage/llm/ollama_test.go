package llm_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"dommax/internal/domain/rules"
	"dommax/internal/storage/llm"
)

// fakeOllama отвечает как /api/chat и запоминает последний запрос.
func fakeOllama(t *testing.T, status int, content string, wait time.Duration) (*httptest.Server, *map[string]any) {
	t.Helper()
	var mu sync.Mutex
	got := map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(raw, &got)
		mu.Unlock()
		if wait > 0 {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(wait):
			}
		}
		w.WriteHeader(status)
		b, _ := json.Marshal(map[string]any{"message": map[string]any{"role": "assistant", "content": content}, "done": true})
		w.Write(b)
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func TestCategoryFromStructuredAnswer(t *testing.T) {
	srv, got := fakeOllama(t, 200, `{"category":"leak"}`, 0)
	o := llm.NewOllama(srv.URL, "qwen3:1.7b", srv.Client())
	code, err := o.Category(t.Context(), "с потолка капает", rules.Categories())
	if err != nil || code != "leak" {
		t.Fatalf("code = %q, err = %v", code, err)
	}

	req := *got
	if req["model"] != "qwen3:1.7b" || req["stream"] != false || req["think"] != false {
		t.Errorf("request = %v", req)
	}
	if opts, _ := req["options"].(map[string]any); opts["temperature"] != 0.0 {
		t.Errorf("options = %v, want temperature 0", req["options"])
	}
	// Ответ ограничен схемой с закрытым списком кодов.
	props := req["format"].(map[string]any)["properties"].(map[string]any)
	enum := props["category"].(map[string]any)["enum"].([]any)
	if len(enum) != len(rules.Categories()) || !slices.Contains(enum, any("lift")) {
		t.Errorf("enum = %v", enum)
	}
	msgs := req["messages"].([]any)
	system := msgs[0].(map[string]any)["content"].(string)
	user := msgs[len(msgs)-1].(map[string]any)["content"].(string)
	if !strings.Contains(system, "lift") || !strings.Contains(system, "Лифт") || user != "с потолка капает" {
		t.Errorf("messages = %v", msgs)
	}
}

func TestCategoryErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		status  int
		content string
	}{
		"код не из списка": {200, `{"category":"teleport"}`},
		"не JSON":          {200, `лифт, наверное`},
		"ошибка сервера":   {500, `{}`},
		"модели нет":       {404, `{}`},
	} {
		t.Run(name, func(t *testing.T) {
			srv, _ := fakeOllama(t, tc.status, tc.content, 0)
			if code, err := llm.NewOllama(srv.URL, "m", srv.Client()).Category(t.Context(), "текст", rules.Categories()); err == nil {
				t.Fatalf("code = %q, want error", code)
			}
		})
	}
}

func TestCategoryRespectsDeadline(t *testing.T) {
	srv, _ := fakeOllama(t, 200, `{"category":"lift"}`, time.Second)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancel()
	_, err := llm.NewOllama(srv.URL, "m", srv.Client()).Category(ctx, "текст", rules.Categories())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want deadline exceeded", err)
	}
}

// Модель на CPU медленная: лишние параллельные запросы сразу получают отказ, а не очередь.
func TestCategoryLimitsConcurrency(t *testing.T) {
	srv, _ := fakeOllama(t, 200, `{"category":"lift"}`, 300*time.Millisecond)
	o := llm.NewOllama(srv.URL, "m", srv.Client())
	var wg sync.WaitGroup
	errs := make(chan error, llm.MaxParallel+3)
	for range llm.MaxParallel + 3 {
		wg.Go(func() {
			_, err := o.Category(t.Context(), "текст", rules.Categories())
			errs <- err
		})
	}
	wg.Wait()
	close(errs)
	busy := 0
	for err := range errs {
		if errors.Is(err, llm.ErrBusy) {
			busy++
		}
	}
	if busy != 3 {
		t.Fatalf("busy = %d, want 3", busy)
	}
}
