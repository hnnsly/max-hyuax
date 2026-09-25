package photos_test

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"testing"
	"time"

	"dommax/internal/app"
	"dommax/internal/app/apptest"
	"dommax/internal/app/issues"
	"dommax/internal/app/photos"
	"dommax/internal/domain/house"
	"dommax/internal/domain/issue"
	"dommax/internal/domain/user"
)

type fixture struct {
	store                           *apptest.MemStore
	files                           *apptest.MemFiles
	issues                          *issues.Service
	svc                             *photos.Service
	anna, stranger, oper, otherOper user.User
	is                              *issue.Issue
}

func setup(t *testing.T) fixture {
	t.Helper()
	s := apptest.New()
	s.Orgs["org-1"] = house.Organization{ID: "org-1", Name: "УК"}
	s.HouseMap["h-1"] = house.House{ID: "h-1", Address: "Ореховый бульвар, 17к2", OrganizationID: "org-1"}
	f := fixture{
		store:     s,
		files:     &apptest.MemFiles{},
		anna:      user.User{ID: 1, Role: user.RoleResident, ConsentVersion: "v1"},
		stranger:  user.User{ID: 2, Role: user.RoleResident, ConsentVersion: "v1"},
		oper:      user.User{ID: 3, Role: user.RoleOperator, OrganizationID: "org-1"},
		otherOper: user.User{ID: 4, Role: user.RoleOperator, OrganizationID: "org-2"},
	}
	for _, u := range []user.User{f.anna, f.stranger, f.oper, f.otherOper} {
		s.AddUser(u)
	}
	now := time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)
	n := 0
	newID := func() string { n++; return fmt.Sprintf("0190a000-0000-7000-8000-%012d", n) }
	f.issues = issues.NewService(s, issues.Config{Now: func() time.Time { return now }, NewID: newID, ConsentVersion: "v1"})
	f.svc = photos.NewService(s, f.files, photos.Config{Now: func() time.Time { return now }, NewID: newID, ConsentVersion: "v1"})
	is, err := f.issues.Report(t.Context(), f.anna, issues.ReportInput{HouseID: "h-1", Category: "lift"})
	if err != nil {
		t.Fatal(err)
	}
	f.is = is
	return f
}

func jpg(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 64, 48)), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestParticipantAddsAndViewsPhotos(t *testing.T) {
	f := setup(t)
	added, err := f.svc.Add(t.Context(), f.anna, f.is.ID(), jpg(t))
	if err != nil || len(added) != 1 {
		t.Fatalf("added = %+v, err = %v", added, err)
	}
	p := added[0]
	if p.IssueID != f.is.ID() || p.UploadedBy != f.anna.ID || p.Width != 64 || p.Height != 48 || p.SizeBytes == 0 {
		t.Fatalf("photo = %+v", p)
	}
	// УК заявки видит фото и тоже может добавить своё, например после ремонта.
	if _, err := f.svc.Add(t.Context(), f.oper, f.is.ID(), jpg(t)); err != nil {
		t.Fatalf("operator add: %v", err)
	}
	list, err := f.svc.List(t.Context(), f.oper, f.is.ID())
	if err != nil || len(list) != 2 || list[0].ID != p.ID {
		t.Fatalf("list = %+v, err = %v", list, err)
	}
	got, data, err := f.svc.Open(t.Context(), f.anna, p.ID)
	if err != nil || got.ID != p.ID || !bytes.HasPrefix(data, []byte{0xFF, 0xD8}) {
		t.Fatalf("open = %+v, %d bytes, err = %v", got, len(data), err)
	}
}

func TestPhotoAccessRules(t *testing.T) {
	f := setup(t)
	added, err := f.svc.Add(t.Context(), f.anna, f.is.ID(), jpg(t))
	if err != nil {
		t.Fatal(err)
	}
	p := added[0]
	for _, u := range []user.User{f.stranger, f.otherOper} {
		if _, err := f.svc.Add(t.Context(), u, f.is.ID(), jpg(t)); !errors.Is(err, app.ErrForbidden) {
			t.Errorf("add by %d: err = %v, want forbidden", u.ID, err)
		}
		if _, err := f.svc.List(t.Context(), u, f.is.ID()); !errors.Is(err, app.ErrForbidden) {
			t.Errorf("list by %d: err = %v, want forbidden", u.ID, err)
		}
		if _, _, err := f.svc.Open(t.Context(), u, p.ID); !errors.Is(err, app.ErrForbidden) {
			t.Errorf("open by %d: err = %v, want forbidden", u.ID, err)
		}
	}
	if _, _, err := f.svc.Open(t.Context(), f.anna, "nope"); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("open unknown: err = %v, want not found", err)
	}
	if _, err := f.svc.Add(t.Context(), f.anna, "nope", jpg(t)); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("add to unknown issue: err = %v, want not found", err)
	}
}

func TestAddLimitsAndValidation(t *testing.T) {
	f := setup(t)
	if _, err := f.svc.Add(t.Context(), f.anna, f.is.ID(), []byte("not an image")); !errors.Is(err, photos.ErrNotImage) {
		t.Fatalf("not image: err = %v", err)
	}
	for range photos.MaxPerIssue {
		if _, err := f.svc.Add(t.Context(), f.anna, f.is.ID(), jpg(t)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.svc.Add(t.Context(), f.anna, f.is.ID(), jpg(t)); !errors.Is(err, photos.ErrTooMany) {
		t.Fatalf("over limit: err = %v, want ErrTooMany", err)
	}
}

func TestNoPhotosForClosedIssue(t *testing.T) {
	f := setup(t)
	for _, st := range []issue.Status{issue.StatusAccepted, issue.StatusDone} {
		if _, err := f.issues.ChangeStatus(t.Context(), f.oper, f.is.ID(), st, "Готово"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.svc.Add(t.Context(), f.anna, f.is.ID(), jpg(t)); !errors.Is(err, issue.ErrClosed) {
		t.Fatalf("closed: err = %v, want ErrClosed", err)
	}
	// Оператор УК может приложить фото-подтверждение выполненного ремонта и к заявке в статусе done.
	if _, err := f.svc.Add(t.Context(), f.oper, f.is.ID(), jpg(t)); err != nil {
		t.Fatalf("operator add proof to done issue: %v", err)
	}
}

// Один плохой файл в пачке отклоняет всю пачку: повтор не создаст дублей уже сохранённых.
func TestAddIsAllOrNothing(t *testing.T) {
	f := setup(t)
	if _, err := f.svc.Add(t.Context(), f.anna, f.is.ID(), jpg(t), []byte("not an image")); !errors.Is(err, photos.ErrNotImage) {
		t.Fatalf("err = %v, want ErrNotImage", err)
	}
	if list, _ := f.svc.List(t.Context(), f.anna, f.is.ID()); len(list) != 0 {
		t.Fatalf("list = %+v, want empty", list)
	}
	if n := f.files.Len(); n != 0 {
		t.Fatalf("files = %d, want 0", n)
	}
}

func TestAddRejectsBatchOverLimit(t *testing.T) {
	f := setup(t)
	if _, err := f.svc.Add(t.Context(), f.anna, f.is.ID(), jpg(t), jpg(t), jpg(t), jpg(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Add(t.Context(), f.anna, f.is.ID(), jpg(t), jpg(t), jpg(t)); !errors.Is(err, photos.ErrTooMany) {
		t.Fatalf("err = %v, want ErrTooMany", err)
	}
	if list, _ := f.svc.List(t.Context(), f.anna, f.is.ID()); len(list) != 4 {
		t.Fatalf("list = %d photos, want 4", len(list))
	}
	if n := f.files.Len(); n != 4 {
		t.Fatalf("files = %d, want 4", n)
	}
}

// Житель без согласия на обработку данных фото не прикладывает; сотруднику УК согласие не нужно.
func TestResidentNeedsConsentToAddPhotos(t *testing.T) {
	f := setup(t)
	noConsent := f.anna
	noConsent.ConsentVersion = ""
	if _, err := f.svc.Add(t.Context(), noConsent, f.is.ID(), jpg(t)); !errors.Is(err, app.ErrConsentRequired) {
		t.Fatalf("err = %v, want ErrConsentRequired", err)
	}
}

// Автор может убрать своё фото; чужое не может никто, даже УК заявки.
func TestUploaderRemovesOwnPhoto(t *testing.T) {
	f := setup(t)
	added, err := f.svc.Add(t.Context(), f.anna, f.is.ID(), jpg(t))
	if err != nil {
		t.Fatal(err)
	}
	p := added[0]
	for _, u := range []user.User{f.oper, f.stranger} {
		if err := f.svc.Remove(t.Context(), u, p.ID); !errors.Is(err, app.ErrForbidden) {
			t.Errorf("remove by %d: err = %v, want forbidden", u.ID, err)
		}
	}
	if err := f.svc.Remove(t.Context(), f.anna, p.ID); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, _, err := f.svc.Open(t.Context(), f.anna, p.ID); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("open after remove: err = %v, want not found", err)
	}
	if n := f.files.Len(); n != 0 {
		t.Fatalf("files = %d, want 0", n)
	}
}

// При удалении аккаунта уходят все фото пользователя, фото других остаются.
func TestForgetUserRemovesOnlyTheirPhotos(t *testing.T) {
	f := setup(t)
	if _, err := f.svc.Add(t.Context(), f.anna, f.is.ID(), jpg(t), jpg(t)); err != nil {
		t.Fatal(err)
	}
	kept, err := f.svc.Add(t.Context(), f.oper, f.is.ID(), jpg(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ForgetUser(t.Context(), f.anna.ID); err != nil {
		t.Fatalf("forget: %v", err)
	}
	list, _ := f.svc.List(t.Context(), f.oper, f.is.ID())
	if len(list) != 1 || list[0].ID != kept[0].ID || f.files.Len() != 1 {
		t.Fatalf("list = %+v, files = %d", list, f.files.Len())
	}
}

// Файл не записался — метаданных тоже нет: в заявке не появится «пустое» фото.
func TestFileFailureLeavesNoMetadata(t *testing.T) {
	f := setup(t)
	f.files.Fail = errors.New("disk full")
	if _, err := f.svc.Add(t.Context(), f.anna, f.is.ID(), jpg(t)); err == nil {
		t.Fatal("want error")
	}
	if list, _ := f.svc.List(t.Context(), f.anna, f.is.ID()); len(list) != 0 {
		t.Fatalf("list = %+v, want empty", list)
	}
}
