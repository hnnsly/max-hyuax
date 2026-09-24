package postgres

import (
	"context"
	"time"

	"dommax/internal/app"
	"dommax/internal/domain/issue"
	"dommax/internal/storage/postgres/sqlcdb"
)

type issueRepo struct{ s *Store }

func (r issueRepo) Create(ctx context.Context, is *issue.Issue) error {
	return r.s.InTx(ctx, func(tx app.Store) error {
		q := tx.(*Store).q
		num, err := q.InsertIssue(ctx, sqlcdb.InsertIssueParams{
			ID:               is.ID(),
			HouseID:          is.HouseID(),
			ObjectID:         is.ObjectID(),
			Category:         is.Category(),
			Title:            is.Title(),
			Description:      is.Description(),
			ResponsibleOrgID: is.ResponsibleOrgID(),
			Status:           string(is.Status()),
			StatusAt:         is.StatusAt(),
			StatusComment:    is.StatusComment(),
			CreatedBy:        is.ReporterID(),
			CreatedAt:        is.CreatedAt(),
			DeadlineAt:       is.Deadline(),
		})
		if err != nil {
			return err
		}
		is.SetNumber(num)
		return saveChildren(ctx, q, is)
	})
}

func (r issueRepo) Save(ctx context.Context, is *issue.Issue) error {
	return r.s.InTx(ctx, func(tx app.Store) error {
		q := tx.(*Store).q
		err := q.UpdateIssueState(ctx, sqlcdb.UpdateIssueStateParams{
			ID:            is.ID(),
			Status:        string(is.Status()),
			StatusAt:      is.StatusAt(),
			StatusComment: is.StatusComment(),
			OverdueAt:     timePtr(is.OverdueAt()),
			DeadlineAt:    is.Deadline(),
			ReopenedAt:    timePtr(is.ReopenedAt()),
		})
		if err != nil {
			return err
		}
		return saveChildren(ctx, q, is)
	})
}

// saveChildren дописывает новых участников и ответы на «выполнено» (существующие пропускаются)
// и события агрегата.
func saveChildren(ctx context.Context, q *sqlcdb.Queries, is *issue.Issue) error {
	for _, p := range is.Participants() {
		err := q.InsertParticipant(ctx, sqlcdb.InsertParticipantParams{IssueID: is.ID(), UserID: p.UserID, JoinedAt: p.JoinedAt})
		if err != nil {
			return err
		}
	}
	for _, a := range is.NewAnswers() {
		err := q.InsertAnswer(ctx, sqlcdb.InsertAnswerParams{IssueID: is.ID(), UserID: a.UserID, DoneAt: a.DoneAt, Fixed: a.Fixed, At: a.At})
		if err != nil {
			return err
		}
	}
	for _, e := range is.PullEvents() {
		err := q.InsertEvent(ctx, sqlcdb.InsertEventParams{
			IssueID: e.IssueID, Kind: string(e.Kind), UserID: e.UserID, Status: string(e.Status), Comment: e.Comment, At: e.At,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (r issueRepo) Get(ctx context.Context, id string) (*issue.Issue, error) {
	row, err := r.s.q.GetIssue(ctx, id)
	if err != nil {
		return nil, notFound(err)
	}
	list, err := r.restore(ctx, []sqlcdb.GetIssueRow{row})
	if err != nil {
		return nil, err
	}
	return list[0], nil
}

func (r issueRepo) GetForUpdate(ctx context.Context, id string) (*issue.Issue, error) {
	if err := r.s.q.LockIssue(ctx, id); err != nil {
		return nil, notFound(err)
	}
	return r.Get(ctx, id)
}

func (r issueRepo) ListByHouse(ctx context.Context, houseID string, limit int) ([]*issue.Issue, error) {
	rows, err := r.s.q.ListHouseIssues(ctx, sqlcdb.ListHouseIssuesParams{HouseID: houseID, MaxRows: int32(limit)})
	if err != nil {
		return nil, err
	}
	return r.restore(ctx, convert(rows))
}

func (r issueRepo) FindSimilar(ctx context.Context, houseID, category, objectID string, since time.Time) ([]*issue.Issue, error) {
	rows, err := r.s.q.FindSimilarIssues(ctx, sqlcdb.FindSimilarIssuesParams{HouseID: houseID, Category: category, ObjectID: objectID, Since: since})
	if err != nil {
		return nil, err
	}
	return r.restore(ctx, convert(rows))
}

func (r issueRepo) Queue(ctx context.Context, orgID string, limit int) ([]*issue.Issue, error) {
	rows, err := r.s.q.ListOrgQueue(ctx, sqlcdb.ListOrgQueueParams{OrgID: orgID, MaxRows: int32(limit)})
	if err != nil {
		return nil, err
	}
	return r.restore(ctx, convert(rows))
}

func (r issueRepo) ListByParticipant(ctx context.Context, userID int64, limit int) ([]*issue.Issue, error) {
	rows, err := r.s.q.ListParticipantIssues(ctx, sqlcdb.ListParticipantIssuesParams{UserID: userID, MaxRows: int32(limit)})
	if err != nil {
		return nil, err
	}
	return r.restore(ctx, convert(rows))
}

func (r issueRepo) ListOverdueUnmarked(ctx context.Context, now time.Time, limit int) ([]*issue.Issue, error) {
	rows, err := r.s.q.ListOverdueUnmarked(ctx, sqlcdb.ListOverdueUnmarkedParams{Now: now, MaxRows: int32(limit)})
	if err != nil {
		return nil, err
	}
	return r.restore(ctx, convert(rows))
}

func (r issueRepo) OverdueInDistrict(ctx context.Context, district string, now time.Time, limit int) ([]*issue.Issue, error) {
	rows, err := r.s.q.ListDistrictOverdue(ctx, sqlcdb.ListDistrictOverdueParams{District: district, Now: now, MaxRows: int32(limit)})
	if err != nil {
		return nil, err
	}
	return r.restore(ctx, convert(rows))
}

func (r issueRepo) Events(ctx context.Context, issueID string) ([]issue.Event, error) {
	rows, err := r.s.q.ListIssueEvents(ctx, issueID)
	return mapSlice(rows, func(e sqlcdb.ListIssueEventsRow) issue.Event {
		return issue.Event{Kind: issue.EventKind(e.Kind), IssueID: issueID, UserID: e.UserID, Status: issue.Status(e.Status), Comment: e.Comment, At: e.At}
	}), err
}

// issueRow — строки разных запросов sqlc с одинаковым набором колонок.
type issueRow interface {
	sqlcdb.GetIssueRow | sqlcdb.ListHouseIssuesRow | sqlcdb.FindSimilarIssuesRow | sqlcdb.ListOrgQueueRow | sqlcdb.ListParticipantIssuesRow |
		sqlcdb.ListOverdueUnmarkedRow | sqlcdb.ListDistrictOverdueRow
}

func convert[T issueRow](rows []T) []sqlcdb.GetIssueRow {
	out := make([]sqlcdb.GetIssueRow, len(rows))
	for i, row := range rows {
		out[i] = sqlcdb.GetIssueRow(row)
	}
	return out
}

// restore собирает агрегаты: участники всех заявок читаются одним запросом.
func (r issueRepo) restore(ctx context.Context, rows []sqlcdb.GetIssueRow) ([]*issue.Issue, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	parts, err := r.s.q.ListParticipants(ctx, ids)
	if err != nil {
		return nil, err
	}
	byIssue := map[string][]issue.Participant{}
	for _, p := range parts {
		byIssue[p.IssueID] = append(byIssue[p.IssueID], issue.Participant{UserID: p.UserID, JoinedAt: p.JoinedAt})
	}
	answerRows, err := r.s.q.ListCurrentAnswers(ctx, ids)
	if err != nil {
		return nil, err
	}
	answers := map[string][]issue.Answer{}
	for _, a := range answerRows {
		answers[a.IssueID] = append(answers[a.IssueID], issue.Answer{UserID: a.UserID, Fixed: a.Fixed, DoneAt: a.DoneAt, At: a.At})
	}
	out := make([]*issue.Issue, len(rows))
	for i, row := range rows {
		out[i] = issue.Restore(issue.NewParams{
			ID:               row.ID,
			HouseID:          row.HouseID,
			ObjectID:         row.ObjectID,
			Category:         row.Category,
			Title:            row.Title,
			Description:      row.Description,
			ResponsibleOrgID: row.ResponsibleOrgID,
			CreatedAt:        row.CreatedAt,
			Deadline:         row.DeadlineAt,
		}, issue.State{
			Number:        row.Number,
			Status:        issue.Status(row.Status),
			StatusAt:      row.StatusAt,
			StatusComment: row.StatusComment,
			OverdueAt:     timeOrZero(row.OverdueAt),
			ReopenedAt:    timeOrZero(row.ReopenedAt),
			Participants:  byIssue[row.ID],
			Answers:       answers[row.ID],
		})
	}
	return out, nil
}
