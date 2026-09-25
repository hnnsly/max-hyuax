// Пакет council — сценарии совета дома: предложения жителей председателю и опросы без юридической силы (ADR-017).
package council

import (
	"context"
	"time"

	"dommax/internal/app"
	"dommax/internal/domain/council"
	"dommax/internal/domain/user"
)

// listLimit — сколько предложений и опросов показывать: для одного дома этого достаточно.
const listLimit = 100

type Config struct {
	Now            func() time.Time
	NewID          func() string
	ConsentVersion string
}

type Service struct {
	store app.Store
	cfg   Config
}

func NewService(store app.Store, cfg Config) *Service { return &Service{store: store, cfg: cfg} }

// PollView — опрос с итогами для экрана жителя.
type PollView struct {
	council.Poll
	app.Tally
	Open bool
}

// PollInput — опрос, который создаёт председатель; ProposalID пустой, если опрос не из предложения.
type PollInput struct {
	ProposalID string
	Question   string
	Options    []string
	Days       int
}

// participant — предлагать и голосовать может житель своего дома, давший согласие на обработку данных.
func (s *Service) participant(u user.User) error {
	if !u.CanTakePart() || u.HouseID == "" {
		return app.ErrForbidden
	}
	if !u.HasConsent(s.cfg.ConsentVersion) {
		return app.ErrConsentRequired
	}
	return nil
}

func (s *Service) Propose(ctx context.Context, u user.User, text string) (council.Proposal, error) {
	if err := s.participant(u); err != nil {
		return council.Proposal{}, err
	}
	p, err := council.NewProposal(s.cfg.NewID(), u.HouseID, u.ID, text, s.cfg.Now())
	if err != nil {
		return council.Proposal{}, err
	}
	return p, s.store.Council().AddProposal(ctx, p)
}

// MyProposals — предложения автора с ответами председателя.
func (s *Service) MyProposals(ctx context.Context, u user.User) ([]council.Proposal, error) {
	return s.store.Council().AuthorProposals(ctx, u.ID, listLimit)
}

// Folder — папка председателя: предложения его дома без имён авторов (имя в сценарий и не попадает).
func (s *Service) Folder(ctx context.Context, u user.User) ([]council.Proposal, error) {
	if !u.IsChairmanOf(u.ChairmanHouseID) {
		return nil, app.ErrForbidden
	}
	return s.store.Council().HouseProposals(ctx, u.ChairmanHouseID, listLimit)
}

// Reply — ответ председателя на предложение своего дома. Чужое предложение для него не существует.
func (s *Service) Reply(ctx context.Context, u user.User, id string, status council.Status, answer string) (council.Proposal, error) {
	p, err := s.store.Council().GetProposal(ctx, id)
	if err != nil {
		return council.Proposal{}, err
	}
	if !u.IsChairmanOf(p.HouseID) {
		return council.Proposal{}, app.ErrNotFound
	}
	// Ответ председателя — такие же данные жителя, как предложение: без согласия нельзя.
	if !u.HasConsent(s.cfg.ConsentVersion) {
		return council.Proposal{}, app.ErrConsentRequired
	}
	if err := p.Reply(status, answer, s.cfg.Now()); err != nil {
		return council.Proposal{}, err
	}
	saved, err := s.store.Council().ReplyProposal(ctx, p)
	if err != nil {
		return council.Proposal{}, err
	}
	if !saved {
		return council.Proposal{}, council.ErrAlreadyAnswered
	}
	return p, nil
}

func (s *Service) CreatePoll(ctx context.Context, u user.User, in PollInput) (PollView, error) {
	if !u.IsChairmanOf(u.ChairmanHouseID) {
		return PollView{}, app.ErrForbidden
	}
	if !u.HasConsent(s.cfg.ConsentVersion) {
		return PollView{}, app.ErrConsentRequired
	}
	if in.ProposalID != "" {
		p, err := s.store.Council().GetProposal(ctx, in.ProposalID)
		if err != nil {
			return PollView{}, err
		}
		if p.HouseID != u.ChairmanHouseID {
			return PollView{}, app.ErrNotFound
		}
	}
	poll, err := council.NewPoll(s.cfg.NewID(), u.ChairmanHouseID, in.ProposalID, in.Question, in.Options, in.Days, s.cfg.Now())
	if err != nil {
		return PollView{}, err
	}
	if err := s.store.Council().AddPoll(ctx, poll); err != nil {
		return PollView{}, err
	}
	return s.view(ctx, poll, u)
}

// Polls — опросы дома жителя, свежие первыми, с итогами и его голосом.
func (s *Service) Polls(ctx context.Context, u user.User) ([]PollView, error) {
	if u.HouseID == "" || !u.CanTakePart() {
		return nil, app.ErrForbidden
	}
	polls, err := s.store.Council().HousePolls(ctx, u.HouseID, listLimit)
	if err != nil {
		return nil, err
	}
	out := make([]PollView, 0, len(polls))
	for _, p := range polls {
		v, err := s.view(ctx, p, u)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// Vote — один голос жителя в опросе своего дома; переголосовать нельзя.
func (s *Service) Vote(ctx context.Context, u user.User, pollID string, option int) (PollView, error) {
	if err := s.participant(u); err != nil {
		return PollView{}, err
	}
	poll, err := s.store.Council().GetPoll(ctx, pollID)
	if err != nil {
		return PollView{}, err
	}
	if poll.HouseID != u.HouseID {
		return PollView{}, app.ErrNotFound
	}
	if err := poll.CheckVote(option, s.cfg.Now()); err != nil {
		return PollView{}, err
	}
	voted, err := s.store.Council().Vote(ctx, poll.ID, u.ID, option)
	if err != nil {
		return PollView{}, err
	}
	if !voted {
		return PollView{}, council.ErrAlreadyVoted
	}
	return s.view(ctx, poll, u)
}

func (s *Service) view(ctx context.Context, p council.Poll, u user.User) (PollView, error) {
	t, err := s.store.Council().Tally(ctx, p, u.ID)
	return PollView{Poll: p, Tally: t, Open: p.Open(s.cfg.Now())}, err
}
