package httpapi

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v3"

	"dommax/internal/app"
	appcouncil "dommax/internal/app/council"
	"dommax/internal/domain/council"
)

// proposalDTO — предложение председателю. Автора в ответе нет ни для кого: председатель его не видит,
// а автор и так знает, что это его предложение.
type proposalDTO struct {
	ID         string     `json:"id"`
	Text       string     `json:"text"`
	Status     string     `json:"status"`
	Answer     string     `json:"answer,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	AnsweredAt *time.Time `json:"answered_at,omitempty"`
}

func toProposalDTO(p council.Proposal) proposalDTO {
	d := proposalDTO{ID: p.ID, Text: p.Text, Status: string(p.Status), Answer: p.Answer, CreatedAt: p.CreatedAt}
	if !p.AnsweredAt.IsZero() {
		d.AnsweredAt = new(p.AnsweredAt)
	}
	return d
}

type pollOptionDTO struct {
	Text  string `json:"text"`
	Votes int    `json:"votes"`
}

// pollDTO — опрос с итогами; my_vote — номер варианта жителя, null — не голосовал.
type pollDTO struct {
	ID         string          `json:"id"`
	ProposalID string          `json:"proposal_id,omitempty"`
	Question   string          `json:"question"`
	Options    []pollOptionDTO `json:"options"`
	Total      int             `json:"total"`
	MyVote     *int            `json:"my_vote"`
	Open       bool            `json:"open"`
	CreatedAt  time.Time       `json:"created_at"`
	ClosesAt   time.Time       `json:"closes_at"`
}

func toPollDTO(v appcouncil.PollView) pollDTO {
	d := pollDTO{ID: v.ID, ProposalID: v.ProposalID, Question: v.Question, Open: v.Open,
		CreatedAt: v.CreatedAt, ClosesAt: v.ClosesAt, Options: make([]pollOptionDTO, len(v.Options))}
	for i, o := range v.Options {
		d.Options[i] = pollOptionDTO{Text: o, Votes: v.Votes[i]}
		d.Total += v.Votes[i]
	}
	if v.Mine >= 0 {
		d.MyVote = new(v.Mine)
	}
	return d
}

func (h *handlers) propose(c fiber.Ctx) error {
	var in struct {
		Text string `json:"text"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	p, err := h.Council.Propose(c.Context(), currentUser(c), in.Text)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(toProposalDTO(p))
}

func (h *handlers) myProposals(c fiber.Ctx) error {
	list, err := h.Council.MyProposals(c.Context(), currentUser(c))
	if err != nil {
		return err
	}
	return c.JSON(mapSlice(list, toProposalDTO))
}

func (h *handlers) councilFolder(c fiber.Ctx) error {
	list, err := h.Council.Folder(c.Context(), currentUser(c))
	if err != nil {
		return err
	}
	return c.JSON(mapSlice(list, toProposalDTO))
}

func (h *handlers) replyProposal(c fiber.Ctx) error {
	var in struct {
		Status string `json:"status"`
		Answer string `json:"answer"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	p, err := h.Council.Reply(c.Context(), currentUser(c), c.Params("id"), council.Status(in.Status), in.Answer)
	if err != nil {
		return err
	}
	return c.JSON(toProposalDTO(p))
}

func (h *handlers) createPoll(c fiber.Ctx) error {
	var in struct {
		ProposalID string   `json:"proposal_id"`
		Question   string   `json:"question"`
		Options    []string `json:"options"`
		Days       int      `json:"days"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	if in.Days == 0 {
		in.Days = 7 // неделя: жители успеют ответить, а решение не затянется
	}
	v, err := h.Council.CreatePoll(c.Context(), currentUser(c), appcouncil.PollInput{
		ProposalID: in.ProposalID, Question: in.Question, Options: in.Options, Days: in.Days,
	})
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(toPollDTO(v))
}

func (h *handlers) housePolls(c fiber.Ctx) error {
	list, err := h.Council.Polls(c.Context(), currentUser(c))
	if err != nil {
		return err
	}
	return c.JSON(mapSlice(list, toPollDTO))
}

func (h *handlers) vote(c fiber.Ctx) error {
	var in struct {
		Option *int `json:"option"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	if in.Option == nil {
		return app.ErrInvalidInput
	}
	v, err := h.Council.Vote(c.Context(), currentUser(c), c.Params("id"), *in.Option)
	if err != nil {
		return err
	}
	return c.JSON(toPollDTO(v))
}

// classifyCouncil — ошибки совета дома; ok = false, если ошибка не отсюда.
func classifyCouncil(err error) (int, string, string, bool) {
	switch {
	case errors.Is(err, council.ErrAlreadyAnswered):
		return fiber.StatusConflict, "already_answered", "На это предложение уже ответили", true
	case errors.Is(err, council.ErrAlreadyVoted):
		return fiber.StatusConflict, "already_voted", "Вы уже проголосовали в этом опросе", true
	case errors.Is(err, council.ErrPollExists):
		return fiber.StatusConflict, "poll_exists", "По этому предложению опрос уже открыт", true
	case errors.Is(err, council.ErrPollClosed):
		return fiber.StatusConflict, "poll_closed", "Опрос уже закончился", true
	case errors.Is(err, council.ErrInvalid):
		return fiber.StatusUnprocessableEntity, "invalid_input", "Проверьте введённые данные", true
	}
	return 0, "", "", false
}
