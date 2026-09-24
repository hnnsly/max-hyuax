package user_test

import (
	"testing"
	"time"

	"dommax/internal/domain/user"
)

var now = time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)

func TestResidentCannotManageIssues(t *testing.T) {
	u := user.User{ID: 1, MaxUserID: 1001, Role: user.RoleResident}
	if u.CanManageIssues("org-1") {
		t.Fatal("resident must not change issue statuses")
	}
}

func TestOperatorManagesOnlyOwnOrganization(t *testing.T) {
	u := user.User{ID: 2, Role: user.RoleOperator, OrganizationID: "org-1"}
	if !u.CanManageIssues("org-1") {
		t.Fatal("operator must manage issues of own organization")
	}
	if u.CanManageIssues("org-2") {
		t.Fatal("operator must not manage issues of another organization")
	}
}

// Район (управа или ГЖИ) только смотрит свой район и не меняет статусы заявок.
func TestDistrictViewsOnlyOwnDistrict(t *testing.T) {
	d := user.User{ID: 3, Role: user.RoleDistrict, District: "Зябликово"}
	if !d.CanViewDistrict() || d.CanManageIssues("org-1") {
		t.Fatalf("district user: view = %v, manage = %v", d.CanViewDistrict(), d.CanManageIssues("org-1"))
	}
	for _, u := range []user.User{
		{ID: 4, Role: user.RoleDistrict}, // район не задан
		{ID: 5, Role: user.RoleOperator, OrganizationID: "org-1", District: "Зябликово"}, // район не роль
	} {
		if u.CanViewDistrict() {
			t.Errorf("user %d must not view the district", u.ID)
		}
	}
}

func TestConsentRequiresCurrentDocVersion(t *testing.T) {
	u := user.User{ID: 1}
	if u.HasConsent("v1") {
		t.Fatal("new user has no consent")
	}
	u.AcceptConsent("v1", now)
	if !u.HasConsent("v1") {
		t.Fatal("consent to v1 must be recorded")
	}
	if u.HasConsent("v2") {
		t.Fatal("new document version needs a new consent")
	}
}

func TestDeleteAnonymizesPersonalData(t *testing.T) {
	u := user.User{ID: 1, MaxUserID: 1001, FirstName: "Анна", Phone: "+79990000000", HouseID: "h-1"}
	u.AcceptConsent("v1", now)
	u.Delete(now)

	// Связь с MAX и адрес тоже стираются: вернувшийся человек не увидит старых заявок.
	if !u.Deleted() || u.FirstName != "" || u.Phone != "" || u.HasConsent("v1") || u.MaxUserID != 0 || u.HouseID != "" {
		t.Fatalf("deleted user keeps personal data: %+v", u)
	}
}
