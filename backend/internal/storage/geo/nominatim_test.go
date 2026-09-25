package geo_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"dommax/internal/storage/geo"
)

// fakeNominatim отвечает как Nominatim и считает запросы; handler задаёт ответ.
func fakeNominatim(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skip("tcp4 listen disabled in sandbox")
	}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("User-Agent") != "dom-test/1.0" {
			t.Errorf("User-Agent = %q", r.Header.Get("User-Agent"))
		}
		if q := r.URL.Query(); q.Get("format") != "jsonv2" || q.Get("accept-language") != "ru" {
			t.Errorf("query = %v", q)
		}
		handler(w, r)
	}))
	srv.Listener = l
	srv.Start()
	t.Cleanup(srv.Close)
	return srv, &calls
}

func newClient(t *testing.T, url string) *geo.Nominatim {
	t.Helper()
	n, err := geo.NewNominatim(url, "dom-test/1.0")
	if err != nil {
		t.Fatal(err)
	}
	n.MinGap = 0 // в тестах без паузы, кроме теста самой паузы
	return n
}

func TestGeocodeHouseLevelOnly(t *testing.T) {
	srv, calls := fakeNominatim(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Errorf("path = %s", r.URL.Path)
		}
		switch r.URL.Query().Get("q") {
		case "Москва, Ореховый бульвар, 15":
			w.Write([]byte(`[{"lat":"55.6128089","lon":"37.7199256","place_rank":30,"address":{"road":"Ореховый бульвар","house_number":"15"}}]`))
		case "Москва, Ореховый бульвар, 17к2":
			// Нашлась только улица: координаты улицы вместо дома не годятся.
			w.Write([]byte(`[{"lat":"55.6088","lon":"37.7044","place_rank":26,"address":{"road":"Ореховый бульвар"}}]`))
		default:
			w.Write([]byte(`[]`))
		}
	})
	n := newClient(t, srv.URL)

	lat, lon, ok, err := n.Geocode(t.Context(), "Ореховый бульвар, 15")
	if err != nil || !ok || lat != 55.6128089 || lon != 37.7199256 {
		t.Fatalf("Geocode = %v %v %v %v", lat, lon, ok, err)
	}
	for _, addr := range []string{"Ореховый бульвар, 17к2", "Нет такой улицы, 1"} {
		if _, _, ok, err := n.Geocode(t.Context(), addr); ok || err != nil {
			t.Errorf("Geocode(%q) ok = %v, err = %v; want not found", addr, ok, err)
		}
	}
	// Повторный запрос берётся из кэша.
	before := calls.Load()
	if _, _, ok, _ := n.Geocode(t.Context(), "Ореховый бульвар, 15"); !ok || calls.Load() != before {
		t.Fatalf("cached Geocode: ok = %v, calls %d → %d", ok, before, calls.Load())
	}
}

func TestReverseShortAddress(t *testing.T) {
	srv, calls := fakeNominatim(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/reverse" {
			t.Errorf("path = %s", r.URL.Path)
		}
		switch r.URL.Query().Get("lat") {
		case "55.6128":
			w.Write([]byte(`{"address":{"road":"Ореховый бульвар","house_number":"15","city":"Москва"}}`))
		case "55.6125":
			w.Write([]byte(`{"address":{"road":"Ясеневая улица"}}`))
		default:
			w.Write([]byte(`{"error":"Unable to geocode"}`))
		}
	})
	n := newClient(t, srv.URL)

	got, ok, err := n.Reverse(t.Context(), 55.61281, 37.71993)
	if err != nil || !ok || got != "Ореховый бульвар, 15" {
		t.Fatalf("Reverse = %q %v %v", got, ok, err)
	}
	if got, ok, _ := n.Reverse(t.Context(), 55.6125, 37.746); !ok || got != "Ясеневая улица" {
		t.Fatalf("Reverse without house = %q %v", got, ok)
	}
	if _, ok, err := n.Reverse(t.Context(), 0.5, 0.5); ok || err != nil {
		t.Fatalf("Reverse nowhere ok = %v, err = %v", ok, err)
	}
	// Точки в пределах 10 м попадают в один ключ кэша.
	before := calls.Load()
	if got, _, _ := n.Reverse(t.Context(), 55.61279, 37.71991); got != "Ореховый бульвар, 15" || calls.Load() != before {
		t.Fatalf("cached Reverse = %q, calls %d → %d", got, before, calls.Load())
	}
}

func TestServerErrorIsError(t *testing.T) {
	srv, _ := fakeNominatim(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "busy", http.StatusServiceUnavailable)
	})
	n := newClient(t, srv.URL)
	if _, _, _, err := n.Geocode(t.Context(), "Ореховый бульвар, 15"); err == nil {
		t.Fatal("Geocode on 503 returned no error")
	}
	if _, _, err := n.Reverse(t.Context(), 55.6, 37.7); err == nil {
		t.Fatal("Reverse on 503 returned no error")
	}
}

// Публичный Nominatim разрешает не больше 1 запроса в секунду: второй запрос ждёт паузу,
// а отменённый контекст прерывает ожидание.
func TestMinGapBetweenRequests(t *testing.T) {
	srv, calls := fakeNominatim(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`[]`)) })
	n := newClient(t, srv.URL)
	n.MinGap = 150 * time.Millisecond

	start := time.Now()
	n.Geocode(t.Context(), "Улица, 1")
	n.Geocode(t.Context(), "Улица, 2")
	if d := time.Since(start); d < n.MinGap {
		t.Fatalf("two requests took %v, want at least %v", d, n.MinGap)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	n.MinGap = time.Hour
	before := calls.Load()
	if _, _, _, err := n.Geocode(ctx, "Улица, 3"); err == nil {
		t.Fatal("Geocode waited past context deadline")
	}
	if calls.Load() != before {
		t.Fatal("request sent after context deadline")
	}
}

func TestNewNominatimValidatesURL(t *testing.T) {
	if _, err := geo.NewNominatim("not a url", "ua"); err == nil {
		t.Fatal("NewNominatim accepted bad URL")
	}
	if _, err := geo.NewNominatim("https://nominatim.example", ""); err == nil {
		t.Fatal("NewNominatim accepted empty User-Agent")
	}
}
