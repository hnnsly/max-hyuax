package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"dommax/internal/app"
	"dommax/internal/domain/council"
	"dommax/internal/storage/postgres/sqlcdb"
)

type councilRepo struct{ q *sqlcdb.Queries }

func (r councilRepo) AddProposal(ctx context.Context, p council.Proposal) error {
	return r.q.InsertProposal(ctx, sqlcdb.InsertProposalParams{
		ID: p.ID, HouseID: p.HouseID, AuthorID: p.AuthorID, Text: p.Text,
		Status: string(p.Status), Answer: p.Answer, CreatedAt: p.CreatedAt,
	})
}

func (r councilRepo) GetProposal(ctx context.Context, id string) (council.Proposal, error) {
	row, err := r.q.GetProposal(ctx, id)
	return toProposal(row), notFound(err)
}

func (r councilRepo) ReplyProposal(ctx context.Context, p council.Proposal) (bool, error) {
	n, err := r.q.ReplyProposal(ctx, sqlcdb.ReplyProposalParams{
		ID: p.ID, Status: string(p.Status), Answer: p.Answer, AnsweredAt: timePtr(p.AnsweredAt),
	})
	return n == 1, err
}

func (r councilRepo) HouseProposals(ctx context.Context, houseID string, limit int) ([]council.Proposal, error) {
	rows, err := r.q.ListHouseProposals(ctx, sqlcdb.ListHouseProposalsParams{HouseID: houseID, MaxRows: int32(limit)})
	return mapSlice(rows, toProposal), err
}

func (r councilRepo) AuthorProposals(ctx context.Context, authorID int64, limit int) ([]council.Proposal, error) {
	rows, err := r.q.ListAuthorProposals(ctx, sqlcdb.ListAuthorProposalsParams{AuthorID: authorID, MaxRows: int32(limit)})
	return mapSlice(rows, toProposal), err
}

func (r councilRepo) AddPoll(ctx context.Context, p council.Poll) error {
	var proposalID *string
	if p.ProposalID != "" {
		proposalID = new(p.ProposalID)
	}
	err := r.q.InsertPoll(ctx, sqlcdb.InsertPollParams{
		ID: p.ID, HouseID: p.HouseID, ProposalID: proposalID, Question: p.Question,
		Options: p.Options, CreatedAt: p.CreatedAt, ClosesAt: p.ClosesAt,
	})
	// Уникальный индекс polls_proposal_idx: по предложению уже открыт опрос.
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" {
		return council.ErrPollExists
	}
	return err
}

func (r councilRepo) GetPoll(ctx context.Context, id string) (council.Poll, error) {
	row, err := r.q.GetPoll(ctx, id)
	return toPoll(row), notFound(err)
}

func (r councilRepo) HousePolls(ctx context.Context, houseID string, limit int) ([]council.Poll, error) {
	rows, err := r.q.ListHousePolls(ctx, sqlcdb.ListHousePollsParams{HouseID: houseID, MaxRows: int32(limit)})
	return mapSlice(rows, toPoll), err
}

func (r councilRepo) Vote(ctx context.Context, pollID string, userID int64, option int) (bool, error) {
	n, err := r.q.InsertVote(ctx, sqlcdb.InsertVoteParams{PollID: pollID, UserID: userID, Option: int32(option)})
	return n == 1, err
}

func (r councilRepo) Tally(ctx context.Context, p council.Poll, userID int64) (app.Tally, error) {
	rows, err := r.q.PollTally(ctx, sqlcdb.PollTallyParams{PollID: p.ID, UserID: userID})
	t := app.Tally{Votes: make([]int, len(p.Options)), Mine: -1}
	for _, row := range rows {
		if int(row.Option) < len(t.Votes) {
			t.Votes[row.Option] = int(row.Votes)
		}
		if row.Mine {
			t.Mine = int(row.Option)
		}
	}
	return t, err
}

func toProposal(p sqlcdb.Proposal) council.Proposal {
	return council.Proposal{
		ID: p.ID, HouseID: p.HouseID, AuthorID: p.AuthorID, Text: p.Text, Status: council.Status(p.Status),
		Answer: p.Answer, CreatedAt: p.CreatedAt, AnsweredAt: timeOrZero(p.AnsweredAt),
	}
}

func toPoll(p sqlcdb.Poll) council.Poll {
	var proposalID string
	if p.ProposalID != nil {
		proposalID = *p.ProposalID
	}
	return council.Poll{
		ID: p.ID, HouseID: p.HouseID, ProposalID: proposalID, Question: p.Question,
		Options: p.Options, CreatedAt: p.CreatedAt, ClosesAt: p.ClosesAt,
	}
}
