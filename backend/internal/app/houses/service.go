// Пакет houses — чтение справочника домов: поиск, ближайшие, карточка дома, вход по QR.
package houses

import (
	"context"
	"fmt"
	"strings"

	"dommax/internal/app"
	"dommax/internal/domain/house"
)

type Service struct{ store app.Store }

func NewService(store app.Store) *Service { return &Service{store: store} }

// Details — дом со всем, что нужно экрану заявки: УК, подъезды, объекты с QR.
type Details struct {
	House        house.House
	Organization house.Organization
	Entrances    []house.Entrance
	Objects      []house.AssetObject
}

func (s *Service) Search(ctx context.Context, query string) ([]house.House, error) {
	query = strings.TrimSpace(query)
	if len([]rune(query)) < 2 {
		return nil, fmt.Errorf("%w: query must be at least 2 characters", app.ErrInvalidInput)
	}
	return s.store.Houses().Search(ctx, query)
}

func (s *Service) Nearest(ctx context.Context, lat, lon float64) ([]house.House, error) {
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return nil, fmt.Errorf("%w: coordinates out of range", app.ErrInvalidInput)
	}
	return s.store.Houses().Nearest(ctx, lat, lon, 5)
}

func (s *Service) Get(ctx context.Context, id string) (Details, error) {
	r := s.store.Houses()
	h, err := r.Get(ctx, id)
	if err != nil {
		return Details{}, err
	}
	d := Details{House: h}
	if d.Organization, err = r.Organization(ctx, h.OrganizationID); err != nil {
		return Details{}, err
	}
	if d.Entrances, err = r.Entrances(ctx, id); err != nil {
		return Details{}, err
	}
	if d.Objects, err = r.Objects(ctx, id); err != nil {
		return Details{}, err
	}
	return d, nil
}

// ByQRCode — вход по QR-коду на объекте: объект и его дом.
func (s *Service) ByQRCode(ctx context.Context, code string) (house.AssetObject, house.House, error) {
	obj, err := s.store.Houses().ObjectByCode(ctx, code)
	if err != nil {
		return house.AssetObject{}, house.House{}, err
	}
	h, err := s.store.Houses().Get(ctx, obj.HouseID)
	return obj, h, err
}
