package issues_test

import (
	"testing"

	"dommax/internal/app"
	"dommax/internal/app/issues"
)

// Оценка УК: срок 50%, оценки жителей 30%, подтверждения 20%; при малом числе закрытых — «мало данных».
func TestScore(t *testing.T) {
	cases := []struct {
		name   string
		c      app.OrgCounts
		score  int
		enough bool
	}{
		{"мало закрытых", app.OrgCounts{ClosedTotal: 2, ClosedOnTime: 2, Confirmed: 2}, 0, false},
		{"всё идеально", app.OrgCounts{ClosedTotal: 10, ClosedOnTime: 10, Confirmed: 10, RatingSum: 50, Ratings: 10}, 100, true},
		// 0.5*0.5 + 0.3*(3/5) + 0.2*0.4 = 0.25 + 0.18 + 0.08
		{"средне", app.OrgCounts{ClosedTotal: 10, ClosedOnTime: 5, Confirmed: 4, RatingSum: 12, Ratings: 4}, 51, true},
		// без оценок: (0.5*0.8 + 0.2*0.5) / 0.7
		{"без оценок", app.OrgCounts{ClosedTotal: 10, ClosedOnTime: 8, Confirmed: 5}, 71, true},
	}
	for _, c := range cases {
		score, enough := issues.Score(c.c)
		if score != c.score || enough != c.enough {
			t.Errorf("%s: score = %d, %v; want %d, %v", c.name, score, enough, c.score, c.enough)
		}
	}
}
