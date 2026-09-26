package issues

import (
	"context"
	"time"

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
	orgs, err := s.districtOrgs(ctx, u.District)
	return DistrictMetrics{District: u.District, Orgs: orgs}, err
}

// districtOrgs — показатели каждой УК, у которой есть дома в районе.
func (s *Service) districtOrgs(ctx context.Context, district string) ([]OrgMetrics, error) {
	orgs, err := s.store.Houses().OrganizationsInDistrict(ctx, district)
	if err != nil {
		return nil, err
	}
	out := make([]OrgMetrics, 0, len(orgs))
	for _, o := range orgs {
		m, err := s.orgMetrics(ctx, o.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, OrgMetrics{Org: o, Metrics: m})
	}
	return out, nil
}

// MapHouse — дом на карте кабинета: сколько заявок открыто и просрочено, сколько дней самой давней просрочке.
type MapHouse struct {
	House          house.House
	Open, Overdue  int
	MaxOverdueDays int
}

// HouseMap — дома на карте (ADR-020): сотруднику УК — дома своей УК, управе — дома района.
// Жителю карта нагрузки не положена: по ней видно, где соседи ждут ремонта.
func (s *Service) HouseMap(ctx context.Context, u user.User) ([]MapHouse, error) {
	var orgID, district string
	switch {
	case u.Role == user.RoleOperator && u.OrganizationID != "":
		orgID = u.OrganizationID
	case u.CanViewDistrict():
		district = u.District
	default:
		return nil, app.ErrForbidden
	}
	now := s.cfg.Now()
	rows, err := s.store.Houses().Load(ctx, orgID, district, now)
	if err != nil {
		return nil, err
	}
	out := make([]MapHouse, len(rows))
	for i, r := range rows {
		out[i] = MapHouse{House: r.House, Open: r.Open, Overdue: r.Overdue, MaxOverdueDays: int(now.Sub(r.OldestOverdue) / (24 * time.Hour))}
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
