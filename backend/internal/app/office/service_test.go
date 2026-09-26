package office_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"dommax/internal/app"
	"dommax/internal/app/apptest"
	"dommax/internal/app/office"
	"dommax/internal/domain/house"
	"dommax/internal/domain/user"
)

func TestMaintenanceAndAppointments(t *testing.T) {
	s := apptest.New()
	s.Orgs["org-1"] = house.Organization{ID: "org-1", Name: "УК 1"}
	s.HouseMap["h-1"] = house.House{ID: "h-1", Address: "Ореховый бульвар, 17к2", OrganizationID: "org-1"}
	anna := user.User{ID: 1, FirstName: "Анна", Role: user.RoleResident, HouseID: "h-1", ConsentVersion: "v1"}
	oper := user.User{ID: 2, FirstName: "Оператор", Role: user.RoleOperator, OrganizationID: "org-1"}
	s.AddUser(anna)
	s.AddUser(oper)

	now := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	id := 0
	svc := office.NewService(s, office.Config{
		Now:            func() time.Time { return now },
		NewID:          func() string { id++; return fmt.Sprintf("id-%d", id) },
		ConsentVersion: "v1",
	})

	// Житель не может создавать плановые работы
	if _, err := svc.CreateMaintenance(t.Context(), anna, office.CreateMaintenanceInput{HouseID: "h-1", Title: "Тест"}); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("resident create maintenance err = %v", err)
	}

	// Сотрудник УК создаёт плановые работы
	alert, err := svc.CreateMaintenance(t.Context(), oper, office.CreateMaintenanceInput{
		HouseID:     "h-1",
		Category:    "heating",
		Title:       "Опрессовка стояков отопления",
		Description: "Плановая проверка теплового узла",
		StartsAt:    now,
		EndsAt:      now.Add(6 * time.Hour),
	})
	if err != nil || alert.ID == "" {
		t.Fatalf("create maintenance = %+v, err = %v", alert, err)
	}

	active, err := svc.ActiveMaintenance(t.Context(), "h-1")
	if err != nil || len(active) != 1 || active[0].Title != "Опрессовка стояков отопления" {
		t.Fatalf("active = %+v, err = %v", active, err)
	}

	// Запись жителя на приём к главному инженеру
	specs := office.Specialists()
	if len(specs) < 3 {
		t.Fatalf("specialists = %+v", specs)
	}
	appt, err := svc.BookAppointment(t.Context(), anna, office.BookInput{
		Specialist: "chief_engineer",
		Topic:      "Замена радиатора отопления в квартире",
		SlotAt:     now.Add(24 * time.Hour),
	})
	if err != nil || appt.Status != "booked" {
		t.Fatalf("book = %+v, err = %v", appt, err)
	}

	mine, err := svc.MyAppointments(t.Context(), anna)
	if err != nil || len(mine) != 1 {
		t.Fatalf("mine = %+v, err = %v", mine, err)
	}

	orgList, err := svc.OrgAppointments(t.Context(), oper)
	if err != nil || len(orgList) != 1 || orgList[0].UserName != "Анна" {
		t.Fatalf("org list = %+v, err = %v", orgList, err)
	}

	// Отмена записи жителем
	cancelled, err := svc.CancelAppointment(t.Context(), anna, appt.ID)
	if err != nil || cancelled.Status != "cancelled" {
		t.Fatalf("cancel = %+v, err = %v", cancelled, err)
	}
}
