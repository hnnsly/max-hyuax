// Пакет house — модели чтения дома: сам дом, подъезды, объекты общего имущества
// и организации. Данные приходят из открытых источников и seed, поведения у них нет.
package house

type OrgType string

const (
	OrgManagementCompany OrgType = "uk"
	OrgHOA               OrgType = "tsj"
	OrgResourceSupplier  OrgType = "rso"
	OrgRegionalOperator  OrgType = "regoperator"
	OrgCapitalRepairFund OrgType = "capital_repair_fund"
	OrgHousingInspection OrgType = "gji"
)

type Organization struct {
	ID              string
	Type            OrgType
	Name            string
	PhoneOffice     string
	PhoneDispatcher string
	PhoneEmergency  string
	Schedule        string
}

type House struct {
	ID             string
	Address        string
	District       string
	YearBuilt      int
	Floors         int
	EntrancesCount int
	OrganizationID string
	Lat, Lon       float64
	Source         string // источник данных; модельные записи помечаются явно
}

type Entrance struct {
	ID      string
	HouseID string
	Number  int
}

// AssetObject — объект общего имущества с QR-кодом (лифт подъезда 2, свет на этаже и т.п.).
type AssetObject struct {
	ID         string
	HouseID    string
	EntranceID string // пусто, если объект относится ко всему дому
	Category   string // код категории из rules
	Label      string
	QRCode     string // непрозрачный id для диплинка startapp=o_<code>
}
