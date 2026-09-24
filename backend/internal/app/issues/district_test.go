package issues_test

import (
	"errors"
	"testing"
	"time"

	"dommax/internal/app"
	"dommax/internal/app/issues"
	"dommax/internal/domain/house"
	"dommax/internal/domain/user"
)

// Район видит все УК своего района со счётчиками и просроченные заявки; соседний район — нет.
func TestDistrictComparesOrganizations(t *testing.T) {
	f := setup(t)
	s := f.store
	s.HouseMap["h-1"] = house.House{ID: "h-1", Address: "Ореховый бульвар, 17к2", OrganizationID: "org-1", District: "Зябликово"}
	s.HouseMap["h-2"] = house.House{ID: "h-2", Address: "Ясеневая улица, 34", OrganizationID: "org-2", District: "Зябликово"}
	s.Orgs["org-3"] = house.Organization{ID: "org-3", Name: "УК соседнего района"}
	s.HouseMap["h-3"] = house.House{ID: "h-3", Address: "Братеевская улица, 1", OrganizationID: "org-3", District: "Братеево"}
	for _, in := range []issues.ReportInput{
		{HouseID: "h-1", Category: "lift"},
		{HouseID: "h-2", Category: "lift"},
		{HouseID: "h-3", Category: "lift"},
	} {
		if _, err := f.svc.Report(t.Context(), f.anna, in); err != nil {
			t.Fatal(err)
		}
	}
	district := user.User{ID: 20, Role: user.RoleDistrict, District: "Зябликово"}
	// Лифт — 1 рабочий день: через трое суток обе заявки района просрочены.
	later := issues.NewService(s, issues.Config{Now: func() time.Time { return now.Add(72 * time.Hour) }, NewID: func() string { return "x" }, ConsentVersion: "v1"})

	m, err := later.DistrictMetrics(t.Context(), district)
	if err != nil {
		t.Fatalf("DistrictMetrics: %v", err)
	}
	if m.District != "Зябликово" || len(m.Orgs) != 2 || m.Orgs[0].Org.ID != "org-1" || m.Orgs[1].Org.ID != "org-2" {
		t.Fatalf("metrics = %+v", m)
	}
	for _, o := range m.Orgs {
		if o.Issues != 1 || o.OpenTotal != 1 || o.OverdueOpen != 1 || o.Week != nil {
			t.Errorf("org %s counts = %+v, week = %v", o.Org.ID, o.OrgCounts, o.Week)
		}
	}

	overdue, err := later.DistrictOverdue(t.Context(), district)
	if err != nil || len(overdue) != 2 {
		t.Fatalf("overdue = %d, err = %v", len(overdue), err)
	}
	for _, is := range overdue {
		if is.HouseID() == "h-3" {
			t.Fatal("issue of another district leaked")
		}
	}
}

func TestDistrictViewsAreForDistrictRoleOnly(t *testing.T) {
	f := setup(t)
	for _, u := range []user.User{f.anna, f.oper, {ID: 21, Role: user.RoleDistrict}} {
		if _, err := f.svc.DistrictMetrics(t.Context(), u); !errors.Is(err, app.ErrForbidden) {
			t.Errorf("metrics for %d: err = %v, want forbidden", u.ID, err)
		}
		if _, err := f.svc.DistrictOverdue(t.Context(), u); !errors.Is(err, app.ErrForbidden) {
			t.Errorf("overdue for %d: err = %v, want forbidden", u.ID, err)
		}
	}
}
