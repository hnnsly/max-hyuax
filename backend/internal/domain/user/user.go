// Пакет user описывает пользователя: жителя или оператора УК, его согласие на
// обработку ПДн и удаление аккаунта.
package user

import "time"

type Role string

const (
	RoleResident Role = "resident"
	RoleOperator Role = "uk_operator"
)

type User struct {
	ID             int64
	MaxUserID      int64
	FirstName      string
	Phone          string // только при согласии жителя, видно лишь УК
	HouseID        string
	Role           Role
	OrganizationID string // для оператора: УК, чьи заявки он ведёт
	ConsentVersion string
	ConsentAt      time.Time
	DeletedAt      time.Time
}

// CanManageIssues сообщает, может ли пользователь менять статусы заявок организации orgID.
func (u User) CanManageIssues(orgID string) bool {
	return u.Role == RoleOperator && u.OrganizationID != "" && u.OrganizationID == orgID
}

// HasConsent проверяет согласие именно на текущую версию документа.
func (u User) HasConsent(docVersion string) bool {
	return u.ConsentVersion != "" && u.ConsentVersion == docVersion
}

func (u *User) AcceptConsent(docVersion string, at time.Time) {
	u.ConsentVersion, u.ConsentAt = docVersion, at
}

// Delete обезличивает аккаунт: ПДн стираются, заявки остаются без имени автора.
func (u *User) Delete(at time.Time) {
	u.FirstName, u.Phone = "", ""
	u.ConsentVersion, u.ConsentAt = "", time.Time{}
	u.DeletedAt = at
}

func (u User) Deleted() bool { return !u.DeletedAt.IsZero() }
