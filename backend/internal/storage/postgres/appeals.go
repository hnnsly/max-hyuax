package postgres

import (
	"context"

	"dommax/internal/app"
	"dommax/internal/storage/postgres/sqlcdb"
)

type appealRepo struct{ q *sqlcdb.Queries }

func (r appealRepo) Sign(ctx context.Context, s app.Signature) error {
	return r.q.SignAppeal(ctx, sqlcdb.SignAppealParams(s))
}

func (r appealRepo) Withdraw(ctx context.Context, issueID string, userID int64) error {
	return r.q.WithdrawAppeal(ctx, sqlcdb.WithdrawAppealParams{IssueID: issueID, UserID: userID})
}

func (r appealRepo) Signatures(ctx context.Context, issueID string) ([]app.Signature, error) {
	rows, err := r.q.ListAppealSignatures(ctx, issueID)
	return mapSlice(rows, func(s sqlcdb.ListAppealSignaturesRow) app.Signature { return app.Signature(s) }), err
}

func (r appealRepo) ForgetUser(ctx context.Context, userID int64) error {
	return r.q.ForgetAppealSignatures(ctx, userID)
}
