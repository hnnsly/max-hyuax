//go:build integration

package httpapi_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5"
)

// signInitData подписывает initData так же, как MAX (dev-max/docs/webapps/validation.md).
func signInitData(params map[string]string) string {
	var lines, pairs []string
	for _, k := range slices.Sorted(maps.Keys(params)) {
		lines = append(lines, k+"="+params[k])
		pairs = append(pairs, k+"="+url.QueryEscape(params[k]))
	}
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(botToken))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(strings.Join(lines, "\n")))
	return strings.Join(pairs, "&") + "&hash=" + hex.EncodeToString(mac.Sum(nil))
}

// maxLogin — вход нового жителя MAX: у него ещё нет согласия на обработку ПДн.
func maxLogin(t *testing.T, maxID int64) string {
	t.Helper()
	raw := signInitData(map[string]string{
		"auth_date": strconv.FormatInt(time.Now().Unix(), 10),
		"user":      `{"id":` + strconv.FormatInt(maxID, 10) + `,"first_name":"Ольга"}`,
	})
	r := expect(t, call(t, "POST", "/api/v1/auth/max", "", map[string]string{"init_data": raw}), 200, "max login")
	return r.body["token"].(string)
}

// otherOrgHouse добавляет дом чужой УК, чтобы проверить права оператора.
func otherOrgHouse(t *testing.T) {
	t.Helper()
	conn, err := pgx.Connect(t.Context(), testDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(t.Context())
	_, err = conn.Exec(t.Context(), `
		INSERT INTO organizations (id, type, name) VALUES ('org-other', 'uk', 'УК «Другая»') ON CONFLICT DO NOTHING;
		INSERT INTO houses (id, address, organization_id, entrances_count) VALUES ('h-other', 'Другая улица, 1', 'org-other', 1) ON CONFLICT DO NOTHING;`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestErrorResponses(t *testing.T) {
	otherOrgHouse(t)
	anna, oper := login(t, "resident"), login(t, "uk_operator")
	fresh := maxLogin(t, 990001)

	// Заявка в чужой УК: оператор «Орехового квартала» не может менять её статус.
	foreign := expect(t, call(t, "POST", "/api/v1/issues", anna, map[string]string{"house_id": "h-other", "category": "lift"}), 201, "foreign issue")
	foreignID := foreign.body["id"].(string)
	// Свежая заявка в статусе «отправлена» для проверки переходов.
	sent := expect(t, call(t, "POST", "/api/v1/issues", anna, map[string]string{"house_id": "h-17k2", "category": "door"}), 201, "sent issue")
	sentID := sent.body["id"].(string)
	const doneID = "0190a000-0000-7000-8000-000000000003" // демо-заявка в статусе «выполнена»
	unknown := uuid.NewV7().String()

	cases := []struct {
		name   string
		app    *fiber.App
		method string
		path   string
		token  string
		body   string
		status int
		code   string
	}{
		{"no token", api, "GET", "/api/v1/me", "", "", 401, "unauthorized"},
		{"broken token", api, "GET", "/api/v1/me", "abc.def", "", 401, "unauthorized"},
		{"demo login disabled", noDemoAPI, "POST", "/api/v1/auth/demo", "", `{"role":"resident"}`, 403, "forbidden"},
		{"unknown demo role", api, "POST", "/api/v1/auth/demo", "", `{"role":"admin"}`, 422, "invalid_input"},
		{"forged initData", api, "POST", "/api/v1/auth/max", "", `{"init_data":"user=%7B%22id%22%3A1%7D&hash=00"}`, 401, "unauthorized"},
		{"resident on UK queue", api, "GET", "/api/v1/uk/issues", anna, "", 403, "forbidden"},
		{"resident on UK metrics", api, "GET", "/api/v1/uk/metrics", anna, "", 403, "forbidden"},
		{"metrics without token", api, "GET", "/api/v1/uk/metrics", "", "", 401, "unauthorized"},
		{"resident on UK houses", api, "GET", "/api/v1/uk/houses", anna, "", 403, "forbidden"},
		{"classify without token", api, "POST", "/api/v1/classify", "", `{"text":"лифт"}`, 401, "unauthorized"},
		{"classify empty text", api, "POST", "/api/v1/classify", anna, `{"text":"  "}`, 422, "invalid_input"},
		{"classify broken JSON", api, "POST", "/api/v1/classify", anna, `{"text":`, 422, "invalid_input"},
		{"appeal without token", api, "POST", "/api/v1/issues/" + sentID + "/appeal", "", "", 401, "unauthorized"},
		{"appeal for unknown issue", api, "POST", "/api/v1/issues/" + unknown + "/appeal", anna, "", 404, "not_found"},
		{"appeal by a non-participant", api, "POST", "/api/v1/issues/" + doneID + "/appeal", anna, "", 403, "forbidden"},
		{"resident changes status", api, "POST", "/api/v1/issues/" + sentID + "/status", anna, `{"status":"accepted"}`, 403, "forbidden"},
		{"operator of another UK", api, "POST", "/api/v1/issues/" + foreignID + "/status", oper, `{"status":"accepted"}`, 403, "forbidden"},
		{"report without consent", api, "POST", "/api/v1/issues", fresh, `{"house_id":"h-17k2","category":"lift"}`, 403, "consent_required"},
		{"join without consent", api, "POST", "/api/v1/issues/" + sentID + "/join", fresh, "", 403, "consent_required"},
		{"unknown issue", api, "GET", "/api/v1/issues/" + unknown, anna, "", 404, "not_found"},
		{"unknown issue timeline", api, "GET", "/api/v1/issues/" + unknown + "/timeline", anna, "", 404, "not_found"},
		{"join unknown issue", api, "POST", "/api/v1/issues/" + unknown + "/join", anna, "", 404, "not_found"},
		{"unknown house", api, "GET", "/api/v1/houses/nope", anna, "", 404, "not_found"},
		{"unknown QR code", api, "GET", "/api/v1/objects/nope", anna, "", 404, "not_found"},
		{"report to unknown house", api, "POST", "/api/v1/issues", anna, `{"house_id":"nope","category":"lift"}`, 404, "not_found"},
		{"set unknown house", api, "POST", "/api/v1/me/house", anna, `{"house_id":"nope"}`, 404, "not_found"},
		{"join closed issue", api, "POST", "/api/v1/issues/" + doneID + "/join", anna, "", 409, "issue_closed"},
		{"sent straight to done", api, "POST", "/api/v1/issues/" + sentID + "/status", oper, `{"status":"done"}`, 409, "invalid_transition"},
		{"reject without reason", api, "POST", "/api/v1/issues/" + sentID + "/status", oper, `{"status":"rejected","comment":"  "}`, 422, "reason_required"},
		{"broken JSON", api, "POST", "/api/v1/issues", anna, `{"house_id":`, 422, "invalid_input"},
		{"unknown category", api, "POST", "/api/v1/issues", anna, `{"house_id":"h-17k2","category":"teleport"}`, 422, "invalid_input"},
		{"object of another house", api, "POST", "/api/v1/issues", anna, `{"house_id":"h-17k2","object_id":"h-15-e1-lift"}`, 422, "invalid_input"},
		{"similar without category", api, "GET", "/api/v1/issues/similar?house_id=h-17k2", anna, "", 422, "invalid_input"},
		{"search query in cp1251", api, "GET", "/api/v1/houses?query=17%EA2", anna, "", 422, "invalid_input"},
		{"search query too short", api, "GET", "/api/v1/houses?query=1", anna, "", 422, "invalid_input"},
		{"coordinates out of range", api, "GET", "/api/v1/houses/nearest?lat=200&lon=37", anna, "", 422, "invalid_input"},
		{"coordinates not numbers", api, "GET", "/api/v1/houses/nearest?lat=abc&lon=37", anna, "", 422, "invalid_input"},
		{"empty consent version", api, "POST", "/api/v1/me/consent", anna, `{"version":""}`, 422, "invalid_input"},
		// Id в адресе не uuid: такой записи нет, это 404, а не сбой сервера.
		{"issue id not a uuid", api, "GET", "/api/v1/issues/abc", anna, "", 404, "not_found"},
		{"photos of issue id not a uuid", api, "GET", "/api/v1/issues/abc/photos", anna, "", 404, "not_found"},
		{"appeal for issue id not a uuid", api, "POST", "/api/v1/issues/abc/appeal", anna, "", 404, "not_found"},
		{"photo id not a uuid", api, "GET", "/api/v1/photos/abc", anna, "", 404, "not_found"},
		{"remove photo id not a uuid", api, "DELETE", "/api/v1/photos/abc", anna, "", 404, "not_found"},
		{"remove unknown photo", api, "DELETE", "/api/v1/photos/" + unknown, anna, "", 404, "not_found"},
		{"remove photo without token", api, "DELETE", "/api/v1/photos/" + unknown, "", "", 401, "unauthorized"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var body []byte
			if tc.body != "" {
				body = []byte(tc.body)
			}
			r := callRaw(t, tc.app, tc.method, tc.path, tc.token, body)
			if r.status != tc.status {
				t.Fatalf("status = %d, want %d, body %v", r.status, tc.status, r.body)
			}
			if !strings.HasPrefix(r.ctype, "application/json") {
				t.Fatalf("content type = %q", r.ctype)
			}
			e, _ := r.body["error"].(map[string]any)
			if e["code"] != tc.code || e["message"] == "" {
				t.Fatalf("error = %v, want code %q with message", r.body, tc.code)
			}
		})
	}
}

func newWebhookRequest(body []byte, secret string) *http.Request {
	req := httptest.NewRequest("POST", "/webhook/max", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Max-Bot-Api-Secret", secret)
	return req
}

func TestWebhookIgnoresRedelivery(t *testing.T) {
	count := func() int {
		webhooks.mu.Lock()
		defer webhooks.mu.Unlock()
		return len(webhooks.got)
	}
	before := count()
	payload := []byte(`{"update_type":"bot_started","timestamp":777001,"chat_id":7,"user":{"user_id":42}}`)
	for range 2 {
		req := newWebhookRequest(payload, "hook-secret")
		res, err := api.Test(req)
		if err != nil || res.StatusCode != 200 {
			t.Fatalf("webhook: %v, %v", res, err)
		}
	}
	bad := newWebhookRequest([]byte(`{`), "hook-secret")
	if res, _ := api.Test(bad); res.StatusCode != 400 {
		t.Fatalf("broken body status = %d, want 400", res.StatusCode)
	}
	deadline := time.Now().Add(2 * time.Second)
	for count() == before && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	if got := count() - before; got != 1 {
		t.Fatalf("handled %d times, want exactly once", got)
	}
}
