package issues

import (
	"context"

	"dommax/internal/domain/issue"
	"dommax/internal/domain/user"
)

// Contact — участник заявки, который оставил телефон для мастера.
type Contact struct {
	FirstName string
	Phone     string
}

// Contacts — телефоны участников для сотрудника ответственной УК, пока заявка открыта: после
// закрытия мастеру звонить незачем. Остальным (соседям, чужой УК, району) возвращается nil без
// ошибки: карточку они видят, контакты — нет. Участников у заявки десятки, поэтому пользователи
// читаются по одному.
func (s *Service) Contacts(ctx context.Context, viewer user.User, is *issue.Issue) ([]Contact, error) {
	if !viewer.CanManageIssues(is.ResponsibleOrgID()) || is.Status().Closed() {
		return nil, nil
	}
	var out []Contact
	for _, p := range is.Participants() {
		u, err := s.store.Users().Get(ctx, p.UserID)
		if err != nil {
			return nil, err
		}
		if u.Deleted() || !u.PhoneShared() {
			continue
		}
		// Роль, взятая через /role на демо-стенде, видит только синтетических демо-жителей.
		if viewer.RoleSwitched && !u.Demo {
			continue
		}
		out = append(out, Contact{FirstName: u.FirstName, Phone: u.Phone})
	}
	return out, nil
}
