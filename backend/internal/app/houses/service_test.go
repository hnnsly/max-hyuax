package houses_test

import (
	"errors"
	"testing"

	"dommax/internal/app"
	"dommax/internal/app/apptest"
	"dommax/internal/app/houses"
	"dommax/internal/domain/house"
)

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
