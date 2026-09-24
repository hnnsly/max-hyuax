package issues

import (
	"testing"
	"time"

	"dommax/internal/app"
)

func TestMedian(t *testing.T) {
	h := time.Hour
	for _, c := range []struct {
		in   []time.Duration
		want time.Duration
		ok   bool
	}{
		{nil, 0, false},
		{[]time.Duration{3 * h}, 3 * h, true},
		{[]time.Duration{5 * h, 1 * h, 3 * h}, 3 * h, true},
		{[]time.Duration{4 * h, 1 * h, 2 * h, 8 * h}, 3 * h, true}, // чётное число: среднее двух средних
	} {
		got := median(c.in)
		if (got != nil) != c.ok || (got != nil && *got != c.want) {
			t.Errorf("median(%v) = %v, want %v (ok=%v)", c.in, got, c.want, c.ok)
		}
	}
}

func TestFirstResponseStatsBucketsByMoscowDay(t *testing.T) {
	// Сегодня четверг 17.09.2026 по Москве.
	today := time.Date(2026, 9, 17, 0, 0, 0, 0, Moscow)
	at := func(day, hour int) time.Time { return time.Date(2026, 9, day, hour, 0, 0, 0, Moscow) }
	resp := func(created time.Time, after time.Duration) app.FirstResponse {
		return app.FirstResponse{CreatedAt: created, RespondedAt: created.Add(after)}
	}
	list := []app.FirstResponse{
		resp(at(17, 9), 2*time.Hour),
		// 00:30 по Москве 11.09 — это ещё 10.09 по UTC; считается днём 11.09.
		resp(time.Date(2026, 9, 10, 21, 30, 0, 0, time.UTC), 6*time.Hour),
		resp(at(11, 12), 4*time.Hour),
		resp(at(9, 12), 10*time.Hour), // прошлая неделя
		resp(at(4, 12), 8*time.Hour),  // прошлая неделя, первый её день
		resp(at(3, 12), 99*time.Hour), // раньше двух недель: не учитывается
	}
	week, prev, days := firstResponseStats(list, today)

	if week == nil || *week != 4*time.Hour {
		t.Errorf("week median = %v, want 4h", week)
	}
	if prev == nil || *prev != 9*time.Hour {
		t.Errorf("prev week median = %v, want 9h", prev)
	}
	if len(days) != 7 || !days[0].Day.Equal(at(11, 0)) || !days[6].Day.Equal(today) {
		t.Fatalf("days = %+v", days)
	}
	if days[0].Median == nil || *days[0].Median != 5*time.Hour {
		t.Errorf("11.09 median = %v, want 5h", days[0].Median)
	}
	if days[6].Median == nil || *days[6].Median != 2*time.Hour {
		t.Errorf("today median = %v, want 2h", days[6].Median)
	}
	for _, d := range days[1:6] {
		if d.Median != nil {
			t.Errorf("%s has median %v, want none", d.Day.Format(time.DateOnly), *d.Median)
		}
	}
}

func TestFirstResponseStatsWithoutData(t *testing.T) {
	week, prev, days := firstResponseStats(nil, time.Date(2026, 9, 17, 0, 0, 0, 0, Moscow))
	if week != nil || prev != nil || len(days) != 7 {
		t.Fatalf("week=%v prev=%v days=%d", week, prev, len(days))
	}
}
