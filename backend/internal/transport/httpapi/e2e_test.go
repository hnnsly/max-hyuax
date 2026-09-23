//go:build integration

package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/gofiber/fiber/v3"

	"dommax/internal/app/auth"
	"dommax/internal/app/houses"
	"dommax/internal/app/issues"
	"dommax/internal/storage/maxapi"
	"dommax/internal/storage/postgres"
	"dommax/internal/storage/postgres/pgtest"
	"dommax/internal/transport/bot"
	"dommax/internal/transport/httpapi"
)

var (
	api      *fiber.App
	webhooks = &recorder{}
)

type recorder struct {
	mu  sync.Mutex
	got []maxapi.UpdateType
}

func (r *recorder) Handle(_ context.Context, u maxapi.Update) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.got = append(r.got, u.Type)
	return nil
}

func TestMain(m *testing.M) {
	ctx := context.Background()
	dsn, drop, err := pgtest.Fresh(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "integration db:", err)
		os.Exit(1)
	}
	store, err := postgres.Open(ctx, dsn)
	if err != nil {
		drop()
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	api = httpapi.New(httpapi.Deps{
		Auth: auth.NewService(store, auth.Config{
			BotToken: "t", SessionSecret: "s", SessionTTL: time.Hour, DemoEnabled: true, Now: time.Now,
		}),
		Issues:         issues.NewService(store, issues.Config{Now: time.Now, NewID: func() string { return uuid.NewV7().String() }, ConsentVersion: "v1"}),
		Houses:         houses.NewService(store),
		Webhook:        bot.NewWebhook("hook-secret", webhooks, store, log),
		Ping:           store.Ping,
		ConsentVersion: "v1",
		Now:            time.Now,
		Log:            log,
	})
	code := m.Run()
	store.Close()
	drop()
	os.Exit(code)
}

type resp struct {
	status int
	body   map[string]any
	list   []any
}

func call(t *testing.T, method, path, token string, body any) resp {
	t.Helper()
	var rd io.Reader = http.NoBody
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := api.Test(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := resp{status: res.StatusCode}
	if len(raw) > 0 && raw[0] == '[' {
		_ = json.Unmarshal(raw, &out.list)
	} else if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out.body)
	}
	return out
}

func expect(t *testing.T, r resp, status int, what string) resp {
	t.Helper()
	if r.status != status {
		t.Fatalf("%s: status %d, want %d, body %v", what, r.status, status, r.body)
	}
	return r
}

func login(t *testing.T, role string) string {
	t.Helper()
	r := expect(t, call(t, "POST", "/api/v1/auth/demo", "", map[string]string{"role": role}), 200, "demo "+role)
	return r.body["token"].(string)
}

func TestResidentReportsNeighbourJoinsOperatorWorks(t *testing.T) {
	expect(t, call(t, "GET", "/api/v1/health", "", nil), 200, "health")
	cats := expect(t, call(t, "GET", "/api/v1/categories", "", nil), 200, "categories")
	if len(cats.list) < 6 {
		t.Fatalf("categories = %v", cats.list)
	}
	expect(t, call(t, "GET", "/api/v1/me", "", nil), 401, "me without token")

	anna, sergey, oper := login(t, "resident"), login(t, "resident_2"), login(t, "uk_operator")

	me := expect(t, call(t, "GET", "/api/v1/me", anna, nil), 200, "me")
	if me.body["role"] != "resident" || me.body["has_consent"] != true {
		t.Fatalf("me = %v", me.body)
	}

	found := expect(t, call(t, "GET", "/api/v1/houses?query="+url.QueryEscape("17к2"), anna, nil), 200, "search")
	if len(found.list) != 1 {
		t.Fatalf("houses = %v", found.list)
	}
	obj := expect(t, call(t, "GET", "/api/v1/objects/h-17k2-e3-lift", anna, nil), 200, "qr object")
	if obj.body["house"].(map[string]any)["id"] != "h-17k2" {
		t.Fatalf("object = %v", obj.body)
	}
	similar := expect(t, call(t, "GET", "/api/v1/issues/similar?house_id=h-17k2&category=lighting&object_id=h-17k2-e1-light", anna, nil), 200, "similar")
	if len(similar.list) == 0 {
		t.Fatal("seeded lighting issue must be found as similar")
	}

	created := expect(t, call(t, "POST", "/api/v1/issues", anna, map[string]string{
		"house_id": "h-17k2", "object_id": "h-17k2-e3-lift", "description": "Лифт стоит на 9 этаже",
	}), 201, "report")
	id := created.body["id"].(string)
	if created.body["category"] != "lift" || created.body["participant_count"] != 1.0 || created.body["joined"] != true {
		t.Fatalf("created = %v", created.body)
	}

	joined := expect(t, call(t, "POST", "/api/v1/issues/"+id+"/join", sergey, nil), 200, "join")
	if joined.body["participant_count"] != 2.0 {
		t.Fatalf("joined = %v", joined.body)
	}
	again := expect(t, call(t, "POST", "/api/v1/issues/"+id+"/join", sergey, nil), 409, "join twice")
	if again.body["error"].(map[string]any)["code"] != "already_joined" {
		t.Fatalf("error = %v", again.body)
	}

	status := map[string]string{"status": "in_progress", "comment": "Мастер едет"}
	expect(t, call(t, "POST", "/api/v1/issues/"+id+"/status", anna, status), 403, "resident changes status")
	expect(t, call(t, "POST", "/api/v1/issues/"+id+"/status", oper, status), 200, "operator changes status")

	queue := expect(t, call(t, "GET", "/api/v1/uk/issues", oper, nil), 200, "uk queue")
	if len(queue.list) < 4 || queue.list[0].(map[string]any)["address"] == "" {
		t.Fatalf("queue = %v", queue.list)
	}
	expect(t, call(t, "GET", "/api/v1/uk/issues", anna, nil), 403, "resident queue")

	detail := expect(t, call(t, "GET", "/api/v1/issues/"+id, sergey, nil), 200, "detail")
	if detail.body["status"] != "in_progress" || detail.body["status_comment"] != "Мастер едет" ||
		detail.body["responsible"].(map[string]any)["name"] == "" || detail.body["basis"] == "" {
		t.Fatalf("detail = %v", detail.body)
	}
	expect(t, call(t, "GET", "/api/v1/issues/"+uuid.NewV7().String(), sergey, nil), 404, "unknown issue")
}

func TestWebhookChecksSecret(t *testing.T) {
	body := map[string]any{"update_type": "bot_started", "timestamp": 1, "chat_id": 7, "user": map[string]any{"user_id": 1}}
	send := func(secret string) int {
		b, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/webhook/max", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Max-Bot-Api-Secret", secret)
		res, err := api.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return res.StatusCode
	}
	if s := send("wrong"); s != 401 {
		t.Fatalf("wrong secret status = %d", s)
	}
	if s := send("hook-secret"); s != 200 {
		t.Fatalf("status = %d", s)
	}
}
