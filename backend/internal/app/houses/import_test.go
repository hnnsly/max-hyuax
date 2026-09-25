package houses_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"dommax/internal/app"
	"dommax/internal/app/houses"
	"dommax/internal/domain/house"
)

// fakeGeo находит только адреса из карты; err отдаётся на любой запрос.
type fakeGeo struct {
	known map[string][2]float64
	err   error
	calls int
}

func (g *fakeGeo) Geocode(_ context.Context, addr string) (float64, float64, bool, error) {
	g.calls++
	if g.err != nil {
		return 0, 0, false, g.err
	}
	c, ok := g.known[addr]
	return c[0], c[1], ok, nil
}

func (g *fakeGeo) Reverse(context.Context, float64, float64) (string, bool, error) {
	return "", false, g.err
}

func (g *fakeGeo) ReverseHouse(context.Context, float64, float64) (app.GeoHouse, bool, error) {
	return app.GeoHouse{}, false, g.err
}

func (g *fakeGeo) SearchHouses(context.Context, string) ([]app.GeoHouse, error) {
	return nil, g.err
}

func validRow(line int, addr string) houses.ImportRow {
	return houses.ImportRow{Line: line, House: house.House{
		Address: addr, District: "Зябликово", OrganizationID: "org-1", YearBuilt: 1984, Floors: 12, EntrancesCount: 3,
	}}
}

func TestImportCreatesUpdatesAndGeocodes(t *testing.T) {
	s, _ := setup()
	geo := &fakeGeo{known: map[string][2]float64{"Ореховый бульвар, 15": {55.61, 37.72}}}
	im := houses.NewImporter(s, geo)

	withCoords := validRow(3, "Ясеневая улица, 32к1")
	withCoords.House.Lat, withCoords.House.Lon = 55.62, 37.75
	rows := []houses.ImportRow{validRow(2, "Ореховый бульвар, 15"), withCoords, validRow(4, "Ореховый  бульвар, 17к3")}
	rep, err := im.Import(t.Context(), rows)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Created != 3 || rep.Updated != 0 || rep.Geocoded != 1 || rep.WithoutCoords != 1 || len(rep.Errors) != 0 {
		t.Fatalf("report = %+v", rep)
	}
	id := houses.ImportID("Ореховый бульвар, 15")
	h := s.HouseMap[id]
	if h.Lat != 55.61 || h.Source != "import" || !strings.HasPrefix(id, "h-") {
		t.Fatalf("geocoded house %s = %+v", id, h)
	}
	// Только адрес без координат уходит в геокодер: у строки 3 они уже есть.
	if geo.calls != 2 {
		t.Fatalf("geocoder calls = %d, want 2", geo.calls)
	}

	// Повторный импорт того же файла обновляет дома и не стирает найденные раньше координаты.
	geo.known = nil
	rep, err = im.Import(t.Context(), rows)
	if err != nil || rep.Created != 0 || rep.Updated != 3 {
		t.Fatalf("second import = %+v, err = %v", rep, err)
	}
	if s.HouseMap[id].Lat != 55.61 {
		t.Fatalf("coordinates lost on reimport: %+v", s.HouseMap[id])
	}
	// Адрес с лишними пробелами и в другом регистре даёт тот же id.
	if houses.ImportID("ореховый бульвар,   15") != id {
		t.Fatal("ImportID depends on spaces or case")
	}
}

func TestImportReportsBadRows(t *testing.T) {
	s, _ := setup()
	im := houses.NewImporter(s, nil) // без геокодера дома импортируются без координат

	bad := func(line int, mutate func(*house.House)) houses.ImportRow {
		r := validRow(line, "Улица, 1")
		mutate(&r.House)
		return r
	}
	rows := []houses.ImportRow{
		bad(2, func(h *house.House) { h.Address = " " }),
		bad(3, func(h *house.House) { h.District = "" }),
		bad(4, func(h *house.House) { h.OrganizationID = "org-missing" }),
		bad(5, func(h *house.House) { h.Floors = 0 }),
		bad(6, func(h *house.House) { h.EntrancesCount = 31 }),
		bad(7, func(h *house.House) { h.YearBuilt = 3000 }),
		bad(8, func(h *house.House) { h.Lat, h.Lon = 91, 37 }),
		bad(9, func(h *house.House) { h.ID = "H 1/../x" }),
		bad(10, func(h *house.House) { h.Source = "Open Data!" }),
		validRow(11, "Улица, 2"),
		validRow(12, "Улица, 2"), // повтор адреса в одном файле
	}
	rep, err := im.Import(t.Context(), rows)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Created != 1 || rep.WithoutCoords != 1 || len(rep.Errors) != 10 {
		t.Fatalf("report = %+v", rep)
	}
	wantLines := []int{2, 3, 4, 5, 6, 7, 8, 9, 10, 12}
	for i, e := range rep.Errors {
		if e.Line != wantLines[i] || e.Reason == "" {
			t.Errorf("error %d = %+v", i, e)
		}
	}
	if !strings.Contains(rep.Errors[9].Reason, "11") {
		t.Errorf("duplicate reason = %q, want mention of line 11", rep.Errors[9].Reason)
	}
}

// Сбой геокодера не останавливает импорт, а отмена контекста останавливает.
func TestImportGeocoderFailures(t *testing.T) {
	s, _ := setup()
	geo := &fakeGeo{err: errors.New("timeout")}
	rep, err := houses.NewImporter(s, geo).Import(t.Context(), []houses.ImportRow{validRow(2, "Улица, 5")})
	if err != nil || rep.Created != 1 || rep.WithoutCoords != 1 {
		t.Fatalf("report = %+v, err = %v", rep, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	geo.err = context.Canceled
	if _, err := houses.NewImporter(s, geo).Import(ctx, []houses.ImportRow{validRow(2, "Улица, 6")}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled import err = %v", err)
	}
}
