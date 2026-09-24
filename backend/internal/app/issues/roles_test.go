package issues_test

import (
	"errors"
	"testing"

	"dommax/internal/app"
	"dommax/internal/app/issues"
	"dommax/internal/domain/issue"
	"dommax/internal/domain/user"
)

// Сотрудник УК и район не становятся участниками заявок, даже с согласием на обработку данных.
func TestOnlyResidentsReportJoinAndAnswer(t *testing.T) {
	f := setup(t)
	district := user.User{ID: 30, Role: user.RoleDistrict, District: "Зябликово", ConsentVersion: "v1"}
	oper := f.oper
	oper.ConsentVersion = "v1"
	is := done(t, f)

	for _, u := range []user.User{oper, district} {
		if _, err := f.svc.Report(t.Context(), u, issues.ReportInput{HouseID: "h-1", Category: "lift"}); !errors.Is(err, app.ErrForbidden) {
			t.Errorf("%s report: err = %v, want forbidden", u.Role, err)
		}
		if _, err := f.svc.Confirm(t.Context(), u, is.ID()); !errors.Is(err, app.ErrForbidden) {
			t.Errorf("%s confirm: err = %v, want forbidden", u.Role, err)
		}
		if _, err := f.svc.Reopen(t.Context(), u, is.ID(), "Не починили"); !errors.Is(err, app.ErrForbidden) {
			t.Errorf("%s reopen: err = %v, want forbidden", u.Role, err)
		}
	}
	open := report(t, f, f.anna)
	for _, u := range []user.User{oper, district} {
		if _, err := f.svc.Join(t.Context(), u, open.ID()); !errors.Is(err, app.ErrForbidden) {
			t.Errorf("%s join: err = %v, want forbidden", u.Role, err)
		}
	}
	if got, _ := f.svc.Get(t.Context(), open.ID()); got.ParticipantCount() != 1 || got.Status() != issue.StatusSent {
		t.Fatalf("issue changed: %d participants, %s", got.ParticipantCount(), got.Status())
	}
}
