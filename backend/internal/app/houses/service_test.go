package houses_test

import (
	"context"
	"errors"
	"testing"

	"dommax/internal/app"
	"dommax/internal/app/apptest"
	"dommax/internal/app/houses"
	"dommax/internal/domain/house"
	"dommax/internal/domain/user"
)

func setup() (*apptest.MemStore, *houses.Service) {
	s := apptest.New()
	s.Orgs["org-1"] = house.Organization{ID: "org-1", Name: "УК"}
	s.HouseMap["h-1"] = house.House{ID: "h-1", Address: "Ореховый бульвар, 17к2", OrganizationID: "org-1"}
	s.HouseMap["h-orphan"] = house.House{ID: "h-orphan", Address: "Без УК", OrganizationID: "org-missing"}
	s.Objects = []house.AssetObject{{ID: "o-1", HouseID: "h-1", Category: "lift", QRCode: "h-1-lift"}}
	return s, houses.NewService(s, nil)
}

func TestNearestValidatesCoordinates(t *testing.T) {
	_, svc := setup()
	for _, c := range [][2]float64{{91, 0}, {-91, 0}, {0, 181}, {0, -181}} {
		if _, err := svc.Nearest(t.Context(), c[0], c[1]); !errors.Is(err, app.ErrInvalidInput) {
			t.Fatalf("Nearest(%v) err = %v", c, err)
		}
	}
	got, err := svc.Nearest(t.Context(), 55.6, 37.7)
	if err != nil || len(got) == 0 {
		t.Fatalf("Nearest = %v, %v", got, err)
	}
}

func TestGetCollectsHouseDetails(t *testing.T) {
	_, svc := setup()
	d, err := svc.Get(t.Context(), "h-1")
	if err != nil || d.Organization.ID != "org-1" || len(d.Objects) != 1 {
		t.Fatalf("details = %+v, err = %v", d, err)
	}
	if _, err := svc.Get(t.Context(), "nope"); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("unknown house err = %v", err)
	}
	if _, err := svc.Get(t.Context(), "h-orphan"); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("house without organization err = %v", err)
	}
}

func TestByQRCode(t *testing.T) {
	_, svc := setup()
	obj, h, err := svc.ByQRCode(t.Context(), "h-1-lift")
	if err != nil || obj.ID != "o-1" || h.ID != "h-1" {
		t.Fatalf("object = %+v, house = %+v, err = %v", obj, h, err)
	}
	if _, _, err := svc.ByQRCode(t.Context(), "nope"); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("unknown code err = %v", err)
	}
}

func TestSearchValidatesQuery(t *testing.T) {
	s := apptest.New()
	s.HouseMap["h-1"] = house.House{ID: "h-1", Address: "Ореховый бульвар, 17к2"}
	svc := houses.NewService(s, nil)

	// Строка в cp1251 вместо UTF-8: так отправляет запрос консоль Windows со старой кодовой страницей.
	for _, q := range []string{"", "1", "17\xea2"} {
		if _, err := svc.Search(t.Context(), q); !errors.Is(err, app.ErrInvalidInput) {
			t.Fatalf("query %q: err = %v, want ErrInvalidInput", q, err)
		}
	}
	got, err := svc.Search(t.Context(), "17к2")
	if err != nil || len(got) != 1 {
		t.Fatalf("got = %v, err = %v", got, err)
	}
}

func TestOperatorHousesAreOwnOnly(t *testing.T) {
	s, svc := setup()
	s.HouseMap["h-2"] = house.House{ID: "h-2", Address: "Ореховый бульвар, 15", OrganizationID: "org-1"}
	oper := user.User{ID: 3, Role: user.RoleOperator, OrganizationID: "org-1"}
	got, err := svc.ForOperator(t.Context(), oper)
	if err != nil || len(got) != 2 || got[0].ID != "h-2" {
		t.Fatalf("houses = %+v, err = %v (want both org-1 houses sorted by address)", got, err)
	}
	for _, u := range []user.User{{ID: 1, Role: user.RoleResident}, {ID: 4, Role: user.RoleOperator}} {
		if _, err := svc.ForOperator(t.Context(), u); !errors.Is(err, app.ErrForbidden) {
			t.Errorf("ForOperator(%+v) err = %v, want forbidden", u, err)
		}
	}
}

// Locate: адрес точки для «Найти дома рядом»; без геокодера и при «не нашёл» пустая строка.
func TestLocate(t *testing.T) {
	s, svc := setup()
	if addr, err := svc.Locate(t.Context(), 55.6, 37.7); addr != "" || err != nil {
		t.Fatalf("without geocoder = %q, %v", addr, err)
	}
	geo := &reverseGeo{addr: "Ореховый бульвар, 15"}
	svc = houses.NewService(s, geo)
	if addr, err := svc.Locate(t.Context(), 55.6, 37.7); addr != "Ореховый бульвар, 15" || err != nil {
		t.Fatalf("Locate = %q, %v", addr, err)
	}
	if _, err := svc.Locate(t.Context(), 91, 0); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("bad coords err = %v", err)
	}
	geo.addr = ""
	if addr, err := svc.Locate(t.Context(), 55.6, 37.7); addr != "" || err != nil {
		t.Fatalf("not found = %q, %v", addr, err)
	}
	geo.err = errors.New("geocoder down")
	if _, err := svc.Locate(t.Context(), 55.6, 37.7); err == nil {
		t.Fatal("geocoder error swallowed by service")
	}
}

type reverseGeo struct {
	fakeGeo
	addr string
	err  error
}

func (g *reverseGeo) Reverse(context.Context, float64, float64) (string, bool, error) {
	return g.addr, g.addr != "", g.err
}
