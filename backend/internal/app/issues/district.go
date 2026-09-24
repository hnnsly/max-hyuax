package issues

import (
	"context"

	"dommax/internal/app"
	"dommax/internal/domain/house"
	"dommax/internal/domain/issue"
	"dommax/internal/domain/user"
)

// OrgMetrics — показатели одной УК в сравнении по району.
type OrgMetrics struct {
	Org house.Organization
	Metrics
}

// DistrictMetrics — сравнение УК района для управы или жилинспекции.
type DistrictMetrics struct {
	District string
	Orgs     []OrgMetrics
}

// DistrictMetrics — показатели каждой УК, у которой есть дома в районе пользователя.
// УК считается целиком: если у неё есть дома в других районах, их заявки тоже входят в счёт.
func (s *Service) DistrictMetrics(ctx context.Context, u user.User) (DistrictMetrics, error) {
	if !u.CanViewDistrict() {
		return DistrictMetrics{}, app.ErrForbidden
	}
	orgs, err := s.store.Houses().OrganizationsInDistrict(ctx, u.District)
	if err != nil {
		return DistrictMetrics{}, err
	}
	out := DistrictMetrics{District: u.District, Orgs: make([]OrgMetrics, 0, len(orgs))}
	for _, o := range orgs {
		m, err := s.orgMetrics(ctx, o.ID)
		if err != nil {
			return DistrictMetrics{}, err
		}
		out.Orgs = append(out.Orgs, OrgMetrics{Org: o, Metrics: m})
	}
	return out, nil
}

// DistrictOverdue — открытые заявки района с истёкшим сроком ответа, самые давние первыми.
func (s *Service) DistrictOverdue(ctx context.Context, u user.User) ([]*issue.Issue, error) {
	if !u.CanViewDistrict() {
		return nil, app.ErrForbidden
	}
	return s.store.Issues().OverdueInDistrict(ctx, u.District, s.cfg.Now(), queueLimit)
}
