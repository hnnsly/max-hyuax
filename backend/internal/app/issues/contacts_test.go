package issues_test

import (
	"testing"

	"dommax/internal/app/issues"
	"dommax/internal/domain/user"
)

// Телефоны участников видит только сотрудник ответственной УК; соседи и чужая УК не видят ничего.
func TestContactsOnlyForResponsibleUK(t *testing.T) {
	f := setup(t)
	is := report(t, f, f.anna)
	if _, err := f.svc.Join(t.Context(), f.sergey, is.ID()); err != nil {
		t.Fatal(err)
	}
	anna := f.anna
	anna.FirstName = "Анна"
	anna.SharePhone("+79991234567")
	f.store.AddUser(anna)
	// Удалённый аккаунт не показывается, даже если телефон остался в старой записи.
	gone := user.User{ID: 7, Role: user.RoleResident, ConsentVersion: "v1", Phone: "+79990000007"}
	f.store.AddUser(gone)
	if _, err := f.svc.Join(t.Context(), gone, is.ID()); err != nil {
		t.Fatal(err)
	}
	gone.Delete(now)
	f.store.AddUser(gone)

	got, err := f.svc.Contacts(t.Context(), f.oper, is)
	if err != nil {
		t.Fatal(err)
	}
	if want := []issues.Contact{{FirstName: "Анна", Phone: "+79991234567"}}; len(got) != 1 || got[0] != want[0] {
		t.Fatalf("contacts = %+v, want %+v", got, want)
	}

	otherUK := user.User{ID: 9, Role: user.RoleOperator, OrganizationID: "org-2"}
	for _, viewer := range []user.User{f.sergey, otherUK} {
		if got, err := f.svc.Contacts(t.Context(), viewer, is); err != nil || got != nil {
			t.Errorf("viewer %d: contacts = %+v, err = %v", viewer.ID, got, err)
		}
	}
}
