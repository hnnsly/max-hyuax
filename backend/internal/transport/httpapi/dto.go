package httpapi

import (
	"math"
	"time"

	"dommax/internal/app"
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
	District       string `json:"district,omitempty"` // для роли района
	HasConsent     bool   `json:"has_consent"`
	ConsentVersion string `json:"consent_version"` // версия, на которую нужно согласие сейчас
	// PhoneShared — житель оставил телефон для мастера; сам номер в ответах о себе не отдаётся.
	PhoneShared bool `json:"phone_shared"`
}

func toUserDTO(u user.User, consentVersion string) userDTO {
	return userDTO{
		ID: u.ID, FirstName: u.FirstName, Role: string(u.Role), HouseID: u.HouseID,
		OrganizationID: u.OrganizationID, District: u.District, HasConsent: u.HasConsent(consentVersion), ConsentVersion: consentVersion,
		PhoneShared: u.PhoneShared(),
	}
}

// contactDTO — участник заявки, оставивший телефон для мастера; видит только УК заявки.
type contactDTO struct {
	FirstName string `json:"first_name"`
	Phone     string `json:"phone"`
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
	// Проверка ремонта жителями: сколько подтвердили текущее «выполнено», ответ текущего
	// пользователя (fixed или null) и до какого момента можно ответить.
	ConfirmedCount int        `json:"confirmed_count"`
	MyAnswer       *string    `json:"my_answer"`
	AnswerUntil    *time.Time `json:"answer_until"`
	ReopenedAt     time.Time  `json:"reopened_at,omitzero"` // когда жители в последний раз вернули заявку в работу
	// Contacts — только в карточке и только для сотрудника ответственной УК.
	Contacts []contactDTO `json:"contacts,omitempty"`
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
		ConfirmedCount: is.ConfirmedCount(), ReopenedAt: is.ReopenedAt(),
	}
	if r, err := rules.Lookup(is.Category()); err == nil {
		d.CategoryTitle = r.Title
	}
	if until := is.AnswerUntil(); !until.IsZero() {
		d.AnswerUntil = &until
	}
	// Ответ «не починили» закрывает круг и возвращает заявку в работу, поэтому здесь бывает только fixed.
	if a, ok := is.AnswerOf(viewer.ID); ok && a.Fixed {
		d.MyAnswer = new("fixed")
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
	Confirmed        int            `json:"confirmed_by_residents"` // из выполненных жители подтвердили ремонт
	Reopened         int            `json:"reopened_by_residents"`  // жители вернули в работу
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
		ClosedTotal: m.ClosedTotal, ClosedOnTime: m.ClosedOnTime, Confirmed: m.Confirmed, Reopened: m.Reopened,
		OpenTotal: m.OpenTotal, OverdueOpen: m.OverdueOpen,
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

// districtMetricsDTO — сравнение УК района за период; длительности в минутах.
type districtMetricsDTO struct {
	District      string           `json:"district"`
	PeriodDays    int              `json:"period_days"`
	SampleData    bool             `json:"sample_data"`
	Organizations []districtOrgDTO `json:"organizations"`
}

type districtOrgDTO struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	FirstResponseMin *int   `json:"first_response_median_min"`
	IssuesTotal      int    `json:"issues_total"`
	OpenTotal        int    `json:"open_total"`
	OverdueOpen      int    `json:"overdue_open"`
	ClosedTotal      int    `json:"closed_total"`
	ClosedOnTime     int    `json:"closed_on_time"`
	Confirmed        int    `json:"confirmed_by_residents"`
	Reopened         int    `json:"reopened_by_residents"`
	SampleData       bool   `json:"sample_data"`
}

func toDistrictMetricsDTO(m issues.DistrictMetrics) districtMetricsDTO {
	d := districtMetricsDTO{District: m.District, PeriodDays: issues.MetricsPeriodDays}
	d.Organizations = mapSlice(m.Orgs, func(o issues.OrgMetrics) districtOrgDTO {
		d.SampleData = d.SampleData || o.SampleData
		return districtOrgDTO{
			ID: o.Org.ID, Name: o.Org.Name, FirstResponseMin: minutes(o.Week),
			IssuesTotal: o.Issues, OpenTotal: o.OpenTotal, OverdueOpen: o.OverdueOpen,
			ClosedTotal: o.ClosedTotal, ClosedOnTime: o.ClosedOnTime, Confirmed: o.Confirmed, Reopened: o.Reopened,
			SampleData: o.SampleData,
		}
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

// photoDTO — фото к заявке; url отдаёт JPEG с тем же токеном сессии.
// mine — фото приложил текущий пользователь и может его убрать; кто приложил чужое, не раскрывается.
type photoDTO struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Mine      bool      `json:"mine"`
	CreatedAt time.Time `json:"created_at"`
}

func photoDTOs(list []app.Photo, viewerID int64) []photoDTO {
	return mapSlice(list, func(p app.Photo) photoDTO {
		return photoDTO{
			ID: p.ID, URL: "/api/v1/photos/" + p.ID, Width: p.Width, Height: p.Height,
			Mine: p.UploadedBy == viewerID, CreatedAt: p.CreatedAt,
		}
	})
}
