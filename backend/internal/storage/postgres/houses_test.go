//go:build integration

package postgres_test

import (
	"testing"

	"dommax/internal/app"
	"dommax/internal/domain/house"
)

// Импорт дома: подъезды и объекты с QR-кодами как у модельных домов, повторный импорт обновляет без дублей,
// дом без координат не попадает в «ближайшие».
func TestHouseUpsert(t *testing.T) {
	ctx := t.Context()
	h := house.House{
		ID: "h-imp-test", Address: "Тестовая улица, 1", District: "Зябликово", YearBuilt: 1985,
		Floors: 9, EntrancesCount: 2, OrganizationID: "org-orekh", Source: "import",
	}
	var created bool
	err := store.InTx(ctx, func(tx app.Store) (err error) {
		created, err = tx.Houses().Upsert(ctx, h)
		return err
	})
	if err != nil || !created {
		t.Fatalf("first Upsert created = %v, err = %v", created, err)
	}
	objs, err := store.Houses().Objects(ctx, h.ID)
	if err != nil || len(objs) != 6 {
		t.Fatalf("objects = %+v, err = %v", objs, err)
	}
	if obj, err := store.Houses().ObjectByCode(ctx, "h-imp-test-e2-lift"); err != nil || obj.EntranceID != "h-imp-test-e2" {
		t.Fatalf("lift of entrance 2 = %+v, err = %v", obj, err)
	}
	near, _ := store.Houses().Nearest(ctx, 0, 0, 50)
	for _, n := range near {
		if n.ID == h.ID {
			t.Fatal("house without coordinates is in Nearest")
		}
	}

	h.EntrancesCount, h.Lat, h.Lon, h.Floors = 3, 55.6, 37.7, 12
	if created, err = store.Houses().Upsert(ctx, h); err != nil || created {
		t.Fatalf("second Upsert created = %v, err = %v", created, err)
	}
	got, _ := store.Houses().Get(ctx, h.ID)
	if got.Floors != 12 || got.Lat != 55.6 || got.EntrancesCount != 3 {
		t.Fatalf("updated house = %+v", got)
	}
	if objs, _ := store.Houses().Objects(ctx, h.ID); len(objs) != 8 {
		t.Fatalf("objects after third entrance = %d, want 8", len(objs))
	}
	// УК должна существовать: ссылочная целостность на уровне базы.
	h.ID, h.OrganizationID = "h-imp-bad", "org-missing"
	if _, err := store.Houses().Upsert(ctx, h); err == nil {
		t.Fatal("Upsert with unknown organization succeeded")
	}
}
