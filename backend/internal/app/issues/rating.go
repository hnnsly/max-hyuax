package issues

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"slices"

	"dommax/internal/app"
	"dommax/internal/domain/issue"
	"dommax/internal/domain/user"
)

// MinClosedForScore — при меньшем числе закрытых за период заявок оценка УК не считается:
// одна-две заявки дают случайный результат.
const MinClosedForScore = 3

// Rate — житель оценивает подтверждённый ремонт от 1 до 5 звёзд (ADR-022).
func (s *Service) Rate(ctx context.Context, u user.User, issueID string, stars int) (*issue.Issue, error) {
	if !u.CanTakePart() {
		return nil, fmt.Errorf("%w: only residents rate repairs", app.ErrForbidden)
	}
	return s.update(ctx, issueID, func(is *issue.Issue) error {
		return is.Rate(u.ID, stars, s.cfg.Now())
	})
}

// Score — оценка УК от 0 до 100 за период: 50% — доля закрытых в срок, 30% — средняя оценка
// ремонтов жителями, 20% — доля ремонтов, которые жители подтвердили. Без оценок их вес делится
// между сроком и подтверждениями. enough = false — закрытых заявок меньше MinClosedForScore.
func Score(c app.OrgCounts) (score int, enough bool) {
	if c.ClosedTotal < MinClosedForScore {
		return 0, false
	}
	onTime := float64(c.ClosedOnTime) / float64(c.ClosedTotal)
	confirmed := min(1, float64(c.Confirmed)/float64(c.ClosedTotal))
	if c.Ratings == 0 {
		return int(math.Round(100 * (0.5*onTime + 0.2*confirmed) / 0.7)), true
	}
	return int(math.Round(100 * (0.5*onTime + 0.3*c.RatingAvg()/5 + 0.2*confirmed))), true
}

// OrgRating — место УК в рейтинге района.
type OrgRating struct {
	OrgMetrics
	Score  int
	Enough bool // хватает ли закрытых заявок для оценки
}

// DistrictRating — рейтинг УК района, открытый жителям.
type DistrictRating struct {
	District string
	MyOrgID  string      // УК дома жителя или своя УК сотрудника; пусто у управы
	Orgs     []OrgRating // сначала с оценкой по убыванию, в конце «мало данных»
}

// DistrictRating — рейтинг УК района: жителю по району его дома, управе по её району,
// сотруднику УК по району домов его УК.
func (s *Service) DistrictRating(ctx context.Context, u user.User) (DistrictRating, error) {
	district, myOrg, err := s.ratingDistrict(ctx, u)
	if err != nil {
		return DistrictRating{}, err
	}
	orgs, err := s.districtOrgs(ctx, district)
	if err != nil {
		return DistrictRating{}, err
	}
	out := DistrictRating{District: district, MyOrgID: myOrg, Orgs: make([]OrgRating, 0, len(orgs))}
	for _, o := range orgs {
		score, enough := Score(o.OrgCounts)
		out.Orgs = append(out.Orgs, OrgRating{OrgMetrics: o, Score: score, Enough: enough})
	}
	slices.SortStableFunc(out.Orgs, func(a, b OrgRating) int {
		if a.Enough != b.Enough {
			if a.Enough {
				return -1
			}
			return 1
		}
		return cmp.Or(cmp.Compare(b.Score, a.Score), cmp.Compare(a.Org.Name, b.Org.Name))
	})
	return out, nil
}

func (s *Service) ratingDistrict(ctx context.Context, u user.User) (district, myOrg string, err error) {
	switch {
	case u.CanViewDistrict():
		return u.District, "", nil
	case u.Role == user.RoleOperator && u.OrganizationID != "":
		hs, err := s.store.Houses().ByOrganization(ctx, u.OrganizationID)
		if err != nil || len(hs) == 0 {
			return "", "", cmp.Or(err, app.ErrNotFound)
		}
		return hs[0].District, u.OrganizationID, nil
	case u.HouseID != "":
		h, err := s.store.Houses().Get(ctx, u.HouseID)
		if err != nil {
			return "", "", err
		}
		return h.District, h.OrganizationID, nil
	}
	return "", "", fmt.Errorf("%w: choose a house first", app.ErrForbidden)
}
