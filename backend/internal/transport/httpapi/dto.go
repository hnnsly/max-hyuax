package httpapi

import (
	"math"
	"time"

	"dommax/internal/app/issues"
	"dommax/internal/domain/house"
	"dommax/internal/domain/issue"
	"dommax/internal/domain/rules"
	"dommax/internal/domain/user"
)

type userDTO struct {
	ID             int64  `json:"id"`
	FirstName      string `json:"first_name"`
	Role           string `json:"role"`
	HouseID        string `json:"house_id,omitempty"`
	OrganizationID string `json:"organization_id,omitempty"`
	HasConsent     bool   `json:"has_consent"`
	ConsentVersion string `json:"consent_version"` // версия, на которую нужно согласие сейчас
}

func toUserDTO(u user.User, consentVersion string) userDTO {
	return userDTO{
		ID: u.ID, FirstName: u.FirstName, Role: string(u.Role), HouseID: u.HouseID,
		OrganizationID: u.OrganizationID, HasConsent: u.HasConsent(consentVersion), ConsentVersion: consentVersion,
	}
}

type sessionDTO struct {
	Token      string    `json:"token"`
	ExpiresAt  time.Time `json:"expires_at"`
	User       userDTO   `json:"user"`
	StartParam string    `json:"start_param,omitempty"`
}

type categoryDTO struct {
	Code         string `json:"code"`
	Title        string `json:"title"`
	Responsible  string `json:"responsible"`
	Basis        string `json:"basis"`
	BusinessDays int    `json:"business_days"`
}

type orgDTO struct {
	ID              string `json:"id"`
	Type            string `json:"type"`
	Name            string `json:"name"`
	PhoneOffice     string `json:"phone_office,omitempty"`
	PhoneDispatcher string `json:"phone_dispatcher,omitempty"`
	PhoneEmergency  string `json:"phone_emergency,omitempty"`
	Schedule        string `json:"schedule,omitempty"`
}

func toOrgDTO(o house.Organization) *orgDTO {
	return &orgDTO{
		ID: o.ID, Type: string(o.Type), Name: o.Name, PhoneOffice: o.PhoneOffice,
		PhoneDispatcher: o.PhoneDispatcher, PhoneEmergency: o.PhoneEmergency, Schedule: o.Schedule,
	}
}

type houseDTO struct {
	ID             string `json:"id"`
	Address        string `json:"address"`
	District       string `json:"district,omitempty"`
	YearBuilt      int    `json:"year_built,omitzero"`
	Floors         int    `json:"floors,omitzero"`
	EntrancesCount int    `json:"entrances_count"`
	OrganizationID string `json:"organization_id"`
	// Source = "model" — модельная запись, интерфейс показывает пометку.
	Source string `json:"source"`
}

func toHouseDTO(h house.House) houseDTO {
	return houseDTO{
		ID: h.ID, Address: h.Address, District: h.District, YearBuilt: h.YearBuilt, Floors: h.Floors,
		EntrancesCount: h.EntrancesCount, OrganizationID: h.OrganizationID, Source: h.Source,
	}
}

type entranceDTO struct {
	ID     string `json:"id"`
	Number int    `json:"number"`
}

type objectDTO struct {
	ID         string `json:"id"`
	HouseID    string `json:"house_id"`
	EntranceID string `json:"entrance_id,omitempty"`
	Category   string `json:"category"`
	Label      string `json:"label"`
	QRCode     string `json:"qr_code"`
}

func toObjectDTO(o house.AssetObject) objectDTO {
	return objectDTO{ID: o.ID, HouseID: o.HouseID, EntranceID: o.EntranceID, Category: o.Category, Label: o.Label, QRCode: o.QRCode}
}

type houseDetailsDTO struct {
	houseDTO     `json:",inline"`
	Organization *orgDTO       `json:"organization"`
	Entrances    []entranceDTO `json:"entrances"`
	Objects      []objectDTO   `json:"objects"`
}

// issueDTO — заявка глазами пользователя: участники не раскрываются, только их число.
type issueDTO struct {
	ID               string    `json:"id"`
	Number           int64     `json:"number"`
	HouseID          string    `json:"house_id"`
	Address          string    `json:"address,omitempty"`
	ObjectID         string    `json:"object_id,omitempty"`
	Place            string    `json:"place,omitempty"` // подпись объекта: «подъезд 2, пассажирский лифт»
	Category         string    `json:"category"`
	CategoryTitle    string    `json:"category_title"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	Status           string    `json:"status"`
	StatusAt         time.Time `json:"status_at"`
	StatusComment    string    `json:"status_comment,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	Deadline         time.Time `json:"deadline"`
	Overdue          bool      `json:"overdue"`
	ParticipantCount int       `json:"participant_count"`
	Joined           bool      `json:"joined"` // текущий пользователь среди участников
	Responsible      *orgDTO   `json:"responsible,omitzero"`
	Basis            string    `json:"basis,omitempty"`
}

// eventDTO — шаг хронологии заявки. Кто именно действовал, не раскрывается.
type eventDTO struct {
	Kind    string    `json:"kind"`
	Status  string    `json:"status"`
	Comment string    `json:"comment,omitempty"`
	At      time.Time `json:"at"`
}

func toIssueDTO(is *issue.Issue, viewer user.User, now time.Time) issueDTO {
	d := issueDTO{
		ID: is.ID(), Number: is.Number(), HouseID: is.HouseID(), ObjectID: is.ObjectID(),
		Category: is.Category(), Title: is.Title(), Description: is.Description(),
		Status: string(is.Status()), StatusAt: is.StatusAt(), StatusComment: is.StatusComment(),
		CreatedAt: is.CreatedAt(), Deadline: is.Deadline(), Overdue: is.IsOverdue(now),
		ParticipantCount: is.ParticipantCount(), Joined: is.HasParticipant(viewer.ID),
	}
	if r, err := rules.Lookup(is.Category()); err == nil {
		d.CategoryTitle = r.Title
	}
	return d
}

// metricsDTO — показатели УК. Длительности в минутах; null — за период нет данных.
type metricsDTO struct {
	FirstResponseMin *int           `json:"first_response_median_min"`
	PrevWeekMin      *int           `json:"prev_week_median_min"`
	ByDay            []dayMedianDTO `json:"first_response_by_day"`
	PeriodDays       int            `json:"period_days"`
	IssuesTotal      int            `json:"issues_total"`
	ReportsPerIssue  float64        `json:"reports_per_issue"`
	ClosedTotal      int            `json:"closed_total"`
	ClosedOnTime     int            `json:"closed_on_time"`
	OpenTotal        int            `json:"open_total"`
	OverdueOpen      int            `json:"overdue_open"`
	SampleData       bool           `json:"sample_data"`
}

type dayMedianDTO struct {
	Date      string `json:"date"` // день подачи по Москве, ГГГГ-ММ-ДД
	MedianMin *int   `json:"median_min"`
}

func minutes(d *time.Duration) *int {
	if d == nil {
		return nil
	}
	m := int(d.Round(time.Minute) / time.Minute)
	return &m
}

func toMetricsDTO(m issues.Metrics) metricsDTO {
	d := metricsDTO{
		FirstResponseMin: minutes(m.Week), PrevWeekMin: minutes(m.PrevWeek),
		PeriodDays: issues.MetricsPeriodDays, IssuesTotal: m.Issues,
		ClosedTotal: m.ClosedTotal, ClosedOnTime: m.ClosedOnTime, OpenTotal: m.OpenTotal, OverdueOpen: m.OverdueOpen,
		SampleData: m.SampleData,
	}
	if m.Issues > 0 {
		d.ReportsPerIssue = math.Round(float64(m.Reports)/float64(m.Issues)*10) / 10
	}
	d.ByDay = mapSlice(m.ByDay, func(day issues.DayMedian) dayMedianDTO {
		return dayMedianDTO{Date: day.Day.Format(time.DateOnly), MedianMin: minutes(day.Median)}
	})
	return d
}

// hintDTO — подсказка категории; все поля null, если категорию не узнали.
type hintDTO struct {
	Category *string `json:"category"`
	Title    *string `json:"title"`
	Source   *string `json:"source"` // llm | rules
}

// appealLinkDTO — ссылка на PDF-обращение: относительный путь, срок действия и имя файла.
type appealLinkDTO struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
	FileName  string    `json:"file_name"`
}
