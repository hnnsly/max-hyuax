// Пакет apptest — хранилище в памяти для юнит-тестов сценариев.
package apptest

import (
	"cmp"
	"context"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"dommax/internal/app"
	"dommax/internal/domain/council"
	"dommax/internal/domain/house"
	"dommax/internal/domain/issue"
	"dommax/internal/domain/user"
)

type MemStore struct {
	mu       sync.Mutex
	HouseMap map[string]house.House
	Orgs     map[string]house.Organization
	Objects  []house.AssetObject
	UserMap  map[int64]user.User
	DemoKeys map[string]int64 // демо-ключ → ID пользователя
	photos   []app.Photo
	issues   map[string]*issue.Issue
	Events   []issue.Event
	nextNum  int64
	nextUID  int64
	outbox   MemOutbox
	council  memCouncil
	pending  map[int64]app.BotPending
}

// MemOutbox — очередь уведомлений в памяти: Pending виден тестам напрямую.
type MemOutbox struct {
	mu      sync.Mutex
	Pending []app.Notification
	cards   map[[2]string]string
}

func (o *MemOutbox) Enqueue(_ context.Context, notes []app.Notification) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, n := range notes {
		if !slices.Contains(o.Pending, n) {
			o.Pending = append(o.Pending, n)
		}
	}
	return nil
}

func (o *MemOutbox) Claim(_ context.Context, n int) ([]app.OutboxItem, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	var out []app.OutboxItem
	for i, p := range o.Pending[:min(n, len(o.Pending))] {
		out = append(out, app.OutboxItem{ID: int64(i + 1), Notification: p, Attempts: 1})
	}
	o.Pending = o.Pending[len(out):]
	return out, nil
}

func (o *MemOutbox) Done(context.Context, int64) error                           { return nil }
func (o *MemOutbox) Retry(context.Context, int64, time.Time, string, bool) error { return nil }
func (o *MemOutbox) Release(context.Context) error                               { return nil }

func (o *MemOutbox) CardMID(_ context.Context, issueID string, userID int64) (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	mid, ok := o.cards[[2]string{issueID, strconv.FormatInt(userID, 10)}]
	if !ok {
		return "", app.ErrNotFound
	}
	return mid, nil
}

func (o *MemOutbox) SaveCardMID(_ context.Context, issueID string, userID int64, mid string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.cards == nil {
		o.cards = map[[2]string]string{}
	}
	o.cards[[2]string{issueID, strconv.FormatInt(userID, 10)}] = mid
	return nil
}

func New() *MemStore {
	return &MemStore{
		HouseMap: map[string]house.House{},
		Orgs:     map[string]house.Organization{},
		UserMap:  map[int64]user.User{},
		issues:   map[string]*issue.Issue{},
		nextNum:  100,
		nextUID:  1000,
	}
}

func (s *MemStore) Issues() app.IssueRepo  { return issueRepo{s} }
func (s *MemStore) Houses() app.HouseRepo  { return houseRepo{s} }
func (s *MemStore) Users() app.UserRepo    { return userRepo{s} }
func (s *MemStore) Outbox() app.OutboxRepo { return &s.outbox }

// InTx в памяти просто вызывает fn: откат в юнит-тестах не проверяется.
func (s *MemStore) InTx(_ context.Context, fn func(app.Store) error) error { return fn(s) }

// AddUser кладёт пользователя с заданным ID.
func (s *MemStore) AddUser(u user.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.UserMap[u.ID] = u
}

// snapshot копирует агрегат, чтобы тест не мог изменить «сохранённое» состояние в обход Save.
func snapshot(is *issue.Issue) *issue.Issue {
	return issue.Restore(issue.NewParams{
		ID: is.ID(), HouseID: is.HouseID(), ObjectID: is.ObjectID(), Category: is.Category(),
		Title: is.Title(), Description: is.Description(), ResponsibleOrgID: is.ResponsibleOrgID(),
		CreatedAt: is.CreatedAt(), Deadline: is.Deadline(),
	}, issue.State{
		Number: is.Number(), Status: is.Status(), StatusAt: is.StatusAt(),
		StatusComment: is.StatusComment(), OverdueAt: is.OverdueAt(), ReopenedAt: is.ReopenedAt(),
		Participants: is.Participants(), Answers: is.Answers(),
	})
}

type issueRepo struct{ s *MemStore }

func (r issueRepo) Create(_ context.Context, is *issue.Issue) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.nextNum++
	is.SetNumber(r.s.nextNum)
	r.s.Events = append(r.s.Events, is.PullEvents()...)
	r.s.issues[is.ID()] = snapshot(is)
	return nil
}

func (r issueRepo) Save(_ context.Context, is *issue.Issue) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, ok := r.s.issues[is.ID()]; !ok {
		return app.ErrNotFound
	}
	r.s.Events = append(r.s.Events, is.PullEvents()...)
	r.s.issues[is.ID()] = snapshot(is)
	return nil
}

func (r issueRepo) Get(_ context.Context, id string) (*issue.Issue, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	is, ok := r.s.issues[id]
	if !ok {
		return nil, app.ErrNotFound
	}
	return snapshot(is), nil
}

func (r issueRepo) GetForUpdate(ctx context.Context, id string) (*issue.Issue, error) {
	return r.Get(ctx, id)
}

func (r issueRepo) filter(keep func(*issue.Issue) bool, order func(a, b *issue.Issue) int, limit int) []*issue.Issue {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []*issue.Issue
	for _, is := range r.s.issues {
		if keep(is) {
			out = append(out, snapshot(is))
		}
	}
	slices.SortFunc(out, order)
	return out[:min(len(out), limit)]
}

func newestFirst(a, b *issue.Issue) int { return b.CreatedAt().Compare(a.CreatedAt()) }

func (r issueRepo) ListByHouse(_ context.Context, houseID string, limit int) ([]*issue.Issue, error) {
	return r.filter(func(is *issue.Issue) bool { return is.HouseID() == houseID }, newestFirst, limit), nil
}

func (r issueRepo) FindSimilar(_ context.Context, houseID, category, objectID string, since time.Time) ([]*issue.Issue, error) {
	return r.filter(func(is *issue.Issue) bool {
		return is.HouseID() == houseID && is.Category() == category && !is.Status().Closed() &&
			!is.CreatedAt().Before(since) && (objectID == "" || is.ObjectID() == "" || is.ObjectID() == objectID)
	}, newestFirst, 5), nil
}

func (r issueRepo) Queue(_ context.Context, orgID string, limit int) ([]*issue.Issue, error) {
	return r.filter(func(is *issue.Issue) bool { return is.ResponsibleOrgID() == orgID }, func(a, b *issue.Issue) int {
		return cmp.Or(cmpBool(a.Status().Closed(), b.Status().Closed()), a.Deadline().Compare(b.Deadline()))
	}, limit), nil
}

func (r issueRepo) ListByParticipant(_ context.Context, userID int64, limit int) ([]*issue.Issue, error) {
	return r.filter(func(is *issue.Issue) bool { return is.HasParticipant(userID) }, newestFirst, limit), nil
}

func (r issueRepo) ListOverdueUnmarked(_ context.Context, now time.Time, limit int) ([]*issue.Issue, error) {
	return r.filter(func(is *issue.Issue) bool { return is.IsOverdue(now) && is.OverdueAt().IsZero() }, newestFirst, limit), nil
}

func (r issueRepo) FirstResponses(_ context.Context, orgID string, since time.Time) ([]app.FirstResponse, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []app.FirstResponse
	for _, is := range r.s.issues {
		if is.ResponsibleOrgID() != orgID || is.CreatedAt().Before(since) {
			continue
		}
		for _, e := range r.s.Events {
			if e.IssueID == is.ID() && e.Kind == issue.EventStatusChanged {
				out = append(out, app.FirstResponse{CreatedAt: is.CreatedAt(), RespondedAt: e.At})
				break
			}
		}
	}
	return out, nil
}

func (r issueRepo) OrgCounts(_ context.Context, orgID string, since, now time.Time) (app.OrgCounts, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var c app.OrgCounts
	for _, is := range r.s.issues {
		if is.ResponsibleOrgID() != orgID {
			continue
		}
		if !is.CreatedAt().Before(since) {
			c.Issues++
			c.Reports += len(is.Participants())
		}
		if !is.ReopenedAt().IsZero() && !is.ReopenedAt().Before(since) {
			c.Reopened++
		}
		switch {
		case !is.Status().Closed():
			c.OpenTotal++
			if is.IsOverdue(now) {
				c.OverdueOpen++
			}
		case !is.StatusAt().Before(since):
			c.ClosedTotal++
			if !is.StatusAt().After(is.Deadline()) {
				c.ClosedOnTime++
			}
			if is.ConfirmedCount() > 0 {
				c.Confirmed++
			}
		}
		// В памяти хранятся только ответы на текущее «выполнено»: оценок прошлых кругов тут нет.
		for _, a := range is.Answers() {
			if a.Stars > 0 && !a.DoneAt.Before(since) {
				c.RatingSum += a.Stars
				c.Ratings++
			}
		}
	}
	return c, nil
}

func (r issueRepo) Events(_ context.Context, issueID string) ([]issue.Event, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []issue.Event
	for _, e := range r.s.Events {
		if e.IssueID == issueID {
			out = append(out, e)
		}
	}
	return out, nil
}

func cmpBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	}
	return -1
}

type houseRepo struct{ s *MemStore }

func (r houseRepo) Search(_ context.Context, query string) ([]house.House, error) {
	var out []house.House
	replacer := strings.NewReplacer(",", " ", ".", " ")
	words := strings.Fields(strings.ToLower(replacer.Replace(query)))
	for _, h := range r.s.HouseMap {
		addr := strings.ToLower(h.Address)
		match := true
		for _, w := range words {
			if !strings.Contains(addr, w) {
				match = false
				break
			}
		}
		if match {
			out = append(out, h)
		}
	}
	slices.SortFunc(out, func(a, b house.House) int { return strings.Compare(a.Address, b.Address) })
	return out, nil
}

func (r houseRepo) Nearest(ctx context.Context, _, _ float64, limit int) ([]house.House, error) {
	all, _ := r.Search(ctx, "")
	return all[:min(len(all), limit)], nil
}

// Upsert в памяти хранит только сам дом: подъезды и объекты проверяет тест на Postgres.
func (r houseRepo) Upsert(_ context.Context, h house.House) (bool, error) {
	_, existed := r.s.HouseMap[h.ID]
	r.s.HouseMap[h.ID] = h
	return !existed, nil
}

func (r houseRepo) Get(_ context.Context, id string) (house.House, error) {
	h, ok := r.s.HouseMap[id]
	if !ok {
		return house.House{}, app.ErrNotFound
	}
	return h, nil
}

func (r houseRepo) Organization(_ context.Context, id string) (house.Organization, error) {
	o, ok := r.s.Orgs[id]
	if !ok {
		return house.Organization{}, app.ErrNotFound
	}
	return o, nil
}

func (r houseRepo) UpsertOrganization(_ context.Context, o house.Organization) error {
	r.s.Orgs[o.ID] = o
	return nil
}

func (r houseRepo) Entrances(context.Context, string) ([]house.Entrance, error) { return nil, nil }

func (r houseRepo) Objects(_ context.Context, houseID string) ([]house.AssetObject, error) {
	var out []house.AssetObject
	for _, o := range r.s.Objects {
		if o.HouseID == houseID {
			out = append(out, o)
		}
	}
	return out, nil
}

func (r houseRepo) ObjectByCode(_ context.Context, code string) (house.AssetObject, error) {
	i := slices.IndexFunc(r.s.Objects, func(o house.AssetObject) bool { return o.QRCode == code })
	if i < 0 {
		return house.AssetObject{}, app.ErrNotFound
	}
	return r.s.Objects[i], nil
}

type userRepo struct{ s *MemStore }

func (r userRepo) Get(_ context.Context, id int64) (user.User, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	u, ok := r.s.UserMap[id]
	if !ok {
		return user.User{}, app.ErrNotFound
	}
	return u, nil
}

func (r userRepo) ByMaxID(_ context.Context, maxUserID int64) (user.User, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, u := range r.s.UserMap {
		if u.MaxUserID == maxUserID {
			return u, nil
		}
	}
	return user.User{}, app.ErrNotFound
}

// ByDemoKey ищет по карте DemoKeys: в домене ключа демо-пользователя нет, он есть только в хранилище.
func (r userRepo) ByDemoKey(_ context.Context, key string) (user.User, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	u, ok := r.s.UserMap[r.s.DemoKeys[key]]
	if !ok {
		return user.User{}, app.ErrNotFound
	}
	return u, nil
}

func (r userRepo) Create(_ context.Context, u user.User) (user.User, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.nextUID++
	u.ID = r.s.nextUID
	r.s.UserMap[u.ID] = u
	return u, nil
}

func (r userRepo) Save(_ context.Context, u user.User) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, ok := r.s.UserMap[u.ID]; !ok {
		return app.ErrNotFound
	}
	r.s.UserMap[u.ID] = u
	return nil
}

func (r houseRepo) ByOrganization(_ context.Context, orgID string) ([]house.House, error) {
	var out []house.House
	for _, h := range r.s.HouseMap {
		if h.OrganizationID == orgID {
			out = append(out, h)
		}
	}
	slices.SortFunc(out, func(a, b house.House) int { return strings.Compare(a.Address, b.Address) })
	return out, nil
}

func (r houseRepo) Load(_ context.Context, orgID, district string, now time.Time) ([]app.HouseLoad, error) {
	var out []app.HouseLoad
	for _, h := range r.s.HouseMap {
		mine := h.OrganizationID == orgID
		if orgID == "" {
			mine = h.District == district
		}
		if !mine || (h.Lat == 0 && h.Lon == 0) {
			continue
		}
		l := app.HouseLoad{House: house.House{ID: h.ID, Address: h.Address, Lat: h.Lat, Lon: h.Lon}, OldestOverdue: now}
		open := issueRepo{r.s}.filter(func(is *issue.Issue) bool {
			return is.HouseID() == h.ID && !is.Status().Closed()
		}, newestFirst, math.MaxInt)
		for _, is := range open {
			l.Open++
			if now.After(is.Deadline()) {
				l.Overdue++
				if is.Deadline().Before(l.OldestOverdue) {
					l.OldestOverdue = is.Deadline()
				}
			}
		}
		out = append(out, l)
	}
	slices.SortFunc(out, func(a, b app.HouseLoad) int { return strings.Compare(a.House.Address, b.House.Address) })
	return out, nil
}

func (r houseRepo) OrganizationsInDistrict(_ context.Context, district string) ([]house.Organization, error) {
	seen := map[string]bool{}
	var out []house.Organization
	for _, h := range r.s.HouseMap {
		if h.District != district || seen[h.OrganizationID] {
			continue
		}
		seen[h.OrganizationID] = true
		if o, ok := r.s.Orgs[h.OrganizationID]; ok {
			out = append(out, o)
		}
	}
	slices.SortFunc(out, func(a, b house.Organization) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

func (r issueRepo) OverdueInDistrict(_ context.Context, district string, now time.Time, limit int) ([]*issue.Issue, error) {
	houses := r.s.HouseMap
	return r.filter(func(is *issue.Issue) bool {
		return houses[is.HouseID()].District == district && is.IsOverdue(now)
	}, func(a, b *issue.Issue) int { return a.Deadline().Compare(b.Deadline()) }, limit), nil
}

func (s *MemStore) Photos() app.PhotoRepo { return photoRepo{s} }

type photoRepo struct{ s *MemStore }

func (r photoRepo) Add(_ context.Context, p app.Photo) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.photos = append(r.s.photos, p)
	return nil
}

func (r photoRepo) ListByIssue(_ context.Context, issueID string) ([]app.Photo, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []app.Photo
	for _, p := range r.s.photos {
		if p.IssueID == issueID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (r photoRepo) Get(_ context.Context, id string) (app.Photo, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, p := range r.s.photos {
		if p.ID == id {
			return p, nil
		}
	}
	return app.Photo{}, app.ErrNotFound
}

func (r photoRepo) ListByUploader(_ context.Context, userID int64) ([]app.Photo, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []app.Photo
	for _, p := range r.s.photos {
		if p.UploadedBy == userID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (r photoRepo) Delete(_ context.Context, id string) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.photos = slices.DeleteFunc(r.s.photos, func(p app.Photo) bool { return p.ID == id })
	return nil
}

// MemFiles — хранилище файлов в памяти для тестов; Fail заставляет Put вернуть ошибку.
type MemFiles struct {
	mu   sync.Mutex
	data map[string][]byte
	Fail error
}

func (f *MemFiles) Put(_ context.Context, key string, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Fail != nil {
		return f.Fail
	}
	if f.data == nil {
		f.data = map[string][]byte{}
	}
	f.data[key] = slices.Clone(data)
	return nil
}

func (f *MemFiles) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.data, key)
	return nil
}

// Len — сколько файлов сейчас лежит в хранилище.
func (f *MemFiles) Len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.data)
}

func (f *MemFiles) Get(_ context.Context, key string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d, ok := f.data[key]
	if !ok {
		return nil, app.ErrNotFound
	}
	return d, nil
}

// memCouncil — предложения, опросы и голоса в памяти.
type memCouncil struct {
	proposals []council.Proposal
	polls     []council.Poll
	votes     map[[2]string]int // {poll id, user id} → вариант
}

func (s *MemStore) Council() app.CouncilRepo { return councilRepo{s} }

type councilRepo struct{ s *MemStore }

func (r councilRepo) AddProposal(_ context.Context, p council.Proposal) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.council.proposals = append(r.s.council.proposals, p)
	return nil
}

func (r councilRepo) GetProposal(_ context.Context, id string) (council.Proposal, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, p := range r.s.council.proposals {
		if p.ID == id {
			return p, nil
		}
	}
	return council.Proposal{}, app.ErrNotFound
}

func (r councilRepo) ReplyProposal(_ context.Context, p council.Proposal) (bool, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for i, old := range r.s.council.proposals {
		if old.ID == p.ID && old.Status == council.StatusNew {
			r.s.council.proposals[i] = p
			return true, nil
		}
	}
	return false, nil
}

func (r councilRepo) HouseProposals(_ context.Context, houseID string, limit int) ([]council.Proposal, error) {
	return r.proposals(func(p council.Proposal) bool { return p.HouseID == houseID }, limit), nil
}

func (r councilRepo) AuthorProposals(_ context.Context, authorID int64, limit int) ([]council.Proposal, error) {
	return r.proposals(func(p council.Proposal) bool { return p.AuthorID == authorID }, limit), nil
}

// proposals — как в Postgres: новые первыми, дальше свежие.
func (r councilRepo) proposals(keep func(council.Proposal) bool, limit int) []council.Proposal {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []council.Proposal
	for _, p := range r.s.council.proposals {
		if keep(p) {
			out = append(out, p)
		}
	}
	slices.SortFunc(out, func(a, b council.Proposal) int {
		return cmp.Or(cmpBool(b.Status == council.StatusNew, a.Status == council.StatusNew), b.CreatedAt.Compare(a.CreatedAt))
	})
	return out[:min(len(out), limit)]
}

func (r councilRepo) AddPoll(_ context.Context, p council.Poll) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, old := range r.s.council.polls {
		if p.ProposalID != "" && old.ProposalID == p.ProposalID {
			return council.ErrPollExists // как уникальный индекс в Postgres
		}
	}
	r.s.council.polls = append(r.s.council.polls, p)
	return nil
}

func (r councilRepo) GetPoll(_ context.Context, id string) (council.Poll, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, p := range r.s.council.polls {
		if p.ID == id {
			return p, nil
		}
	}
	return council.Poll{}, app.ErrNotFound
}

func (r councilRepo) HousePolls(_ context.Context, houseID string, limit int) ([]council.Poll, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []council.Poll
	for _, p := range r.s.council.polls {
		if p.HouseID == houseID {
			out = append(out, p)
		}
	}
	slices.SortFunc(out, func(a, b council.Poll) int { return b.CreatedAt.Compare(a.CreatedAt) })
	return out[:min(len(out), limit)], nil
}

func (r councilRepo) Vote(_ context.Context, pollID string, userID int64, option int) (bool, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if r.s.council.votes == nil {
		r.s.council.votes = map[[2]string]int{}
	}
	key := [2]string{pollID, strconv.FormatInt(userID, 10)}
	if _, voted := r.s.council.votes[key]; voted {
		return false, nil
	}
	r.s.council.votes[key] = option
	return true, nil
}

func (r councilRepo) Tally(_ context.Context, p council.Poll, userID int64) (app.Tally, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	t := app.Tally{Votes: make([]int, len(p.Options)), Mine: -1}
	for key, option := range r.s.council.votes {
		if key[0] != p.ID {
			continue
		}
		t.Votes[option]++
		if key[1] == strconv.FormatInt(userID, 10) {
			t.Mine = option
		}
	}
	return t, nil
}

func (s *MemStore) Pending() app.BotPendingRepo { return pendingRepo{s} }

type pendingRepo struct{ s *MemStore }

func (r pendingRepo) Set(_ context.Context, userID int64, p app.BotPending) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if r.s.pending == nil {
		r.s.pending = map[int64]app.BotPending{}
	}
	r.s.pending[userID] = p
	return nil
}

func (r pendingRepo) Take(_ context.Context, userID int64, now time.Time) (app.BotPending, bool, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	p, ok := r.s.pending[userID]
	delete(r.s.pending, userID)
	return p, ok && now.Before(p.ExpiresAt), nil
}

func (r userRepo) Chairmen(_ context.Context, houseID string) ([]user.User, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []user.User
	for _, u := range r.s.UserMap {
		if u.ChairmanHouseID == houseID && !u.Deleted() {
			out = append(out, u)
		}
	}
	slices.SortFunc(out, func(a, b user.User) int { return cmp.Compare(a.ID, b.ID) })
	return out, nil
}
