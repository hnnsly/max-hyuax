package houses_test

import (
	"errors"
	"testing"

	"dommax/internal/app"
	"dommax/internal/app/apptest"
	"dommax/internal/app/houses"
	"dommax/internal/domain/house"
)

func setup() (*apptest.MemStore, *houses.Service) {
	s := apptest.New()
	s.Orgs["org-1"] = house.Organization{ID: "org-1", Name: "УК"}
	s.HouseMap["h-1"] = house.House{ID: "h-1", Address: "Ореховый бульвар, 17к2", OrganizationID: "org-1"}
	s.HouseMap["h-orphan"] = house.House{ID: "h-orphan", Address: "Без УК", OrganizationID: "org-missing"}
	s.Objects = []house.AssetObject{{ID: "o-1", HouseID: "h-1", Category: "lift", QRCode: "h-1-lift"}}
	return s, houses.NewService(s)
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
	svc := houses.NewService(s)

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
