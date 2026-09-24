package postgres

import (
	"context"

	"dommax/internal/app"
	"dommax/internal/storage/postgres/sqlcdb"
)

type photoRepo struct{ q *sqlcdb.Queries }

func (r photoRepo) Add(ctx context.Context, p app.Photo) error {
	return r.q.InsertPhoto(ctx, sqlcdb.InsertPhotoParams{
		ID: p.ID, IssueID: p.IssueID, UploadedBy: p.UploadedBy,
		Width: int32(p.Width), Height: int32(p.Height), SizeBytes: int32(p.SizeBytes), CreatedAt: p.CreatedAt,
	})
}

func (r photoRepo) ListByIssue(ctx context.Context, issueID string) ([]app.Photo, error) {
	rows, err := r.q.ListIssuePhotos(ctx, issueID)
	return mapSlice(rows, toPhoto), err
}

func (r photoRepo) Get(ctx context.Context, id string) (app.Photo, error) {
	row, err := r.q.GetPhoto(ctx, id)
	if err != nil {
		return app.Photo{}, notFound(err)
	}
	return toPhoto(row), nil
}

func (r photoRepo) ListByUploader(ctx context.Context, userID int64) ([]app.Photo, error) {
	rows, err := r.q.ListUploaderPhotos(ctx, userID)
	return mapSlice(rows, toPhoto), err
}

func (r photoRepo) Delete(ctx context.Context, id string) error {
	return r.q.DeletePhoto(ctx, id)
}

func toPhoto(p sqlcdb.IssuePhoto) app.Photo {
	return app.Photo{
		ID: p.ID, IssueID: p.IssueID, UploadedBy: p.UploadedBy,
		Width: int(p.Width), Height: int(p.Height), SizeBytes: int(p.SizeBytes), CreatedAt: p.CreatedAt,
	}
}
