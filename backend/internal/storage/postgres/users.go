package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"dommax/internal/domain/user"
	"dommax/internal/storage/postgres/sqlcdb"
)

type userRepo struct{ q *sqlcdb.Queries }

func (r userRepo) Get(ctx context.Context, id int64) (user.User, error) {
	row, err := r.q.GetUser(ctx, id)
	return toUser(row), notFound(err)
}

func (r userRepo) ByMaxID(ctx context.Context, maxUserID int64) (user.User, error) {
	row, err := r.q.GetUserByMaxID(ctx, pgtype.Int8{Int64: maxUserID, Valid: true})
	return toUser(row), notFound(err)
}

func (r userRepo) ByDemoKey(ctx context.Context, key string) (user.User, error) {
	row, err := r.q.GetUserByDemoKey(ctx, pgtype.Text{String: key, Valid: true})
	return toUser(row), notFound(err)
}

func (r userRepo) Create(ctx context.Context, u user.User) (user.User, error) {
	row, err := r.q.InsertUser(ctx, sqlcdb.InsertUserParams{
		MaxUserID: pgtype.Int8{Int64: u.MaxUserID, Valid: u.MaxUserID != 0},
		FirstName: u.FirstName,
		Role:      string(u.Role),
	})
	return toUser(row), err
}

func (r userRepo) Save(ctx context.Context, u user.User) error {
	return r.q.UpdateUser(ctx, sqlcdb.UpdateUserParams{
		ID:             u.ID,
		MaxUserID:      pgtype.Int8{Int64: u.MaxUserID, Valid: u.MaxUserID != 0},
		FirstName:      u.FirstName,
		Phone:          u.Phone,
		HouseID:        u.HouseID,
		ConsentVersion: u.ConsentVersion,
		ConsentAt:      timePtr(u.ConsentAt),
		DeletedAt:      timePtr(u.DeletedAt),
	})
}

func toUser(r sqlcdb.User) user.User {
	return user.User{
		ID:             r.ID,
		MaxUserID:      r.MaxUserID.Int64,
		FirstName:      r.FirstName,
		Phone:          r.Phone,
		HouseID:        r.HouseID.String,
		Role:           user.Role(r.Role),
		OrganizationID: r.OrganizationID.String,
		District:       r.District,
		ConsentVersion: r.ConsentVersion,
		ConsentAt:      timeOrZero(r.ConsentAt),
		DeletedAt:      timeOrZero(r.DeletedAt),
	}
}

// timePtr и timeOrZero переводят нулевое время домена в NULL базы и обратно.
func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return new(t)
}

func timeOrZero(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
