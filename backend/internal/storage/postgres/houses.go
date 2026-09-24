package postgres

import (
	"context"

	"dommax/internal/domain/house"
	"dommax/internal/storage/postgres/sqlcdb"
)

type houseRepo struct{ q *sqlcdb.Queries }

func (r houseRepo) Search(ctx context.Context, query string) ([]house.House, error) {
	rows, err := r.q.SearchHouses(ctx, query)
	return mapSlice(rows, toHouse), err
}

func (r houseRepo) Nearest(ctx context.Context, lat, lon float64, limit int) ([]house.House, error) {
	rows, err := r.q.NearestHouses(ctx, sqlcdb.NearestHousesParams{Lat: lat, Lon: lon, MaxRows: int32(limit)})
	return mapSlice(rows, toHouse), err
}

func (r houseRepo) ByOrganization(ctx context.Context, orgID string) ([]house.House, error) {
	rows, err := r.q.ListOrganizationHouses(ctx, orgID)
	return mapSlice(rows, toHouse), err
}

func (r houseRepo) Get(ctx context.Context, id string) (house.House, error) {
	row, err := r.q.GetHouse(ctx, id)
	return toHouse(row), notFound(err)
}

func (r houseRepo) Organization(ctx context.Context, id string) (house.Organization, error) {
	o, err := r.q.GetOrganization(ctx, id)
	return house.Organization{
		ID:              o.ID,
		Type:            house.OrgType(o.Type),
		Name:            o.Name,
		PhoneOffice:     o.PhoneOffice,
		PhoneDispatcher: o.PhoneDispatcher,
		PhoneEmergency:  o.PhoneEmergency,
		Schedule:        o.Schedule,
	}, notFound(err)
}

func (r houseRepo) Entrances(ctx context.Context, houseID string) ([]house.Entrance, error) {
	rows, err := r.q.ListEntrances(ctx, houseID)
	return mapSlice(rows, func(e sqlcdb.Entrance) house.Entrance {
		return house.Entrance{ID: e.ID, HouseID: e.HouseID, Number: int(e.Number)}
	}), err
}

func (r houseRepo) Objects(ctx context.Context, houseID string) ([]house.AssetObject, error) {
	rows, err := r.q.ListHouseObjects(ctx, houseID)
	return mapSlice(rows, func(o sqlcdb.ListHouseObjectsRow) house.AssetObject {
		return toObject(sqlcdb.GetObjectByCodeRow(o))
	}), err
}

func (r houseRepo) ObjectByCode(ctx context.Context, code string) (house.AssetObject, error) {
	row, err := r.q.GetObjectByCode(ctx, code)
	return toObject(row), notFound(err)
}

func toHouse(h sqlcdb.House) house.House {
	return house.House{
		ID:             h.ID,
		Address:        h.Address,
		District:       h.District,
		YearBuilt:      int(h.YearBuilt),
		Floors:         int(h.Floors),
		EntrancesCount: int(h.EntrancesCount),
		OrganizationID: h.OrganizationID,
		Lat:            h.Lat,
		Lon:            h.Lon,
		Source:         h.Source,
	}
}

func toObject(o sqlcdb.GetObjectByCodeRow) house.AssetObject {
	return house.AssetObject{ID: o.ID, HouseID: o.HouseID, EntranceID: o.EntranceID, Category: o.Category, Label: o.Label, QRCode: o.QrCode}
}

func mapSlice[S, D any](in []S, f func(S) D) []D {
	out := make([]D, len(in))
	for i, v := range in {
		out[i] = f(v)
	}
	return out
}
