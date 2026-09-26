package httpapi

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"dommax/internal/app"
	"dommax/internal/app/office"
	"dommax/internal/domain/rules"
	"dommax/internal/domain/user"
)

type maintenanceDTO struct {
	ID            string    `json:"id"`
	HouseID       string    `json:"house_id"`
	Address       string    `json:"address,omitempty"`
	Category      string    `json:"category"`
	CategoryTitle string    `json:"category_title"`
	Title         string    `json:"title"`
	Description   string    `json:"description,omitempty"`
	StartsAt      time.Time `json:"starts_at"`
	EndsAt        time.Time `json:"ends_at"`
	CreatedAt     time.Time `json:"created_at"`
}

func toMaintenanceDTO(m app.MaintenanceAlert) maintenanceDTO {
	catTitle := m.Category
	if r, err := rules.Lookup(m.Category); err == nil {
		catTitle = r.Title
	} else {
		switch m.Category {
		case "water":
			catTitle = "Холодная и горячая вода"
		case "electricity":
			catTitle = "Электроснабжение"
		}
	}
	return maintenanceDTO{
		ID:            m.ID,
		HouseID:       m.HouseID,
		Address:       m.Address,
		Category:      m.Category,
		CategoryTitle: catTitle,
		Title:         m.Title,
		Description:   m.Description,
		StartsAt:      m.StartsAt,
		EndsAt:        m.EndsAt,
		CreatedAt:     m.CreatedAt,
	}
}

type specialistDTO struct {
	Code        string `json:"code"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Schedule    string `json:"schedule"`
}

type appointmentDTO struct {
	ID              string    `json:"id"`
	HouseID         string    `json:"house_id"`
	Address         string    `json:"address,omitempty"`
	UserName        string    `json:"user_name,omitempty"`
	Specialist      string    `json:"specialist"`
	SpecialistTitle string    `json:"specialist_title"`
	Topic           string    `json:"topic"`
	SlotAt          time.Time `json:"slot_at"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
}

func toAppointmentDTO(a app.Appointment, viewer user.User) appointmentDTO {
	specTitle := a.Specialist
	if s, ok := office.LookupSpecialist(a.Specialist); ok {
		specTitle = s.Title
	}
	userName := ""
	// Сотрудник УК видит имя жителя (если сам не является переключённой тестовой ролью для чужих).
	if viewer.Role == user.RoleOperator && !viewer.RoleSwitched {
		userName = a.UserName
	}
	return appointmentDTO{
		ID:              a.ID,
		HouseID:         a.HouseID,
		Address:         a.Address,
		UserName:        userName,
		Specialist:      a.Specialist,
		SpecialistTitle: specTitle,
		Topic:           a.Topic,
		SlotAt:          a.SlotAt,
		Status:          a.Status,
		CreatedAt:       a.CreatedAt,
	}
}

func (h *handlers) houseMaintenance(c fiber.Ctx) error {
	if h.Office == nil {
		return c.JSON([]maintenanceDTO{})
	}
	list, err := h.Office.ActiveMaintenance(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(mapSlice(list, toMaintenanceDTO))
}

func (h *handlers) ukMaintenance(c fiber.Ctx) error {
	if h.Office == nil {
		return c.JSON([]maintenanceDTO{})
	}
	list, err := h.Office.ListOrgMaintenance(c.Context(), currentUser(c))
	if err != nil {
		return err
	}
	return c.JSON(mapSlice(list, toMaintenanceDTO))
}

func (h *handlers) createMaintenance(c fiber.Ctx) error {
	var in struct {
		HouseID     string    `json:"house_id"`
		Category    string    `json:"category"`
		Title       string    `json:"title"`
		Description string    `json:"description"`
		StartsAt    time.Time `json:"starts_at"`
		EndsAt      time.Time `json:"ends_at"`
		Hours       int       `json:"hours"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	starts := in.StartsAt
	if starts.IsZero() {
		starts = h.Now()
	}
	ends := in.EndsAt
	if ends.IsZero() {
		hours := in.Hours
		if hours <= 0 {
			hours = 4
		}
		ends = starts.Add(time.Duration(hours) * time.Hour)
	}
	m, err := h.Office.CreateMaintenance(c.Context(), currentUser(c), office.CreateMaintenanceInput{
		HouseID:     in.HouseID,
		Category:    in.Category,
		Title:       in.Title,
		Description: in.Description,
		StartsAt:    starts,
		EndsAt:      ends,
	})
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(toMaintenanceDTO(m))
}

func (h *handlers) deleteMaintenance(c fiber.Ctx) error {
	if err := h.Office.DeleteMaintenance(c.Context(), currentUser(c), c.Params("id")); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *handlers) specialists(c fiber.Ctx) error {
	list := office.Specialists()
	return c.JSON(mapSlice(list, func(s office.Specialist) specialistDTO {
		return specialistDTO(s)
	}))
}

func (h *handlers) bookAppointment(c fiber.Ctx) error {
	var in struct {
		Specialist string    `json:"specialist"`
		Topic      string    `json:"topic"`
		SlotAt     time.Time `json:"slot_at"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	u := currentUser(c)
	a, err := h.Office.BookAppointment(c.Context(), u, office.BookInput{
		Specialist: in.Specialist,
		Topic:      in.Topic,
		SlotAt:     in.SlotAt,
	})
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(toAppointmentDTO(a, u))
}

func (h *handlers) myAppointments(c fiber.Ctx) error {
	if h.Office == nil {
		return c.JSON([]appointmentDTO{})
	}
	u := currentUser(c)
	list, err := h.Office.MyAppointments(c.Context(), u)
	if err != nil {
		return err
	}
	return c.JSON(mapSlice(list, func(a app.Appointment) appointmentDTO {
		return toAppointmentDTO(a, u)
	}))
}

func (h *handlers) ukAppointments(c fiber.Ctx) error {
	if h.Office == nil {
		return c.JSON([]appointmentDTO{})
	}
	u := currentUser(c)
	list, err := h.Office.OrgAppointments(c.Context(), u)
	if err != nil {
		return err
	}
	return c.JSON(mapSlice(list, func(a app.Appointment) appointmentDTO {
		return toAppointmentDTO(a, u)
	}))
}

func (h *handlers) cancelAppointment(c fiber.Ctx) error {
	u := currentUser(c)
	a, err := h.Office.CancelAppointment(c.Context(), u, c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(toAppointmentDTO(a, u))
}
