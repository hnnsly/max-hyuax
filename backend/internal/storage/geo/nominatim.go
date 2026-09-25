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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"dommax/internal/app"
)

const (
	maxCache = 2000
	city     = "Москва"
)

// Nominatim — клиент API Nominatim. Безопасен для одновременного использования.
type Nominatim struct {
	MinGap time.Duration

	base      *url.URL
	userAgent string
	client    *http.Client

	mu    sync.Mutex
	next  time.Time
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
	Footway      string `json:"footway"`
	Square       string `json:"square"`
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
	if !inBBox {
		return false
	}
	a := p.Address
	if strings.Contains(a.State, "Московская область") || strings.Contains(a.County, "Московская область") || a.ISO == "RU-MOS" {
		return false
	}
	return true
}

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

func extractRoad(a address) string {
	for _, r := range []string{a.Road, a.Pedestrian, a.Footway, a.Square} {
		if strings.TrimSpace(r) != "" {
			return strings.TrimSpace(r)
		}
	}
	return ""
}

var (
	reCorpNorm = regexp.MustCompile(`(?i)(\d+)\s*([кксс])\s*(\d+)`)
	reEndHouse = regexp.MustCompile(`(?i)(?:^|[\s,])(?:д\.?|дом)?\s*(\d+[а-яa-z0-9/\-\s]*)$`)
)

func normalizeForOSM(q string) string {
	q = strings.TrimSpace(q)
	q = reCorpNorm.ReplaceAllString(q, "$1 $2$3")
	return q
}

func cleanHouseNumber(hn string) string {
	hn = strings.TrimSpace(hn)
	hn = strings.ReplaceAll(hn, " ", "")
	return hn
}

func splitStreetAndHouse(q string) (street, houseNum string) {
	q = strings.TrimSpace(q)
	loc := reEndHouse.FindStringSubmatchIndex(q)
	if loc == nil {
		return q, ""
	}
	h := strings.TrimSpace(q[loc[2]:loc[3]])
	s := strings.TrimSpace(strings.TrimSuffix(q[:loc[0]], ","))
	if s == "" {
		return q, ""
	}
	return s, cleanHouseNumber(h)
}

// Geocode ищет координаты дома по адресу реестра («Ореховый бульвар, 15»).
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
			road := extractRoad(p.Address)
			if road == "" {
				return result{}, nil
			}
			if p.Address.HouseNumber != "" {
				road += ", " + cleanHouseNumber(p.Address.HouseNumber)
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

// SearchHouses ищет дома по текстовому запросу в Москве с поддержкой корпусов и fallback на улицу.
func (n *Nominatim) SearchHouses(ctx context.Context, query string) ([]app.GeoHouse, error) {
	q := strings.TrimSpace(query)
	streetPart, userHouseNum := splitStreetAndHouse(q)

	normQ := normalizeForOSM(q)
	if !strings.HasPrefix(strings.ToLower(normQ), "москва") {
		normQ = city + ", " + normQ
	}

	places, err := n.queryPlaces(ctx, normQ)
	if err != nil {
		return nil, err
	}

	out := n.extractGeoHouses(places, userHouseNum)

	// Fallback: если конкретный номер дома/корпус в OSM не нашёлся, но пользователь указал дом,
	// ищем улицу в Москве и формируем дом в её районе.
	if len(out) == 0 && userHouseNum != "" && streetPart != "" {
		streetQ := city + ", " + streetPart
		streetPlaces, err := n.queryPlaces(ctx, streetQ)
		if err == nil && len(streetPlaces) > 0 {
			out = n.extractGeoHouses(streetPlaces, userHouseNum)
		}
	}

	return out, nil
}

func (n *Nominatim) queryPlaces(ctx context.Context, q string) ([]place, error) {
	body, err := n.fetchRaw(ctx, "/search", url.Values{
		"q":            {q},
		"limit":        {"8"},
		"countrycodes": {"ru"},
	})
	if err != nil {
		return nil, err
	}
	var places []place
	if err := json.Unmarshal(body, &places); err != nil {
		return nil, err
	}
	return places, nil
}

func (n *Nominatim) extractGeoHouses(places []place, userHouseNum string) []app.GeoHouse {
	var out []app.GeoHouse
	seen := map[string]bool{}

	for _, p := range places {
		road := extractRoad(p.Address)
		if road == "" {
			continue
		}
		latF, err1 := strconv.ParseFloat(p.Lat, 64)
		lonF, err2 := strconv.ParseFloat(p.Lon, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		if !isMoscow(p, latF, lonF) {
			continue
		}
		district := extractDistrict(p.Address)

		switch {
		case p.Address.HouseNumber != "":
			addr := road + ", " + cleanHouseNumber(p.Address.HouseNumber)
			if !seen[addr] {
				out = append(out, app.GeoHouse{Address: addr, District: district, Lat: latF, Lon: lonF, InMoscow: true})
				seen[addr] = true
			}
		case userHouseNum != "":
			addr := road + ", " + userHouseNum
			if !seen[addr] {
				out = append(out, app.GeoHouse{Address: addr, District: district, Lat: latF, Lon: lonF, InMoscow: true})
				seen[addr] = true
			}
		default:
			// Пользователь ввёл только название улицы (без номера): предлагаем дома 1, 2, 3 на этой улице
			for _, num := range []string{"1", "2", "3"} {
				addr := road + ", " + num
				if !seen[addr] {
					out = append(out, app.GeoHouse{Address: addr, District: district, Lat: latF, Lon: lonF, InMoscow: true})
					seen[addr] = true
				}
			}
		}
	}
	return out
}

func (n *Nominatim) waitTurn(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	for {
		now := time.Now()
		if !now.Before(n.next) {
			n.next = now.Add(n.MinGap)
			return nil
		}
		wait := n.next.Sub(now)
		n.mu.Unlock()
		select {
		case <-ctx.Done():
			n.mu.Lock()
			return ctx.Err()
		case <-time.After(wait):
		}
		n.mu.Lock()
	}
}

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
	if err := n.waitTurn(ctx); err != nil {
		return nil, err
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
