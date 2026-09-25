// Пакет houses — справочник домов: поиск, ближайшие, карточка дома, вход по QR, импорт реестра.
package houses

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"dommax/internal/app"
	"dommax/internal/domain/house"
	"dommax/internal/domain/user"
)

// LocateTimeout — сколько ждать обратный адрес: публичный геокодер бывает медленным, экран не должен висеть.
const LocateTimeout = 3 * time.Second

type Service struct {
	store app.Store
	geo   app.Geocoder // nil — геокодер выключен
}

func NewService(store app.Store, geo app.Geocoder) *Service { return &Service{store: store, geo: geo} }

// Details — дом со всем, что нужно экрану заявки: УК, подъезды, объекты с QR.
type Details struct {
	House        house.House
	Organization house.Organization
	Entrances    []house.Entrance
	Objects      []house.AssetObject
}

func (s *Service) Search(ctx context.Context, query string) ([]house.House, error) {
	query = strings.TrimSpace(query)
	if !utf8.ValidString(query) || utf8.RuneCountInString(query) < 2 {
		return nil, fmt.Errorf("%w: query must be valid UTF-8, at least 2 characters", app.ErrInvalidInput)
	}
	return s.store.Houses().Search(ctx, query)
}

func (s *Service) Nearest(ctx context.Context, lat, lon float64) ([]house.House, error) {
	if err := checkCoords(lat, lon); err != nil {
		return nil, err
	}
	return s.store.Houses().Nearest(ctx, lat, lon, 5)
}

// Locate — адрес, где стоит житель, для «Найти дома рядом» (ADR-016).
// Пустая строка без ошибки — геокодер выключен или адрес не найден.
func (s *Service) Locate(ctx context.Context, lat, lon float64) (string, error) {
	if err := checkCoords(lat, lon); err != nil {
		return "", err
	}
	if s.geo == nil {
		return "", nil
	}
	ctx, cancel := context.WithTimeout(ctx, LocateTimeout)
	defer cancel()
	addr, ok, err := s.geo.Reverse(ctx, lat, lon)
	if err != nil || !ok {
		return "", err
	}
	return addr, nil
}

func checkCoords(lat, lon float64) error {
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return fmt.Errorf("%w: coordinates out of range", app.ErrInvalidInput)
	}
	return nil
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

// ForOperator — дома УК оператора по адресу: для наклеек с QR-кодами.
func (s *Service) ForOperator(ctx context.Context, u user.User) ([]house.House, error) {
	if u.Role != user.RoleOperator || u.OrganizationID == "" {
		return nil, app.ErrForbidden
	}
	return s.store.Houses().ByOrganization(ctx, u.OrganizationID)
}
