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

	"dommax/internal/app"
)

const (
	// maxCache — сколько ответов держать в памяти; при переполнении кэш очищается целиком.
	maxCache = 2000
	// city — Москва: адрес реестра дополняется городом.
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
	district string
	inMoscow bool
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
	Road         string `json:"road"`
	Pedestrian   string `json:"pedestrian"`
	HouseNumber  string `json:"house_number"`
	Suburb       string `json:"suburb"`
	CityDistrict string `json:"city_district"`
	Municipality string `json:"municipality"`
	County       string `json:"county"`
	City         string `json:"city"`
	Town         string `json:"town"`
	Village      string `json:"village"`
	State        string `json:"state"`
	Country      string `json:"country"`
	ISO          string `json:"ISO3166-2-lvl4"`
}

func isMoscow(p place, lat, lon float64) bool {
	inBBox := lat >= 55.10 && lat <= 56.10 && lon >= 36.80 && lon <= 38.00
	a := p.Address
	if strings.Contains(a.State, "Московская область") || a.ISO == "RU-MOS" {
		return false
	}
	nameMatch := a.City == "Москва" || a.State == "Москва" ||
		strings.Contains(a.State, "Москва") || a.ISO == "RU-MOW"
	return inBBox && (nameMatch || (a.State == "" && a.City == ""))
}

// CleanDistrictName очищает название района Москвы от служебных слов.
func CleanDistrictName(raw string) string {
	d := strings.TrimSpace(raw)
	for _, prefix := range []string{
		"муниципальный округ ", "Муниципальный округ ",
		"район ", "Район ",
		"поселение ", "Поселение ",
		"городское поселение ",
	} {
		d = strings.TrimPrefix(d, prefix)
	}
	for _, suffix := range []string{
		" район", " Район",
		" муниципальный округ",
	} {
		d = strings.TrimSuffix(d, suffix)
	}
	return strings.TrimSpace(d)
}

func extractDistrict(a address) string {
	for _, candidate := range []string{a.Suburb, a.Municipality, a.CityDistrict, a.County} {
		d := CleanDistrictName(candidate)
		if d != "" && !strings.Contains(strings.ToLower(d), "административный округ") {
			return d
		}
	}
	for _, candidate := range []string{a.CityDistrict, a.Suburb} {
		if d := CleanDistrictName(candidate); d != "" {
			return d
		}
	}
	return "Центральный"
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
	gh, ok, err := n.ReverseHouse(ctx, lat, lon)
	return gh.Address, ok, err
}

// ReverseHouse определяет адрес дома, район и принадлежность к Москве по координатам.
func (n *Nominatim) ReverseHouse(ctx context.Context, lat, lon float64) (app.GeoHouse, bool, error) {
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
			latF, _ := strconv.ParseFloat(p.Lat, 64)
			lonF, _ := strconv.ParseFloat(p.Lon, 64)
			if latF == 0 {
				latF = lat
			}
			if lonF == 0 {
				lonF = lon
			}
			return result{
				lat:      latF,
				lon:      lonF,
				address:  road,
				district: extractDistrict(p.Address),
				inMoscow: isMoscow(p, latF, lonF),
				ok:       true,
			}, nil
		})
	if err != nil || !r.ok {
		return app.GeoHouse{}, false, err
	}
	return app.GeoHouse{
		Address:  r.address,
		District: r.district,
		Lat:      r.lat,
		Lon:      r.lon,
		InMoscow: r.inMoscow,
	}, true, nil
}

// SearchHouses ищет дома по текстовому запросу в Москве.
func (n *Nominatim) SearchHouses(ctx context.Context, query string) ([]app.GeoHouse, error) {
	q := strings.TrimSpace(query)
	if !strings.HasPrefix(strings.ToLower(q), "москва") {
		q = city + ", " + q
	}
	body, err := n.fetchRaw(ctx, "/search", url.Values{"q": {q}, "limit": {"5"}, "countrycodes": {"ru"}})
	if err != nil {
		return nil, err
	}
	var places []place
	if err := json.Unmarshal(body, &places); err != nil {
		return nil, err
	}
	var out []app.GeoHouse
	for _, p := range places {
		road := p.Address.Road
		if road == "" {
			road = p.Address.Pedestrian
		}
		if road == "" || p.Address.HouseNumber == "" {
			continue
		}
		road += ", " + p.Address.HouseNumber
		latF, err1 := strconv.ParseFloat(p.Lat, 64)
		lonF, err2 := strconv.ParseFloat(p.Lon, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		if !isMoscow(p, latF, lonF) {
			continue
		}
		out = append(out, app.GeoHouse{
			Address:  road,
			District: extractDistrict(p.Address),
			Lat:      latF,
			Lon:      lonF,
			InMoscow: true,
		})
	}
	return out, nil
}

// cached отдаёт ответ из кэша или запрашивает его, соблюдая паузу между запросами.
// Ошибки не кэшируются: следующий вызов попробует снова.
func (n *Nominatim) cached(ctx context.Context, key, path string, params url.Values, parse func([]byte) (result, error)) (result, error) {
	n.mu.Lock()
	if r, hit := n.cache[key]; hit {
		n.mu.Unlock()
		return r, nil
	}
	n.mu.Unlock()

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

func (n *Nominatim) fetchRaw(ctx context.Context, path string, params url.Values) ([]byte, error) {
	n.mu.Lock()
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
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	u := n.base.JoinPath(path)
	params.Set("format", "jsonv2")
	params.Set("addressdetails", "1")
	params.Set("accept-language", "ru")
	u.RawQuery = params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", n.userAgent)
	resp, err := n.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geocoder %s: status %d", path, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

func (n *Nominatim) fetch(ctx context.Context, path string, params url.Values, parse func([]byte) (result, error)) (result, error) {
	body, err := n.fetchRaw(ctx, path, params)
	if err != nil {
		return result{}, err
	}
	r, err := parse(body)
	if err != nil {
		return result{}, fmt.Errorf("geocoder %s: %w", path, err)
	}
	return r, nil
}
