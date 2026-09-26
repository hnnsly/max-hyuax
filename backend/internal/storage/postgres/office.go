package postgres

import (
	"context"
	"time"

	"dommax/internal/app"
	"dommax/internal/storage/postgres/sqlcdb"
)

type officeRepo struct{ q *sqlcdb.Queries }

func (r officeRepo) AddMaintenance(ctx context.Context, m app.MaintenanceAlert) error {
	return r.q.InsertMaintenanceAlert(ctx, sqlcdb.InsertMaintenanceAlertParams{
		ID:          m.ID,
		HouseID:     m.HouseID,
		Category:    m.Category,
		Title:       m.Title,
		Description: m.Description,
		StartsAt:    m.StartsAt,
		EndsAt:      m.EndsAt,
		CreatedBy:   m.CreatedBy,
		CreatedAt:   m.CreatedAt,
	})
}

func (r officeRepo) ActiveMaintenanceByHouse(ctx context.Context, houseID string, now time.Time) ([]app.MaintenanceAlert, error) {
	rows, err := r.q.ListActiveMaintenanceByHouse(ctx, sqlcdb.ListActiveMaintenanceByHouseParams{
		HouseID: houseID,
		Now:     now,
	})
	return mapSlice(rows, func(row sqlcdb.ListActiveMaintenanceByHouseRow) app.MaintenanceAlert {
		return app.MaintenanceAlert{
			ID:          row.ID,
			HouseID:     row.HouseID,
			Category:    row.Category,
			Title:       row.Title,
			Description: row.Description,
			StartsAt:    row.StartsAt,
			EndsAt:      row.EndsAt,
			CreatedBy:   row.CreatedBy,
			CreatedAt:   row.CreatedAt,
		}
	}), err
}

func (r officeRepo) ListMaintenanceByOrg(ctx context.Context, orgID string, limit int) ([]app.MaintenanceAlert, error) {
	rows, err := r.q.ListMaintenanceByOrg(ctx, sqlcdb.ListMaintenanceByOrgParams{
		OrgID:   orgID,
		MaxRows: int32(limit),
	})
	return mapSlice(rows, func(row sqlcdb.ListMaintenanceByOrgRow) app.MaintenanceAlert {
		return app.MaintenanceAlert{
			ID:          row.ID,
			HouseID:     row.HouseID,
			Address:     row.Address,
			Category:    row.Category,
			Title:       row.Title,
			Description: row.Description,
			StartsAt:    row.StartsAt,
			EndsAt:      row.EndsAt,
			CreatedBy:   row.CreatedBy,
			CreatedAt:   row.CreatedAt,
		}
	}), err
}

func (r officeRepo) DeleteMaintenance(ctx context.Context, id string) error {
	return r.q.DeleteMaintenanceAlert(ctx, id)
}

func (r officeRepo) AddAppointment(ctx context.Context, a app.Appointment) error {
	return r.q.InsertAppointment(ctx, sqlcdb.InsertAppointmentParams{
		ID:         a.ID,
		HouseID:    a.HouseID,
		UserID:     a.UserID,
		Specialist: a.Specialist,
		Topic:      a.Topic,
		SlotAt:     a.SlotAt,
		Status:     a.Status,
		CreatedAt:  a.CreatedAt,
	})
}

func (r officeRepo) GetAppointment(ctx context.Context, id string) (app.Appointment, error) {
	row, err := r.q.GetAppointment(ctx, id)
	if err != nil {
		return app.Appointment{}, notFound(err)
	}
	return app.Appointment{
		ID:         row.ID,
		HouseID:    row.HouseID,
		Address:    row.Address,
		UserID:     row.UserID,
		UserName:   row.UserName,
		Specialist: row.Specialist,
		Topic:      row.Topic,
		SlotAt:     row.SlotAt,
		Status:     row.Status,
		CreatedAt:  row.CreatedAt,
	}, nil
}

func (r officeRepo) CancelAppointment(ctx context.Context, id string) error {
	return r.q.CancelAppointment(ctx, id)
}

func (r officeRepo) ListUserAppointments(ctx context.Context, userID int64, limit int) ([]app.Appointment, error) {
	rows, err := r.q.ListUserAppointments(ctx, sqlcdb.ListUserAppointmentsParams{
		UserID:  userID,
		MaxRows: int32(limit),
	})
	return mapSlice(rows, func(row sqlcdb.ListUserAppointmentsRow) app.Appointment {
		return app.Appointment{
			ID:         row.ID,
			HouseID:    row.HouseID,
			Address:    row.Address,
			UserID:     row.UserID,
			UserName:   row.UserName,
			Specialist: row.Specialist,
			Topic:      row.Topic,
			SlotAt:     row.SlotAt,
			Status:     row.Status,
			CreatedAt:  row.CreatedAt,
		}
	}), err
}

func (r officeRepo) ListOrgAppointments(ctx context.Context, orgID string, limit int) ([]app.Appointment, error) {
	rows, err := r.q.ListOrgAppointments(ctx, sqlcdb.ListOrgAppointmentsParams{
		OrgID:   orgID,
		MaxRows: int32(limit),
	})
	return mapSlice(rows, func(row sqlcdb.ListOrgAppointmentsRow) app.Appointment {
		return app.Appointment{
			ID:         row.ID,
			HouseID:    row.HouseID,
			Address:    row.Address,
			UserID:     row.UserID,
			UserName:   row.UserName,
			Specialist: row.Specialist,
			Topic:      row.Topic,
			SlotAt:     row.SlotAt,
			Status:     row.Status,
			CreatedAt:  row.CreatedAt,
		}
	}), err
}
