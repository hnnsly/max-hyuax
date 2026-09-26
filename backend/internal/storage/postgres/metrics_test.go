//go:build integration

package postgres_test

import (
	"slices"
	"sync"
	"testing"
	"time"

	"dommax/internal/app"
	"dommax/internal/storage/postgres"
	"dommax/internal/storage/postgres/pgtest"
)

// freshStore — отдельная база с одним только засевом: общие тесты пакета добавляют свои заявки.
func freshStore(t *testing.T) *postgres.Store {
	t.Helper()
	dsn, drop, err := pgtest.Fresh(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(drop)
	s, err := postgres.Open(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

const month = 30 * 24 * time.Hour

// Числа выводятся из засева 00002 (3 демо-заявки), 00006 (22 синтетические) и 00009
// (подтверждения у 14 выполненных из 16, одна заявка возвращена жителями в работу).
func TestSampleSeedGivesMetrics(t *testing.T) {
	s := freshStore(t)
	now := time.Now()
	got, err := s.Issues().OrgCounts(t.Context(), "org-orekh", now.Add(-month), now)
	if err != nil {
		t.Fatal(err)
	}
	// Оценки примера данных (00017): у каждого подтверждённого ремонта Орехового квартала 3–5 звёзд.
	want := app.OrgCounts{
		Issues: 25, Reports: 98, ClosedTotal: 17, ClosedOnTime: 14, Confirmed: 14, Reopened: 1, OpenTotal: 8, OverdueOpen: 2,
		RatingSum: 55, Ratings: 14, SampleData: true,
	}
	if got != want {
		t.Errorf("counts = %+v, want %+v", got, want)
	}

	resp, err := s.Issues().FirstResponses(t.Context(), "org-orekh", now.Add(-14*24*time.Hour))
	if err != nil || len(resp) != 19 {
		t.Fatalf("first responses = %d, err = %v", len(resp), err)
	}
	for _, r := range resp {
		if !r.RespondedAt.After(r.CreatedAt) {
			t.Errorf("response %v is not after creation %v", r.RespondedAt, r.CreatedAt)
		}
	}

	none, err := s.Issues().OrgCounts(t.Context(), "org-unknown", now.Add(-month), now)
	if err != nil || none != (app.OrgCounts{}) {
		t.Errorf("unknown org counts = %+v, err = %v", none, err)
	}
}

func TestShiftSampleDataKeepsExampleFresh(t *testing.T) {
	s := freshStore(t)
	now := time.Now()
	if days, err := s.ShiftSampleData(t.Context(), now); err != nil || days != 0 {
		t.Fatalf("fresh seed shift = %d, err = %v", days, err)
	}
	before, err := s.Issues().OrgCounts(t.Context(), "org-orekh", now.Add(-month), now)
	if err != nil {
		t.Fatal(err)
	}

	later := now.Add(3*24*time.Hour + time.Hour)
	days, err := s.ShiftSampleData(t.Context(), later)
	if err != nil || days != 3 {
		t.Fatalf("shift = %d, err = %v", days, err)
	}
	after, err := s.Issues().OrgCounts(t.Context(), "org-orekh", later.Add(-month), later)
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Errorf("after shift counts = %+v, want %+v", after, before)
	}
	if days, err := s.ShiftSampleData(t.Context(), later); err != nil || days != 0 {
		t.Errorf("repeated shift = %d, err = %v", days, err)
	}
}

// Засев 00011: три УК в Зябликово, просроченные заявки района — две у «Орехового квартала» и три у «Каширского».
func TestDistrictSeed(t *testing.T) {
	s := freshStore(t)
	orgs, err := s.Houses().OrganizationsInDistrict(t.Context(), "Зябликово")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, o := range orgs {
		names = append(names, o.Name)
	}
	if want := []string{"УК «Каширский квартал»", "УК «Ореховый квартал»", "УК «Ясеневый двор»"}; !slices.Equal(names, want) {
		t.Fatalf("organizations = %v, want %v", names, want)
	}
	overdue, err := s.Issues().OverdueInDistrict(t.Context(), "Зябликово", time.Now(), 50)
	if err != nil || len(overdue) != 5 {
		t.Fatalf("overdue = %d, err = %v", len(overdue), err)
	}
	for i := 1; i < len(overdue); i++ {
		if overdue[i].Deadline().Before(overdue[i-1].Deadline()) {
			t.Fatal("overdue issues must go oldest deadline first")
		}
	}
	if none, _ := s.Issues().OverdueInDistrict(t.Context(), "Братеево", time.Now(), 50); len(none) != 0 {
		t.Fatalf("another district = %d issues", len(none))
	}
	d, err := s.Users().ByDemoKey(t.Context(), "district_demo")
	if err != nil || !d.CanViewDistrict() || d.District != "Зябликово" {
		t.Fatalf("district demo user = %+v, err = %v", d, err)
	}
	kashir, err := s.Issues().OrgCounts(t.Context(), "org-kashir", time.Now().Add(-month), time.Now())
	if err != nil || kashir.OverdueOpen != 3 || kashir.Confirmed != 1 || !kashir.SampleData {
		t.Fatalf("kashir counts = %+v, err = %v", kashir, err)
	}
}

// Два экземпляра api запускают сдвиг одновременно: даты сдвигаются один раз, а не дважды.
func TestShiftSampleDataRunsOnceConcurrently(t *testing.T) {
	s := freshStore(t)
	later := time.Now().Add(5*24*time.Hour + time.Hour)
	var wg sync.WaitGroup
	shifted := make([]int, 4)
	for i := range shifted {
		wg.Go(func() {
			days, err := s.ShiftSampleData(t.Context(), later)
			if err != nil {
				t.Error(err)
			}
			shifted[i] = days
		})
	}
	wg.Wait()
	total := 0
	for _, d := range shifted {
		total += d
	}
	if total != 5 {
		t.Fatalf("shifts = %v, want 5 days in total", shifted)
	}
}
