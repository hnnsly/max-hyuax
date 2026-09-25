//go:build integration

package httpapi_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5"

	"dommax/internal/app/appeal"
	"dommax/internal/app/apptest"
	"dommax/internal/app/auth"
	"dommax/internal/app/council"
	"dommax/internal/app/hints"
	"dommax/internal/app/houses"
	"dommax/internal/app/issues"
	"dommax/internal/app/photos"
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
				BotToken: botToken, SessionSecret: "s", SessionTTL: time.Hour, DemoEnabled: demo, ConsentVersion: "v1", Now: time.Now,
			}),
			Issues:         issues.NewService(store, issues.Config{Now: time.Now, NewID: func() string { return uuid.NewV7().String() }, ConsentVersion: "v1"}),
			Houses:         houses.NewService(store, nil),
			Hints:          hints.NewService(nil, time.Second, log),
			Appeal:         appeal.NewService(store, appeal.Config{Secret: []byte("s"), TTL: 10 * time.Minute, Now: time.Now}),
			Photos:         photos.NewService(store, &apptest.MemFiles{}, photos.Config{Now: time.Now, NewID: func() string { return uuid.NewV7().String() }, ConsentVersion: "v1"}),
			Council:        council.NewService(store, council.Config{Now: time.Now, NewID: func() string { return uuid.NewV7().String() }, ConsentVersion: "v1"}),
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

// uploadPhotos отправляет файлы полем photo в multipart/form-data.
func uploadPhotos(t *testing.T, path, token string, files ...[]byte) resp {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for i, f := range files {
		w, err := mw.CreateFormFile("photo", fmt.Sprintf("p%d.jpg", i))
		if err != nil {
			t.Fatal(err)
		}
		w.Write(f)
	}
	mw.Close()
	req := httptest.NewRequest("POST", path, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := api.Test(req)
	if err != nil {
		t.Fatal(err)
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

func testJPEG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 80, 60)), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPhotosUploadListAndOpen(t *testing.T) {
	anna, sergey, oper := login(t, "resident"), login(t, "resident_2"), login(t, "uk_operator")
	id := expect(t, call(t, "POST", "/api/v1/issues", anna, map[string]string{"house_id": "h-17k2", "category": "door"}), 201, "report").body["id"].(string)
	path := "/api/v1/issues/" + id + "/photos"

	added := uploadPhotos(t, path, anna, testJPEG(t), testJPEG(t))
	if added.status != 201 || len(added.list) != 2 {
		t.Fatalf("upload = %d %v %v", added.status, added.body, added.list)
	}
	first := added.list[0].(map[string]any)
	if first["width"] != 80.0 || !strings.HasPrefix(first["url"].(string), "/api/v1/photos/") || first["mine"] != true {
		t.Fatalf("photo = %v", first)
	}

	list := expect(t, call(t, "GET", path, oper, nil), 200, "operator list").list
	if len(list) != 2 || list[0].(map[string]any)["mine"] != false {
		t.Fatalf("list = %v", list)
	}
	res, err := api.Test(func() *http.Request {
		r := httptest.NewRequest("GET", first["url"].(string), http.NoBody)
		r.Header.Set("Authorization", "Bearer "+anna)
		return r
	}())
	if err != nil {
		t.Fatal(err)
	}
	img, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "image/jpeg" || !bytes.HasPrefix(img, []byte{0xFF, 0xD8}) {
		t.Fatalf("open photo: %d %q", res.StatusCode, res.Header.Get("Content-Type"))
	}

	// Сосед, который не присоединился, фото не видит и добавить не может.
	expect(t, call(t, "GET", path, sergey, nil), 403, "non-participant list")
	expect(t, call(t, "GET", first["url"].(string), sergey, nil), 403, "non-participant open")
	if r := uploadPhotos(t, path, sergey, testJPEG(t)); r.status != 403 {
		t.Fatalf("non-participant upload = %d", r.status)
	}

	for name, tc := range map[string]struct {
		files  [][]byte
		status int
		code   string
	}{
		"не картинка":   {[][]byte{[]byte("%PDF-1.7")}, 415, "unsupported_media"},
		"больше 5 МБ":   {[][]byte{append(testJPEG(t), make([]byte, 5<<20)...)}, 413, "photo_too_large"},
		"без файлов":    {nil, 422, "invalid_input"},
		"четыре за раз": {[][]byte{testJPEG(t), testJPEG(t), testJPEG(t), testJPEG(t)}, 422, "invalid_input"},
	} {
		r := uploadPhotos(t, path, anna, tc.files...)
		if r.status != tc.status || r.body["error"].(map[string]any)["code"] != tc.code {
			t.Errorf("%s: %d %v, want %d %s", name, r.status, r.body, tc.status, tc.code)
		}
	}
	// Пачка с одним плохим файлом не сохраняет и хорошие.
	if r := uploadPhotos(t, path, anna, testJPEG(t), []byte("%PDF-1.7")); r.status != 415 {
		t.Fatalf("mixed batch = %d %v", r.status, r.body)
	}
	if n := len(expect(t, call(t, "GET", path, anna, nil), 200, "list after mixed batch").list); n != 2 {
		t.Fatalf("after mixed batch %d photos, want 2", n)
	}
	// Лимит на заявку: 2 уже есть, пачка из трёх проходит, следующая пачка из двух — 409 целиком.
	if r := uploadPhotos(t, path, anna, testJPEG(t), testJPEG(t), testJPEG(t)); r.status != 201 {
		t.Fatalf("fill up = %d %v", r.status, r.body)
	}
	if r := uploadPhotos(t, path, anna, testJPEG(t), testJPEG(t)); r.status != 409 || r.body["error"].(map[string]any)["code"] != "too_many_photos" {
		t.Fatalf("over limit = %d %v", r.status, r.body)
	}
	if n := len(expect(t, call(t, "GET", path, anna, nil), 200, "list after limit").list); n != 5 {
		t.Fatalf("after over-limit batch %d photos, want 5", n)
	}

	// Убрать фото может только тот, кто его приложил.
	photoPath := first["url"].(string)
	expect(t, call(t, "DELETE", photoPath, oper, nil), 403, "operator removes resident photo")
	expect(t, call(t, "DELETE", photoPath, anna, nil), 204, "uploader removes photo")
	expect(t, call(t, "GET", photoPath, anna, nil), 404, "removed photo")
}

// Жители проверяют ремонт: один подтверждает, другой возвращает заявку в работу с комментарием.
func TestConfirmAndReopenRepair(t *testing.T) {
	anna, sergey, oper := login(t, "resident"), login(t, "resident_2"), login(t, "uk_operator")
	id := expect(t, call(t, "POST", "/api/v1/issues", anna, map[string]string{"house_id": "h-17k2", "category": "lighting"}), 201, "report").body["id"].(string)
	path := "/api/v1/issues/" + id
	expect(t, call(t, "POST", path+"/join", sergey, nil), 200, "join")
	expect(t, call(t, "POST", path+"/confirm", sergey, nil), 409, "confirm before done")
	for _, st := range []string{"in_progress", "done"} {
		expect(t, call(t, "POST", path+"/status", oper, map[string]string{"status": st, "comment": "Заменили лампы"}), 200, st)
	}

	card := expect(t, call(t, "POST", path+"/confirm", sergey, nil), 200, "confirm").body
	if card["confirmed_count"] != 1.0 || card["my_answer"] != "fixed" || card["answer_until"] == nil {
		t.Fatalf("after confirm = %v", card)
	}
	if again := expect(t, call(t, "POST", path+"/confirm", sergey, nil), 409, "confirm twice").body; again["error"].(map[string]any)["code"] != "already_answered" {
		t.Fatalf("twice = %v", again)
	}
	expect(t, call(t, "POST", path+"/confirm", oper, nil), 403, "operator confirms")
	if empty := expect(t, call(t, "POST", path+"/reopen", anna, map[string]string{"comment": " "}), 422, "reopen without comment").body; empty["error"].(map[string]any)["code"] != "comment_required" {
		t.Fatalf("empty comment = %v", empty)
	}
	reopened := expect(t, call(t, "POST", path+"/reopen", anna, map[string]string{"comment": "На третьем этаже темно"}), 200, "reopen").body
	if reopened["status"] != "in_progress" || reopened["reopened_at"] == nil || reopened["confirmed_count"] != 0.0 {
		t.Fatalf("after reopen = %v", reopened)
	}
	if nd := expect(t, call(t, "POST", path+"/confirm", sergey, nil), 409, "confirm reopened").body; nd["error"].(map[string]any)["code"] != "not_done" {
		t.Fatalf("confirm reopened = %v", nd)
	}

	kinds := map[string]string{}
	for _, e := range expect(t, call(t, "GET", path+"/timeline", anna, nil), 200, "timeline").list {
		ev := e.(map[string]any)
		kinds[ev["kind"].(string)], _ = ev["comment"].(string)
	}
	if _, ok := kinds["confirmed"]; !ok || kinds["reopened"] != "На третьем этаже темно" {
		t.Fatalf("timeline kinds = %v", kinds)
	}
}

// signContact подписывает номер, как клиент MAX в requestContact (dev-max/docs/webapps/bridge.md).
func signContact(authDate, phone string, maxID int64) string {
	mac := hmac.New(sha256.New, []byte(botToken))
	mac.Write([]byte("authDate=" + authDate + "\nphone=" + strings.TrimPrefix(phone, "+") + "\nuserId=" + strconv.FormatInt(maxID, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}

// Житель MAX оставляет телефон для мастера: его видит только УК заявки, убрать можно в любой момент.
func TestPhoneForTheRepairman(t *testing.T) {
	const maxID = 990777
	olga, sergey, oper := maxLogin(t, maxID), login(t, "resident_2"), login(t, "uk_operator")
	expect(t, call(t, "POST", "/api/v1/me/consent", olga, map[string]string{"version": "v1"}), 200, "consent")
	id := expect(t, call(t, "POST", "/api/v1/issues", olga, map[string]string{"house_id": "h-17k2", "category": "door"}), 201, "report").body["id"].(string)
	expect(t, call(t, "POST", "/api/v1/issues/"+id+"/join", sergey, nil), 200, "join")

	now := strconv.FormatInt(time.Now().Unix(), 10)
	contact := map[string]string{"phone": "+79991234567", "auth_date": now, "hash": signContact(now, "+79991234567", maxID)}
	forged := map[string]string{"phone": "+79990000000", "auth_date": now, "hash": contact["hash"]}
	if bad := expect(t, call(t, "POST", "/api/v1/me/phone", olga, forged), 422, "forged phone").body; bad["error"].(map[string]any)["code"] != "invalid_contact" {
		t.Fatalf("forged = %v", bad)
	}
	expect(t, call(t, "POST", "/api/v1/me/phone", sergey, contact), 403, "demo user shares phone")
	me := expect(t, call(t, "POST", "/api/v1/me/phone", olga, contact), 200, "share phone").body
	if me["phone_shared"] != true || me["phone"] != nil {
		t.Fatalf("me = %v, want phone_shared without the number", me)
	}

	contacts := func(token string) any {
		return expect(t, call(t, "GET", "/api/v1/issues/"+id, token, nil), 200, "card").body["contacts"]
	}
	if got := contacts(oper).([]any); len(got) != 1 || got[0].(map[string]any)["phone"] != "+79991234567" || got[0].(map[string]any)["first_name"] != "Ольга" {
		t.Fatalf("operator contacts = %v", got)
	}
	if got := contacts(sergey); got != nil {
		t.Fatalf("neighbour sees contacts: %v", got)
	}

	me = expect(t, call(t, "DELETE", "/api/v1/me/phone", olga, nil), 200, "hide phone").body
	if me["phone_shared"] != false {
		t.Fatalf("after hide = %v", me)
	}
	if got := contacts(oper); got != nil {
		t.Fatalf("contacts after hide = %v", got)
	}

	// У демо-жительницы Анны синтетический телефон: УК видит его в демо без клиента MAX.
	annaIssue := expect(t, call(t, "POST", "/api/v1/issues", login(t, "resident"), map[string]string{"house_id": "h-17k2", "category": "door"}), 201, "anna report").body["id"].(string)
	card := expect(t, call(t, "GET", "/api/v1/issues/"+annaIssue, oper, nil), 200, "anna card").body
	if got, _ := card["contacts"].([]any); len(got) != 1 || got[0].(map[string]any)["phone"] != "+79990000001" {
		t.Fatalf("demo contacts = %v", card["contacts"])
	}
}

// Кабинет района: сравнение трёх УК Зябликово и просроченные заявки района, только чтение.
func TestDistrictCabinet(t *testing.T) {
	district := login(t, "district")
	me := expect(t, call(t, "GET", "/api/v1/me", district, nil), 200, "district me").body
	if me["role"] != "district" || me["district"] != "Зябликово" {
		t.Fatalf("me = %v", me)
	}
	m := expect(t, call(t, "GET", "/api/v1/district/metrics", district, nil), 200, "district metrics").body
	orgs := m["organizations"].([]any)
	if m["district"] != "Зябликово" || m["period_days"] != 30.0 || len(orgs) != 3 {
		t.Fatalf("metrics = %v", m)
	}
	byID := map[string]map[string]any{}
	for _, o := range orgs {
		byID[o.(map[string]any)["id"].(string)] = o.(map[string]any)
	}
	if k := byID["org-kashir"]; k == nil || k["overdue_open"].(float64) < 3 || k["name"] != "УК «Каширский квартал»" || k["sample_data"] != true {
		t.Fatalf("kashir = %v", k)
	}
	if y := byID["org-yasen"]; y == nil || y["overdue_open"] != 0.0 || y["confirmed_by_residents"] != 5.0 {
		t.Fatalf("yasen = %v", y)
	}

	overdue := expect(t, call(t, "GET", "/api/v1/district/overdue", district, nil), 200, "district overdue").list
	if len(overdue) < 5 {
		t.Fatalf("overdue = %d issues", len(overdue))
	}
	for _, it := range overdue {
		if is := it.(map[string]any); is["overdue"] != true || is["address"] == "" {
			t.Fatalf("overdue issue = %v", is)
		}
	}
	// Карточку район открывает, но менять статус не может.
	id := overdue[0].(map[string]any)["id"].(string)
	expect(t, call(t, "GET", "/api/v1/issues/"+id, district, nil), 200, "district opens the card")
	expect(t, call(t, "POST", "/api/v1/issues/"+id+"/status", district, map[string]string{"status": "done"}), 403, "district changes status")
	for _, role := range []string{"resident", "uk_operator"} {
		expect(t, call(t, "GET", "/api/v1/district/metrics", login(t, role), nil), 403, role+" on district metrics")
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
	old, oper := login(t, "resident_2"), login(t, "uk_operator")
	// Фото удалённого аккаунта исчезают из заявки: на снимках бывают люди и двери квартир.
	id := expect(t, call(t, "POST", "/api/v1/issues", old, map[string]string{"house_id": "h-17k2", "category": "door"}), 201, "report").body["id"].(string)
	photosPath := "/api/v1/issues/" + id + "/photos"
	if r := uploadPhotos(t, photosPath, old, testJPEG(t)); r.status != 201 {
		t.Fatalf("upload = %d %v", r.status, r.body)
	}
	expect(t, call(t, "DELETE", "/api/v1/me", old, nil), 204, "delete account")
	expect(t, call(t, "GET", "/api/v1/me", old, nil), 401, "token of deleted account")
	if n := len(expect(t, call(t, "GET", photosPath, oper, nil), 200, "photos after delete").list); n != 0 {
		t.Fatalf("photos after account deletion = %d, want 0", n)
	}

	// Демо-пользователь возвращается в исходное состояние: проверка по DATA-API после удаления проходит.
	again := login(t, "resident_2")
	me := expect(t, call(t, "GET", "/api/v1/me", again, nil), 200, "restored demo user").body
	if me["first_name"] != "Сергей" || me["has_consent"] != true || me["house_id"] != "h-17k2" {
		t.Fatalf("restored = %v, want the demo user as seeded", me)
	}
	expect(t, call(t, "POST", "/api/v1/issues", again, map[string]string{"house_id": "h-17k2", "category": "lift"}), 201, "report after restore")
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

func TestHouseReportPDF(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/houses/h-17k2/report.pdf", http.NoBody)
	res, err := api.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	raw, _ := io.ReadAll(res.Body)
	if !bytes.HasPrefix(raw, []byte("%PDF-1.4")) {
		t.Fatalf("not a PDF: %q", raw[:min(16, len(raw))])
	}
	expect(t, call(t, "GET", "/api/v1/houses/no-such-house/report.pdf", "", nil), 404, "missing house report")
}
