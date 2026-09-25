// Пакет geo — геокодер на открытых данных OpenStreetMap через API, совместимое с Nominatim (ADR-016).
package geo

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// maxCache — сколько ответов держать в памяти; при переполнении кэш очищается целиком.
	maxCache = 2000
	// city — пилот в Москве (ADR-002): адрес реестра дополняется городом.
	city = "Москва"
)

// Nominatim — клиент API Nominatim. Безопасен для одновременного использования.
type Nominatim struct {
	// MinGap — пауза между запросами: правила публичного сервера разрешают 1 запрос в секунду.
	MinGap time.Duration

	base      *url.URL
	userAgent string
	client    *http.Client

	mu    sync.Mutex
	next  time.Time // когда можно отправить следующий запрос
	cache map[string]result
}

type result struct {
	lat, lon float64
	address  string
	ok       bool
}

func NewNominatim(baseURL, userAgent string) (*Nominatim, error) {
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("geocoder url %q: want http(s)://host", baseURL)
	}
	if userAgent == "" {
		return nil, errors.New("geocoder User-Agent is required by the Nominatim usage policy")
	}
	return &Nominatim{
		MinGap:    time.Second,
		base:      u,
		userAgent: userAgent,
		client:    &http.Client{Timeout: 10 * time.Second},
		cache:     map[string]result{},
	}, nil
}

type place struct {
	Lat       string  `json:"lat"`
	Lon       string  `json:"lon"`
	PlaceRank int     `json:"place_rank"`
	Address   address `json:"address"`
}

type address struct {
	Road        string `json:"road"`
	Pedestrian  string `json:"pedestrian"`
	HouseNumber string `json:"house_number"`
}

// Geocode ищет координаты дома по адресу реестра («Ореховый бульвар, 15»).
// Подходит только результат уровня дома: координаты улицы вместо дома хуже, чем никаких.
func (n *Nominatim) Geocode(ctx context.Context, addr string) (lat, lon float64, ok bool, err error) {
	q := city + ", " + strings.TrimSpace(addr)
	r, err := n.cached(ctx, "g:"+q, "/search", url.Values{"q": {q}, "limit": {"1"}, "countrycodes": {"ru"}},
		func(body []byte) (result, error) {
			var places []place
			if err := json.Unmarshal(body, &places); err != nil {
				return result{}, err
			}
			if len(places) == 0 || places[0].PlaceRank < 30 || places[0].Address.HouseNumber == "" {
				return result{}, nil
			}
			lat, err1 := strconv.ParseFloat(places[0].Lat, 64)
			lon, err2 := strconv.ParseFloat(places[0].Lon, 64)
			if err := errors.Join(err1, err2); err != nil {
				return result{}, err
			}
			return result{lat: lat, lon: lon, ok: true}, nil
		})
	return r.lat, r.lon, r.ok, err
}

// Reverse возвращает короткий адрес точки: «улица, дом» или только улицу.
func (n *Nominatim) Reverse(ctx context.Context, lat, lon float64) (string, bool, error) {
	// Округление до 4 знаков — около 10 м: соседние точки берутся из кэша.
	la, lo := strconv.FormatFloat(lat, 'f', 4, 64), strconv.FormatFloat(lon, 'f', 4, 64)
	r, err := n.cached(ctx, "r:"+la+","+lo, "/reverse", url.Values{"lat": {la}, "lon": {lo}, "zoom": {"18"}},
		func(body []byte) (result, error) {
			var p place
			if err := json.Unmarshal(body, &p); err != nil {
				return result{}, err
			}
			road := p.Address.Road
			if road == "" {
				road = p.Address.Pedestrian
			}
			if road == "" {
				return result{}, nil // «Unable to geocode» или точка вне улиц
			}
			if p.Address.HouseNumber != "" {
				road += ", " + p.Address.HouseNumber
			}
			return result{address: road, ok: true}, nil
		})
	return r.address, r.ok, err
}

// cached отдаёт ответ из кэша или запрашивает его, соблюдая паузу между запросами.
// Ошибки не кэшируются: следующий вызов попробует снова.
func (n *Nominatim) cached(ctx context.Context, key, path string, params url.Values, parse func([]byte) (result, error)) (result, error) {
	n.mu.Lock()
	if r, hit := n.cache[key]; hit {
		n.mu.Unlock()
		return r, nil
	}
	// Очередь по времени: каждый запрос бронирует свой слот, ждёт его без блокировки остальных.
	slot := time.Now()
	if n.next.After(slot) {
		slot = n.next
	}
	n.next = slot.Add(n.MinGap)
	n.mu.Unlock()

	if wait := time.Until(slot); wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return result{}, ctx.Err()
		case <-timer.C:
		}
	}
	r, err := n.fetch(ctx, path, params, parse)
	if err != nil {
		return result{}, err
	}
	n.mu.Lock()
	if len(n.cache) >= maxCache {
		clear(n.cache)
	}
	n.cache[key] = r
	n.mu.Unlock()
	return r, nil
}

func (n *Nominatim) fetch(ctx context.Context, path string, params url.Values, parse func([]byte) (result, error)) (result, error) {
	u := n.base.JoinPath(path)
	params.Set("format", "jsonv2")
	params.Set("addressdetails", "1")
	params.Set("accept-language", "ru")
	u.RawQuery = params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return result{}, err
	}
	req.Header.Set("User-Agent", n.userAgent)
	resp, err := n.client.Do(req)
	if err != nil {
		return result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return result{}, fmt.Errorf("geocoder %s: status %d", path, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return result{}, err
	}
	r, err := parse(body)
	if err != nil {
		return result{}, fmt.Errorf("geocoder %s: %w", path, err)
	}
	return r, nil
}
