// Пакет rules — справочник категорий заявок: кто отвечает и в какой срок.
// Значения — данные пилотного региона; для другого региона меняется справочник, а не логика.
package rules

import (
	"errors"
	"slices"
	"strings"
	"time"
)

type Responsible string

const (
	ResponsibleManagementCompany Responsible = "management_company"
	ResponsibleResourceSupplier  Responsible = "resource_supplier"
)

var ErrUnknownCategory = errors.New("rules: unknown category")

type Rule struct {
	Code         string
	Title        string
	Responsible  Responsible
	Basis        string
	BusinessDays int
	Keywords     []string
}

// Deadline возвращает конец последнего рабочего дня, отведённого на ответ.
func (r Rule) Deadline(created time.Time) time.Time {
	return AddBusinessDays(created, r.BusinessDays)
}

const basisResponse = "Правила управления МКД (ПП № 416): ответ на обращение в установленный срок"

// CodeOther — категория «Другое общее имущество», когда точнее определить нельзя.
const CodeOther = "other"

// Справочник пилота. Сроки — модельные значения MVP; перед реальным пилотом
// их нужно сверить с регламентом управляющей компании.
var catalog = []Rule{
	{Code: "lift", Title: "Лифт", Responsible: ResponsibleManagementCompany, Basis: basisResponse, BusinessDays: 1,
		Keywords: []string{"лифт", "кабина", "застрял", "застревает"}},
	{Code: "leak", Title: "Протечка", Responsible: ResponsibleManagementCompany, Basis: basisResponse, BusinessDays: 1,
		Keywords: []string{"протеч", "течёт", "течет", "капает", "затоп", "кровл", "крыша"}},
	{Code: "heating", Title: "Отопление и горячая вода", Responsible: ResponsibleManagementCompany, Basis: basisResponse, BusinessDays: 1,
		Keywords: []string{"отоплен", "батаре", "радиатор", "холодно", "горячей воды", "горячая вода", "стояк"}},
	{Code: "lighting", Title: "Свет в подъезде", Responsible: ResponsibleManagementCompany, Basis: basisResponse, BusinessDays: 3,
		Keywords: []string{"свет", "лампа", "лампоч", "темно", "светильник"}},
	{Code: "door", Title: "Дверь подъезда и домофон", Responsible: ResponsibleManagementCompany, Basis: basisResponse, BusinessDays: 3,
		Keywords: []string{"двер", "домофон", "замок", "доводчик"}},
	{Code: "garbage", Title: "Мусоропровод и уборка", Responsible: ResponsibleManagementCompany, Basis: basisResponse, BusinessDays: 3,
		Keywords: []string{"мусор", "грязно", "уборк", "запах"}},
	{Code: CodeOther, Title: "Другое общее имущество", Responsible: ResponsibleManagementCompany, Basis: basisResponse, BusinessDays: 10},
}

// Categories возвращает все категории в порядке показа.
func Categories() []Rule { return slices.Clone(catalog) }

func Lookup(code string) (Rule, error) {
	i := slices.IndexFunc(catalog, func(r Rule) bool { return r.Code == code })
	if i < 0 {
		return Rule{}, ErrUnknownCategory
	}
	return catalog[i], nil
}

// Classify определяет категорию по ключевым словам: побеждает категория с наибольшим
// числом совпавших основ, при равенстве — первая по порядку. Без совпадений — false.
// Это подсказка: житель подтверждает категорию сам.
func Classify(text string) (Rule, bool) {
	text = strings.ToLower(text)
	var best Rule
	var bestScore int
	for _, r := range catalog {
		score := 0
		for _, kw := range r.Keywords {
			if strings.Contains(text, kw) {
				score++
			}
		}
		if score > bestScore {
			best, bestScore = r, score
		}
	}
	return best, bestScore > 0
}

// AddBusinessDays отсчитывает n рабочих дней (пн–пт) после from и возвращает конец
// этого дня в часовом поясе from. Праздники пока не учитываются.
func AddBusinessDays(from time.Time, n int) time.Time {
	d := time.Date(from.Year(), from.Month(), from.Day(), 23, 59, 59, 0, from.Location())
	for added := 0; added < n; {
		d = d.AddDate(0, 0, 1)
		if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
			added++
		}
	}
	return d
}
