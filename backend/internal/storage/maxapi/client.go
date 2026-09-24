// Пакет maxapi — тонкий клиент MAX Bot API (platform-api2). Покрывает только методы,
// которые нужны продукту; лимиты частоты соблюдает вызывающий код (outbox).
package maxapi

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

const DefaultBaseURL = "https://platform-api2.max.ru"

type Client struct {
	base  string
	token string
	http  *http.Client
}

func New(baseURL, token string, hc *http.Client) *Client {
	return &Client{base: baseURL, token: token, http: hc}
}

// Error — ответ Bot API с кодом не из 2xx.
type Error struct {
	Status  int
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("maxapi: %d %s: %s", e.Status, e.Code, e.Message)
}

// RateLimited — MAX ответил «слишком много запросов», отправку нужно притормозить.
func (e *Error) RateLimited() bool { return e.Status == http.StatusTooManyRequests }

// HTTPClient доверяет корневому сертификату Минцифры, которым подписан platform-api2.
// caFile добавляет PEM к системному пулу; insecure отключает проверку и нужен только
// на машине разработчика без сертификата.
func HTTPClient(caFile string, insecure bool) (*http.Client, error) {
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	switch {
	case insecure:
		tlsCfg.InsecureSkipVerify = true
	case caFile != "":
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("maxapi: read CA file: %w", err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("maxapi: no PEM certificates in %s", caFile)
		}
		tlsCfg.RootCAs = pool
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = tlsCfg
	// Long polling держит запрос до 90 с, поэтому общий таймаут клиента не задаём.
	return &http.Client{Transport: tr}, nil
}

func (c *Client) Me(ctx context.Context) (User, error) {
	var me User
	err := c.do(ctx, http.MethodGet, "/me", nil, nil, &me)
	return me, err
}

// Updates получает события через long polling. marker 0 означает «без маркера»: вернётся только последнее обновление.
func (c *Client) Updates(ctx context.Context, marker int64, timeout time.Duration) (UpdatesPage, error) {
	q := url.Values{}
	if marker != 0 {
		q.Set("marker", strconv.FormatInt(marker, 10))
	}
	q.Set("timeout", strconv.Itoa(int(timeout/time.Second)))
	var page UpdatesPage
	err := c.do(ctx, http.MethodGet, "/updates", q, nil, &page)
	return page, err
}

func (c *Client) Send(ctx context.Context, to Target, m NewMessage) (Message, error) {
	var out struct {
		Message Message `json:"message"`
	}
	err := c.do(ctx, http.MethodPost, "/messages", to.query(), m, &out)
	return out.Message, err
}

// Edit заменяет текст и клавиатуру отправленного сообщения; так обновляется живая карточка.
func (c *Client) Edit(ctx context.Context, mid string, m NewMessage) error {
	return c.doResult(ctx, http.MethodPut, "/messages", url.Values{"message_id": {mid}}, m)
}

// Subscribe включает доставку событий на webhook. Пока подписка активна, long polling не работает.
func (c *Client) Subscribe(ctx context.Context, url, secret string, types []UpdateType) error {
	body := struct {
		URL         string       `json:"url"`
		UpdateTypes []UpdateType `json:"update_types,omitempty"`
		Secret      string       `json:"secret,omitempty"`
	}{URL: url, UpdateTypes: types, Secret: secret}
	return c.doResult(ctx, http.MethodPost, "/subscriptions", nil, body)
}

// Command — команда в меню бота (PATCH /me/commands): name 1-64 символа, description 1-128.
type Command struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// SetCommands заменяет список команд бота.
func (c *Client) SetCommands(ctx context.Context, cmds []Command) error {
	body := struct {
		Commands []Command `json:"commands"`
	}{cmds}
	var out struct {
		Commands []Command `json:"commands"`
	}
	return c.do(ctx, http.MethodPatch, "/me/commands", nil, body, &out)
}

func (c *Client) Answer(ctx context.Context, callbackID string, a CallbackAnswer) error {
	return c.doResult(ctx, http.MethodPost, "/answers", url.Values{"callback_id": {callbackID}}, a)
}

func (t Target) query() url.Values {
	if t.chatID != 0 {
		return url.Values{"chat_id": {strconv.FormatInt(t.chatID, 10)}}
	}
	return url.Values{"user_id": {strconv.FormatInt(t.userID, 10)}}
}

// doResult вызывает метод, который отвечает {"success": bool, "message": string}.
func (c *Client) doResult(ctx context.Context, method, path string, q url.Values, body any) error {
	var res struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := c.do(ctx, method, path, q, body, &res); err != nil {
		return err
	}
	if !res.Success {
		return &Error{Status: http.StatusOK, Code: "unsuccessful", Message: res.Message}
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, path string, q url.Values, body, out any) error {
	u := c.base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var rd io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("maxapi: encode %s: %w", path, err)
		}
		rd = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("maxapi: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		apiErr := &Error{Status: resp.StatusCode}
		// Тело ответа только поясняет ошибку; даже если оно битое, статус всё равно вернётся.
		_ = json.UnmarshalRead(io.LimitReader(resp.Body, 64<<10), apiErr)
		return apiErr
	}
	if err := json.UnmarshalRead(resp.Body, out); err != nil {
		return fmt.Errorf("maxapi: decode %s: %w", path, err)
	}
	return nil
}

// downloadTimeout — у клиента общего таймаута нет (long polling), поэтому скачивание ограничено отдельно.
const downloadTimeout = 30 * time.Second

// Download скачивает файл по ссылке из вложения (CDN MAX) не больше limit байт.
// Токен бота не передаётся: он нужен только самому Bot API. Ссылка подписана и ведёт
// к фото жителя, поэтому в текст ошибки (и в лог) она не попадает.
func (c *Client) Download(ctx context.Context, fileURL string, limit int64) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("maxapi: download: %w", withoutURL(err))
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("maxapi: download: %w", withoutURL(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &Error{Status: resp.StatusCode, Code: "download_failed"}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("maxapi: download: %w", withoutURL(err))
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("maxapi: download: file is larger than %d bytes", limit)
	}
	return data, nil
}

// withoutURL снимает с ошибки net/http обёртку *url.Error, в тексте которой есть адрес запроса.
func withoutURL(err error) error {
	if ue, ok := errors.AsType[*url.Error](err); ok {
		return ue.Err
	}
	return err
}
