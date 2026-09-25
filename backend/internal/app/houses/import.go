package houses

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"dommax/internal/app"
	"dommax/internal/domain/house"
)

// ImportRow — строка реестра домов; Line — номер строки файла для отчёта.
type ImportRow struct {
	Line  int
	House house.House
}

// RowError — строка, которую не импортировали, и причина для того, кто загружает реестр.
type RowError struct {
	Line   int
	Reason string
}

// ImportReport — итог импорта: сколько домов создано и обновлено, сколько осталось без координат.
type ImportReport struct {
	Created, Updated int
	Geocoded         int // координаты нашёл геокодер
	WithoutCoords    int // координат нет: дом не попадёт в «Найти дома рядом»
	Errors           []RowError
}

// Importer загружает реестр домов (ADR-016): дом с подъездами и объектами с QR-кодами.
type Importer struct {
	store app.Store
	geo   app.Geocoder // nil — геокодер выключен
}

func NewImporter(store app.Store, geo app.Geocoder) *Importer {
	return &Importer{store: store, geo: geo}
}

var (
	// id дома попадает в id объектов и в QR-диплинк, поэтому только безопасные символы.
	houseIDRe = regexp.MustCompile(`^[a-z0-9-]{1,40}$`)
	sourceRe  = regexp.MustCompile(`^[a-z_]{1,20}$`)
)

// ImportID — id дома из адреса: повторный импорт того же адреса обновляет тот же дом.
func ImportID(address string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(normalizeSpaces(address))))
	return "h-" + hex.EncodeToString(sum[:5])
}

func normalizeSpaces(s string) string { return strings.Join(strings.Fields(s), " ") }

// Import сохраняет строки по одной: плохая строка попадает в отчёт и не мешает остальным.
// Ошибка возвращается, только если импорт нельзя продолжать (база, отмена контекста).
func (im *Importer) Import(ctx context.Context, rows []ImportRow) (ImportReport, error) {
	var rep ImportReport
	seen := map[string]int{} // id дома → строка, где он уже был
	for _, row := range rows {
		h, reason := im.prepare(ctx, row.House)
		if reason == "" {
			if first, dup := seen[h.ID]; dup {
				reason = fmt.Sprintf("дом уже есть в строке %d", first)
			}
		}
		if reason != "" {
			rep.Errors = append(rep.Errors, RowError{Line: row.Line, Reason: reason})
			continue
		}
		seen[h.ID] = row.Line

		geocoded, err := im.locate(ctx, &h)
		if err != nil {
			return rep, err
		}
		var created bool
		err = im.store.InTx(ctx, func(tx app.Store) (err error) {
			created, err = tx.Houses().Upsert(ctx, h)
			return err
		})
		if err != nil {
			return rep, fmt.Errorf("line %d: %w", row.Line, err)
		}
		if created {
			rep.Created++
		} else {
			rep.Updated++
		}
		switch {
		case geocoded:
			rep.Geocoded++
		case h.Lat == 0 && h.Lon == 0:
			rep.WithoutCoords++
		}
	}
	return rep, nil
}

// prepare нормализует строку и проверяет её; непустая причина — строку не импортировать.
func (im *Importer) prepare(ctx context.Context, h house.House) (house.House, string) {
	h.Address = normalizeSpaces(h.Address)
	h.District = normalizeSpaces(h.District)
	h.OrganizationID = strings.TrimSpace(h.OrganizationID)
	h.Source = strings.TrimSpace(h.Source)
	if h.Source == "" {
		h.Source = "import"
	}
	if h.ID = strings.TrimSpace(h.ID); h.ID == "" && h.Address != "" {
		h.ID = ImportID(h.Address)
	}
	switch n := utf8.RuneCountInString(h.Address); {
	case n < 3 || n > 200:
		return h, "нет адреса или он длиннее 200 символов"
	case h.District == "":
		return h, "не указан район"
	case !houseIDRe.MatchString(h.ID):
		return h, "id дома: только латиница в нижнем регистре, цифры и дефис, до 40 символов"
	case !sourceRe.MatchString(h.Source):
		return h, "источник данных: только латиница в нижнем регистре и _"
	case h.Floors < 1 || h.Floors > 100:
		return h, "этажность от 1 до 100"
	case h.EntrancesCount < 1 || h.EntrancesCount > 30:
		return h, "подъездов от 1 до 30"
	case h.YearBuilt != 0 && (h.YearBuilt < 1700 || h.YearBuilt > time.Now().Year()):
		return h, "год постройки от 1700 до текущего"
	case h.Lat < -90 || h.Lat > 90 || h.Lon < -180 || h.Lon > 180:
		return h, "координаты вне диапазона"
	}
	if _, err := im.store.Houses().Organization(ctx, h.OrganizationID); err != nil {
		return h, fmt.Sprintf("УК %q не найдена", h.OrganizationID)
	}
	return h, ""
}

// locate дополняет координаты, если их нет в строке: берёт найденные при прошлом импорте
// или спрашивает геокодер. Сбой геокодера не мешает импорту, дом останется без координат.
func (im *Importer) locate(ctx context.Context, h *house.House) (geocoded bool, err error) {
	if h.Lat != 0 || h.Lon != 0 {
		return false, nil
	}
	if old, err := im.store.Houses().Get(ctx, h.ID); err == nil && (old.Lat != 0 || old.Lon != 0) {
		h.Lat, h.Lon = old.Lat, old.Lon
		return false, nil
	}
	if im.geo == nil {
		return false, nil
	}
	lat, lon, ok, err := im.geo.Geocode(ctx, h.Address)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return false, ctxErr
	}
	if err != nil || !ok {
		return false, nil
	}
	h.Lat, h.Lon = lat, lon
	return true, nil
}
