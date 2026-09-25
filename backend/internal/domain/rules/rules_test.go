package rules_test

import (
	"errors"
	"testing"
	"time"

	"dommax/internal/domain/rules"
)

var msk = time.FixedZone("MSK", 3*60*60)

func TestAddBusinessDaysSkipsWeekends(t *testing.T) {
	// Четверг 17.09.2026 + 2 рабочих дня = понедельник 21.09.2026 (суббота и воскресенье пропускаются).
	from := time.Date(2026, 9, 17, 10, 0, 0, 0, msk)
	got := rules.AddBusinessDays(from, 2)
	want := time.Date(2026, 9, 21, 23, 59, 59, 0, msk)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestAddBusinessDaysFromSaturdayStartsMonday(t *testing.T) {
	from := time.Date(2026, 9, 19, 12, 0, 0, 0, msk)
	got := rules.AddBusinessDays(from, 1)
	want := time.Date(2026, 9, 21, 23, 59, 59, 0, msk)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestAddBusinessDaysSkipsPublicHolidays(t *testing.T) {
	// 31 декабря + 1 рабочий день пропускает все каникулы 1-8 января и выпадает на 9 января (пятницу)
	from := time.Date(2025, 12, 31, 15, 0, 0, 0, msk)
	got := rules.AddBusinessDays(from, 1)
	want := time.Date(2026, 1, 9, 23, 59, 59, 0, msk)
	if !got.Equal(want) {
		t.Fatalf("new year holidays: got %v, want %v", got, want)
	}
}

func TestLookupKnownCategory(t *testing.T) {
	r, err := rules.Lookup("lift")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if r.Responsible != rules.ResponsibleManagementCompany || r.Title == "" || r.Basis == "" {
		t.Fatalf("rule = %+v", r)
	}
	created := time.Date(2026, 9, 17, 8, 10, 0, 0, msk)
	if d := r.Deadline(created); !d.After(created) {
		t.Fatalf("deadline %v is not after creation", d)
	}
}

func TestLookupUnknownCategory(t *testing.T) {
	if _, err := rules.Lookup("teleport"); !errors.Is(err, rules.ErrUnknownCategory) {
		t.Fatalf("err = %v, want ErrUnknownCategory", err)
	}
}

func TestClassifyByKeywords(t *testing.T) {
	cases := map[string]string{
		"Во втором подъезде не работает лифт": "lift",
		"Не горит свет на 5 этаже":            "lighting",
		"С потолка капает, протечка с крыши":  "leak",
		"Батареи совсем холодные":             "heating",
		"Сломан ДОМОФОН в первом подъезде":    "door",
		"Мусоропровод забит, запах":           "garbage",
	}
	for text, want := range cases {
		r, ok := rules.Classify(text)
		if !ok || r.Code != want {
			t.Errorf("Classify(%q) = %q, %v; want %q", text, r.Code, ok, want)
		}
	}
	if _, ok := rules.Classify("Непонятно что случилось"); ok {
		t.Error("text without keywords must not be classified")
	}
}

func TestCategoriesAreOrderedAndComplete(t *testing.T) {
	cats := rules.Categories()
	if len(cats) < 6 {
		t.Fatalf("categories = %d, want at least 6", len(cats))
	}
	for _, c := range cats {
		if _, err := rules.Lookup(c.Code); err != nil {
			t.Fatalf("category %q from list is not resolvable: %v", c.Code, err)
		}
	}
}
