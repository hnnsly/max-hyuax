// Пакет houses — справочник домов: поиск, ближайшие, карточка дома, вход по QR, импорт реестра.
package houses

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"dommax/internal/app"
	"dommax/internal/domain/house"
	"dommax/internal/domain/user"
)

// LocateTimeout — сколько ждать ответ геокодера: публичный геокодер бывает медленным, экран не должен висеть.
const LocateTimeout = 6 * time.Second

// MaxNearRadiusMeters — радиус (300 м), в пределах которого дом из БД считается стоящим рядом с жителем.
const MaxNearRadiusMeters = 300.0

// ErrOutsideMoscow возвращается, когда точка геолокации находится за пределами Москвы.
var ErrOutsideMoscow = errors.New("houses: location is outside Moscow")

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
	list, err := s.store.Houses().Search(ctx, query)
	if err != nil {
		return nil, err
	}
	// OpenStreetMap спрашиваем, если в локальной базе мало результатов.
	if len(list) < 5 && s.geo != nil && utf8.RuneCountInString(query) >= 3 {
		gctx, cancel := context.WithTimeout(ctx, LocateTimeout)
		osmHouses, gerr := s.geo.SearchHouses(gctx, query)
		cancel()
		if gerr == nil && len(osmHouses) > 0 {
			// Дом из базы с тем же адресом (например, модельный) не дублируем.
			seen := make(map[string]bool, len(list))
			for _, h := range list {
				seen[addressKey(h.Address)] = true
			}
			for _, gh := range osmHouses {
				key := addressKey(gh.Address)
				if !gh.InMoscow || gh.Address == "" || seen[key] {
					continue
				}
				if prov, err := s.provisionGeoHouse(ctx, gh); err == nil {
					list = append(list, prov)
					seen[key] = true
				}
			}
		}
	}
	return list, nil
}

func (s *Service) Nearest(ctx context.Context, lat, lon float64) ([]house.House, error) {
	if err := checkCoords(lat, lon); err != nil {
		return nil, err
	}
	near, err := s.store.Houses().Nearest(ctx, lat, lon, 5)
	if err != nil {
		return nil, err
	}
	// Если геокодер выключен (например, в локальных юнит-тестах) — отдаём ближайшие из БД как есть.
	if s.geo == nil {
		return near, nil
	}

	// Оставляем только дома в реальном радиусе 300 метров от жителя.
	var closeHouses []house.House
	for _, h := range near {
		if h.Lat != 0 && h.Lon != 0 && distanceMeters(lat, lon, h.Lat, h.Lon) <= MaxNearRadiusMeters {
			closeHouses = append(closeHouses, h)
		}
	}
	if len(closeHouses) > 0 {
		return closeHouses, nil
	}

	// Дома рядом в БД ещё нет: определяем дом по координатам через OpenStreetMap.
	gctx, cancel := context.WithTimeout(ctx, LocateTimeout)
	gh, ok, gerr := s.geo.ReverseHouse(gctx, lat, lon)
	cancel()

	if gerr == nil && ok {
		if !gh.InMoscow {
			return nil, ErrOutsideMoscow
		}
		if gh.Address != "" {
			if prov, perr := s.provisionGeoHouse(ctx, gh); perr == nil {
				return []house.House{prov}, nil
			}
		}
	}

	if !isMoscowBBox(lat, lon) {
		return nil, ErrOutsideMoscow
	}
	return closeHouses, nil
}

// addressKey — адрес без различий в регистре и пробелах: по нему сравниваются дома из базы и из OSM.
func addressKey(a string) string { return strings.ToLower(normalizeSpaces(a)) }

// provisionGeoHouse сохраняет дом, найденный в OpenStreetMap. УК определяется по району
// (ГБУ «Жилищник»), это приближение: интерфейс помечает такие дома (source = osm).
func (s *Service) provisionGeoHouse(ctx context.Context, gh app.GeoHouse) (house.House, error) {
	dist := normalizeSpaces(gh.District)
	orgID := "org-gbu-" + translitSlug(dist)
	orgName := fmt.Sprintf("ГБУ «Жилищник района %s»", dist)
	if dist == "" {
		// Район не определился: отвечает городская диспетчерская, а не случайный «Жилищник».
		orgID, orgName = "org-msk-edc", "Единый диспетчерский центр Москвы"
	}

	org := house.Organization{
		ID:              orgID,
		Type:            house.OrgManagementCompany,
		Name:            orgName,
		PhoneOffice:     "+7 495 539-53-53",
		PhoneDispatcher: "+7 495 539-53-53",
		PhoneEmergency:  "+7 495 539-53-53",
		Schedule:        "круглосуточно",
	}

	addr := normalizeSpaces(gh.Address)
	h := house.House{
		ID:       ImportID(addr),
		Address:  addr,
		District: dist,
		// Год и этажность в OSM обычно не указаны: 0 значит «неизвестно», интерфейс их не показывает.
		// Четыре подъезда условны: они нужны, чтобы у дома были объекты с QR-кодами.
		EntrancesCount: 4,
		OrganizationID: orgID,
		Lat:            gh.Lat,
		Lon:            gh.Lon,
		Source:         "osm",
	}

	err := s.store.InTx(ctx, func(tx app.Store) error {
		if err := tx.Houses().UpsertOrganization(ctx, org); err != nil {
			return err
		}
		_, err := tx.Houses().Upsert(ctx, h)
		return err
	})
	if err != nil {
		return house.House{}, err
	}
	return h, nil
}

func isMoscowBBox(lat, lon float64) bool {
	return lat >= 54.20 && lat <= 56.95 && lon >= 35.10 && lon <= 40.25
}

func distanceMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000.0 // радиус Земли в метрах
	dLat := (lat2 - lat1) * math.Pi / 180.0
	dLon := (lon2 - lon1) * math.Pi / 180.0
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180.0)*math.Cos(lat2*math.Pi/180.0)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func translitSlug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch r {
		case 'а':
			b.WriteString("a")
		case 'б':
			b.WriteString("b")
		case 'в':
			b.WriteString("v")
		case 'г':
			b.WriteString("g")
		case 'д':
			b.WriteString("d")
		case 'е', 'ё':
			b.WriteString("e")
		case 'ж':
			b.WriteString("zh")
		case 'з':
			b.WriteString("z")
		case 'и', 'й':
			b.WriteString("i")
		case 'к':
			b.WriteString("k")
		case 'л':
			b.WriteString("l")
		case 'м':
			b.WriteString("m")
		case 'н':
			b.WriteString("n")
		case 'о':
			b.WriteString("o")
		case 'п':
			b.WriteString("p")
		case 'р':
			b.WriteString("r")
		case 'с':
			b.WriteString("s")
		case 'т':
			b.WriteString("t")
		case 'у':
			b.WriteString("u")
		case 'ф':
			b.WriteString("f")
		case 'х':
			b.WriteString("kh")
		case 'ц':
			b.WriteString("ts")
		case 'ч':
			b.WriteString("ch")
		case 'ш':
			b.WriteString("sh")
		case 'щ':
			b.WriteString("shch")
		case 'ы':
			b.WriteString("y")
		case 'э':
			b.WriteString("e")
		case 'ю':
			b.WriteString("yu")
		case 'я':
			b.WriteString("ya")
		case ' ', '-':
			b.WriteString("-")
		default:
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				b.WriteRune(r)
			}
		}
	}
	res := strings.Trim(b.String(), "-")
	if res == "" {
		return "msk"
	}
	if len(res) > 28 {
		res = res[:28]
	}
	return res
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
