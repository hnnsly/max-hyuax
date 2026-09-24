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
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5"

	"dommax/internal/app/appeal"
	"dommax/internal/app/auth"
	"dommax/internal/app/hints"
	"dommax/internal/app/houses"
	"dommax/internal/app/issues"
	"dommax/internal/storage/maxapi"
	"dommax/internal/storage/postgres"
	"dommax/internal/storage/postgres/pgtest"
	"dommax/internal/transport/bot"
	"dommax/internal/transport/httpapi"
)

var (
	api       *fiber.App
	noDemoAPI *fiber.App // тот же API с выключенным демо-входом
	testDSN   string
	webhooks  = &recorder{}
)

const botToken = "t"

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
	testDSN = dsn
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	deps := func(demo bool) httpapi.Deps {
		return httpapi.Deps{
			Auth: auth.NewService(store, auth.Config{
				BotToken: botToken, SessionSecret: "s", SessionTTL: time.Hour, DemoEnabled: demo, Now: time.Now,
			}),
			Issues:         issues.NewService(store, issues.Config{Now: time.Now, NewID: func() string { return uuid.NewV7().String() }, ConsentVersion: "v1"}),
			Houses:         houses.NewService(store),
			Hints:          hints.NewService(nil, time.Second, log),
			Appeal:         appeal.NewService(store, appeal.Config{Secret: []byte("s"), TTL: 10 * time.Minute, Now: time.Now}),
			Webhook:        bot.NewWebhook("hook-secret", webhooks, store, log),
			Ping:           store.Ping,
			ConsentVersion: "v1",
			Now:            time.Now,
			Log:            log,
		}
	}
	api = httpapi.New(deps(true))
	noDemoAPI = httpapi.New(deps(false))
	code := m.Run()
	store.Close()
	drop()
	os.Exit(code)
}

type resp struct {
	status int
	ctype  string
	body   map[string]any
	list   []any
}

func call(t *testing.T, method, path, token string, body any) resp {
	t.Helper()
	var raw []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		raw = b
	}
	return callRaw(t, api, method, path, token, raw)
}

// callRaw отправляет тело как есть: нужен для битого JSON и неверной кодировки.
func callRaw(t *testing.T, app *fiber.App, method, path, token string, body []byte) resp {
	t.Helper()
	var rd io.Reader = http.NoBody
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := resp{status: res.StatusCode, ctype: res.Header.Get("Content-Type")}
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
	if detail.body["place"] != "подъезд 3, пассажирский лифт" {
		t.Fatalf("place = %v", detail.body["place"])
	}
	expect(t, call(t, "GET", "/api/v1/issues/"+uuid.NewV7().String(), sergey, nil), 404, "unknown issue")

	mine := expect(t, call(t, "GET", "/api/v1/me/issues", sergey, nil), 200, "my issues")
	if len(mine.list) == 0 || mine.list[0].(map[string]any)["place"] == "" || mine.list[0].(map[string]any)["address"] == "" {
		t.Fatalf("mine = %v", mine.list)
	}
	tl := expect(t, call(t, "GET", "/api/v1/issues/"+id+"/timeline", sergey, nil), 200, "timeline")
	if len(tl.list) != 3 {
		t.Fatalf("timeline = %v, want created, joined, status_changed", tl.list)
	}
	last := tl.list[2].(map[string]any)
	if last["kind"] != "status_changed" || last["comment"] != "Мастер едет" || last["user_id"] != nil {
		t.Fatalf("last event = %v (no user ids must leak)", last)
	}
}

func TestOperatorSeesMetrics(t *testing.T) {
	oper := login(t, "uk_operator")
	m := expect(t, call(t, "GET", "/api/v1/uk/metrics", oper, nil), 200, "uk metrics").body
	for _, key := range []string{"first_response_median_min", "prev_week_median_min", "reports_per_issue",
		"issues_total", "closed_total", "closed_on_time", "open_total", "overdue_open", "period_days"} {
		if _, ok := m[key]; !ok {
			t.Errorf("metrics have no %q: %v", key, m)
		}
	}
	days, _ := m["first_response_by_day"].([]any)
	if len(days) != 7 || m["sample_data"] != true || m["period_days"] != float64(30) {
		t.Fatalf("metrics = %v", m)
	}
	if day := days[6].(map[string]any); len(day["date"].(string)) != len("2026-09-17") {
		t.Errorf("day = %v", day)
	}
	if m["closed_total"].(float64) < m["closed_on_time"].(float64) || m["reports_per_issue"].(float64) < 1 {
		t.Errorf("inconsistent metrics: %v", m)
	}
}

func TestAppealPDFForOverdueIssue(t *testing.T) {
	anna, sergey := login(t, "resident"), login(t, "resident_2")
	created := expect(t, call(t, "POST", "/api/v1/issues", anna, map[string]string{
		"house_id": "h-17k2", "object_id": "h-17k2-e1-lift", "description": "Кабина не приходит",
	}), 201, "report").body
	id := created["id"].(string)
	path := "/api/v1/issues/" + id + "/appeal"

	early := expect(t, call(t, "POST", path, anna, nil), 409, "appeal before deadline").body
	if early["error"].(map[string]any)["code"] != "not_overdue" {
		t.Fatalf("early = %v", early)
	}
	conn, err := pgx.Connect(t.Context(), testDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(t.Context())
	if _, err := conn.Exec(t.Context(), `UPDATE issues SET deadline_at = now() - interval '1 hour' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}

	expect(t, call(t, "POST", path, sergey, nil), 403, "not a participant")
	link := expect(t, call(t, "POST", path, anna, nil), 200, "appeal link").body
	url, _ := link["url"].(string)
	if !strings.HasPrefix(url, "/api/v1/appeal/") || link["expires_at"] == nil || link["file_name"] == nil {
		t.Fatalf("link = %v", link)
	}

	// Ссылка открывается без заголовка авторизации: так скачивает WebApp.downloadFile в MAX.
	res, err := api.Test(httptest.NewRequest("GET", url, http.NoBody))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "application/pdf" ||
		!strings.Contains(res.Header.Get("Content-Disposition"), "attachment") || !bytes.HasPrefix(body, []byte("%PDF-")) {
		t.Fatalf("pdf: %d %q %q %q", res.StatusCode, res.Header.Get("Content-Type"), res.Header.Get("Content-Disposition"), body[:min(len(body), 8)])
	}
	bad := callRaw(t, api, "GET", "/api/v1/appeal/forged.token", "", nil)
	if bad.status != 403 || bad.body["error"].(map[string]any)["code"] != "forbidden" {
		t.Fatalf("forged link = %d %v", bad.status, bad.body)
	}
}

func TestClassifyHint(t *testing.T) {
	anna := login(t, "resident")
	got := expect(t, call(t, "POST", "/api/v1/classify", anna, map[string]string{"text": "Не горит свет на пятом этаже"}), 200, "classify").body
	if got["category"] != "lighting" || got["title"] != "Свет в подъезде" || got["source"] != "rules" {
		t.Fatalf("hint = %v", got)
	}
	none := expect(t, call(t, "POST", "/api/v1/classify", anna, map[string]string{"text": "добрый день"}), 200, "classify nothing").body
	if v, ok := none["category"]; !ok || v != nil {
		t.Fatalf("no hint = %v, want category null", none)
	}
}

func TestDeleteAccountAndDemoRestore(t *testing.T) {
	old := login(t, "resident_2")
	expect(t, call(t, "DELETE", "/api/v1/me", old, nil), 204, "delete account")
	expect(t, call(t, "GET", "/api/v1/me", old, nil), 401, "token of deleted account")

	again := login(t, "resident_2")
	me := expect(t, call(t, "GET", "/api/v1/me", again, nil), 200, "restored demo user").body
	if me["first_name"] != "Сергей" || me["has_consent"] != false {
		t.Fatalf("restored = %v, want name back and consent required", me)
	}
	// Согласие возвращаем, чтобы демо-пользователь остался рабочим для остальных тестов.
	expect(t, call(t, "POST", "/api/v1/me/consent", again, map[string]string{"version": "v1"}), 200, "consent again")
}

func TestOperatorListsOwnHouses(t *testing.T) {
	otherOrgHouse(t)
	list := expect(t, call(t, "GET", "/api/v1/uk/houses", login(t, "uk_operator"), nil), 200, "uk houses").list
	if len(list) != 6 {
		t.Fatalf("houses = %v, want 6 houses of org-orekh", list)
	}
	for _, h := range list {
		if h.(map[string]any)["id"] == "h-other" {
			t.Fatal("house of another UK must not be listed")
		}
	}
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
