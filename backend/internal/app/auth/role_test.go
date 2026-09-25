package auth_test

import (
	"errors"
	"testing"
	"time"

	"dommax/internal/app"
	"dommax/internal/app/apptest"
	"dommax/internal/app/auth"
	"dommax/internal/domain/house"
	"dommax/internal/domain/user"
)

// Смена роли нужна комиссии на демо-стенде; вне демо-режима её нет, а взятая роль помечается.
func TestSwitchRole(t *testing.T) {
	s := apptest.New()
	s.Orgs["org-1"] = house.Organization{ID: "org-1"}
	s.HouseMap["h-1"] = house.House{ID: "h-1", OrganizationID: "org-1", District: "Зябликово"}
	u := user.User{ID: 1, Role: user.RoleResident, HouseID: "h-1"}
	s.AddUser(u)
	now := func() time.Time { return time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC) }

	prod := auth.NewService(s, auth.Config{Now: now})
	if _, err := prod.SwitchRole(t.Context(), u, "uk_operator"); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("switch without demo err = %v", err)
	}

	demo := auth.NewService(s, auth.Config{Now: now, DemoEnabled: true})
	op, err := demo.SwitchRole(t.Context(), u, "uk_operator")
	if err != nil || op.Role != user.RoleOperator || op.OrganizationID != "org-1" || !op.RoleSwitched {
		t.Fatalf("operator = %+v, err = %v", op, err)
	}
	ch, err := demo.SwitchRole(t.Context(), op, "chairman")
	if err != nil || !ch.IsChairmanOf("h-1") || !ch.RoleSwitched {
		t.Fatalf("chairman = %+v, err = %v", ch, err)
	}
	back, err := demo.SwitchRole(t.Context(), ch, "resident")
	if err != nil || back.Role != user.RoleResident || back.RoleSwitched || back.ChairmanHouseID != "" {
		t.Fatalf("resident = %+v, err = %v", back, err)
	}
}
