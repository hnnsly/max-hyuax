package httpapi

import (
	"context"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"dommax/internal/app"
	"dommax/internal/app/auth"
	"dommax/internal/app/houses"
	"dommax/internal/app/issues"
	"dommax/internal/domain/issue"
	"dommax/internal/domain/rules"
	"dommax/internal/domain/user"
)

func (h *handlers) session(c fiber.Ctx, s auth.Session) error {
	return c.JSON(sessionDTO{Token: s.Token, ExpiresAt: s.ExpiresAt, User: toUserDTO(s.User, h.ConsentVersion), StartParam: s.StartParam})
}

func (h *handlers) loginMax(c fiber.Ctx) error {
	var in struct {
		InitData string `json:"init_data"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	s, err := h.Auth.LoginMax(c.Context(), in.InitData)
	if err != nil {
		return err
	}
	return h.session(c, s)
}

func (h *handlers) loginDemo(c fiber.Ctx) error {
	var in struct {
		Role string `json:"role"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	s, err := h.Auth.LoginDemo(c.Context(), in.Role)
	if err != nil {
		return err
	}
	return h.session(c, s)
}

func (h *handlers) categories(c fiber.Ctx) error {
	var out []categoryDTO
	for _, r := range rules.Categories() {
		out = append(out, categoryDTO{Code: r.Code, Title: r.Title, Responsible: string(r.Responsible), Basis: r.Basis, BusinessDays: r.BusinessDays})
	}
	return c.JSON(out)
}

func (h *handlers) me(c fiber.Ctx) error {
	return c.JSON(toUserDTO(currentUser(c), h.ConsentVersion))
}

func (h *handlers) acceptConsent(c fiber.Ctx) error {
	var in struct {
		Version string `json:"version"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	u, err := h.Auth.AcceptConsent(c.Context(), currentUser(c), in.Version)
	if err != nil {
		return err
	}
	return c.JSON(toUserDTO(u, h.ConsentVersion))
}

func (h *handlers) setHouse(c fiber.Ctx) error {
	var in struct {
		HouseID string `json:"house_id"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	u, err := h.Auth.SetHouse(c.Context(), currentUser(c), in.HouseID)
	if err != nil {
		return err
	}
	return c.JSON(toUserDTO(u, h.ConsentVersion))
}

func (h *handlers) deleteAccount(c fiber.Ctx) error {
	if err := h.Auth.DeleteAccount(c.Context(), currentUser(c)); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *handlers) searchHouses(c fiber.Ctx) error {
	list, err := h.Houses.Search(c.Context(), c.Query("query"))
	if err != nil {
		return err
	}
	return c.JSON(mapSlice(list, toHouseDTO))
}

func (h *handlers) nearestHouses(c fiber.Ctx) error {
	lat, errLat := strconv.ParseFloat(c.Query("lat"), 64)
	lon, errLon := strconv.ParseFloat(c.Query("lon"), 64)
	if errLat != nil || errLon != nil {
		return app.ErrInvalidInput
	}
	list, err := h.Houses.Nearest(c.Context(), lat, lon)
	if err != nil {
		return err
	}
	return c.JSON(mapSlice(list, toHouseDTO))
}

func (h *handlers) getHouse(c fiber.Ctx) error {
	d, err := h.Houses.Get(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	out := houseDetailsDTO{houseDTO: toHouseDTO(d.House), Organization: toOrgDTO(d.Organization), Objects: mapSlice(d.Objects, toObjectDTO)}
	for _, e := range d.Entrances {
		out.Entrances = append(out.Entrances, entranceDTO{ID: e.ID, Number: e.Number})
	}
	return c.JSON(out)
}

func (h *handlers) objectByCode(c fiber.Ctx) error {
	obj, hs, err := h.Houses.ByQRCode(c.Context(), c.Params("code"))
	if err != nil {
		return err
	}
	return c.JSON(map[string]any{"object": toObjectDTO(obj), "house": toHouseDTO(hs)})
}

func (h *handlers) houseIssues(c fiber.Ctx) error {
	list, err := h.Issues.ListByHouse(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return h.sendList(c, list)
}

func (h *handlers) similarIssues(c fiber.Ctx) error {
	list, err := h.Issues.FindSimilar(c.Context(), c.Query("house_id"), c.Query("category"), c.Query("object_id"))
	if err != nil {
		return err
	}
	return h.sendList(c, list)
}

func (h *handlers) reportIssue(c fiber.Ctx) error {
	var in struct {
		HouseID     string `json:"house_id"`
		ObjectID    string `json:"object_id"`
		Category    string `json:"category"`
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	u := currentUser(c)
	is, err := h.Issues.Report(c.Context(), u, issues.ReportInput{
		HouseID: in.HouseID, ObjectID: in.ObjectID, Category: in.Category, Title: in.Title, Description: in.Description,
	})
	if err != nil {
		return err
	}
	return h.sendOne(c, fiber.StatusCreated, is)
}

// getIssue отдаёт карточку заявки: плюс адрес, ответственная организация и основание срока.
func (h *handlers) getIssue(c fiber.Ctx) error {
	u := currentUser(c)
	is, err := h.Issues.Get(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	one := []issueDTO{toIssueDTO(is, u, h.Now())}
	if err := h.locate(c.Context(), one); err != nil {
		return err
	}
	d := one[0]
	details, err := h.Houses.Get(c.Context(), is.HouseID())
	if err != nil {
		return err
	}
	if details.Organization.ID == is.ResponsibleOrgID() {
		d.Responsible = toOrgDTO(details.Organization)
	}
	if r, err := rules.Lookup(is.Category()); err == nil {
		d.Basis = r.Basis
	}
	return c.JSON(d)
}

func (h *handlers) joinIssue(c fiber.Ctx) error {
	u := currentUser(c)
	is, err := h.Issues.Join(c.Context(), u, c.Params("id"))
	if err != nil {
		return err
	}
	return h.sendOne(c, fiber.StatusOK, is)
}

func (h *handlers) changeStatus(c fiber.Ctx) error {
	var in struct {
		Status  string `json:"status"`
		Comment string `json:"comment"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	u := currentUser(c)
	is, err := h.Issues.ChangeStatus(c.Context(), u, c.Params("id"), issue.Status(in.Status), in.Comment)
	if err != nil {
		return err
	}
	return h.sendOne(c, fiber.StatusOK, is)
}

// ukQueue — очередь оператора УК.
func (h *handlers) ukQueue(c fiber.Ctx) error {
	list, err := h.Issues.Queue(c.Context(), currentUser(c))
	if err != nil {
		return err
	}
	return h.sendList(c, list)
}

func (h *handlers) ukMetrics(c fiber.Ctx) error {
	m, err := h.Issues.Metrics(c.Context(), currentUser(c))
	if err != nil {
		return err
	}
	return c.JSON(toMetricsDTO(m))
}

func (h *handlers) myIssues(c fiber.Ctx) error {
	list, err := h.Issues.Mine(c.Context(), currentUser(c))
	if err != nil {
		return err
	}
	return h.sendList(c, list)
}

func (h *handlers) timeline(c fiber.Ctx) error {
	events, err := h.Issues.Timeline(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(mapSlice(events, func(e issue.Event) eventDTO {
		return eventDTO{Kind: string(e.Kind), Status: string(e.Status), Comment: e.Comment, At: e.At}
	}))
}

// sendList отдаёт заявки с адресом дома и подписью объекта.
func (h *handlers) sendList(c fiber.Ctx, list []*issue.Issue) error {
	out := h.issueList(currentUser(c), list)
	if err := h.locate(c.Context(), out); err != nil {
		return err
	}
	return c.JSON(out)
}

// locate заполняет адрес и место; каждый дом запрашивается один раз.
func (h *handlers) locate(ctx context.Context, out []issueDTO) error {
	homes := map[string]houses.Details{}
	for i := range out {
		d, ok := homes[out[i].HouseID]
		if !ok {
			var err error
			if d, err = h.Houses.Get(ctx, out[i].HouseID); err != nil {
				return err
			}
			homes[out[i].HouseID] = d
		}
		out[i].Address = d.House.Address
		for _, o := range d.Objects {
			if o.ID == out[i].ObjectID {
				out[i].Place = o.Label
			}
		}
	}
	return nil
}

func (h *handlers) issueList(viewer user.User, list []*issue.Issue) []issueDTO {
	now := h.Now()
	return mapSlice(list, func(is *issue.Issue) issueDTO { return toIssueDTO(is, viewer, now) })
}

func mapSlice[S, D any](in []S, f func(S) D) []D {
	out := make([]D, len(in))
	for i, v := range in {
		out[i] = f(v)
	}
	return out
}

// sendOne отдаёт одну заявку с адресом дома и подписью объекта.
func (h *handlers) sendOne(c fiber.Ctx, status int, is *issue.Issue) error {
	one := []issueDTO{toIssueDTO(is, currentUser(c), h.Now())}
	if err := h.locate(c.Context(), one); err != nil {
		return err
	}
	return c.Status(status).JSON(one[0])
}
