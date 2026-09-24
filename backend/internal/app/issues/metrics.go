package issues

import (
	"context"
	"slices"
	"time"

	"dommax/internal/app"
	"dommax/internal/domain/user"
)

// MetricsPeriodDays — за сколько дней считаются счётчики метрик УК.
const MetricsPeriodDays = 30

// Metrics — показатели УК для дашборда.
type Metrics struct {
	Week     *time.Duration // медиана первого ответа по заявкам, поданным за последние 7 дней
	PrevWeek *time.Duration // то же за 7 дней до этого
	ByDay    []DayMedian    // 7 дней по Москве, от раннего к сегодняшнему
	app.OrgCounts
}

// DayMedian — медиана первого ответа по заявкам, поданным в этот день; nil, если ответов не было.
type DayMedian struct {
	Day    time.Time // полночь по Москве
	Median *time.Duration
}

// Metrics — показатели своей УК; доступны только её оператору.
func (s *Service) Metrics(ctx context.Context, u user.User) (Metrics, error) {
	if u.Role != user.RoleOperator || u.OrganizationID == "" {
		return Metrics{}, app.ErrForbidden
	}
	return s.orgMetrics(ctx, u.OrganizationID)
}

// orgMetrics — показатели одной УК: для её кабинета и для сравнения в кабинете района.
func (s *Service) orgMetrics(ctx context.Context, orgID string) (Metrics, error) {
	now := s.cfg.Now()
	y, m, d := now.In(Moscow).Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, Moscow)

	resp, err := s.store.Issues().FirstResponses(ctx, orgID, today.AddDate(0, 0, -13))
	if err != nil {
		return Metrics{}, err
	}
	counts, err := s.store.Issues().OrgCounts(ctx, orgID, now.AddDate(0, 0, -MetricsPeriodDays), now)
	if err != nil {
		return Metrics{}, err
	}
	out := Metrics{OrgCounts: counts}
	out.Week, out.PrevWeek, out.ByDay = firstResponseStats(resp, today)
	return out, nil
}

// firstResponseStats раскладывает время первого ответа по московским дням подачи:
// последние 7 дней (включая сегодня) и 7 дней до них.
func firstResponseStats(list []app.FirstResponse, today time.Time) (week, prev *time.Duration, days []DayMedian) {
	weekStart, prevStart := today.AddDate(0, 0, -6), today.AddDate(0, 0, -13)
	perDay := make([][]time.Duration, 7)
	var thisWeek, lastWeek []time.Duration
	for _, r := range list {
		y, m, d := r.CreatedAt.In(Moscow).Date()
		day := time.Date(y, m, d, 0, 0, 0, 0, Moscow)
		took := r.RespondedAt.Sub(r.CreatedAt)
		switch {
		case !day.Before(weekStart) && !day.After(today):
			thisWeek = append(thisWeek, took)
			i := int(day.Sub(weekStart).Hours()+12) / 24 // +12 ч сглаживает возможный переход времени
			perDay[i] = append(perDay[i], took)
		case !day.Before(prevStart) && day.Before(weekStart):
			lastWeek = append(lastWeek, took)
		}
	}
	days = make([]DayMedian, 7)
	for i := range days {
		days[i] = DayMedian{Day: weekStart.AddDate(0, 0, i), Median: median(perDay[i])}
	}
	return median(thisWeek), median(lastWeek), days
}

// median — медиана длительностей; nil для пустого списка.
func median(ds []time.Duration) *time.Duration {
	if len(ds) == 0 {
		return nil
	}
	s := slices.Sorted(slices.Values(ds))
	m := s[len(s)/2]
	if len(s)%2 == 0 {
		m = (s[len(s)/2-1] + m) / 2
	}
	return &m
}
