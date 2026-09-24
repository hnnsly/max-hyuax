// Пакет user описывает пользователя: жителя или оператора УК, его согласие на
// обработку ПДн и удаление аккаунта.
package user

import "time"

type Role string

const (
	RoleResident Role = "resident"
	RoleOperator Role = "uk_operator"
	// RoleDistrict — управа района или жилинспекция: сравнивает УК района, только смотрит.
	RoleDistrict Role = "district"
)

type User struct {
	ID             int64
	MaxUserID      int64
	FirstName      string
	Phone          string // только при согласии жителя, видно лишь УК
	HouseID        string
	Role           Role
	OrganizationID string // для оператора: УК, чьи заявки он ведёт
	District       string // для района: какой район он смотрит
	ConsentVersion string
	ConsentAt      time.Time
	DeletedAt      time.Time
}

// CanManageIssues сообщает, может ли пользователь менять статусы заявок организации orgID.
func (u User) CanManageIssues(orgID string) bool {
	return u.Role == RoleOperator && u.OrganizationID != "" && u.OrganizationID == orgID
}

// CanViewDistrict — пользователь района видит сравнение УК и просроченные заявки своего района.
func (u User) CanViewDistrict() bool {
	return u.Role == RoleDistrict && u.District != ""
}

// HasConsent проверяет согласие именно на текущую версию документа.
func (u User) HasConsent(docVersion string) bool {
	return u.ConsentVersion != "" && u.ConsentVersion == docVersion
}

func (u *User) AcceptConsent(docVersion string, at time.Time) {
	u.ConsentVersion, u.ConsentAt = docVersion, at
}

// Delete обезличивает аккаунт: ПДн стираются, заявки остаются без имени автора.
// Связь с MAX и адрес тоже стираются: если человек вернётся, у него будет новый аккаунт
// без старых заявок.
func (u *User) Delete(at time.Time) {
	u.FirstName, u.Phone, u.HouseID = "", "", ""
	u.MaxUserID = 0
	u.ConsentVersion, u.ConsentAt = "", time.Time{}
	u.DeletedAt = at
}

func (u User) Deleted() bool { return !u.DeletedAt.IsZero() }
