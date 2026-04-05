package budget

import (
	"strconv"
	"sync"
)

type userState struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type userRole struct {
	Name    string `json:"name"`
	Created string `json:"created"`
	Updated string `json:"updated"`
}

type user struct {
	ID        int64     `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email"`
	Created   string    `json:"created"`
	Updated   string    `json:"updated"`
	Enabled   bool      `json:"enabled"`
	State     userState `json:"state"`
	Role      userRole  `json:"role"`
}

type group struct {
	Name      string `json:"name"`
	UserCount int    `json:"user_count"`
}

type groupDetails struct {
	Name  string `json:"name"`
	Users []user `json:"users"`
}

type costCenter struct {
	Name           string `json:"name"`
	ManagerEmail   string `json:"manager_email"`
	UserCount      int    `json:"user_count"`
	GroupCount     int    `json:"group_count"`
	GroupUserCount int    `json:"group_user_count"`
	Enabled        bool   `json:"enabled"`
}

type costCenterDetails struct {
	Name    string         `json:"name"`
	Manager user           `json:"manager"`
	Users   []user         `json:"users"`
	Groups  []groupDetails `json:"groups"`
	Enabled bool           `json:"enabled"`
}

type costCenterData struct {
	managerID     int64
	userIDs       []int64
	detailUserIDs []int64
	groupNames    []string
	enabled       bool
	usersNil      bool
}

// ── Budget types ──

type budget struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Year    int    `json:"year"`
	Limit   string `json:"limit"`
	Current string `json:"current"`
}

type userBudgetAllocation struct {
	Limit     string `json:"limit"`
	Current   string `json:"current"`
	UserID    int64  `json:"user_id"`
	UserEmail string `json:"user_email"`
	BudgetID  int64  `json:"budget_id"`
	Enabled   bool   `json:"enabled"`
}

type costCenterBudgetAllocation struct {
	Limit      string `json:"limit"`
	Current    string `json:"current"`
	CostCenter string `json:"cost_center"`
	BudgetID   int64  `json:"budget_id"`
	Enabled    bool   `json:"enabled"`
}

type budgetDetails struct {
	ID                int64                        `json:"id"`
	Name              string                       `json:"name"`
	Year              int                          `json:"year"`
	Limit             string                       `json:"limit"`
	Current           string                       `json:"current"`
	UserBudgets       []userBudgetAllocation       `json:"user_budgets"`
	CostCenterBudgets []costCenterBudgetAllocation `json:"cost_center_budgets"`
}

type budgetData struct {
	name               string
	year               int
	limit              string
	current            string
	userAllocations    []userBudgetAllocation
	ccAllocations      []costCenterBudgetAllocation
	userAllocationsNil bool
}

type userApprovalRule struct {
	ID            int64  `json:"id"`
	Threshold     string `json:"threshold"`
	ApproverID    int64  `json:"approver_id"`
	ApproverEmail string `json:"approver_email"`
	BudgetID      int64  `json:"budget_id"`
	UserID        int64  `json:"user_id"`
	Level         int    `json:"level"`
	SendEmail     bool   `json:"send_email"`
}

type ccApprovalRule struct {
	ID            int64  `json:"id"`
	Threshold     string `json:"threshold"`
	ApproverID    int64  `json:"approver_id"`
	ApproverEmail string `json:"approver_email"`
	BudgetID      int64  `json:"budget_id"`
	CostCenter    string `json:"cost_center"`
	Level         int    `json:"level"`
	SendEmail     bool   `json:"send_email"`
}

type paginatedResponse struct {
	TotalNumber int `json:"total_number"`
	CurrentPage int `json:"current_page"`
	TotalPages  int `json:"total_pages"`
	Items       any `json:"items"`
}

// store holds mutable fixture data, protected by a mutex.
type store struct {
	mu              sync.RWMutex
	users           []user
	groups          map[string][]int64 // group name → user IDs
	groupOrder      []string
	costCenters     map[string]*costCenterData
	costCenterOrder []string
	// Budget domain
	budgets      map[int64]*budgetData
	budgetOrder  []int64
	nextBudgetID int64
	// Approval rules
	userRules      map[int64]*userApprovalRule
	userRuleOrder  []int64
	nextUserRuleID int64
	ccRules        map[int64]*ccApprovalRule
	ccRuleOrder    []int64
	nextCcRuleID   int64
}

const (
	adminRoleTimestamp       = "2026-02-27T10:08:12.215422+01:00"
	simpleUserRoleTimestamp  = "2026-03-06T11:41:32.512891+01:00"
	approverRoleTimestamp    = "2026-03-09T16:18:02.819063+01:00"
	afcRoleTimestamp         = "2026-03-09T16:21:28.918067+01:00"
	ceoRoleTimestamp         = "2026-03-10T11:48:55.499349+01:00"
	applicationRoleTimestamp = "2026-04-01T11:39:46.176465+02:00"
)

func fixtureUser(id int64, firstName, lastName, email, created, updated, roleName, roleTimestamp string) user {
	return user{
		ID:        id,
		FirstName: firstName,
		LastName:  lastName,
		Email:     email,
		Created:   created,
		Updated:   updated,
		Enabled:   true,
		State: userState{
			Name:    "ACTIVE",
			Enabled: true,
		},
		Role: userRole{
			Name:    roleName,
			Created: roleTimestamp,
			Updated: roleTimestamp,
		},
	}
}

var allUsers = []user{
	fixtureUser(28, "Alessandra", "Ferrari", "alessandra.ferrari@cdlan.it", "2026-02-12T15:36:52.307346+01:00", "2026-02-12T15:36:52.307346+01:00", "ADMIN", adminRoleTimestamp),
	fixtureUser(27, "Alessandro", "Leocata", "alessandro.leocata@cdlan.it", "2026-02-12T15:36:09.649194+01:00", "2026-02-12T15:36:09.649194+01:00", "SIMPLE USER", simpleUserRoleTimestamp),
	fixtureUser(35, "Alessandro", "Leva", "alessandro.leva@cdlan.it", "2026-02-12T15:41:50.221252+01:00", "2026-02-12T15:41:50.221252+01:00", "SIMPLE USER", simpleUserRoleTimestamp),
	fixtureUser(20, "Andrea", "Doati", "andrea.doati@cdlan.it", "2026-02-12T14:57:56.668733+01:00", "2026-02-12T14:57:56.668733+01:00", "SIMPLE USER", simpleUserRoleTimestamp),
	fixtureUser(39, "Andrea", "Maldotti", "andrea.maldotti@cdlan.it", "2026-02-12T15:43:17.696835+01:00", "2026-02-12T15:43:17.696835+01:00", "SIMPLE USER", simpleUserRoleTimestamp),
	fixtureUser(6, "Carolina", "Ronchi", "carolina.ronchi@cdlan.it", "2026-02-12T14:45:47.181301+01:00", "2026-02-12T14:45:47.181301+01:00", "AFC", afcRoleTimestamp),
	fixtureUser(19, "Cinzia", "Dalla Torre", "cinzia.dallatorre@cdlan.it", "2026-02-12T14:57:28.69892+01:00", "2026-02-12T14:57:28.69892+01:00", "APPROVER", approverRoleTimestamp),
	fixtureUser(41, "Corrado", "Del Po", "corrado.delpo@cdlan.it", "2026-03-06T12:54:51.820796+01:00", "2026-03-06T12:54:51.820796+01:00", "CEO", ceoRoleTimestamp),
	fixtureUser(16, "Daniele", "Paleari", "daniele.paleari@cdlan.it", "2026-02-12T14:55:27.037962+01:00", "2026-02-12T14:55:27.037962+01:00", "SIMPLE USER", simpleUserRoleTimestamp),
	fixtureUser(29, "Davide", "Collovigh", "davide.collovigh@cdlan.it", "2026-02-12T15:37:22.558194+01:00", "2026-02-12T15:37:22.558194+01:00", "SIMPLE USER", simpleUserRoleTimestamp),
	fixtureUser(38, "Emanuele", "Damiani", "emanuele.damiani@cdlan.it", "2026-02-12T15:42:51.904244+01:00", "2026-02-12T15:42:51.904244+01:00", "SIMPLE USER", simpleUserRoleTimestamp),
	fixtureUser(5, "Eva", "Grimaldi", "eva.grimaldi@cdlan.it", "2026-02-12T14:45:26.289827+01:00", "2026-02-12T14:45:26.289827+01:00", "ADMIN", adminRoleTimestamp),
	fixtureUser(34, "Fabio", "Gallo", "fabio.gallo@cdlan.it", "2026-02-12T15:41:27.837627+01:00", "2026-02-12T15:41:27.837627+01:00", "APPROVER", approverRoleTimestamp),
	fixtureUser(1, "Federico", "Dei Cas", "federico.deicas@cdlan.it", "2026-02-09T14:34:47.52948+01:00", "2026-02-09T14:34:47.52948+01:00", "SIMPLE USER", simpleUserRoleTimestamp),
	fixtureUser(37, "Filippo", "Cecchini", "filippo.cecchini@cdlan.it", "2026-02-12T15:42:30.790144+01:00", "2026-02-12T15:42:30.790144+01:00", "SIMPLE USER", simpleUserRoleTimestamp),
	fixtureUser(40, "Auto", "Forwarder", "forwarder@cdlan.it", "2026-02-27T09:25:25.936311+01:00", "2026-02-27T09:25:25.936311+01:00", "ADMIN", adminRoleTimestamp),
	fixtureUser(30, "Francesco", "Sorbo", "francesco.sorbo@cdlan.it", "2026-02-12T15:38:16.368362+01:00", "2026-02-12T15:38:16.368362+01:00", "SIMPLE USER", simpleUserRoleTimestamp),
	fixtureUser(33, "Gabriele", "Avola", "gabriele.avola@cdlan.it", "2026-02-12T15:40:29.126052+01:00", "2026-02-12T15:40:29.126052+01:00", "SIMPLE USER", simpleUserRoleTimestamp),
	fixtureUser(32, "Gabriele", "Turra", "gabriele.turra@cdlan.it", "2026-02-12T15:40:05.687459+01:00", "2026-02-12T15:40:05.687459+01:00", "APPROVER", approverRoleTimestamp),
	fixtureUser(26, "Gennaro", "Ambrosio", "gennaro.ambrosio@cdlan.it", "2026-02-12T15:35:50.149123+01:00", "2026-02-12T15:35:50.149123+01:00", "SIMPLE USER", simpleUserRoleTimestamp),
	fixtureUser(42, "Gianmassimo", "Cambianica", "gianmassimo.cambianica@cdlan.it", "2026-03-06T18:13:13.576437+01:00", "2026-03-06T18:13:13.576437+01:00", "SIMPLE USER", simpleUserRoleTimestamp),
	fixtureUser(9, "Giorgia", "Degliuomini", "giorgia.degliuomini@cdlan.it", "2026-02-12T14:46:48.840246+01:00", "2026-02-12T14:46:48.840246+01:00", "SIMPLE USER", simpleUserRoleTimestamp),
	fixtureUser(46, "Giovanni", "Pagliaroli", "giovanni.pagliaroli@cdlan.it", "2026-03-06T18:15:11.329412+01:00", "2026-03-06T18:15:11.329412+01:00", "APPROVER", approverRoleTimestamp),
	fixtureUser(10, "Giulia", "Mezzolla", "giulia.mezzolla@cdlan.it", "2026-02-12T14:47:08.647804+01:00", "2026-02-12T14:47:08.647804+01:00", "APPROVER", approverRoleTimestamp),
	fixtureUser(22, "Ivan", "Chinaglia", "ivan.chinaglia@cdlan.it", "2026-02-12T15:34:02.456402+01:00", "2026-02-12T15:34:02.456402+01:00", "SIMPLE USER", simpleUserRoleTimestamp),
	fixtureUser(36, "Ivan", "Lago", "ivan.lago@cdlan.it", "2026-02-12T15:42:12.101337+01:00", "2026-02-12T15:42:12.101337+01:00", "SIMPLE USER", simpleUserRoleTimestamp),
	fixtureUser(44, "Laura", "Galbiati", "laura.galbiati@cdlan.it", "2026-03-06T18:14:23.29059+01:00", "2026-03-06T18:14:23.29059+01:00", "APPROVER", approverRoleTimestamp),
	fixtureUser(18, "Lorenzo", "Angelucci", "lorenzo.angelucci@cdlan.it", "2026-02-12T14:57:06.758626+01:00", "2026-02-12T14:57:06.758626+01:00", "SIMPLE USER", simpleUserRoleTimestamp),
	fixtureUser(15, "Luca", "Fadda", "luca.fadda@cdlan.it", "2026-02-12T14:54:58.903434+01:00", "2026-02-12T14:54:58.903434+01:00", "SIMPLE USER", simpleUserRoleTimestamp),
	fixtureUser(43, "Marco", "Perletti", "marco.perletti@cdlan.it", "2026-03-06T18:13:49.299395+01:00", "2026-03-06T18:13:49.299395+01:00", "SIMPLE USER", simpleUserRoleTimestamp),
	fixtureUser(23, "Marco", "Ziglioli", "marco.ziglioli@cdlan.it", "2026-02-12T15:34:24.433194+01:00", "2026-02-12T15:34:24.433194+01:00", "SIMPLE USER", simpleUserRoleTimestamp),
	fixtureUser(14, "Marta", "Savoldi", "marta.savoldi@cdlan.it", "2026-02-12T14:54:40.095271+01:00", "2026-02-12T14:54:40.095271+01:00", "APPROVER", approverRoleTimestamp),
	fixtureUser(3, "Matteo", "Pastori", "matteo.pastori@cdlan.it", "2026-02-12T14:44:49.433458+01:00", "2026-02-12T14:44:49.433458+01:00", "AFC", afcRoleTimestamp),
	fixtureUser(4, "Matteo", "Redaelli", "matteo.redaelli@cdlan.it", "2026-02-12T14:45:06.334189+01:00", "2026-02-12T14:45:06.334189+01:00", "SIMPLE USER", simpleUserRoleTimestamp),
	fixtureUser(7, "Paola", "Smania", "paola.smania@cdlan.it", "2026-02-12T14:46:04.622686+01:00", "2026-02-12T14:46:04.622686+01:00", "AFC", afcRoleTimestamp),
	fixtureUser(45, "Paolo", "Maladosa", "paolo.maladosa@cdlan.it", "2026-03-06T18:14:48.527379+01:00", "2026-03-06T18:14:48.527379+01:00", "SIMPLE USER", simpleUserRoleTimestamp),
	fixtureUser(12, "Roberta", "Catignano", "roberta.catignano@cdlan.it", "2026-02-12T14:53:56.513319+01:00", "2026-02-12T14:53:56.513319+01:00", "SIMPLE USER", simpleUserRoleTimestamp),
	fixtureUser(8, "Roberta", "Scattolin", "roberta.scattolin@cdlan.it", "2026-02-12T14:46:25.923383+01:00", "2026-02-12T14:46:25.923383+01:00", "SIMPLE USER", simpleUserRoleTimestamp),
	fixtureUser(24, "Salvatore", "Sciacco", "salvatore.sciacco@cdlan.it", "2026-02-12T15:34:46.068242+01:00", "2026-02-12T15:34:46.068242+01:00", "APPROVER", approverRoleTimestamp),
	fixtureUser(21, "Sara", "Gusmini", "sara.gusmini@cdlan.it", "2026-02-12T15:33:42.770999+01:00", "2026-02-12T15:33:42.770999+01:00", "APPROVER", approverRoleTimestamp),
	fixtureUser(25, "Stefano", "Vatta", "stefano.vatta@cdlan.it", "2026-02-12T15:35:25.945845+01:00", "2026-02-12T15:35:25.945845+01:00", "APPROVER", approverRoleTimestamp),
	fixtureUser(31, "Teresa", "Chirico", "teresa.chirico@cdlan.it", "2026-02-12T15:38:34.964713+01:00", "2026-02-12T15:38:34.964713+01:00", "Application", applicationRoleTimestamp),
	fixtureUser(11, "Valentina", "Falcone", "valentina.falcone@cdlan.it", "2026-02-12T14:53:38.147442+01:00", "2026-02-12T14:53:38.147442+01:00", "APPROVER", approverRoleTimestamp),
	fixtureUser(13, "Valeria", "Perego", "valeria.perego@cdlan.it", "2026-02-12T14:54:20.967512+01:00", "2026-02-12T14:54:20.967512+01:00", "SIMPLE USER", simpleUserRoleTimestamp),
	fixtureUser(47, "Vittoria", "Regazzi", "vittoria.regazzi@cdlan.it", "2026-03-06T18:54:56.992567+01:00", "2026-03-06T18:54:56.992567+01:00", "SIMPLE USER", simpleUserRoleTimestamp),
}

var db = &store{
	users: allUsers,
	groups: map[string][]int64{
		"AI process automation": {24},
		"Amministrazione":       {5, 6, 3, 7},
		"Applications":          {28, 40, 1, 30, 29, 31},
		"Board":                 {5, 41},
		"Communication":         {16, 14, 15},
		"Compliance":            {20, 19, 5},
		"CustXP":                {9, 10, 23, 22, 21, 42, 45, 47},
		"DC":                    {27, 26, 25},
		"MeA":                   {5, 41, 42},
		"Office - MI":           {10, 44},
		"Office - TR":           {44, 10, 42},
		"Operations":            {8, 5, 9, 44},
		"People":                {8, 9, 10, 44},
		"Tecnici":               {34, 37, 33, 32, 36, 18, 4, 35, 39, 38, 46, 43},
	},
	groupOrder: []string{
		"AI process automation",
		"Amministrazione",
		"Applications",
		"Board",
		"Communication",
		"Compliance",
		"CustXP",
		"DC",
		"MeA",
		"Office - MI",
		"Office - TR",
		"Operations",
		"People",
		"Tecnici",
	},
	costCenters: map[string]*costCenterData{
		"AI Process":             {managerID: 24, userIDs: []int64{24}, groupNames: []string{"AI process automation"}, enabled: true, usersNil: true},
		"Amministrazione":        {managerID: 3, userIDs: []int64{5, 6, 3, 7}, groupNames: []string{"Amministrazione"}, enabled: true, usersNil: true},
		"Applications":           {managerID: 28, userIDs: []int64{28, 40, 1, 30, 29, 31}, groupNames: []string{"Applications"}, enabled: true, usersNil: true},
		"Assurance":              {managerID: 5, userIDs: []int64{34, 37, 33, 32, 36, 18, 4, 35, 39, 38, 46, 43}, groupNames: []string{"Tecnici"}, enabled: true, usersNil: true},
		"Board":                  {managerID: 5, userIDs: []int64{5, 41}, groupNames: []string{"Board"}, enabled: true, usersNil: true},
		"CEO":                    {managerID: 41, userIDs: []int64{41}, groupNames: []string{}, enabled: true},
		"Communication":          {managerID: 14, userIDs: []int64{16, 14, 15}, groupNames: []string{"Communication"}, enabled: true, usersNil: true},
		"Compliance":             {managerID: 19, userIDs: []int64{19, 34, 46}, groupNames: []string{"Compliance"}, enabled: true, usersNil: true},
		"CustXP":                 {managerID: 21, userIDs: []int64{9, 10, 23, 22, 21, 42, 45, 47}, groupNames: []string{"CustXP"}, enabled: true, usersNil: true},
		"Data Center":            {managerID: 25, userIDs: []int64{27, 26, 25}, groupNames: []string{"DC"}, enabled: true, usersNil: true},
		"Delivery CLOUD":         {managerID: 34, userIDs: []int64{34, 37, 33, 32, 36, 18, 4, 35, 39, 38, 46, 43, 1, 29, 30, 31}, groupNames: []string{"Tecnici", "Operations"}, enabled: true, usersNil: true},
		"Delivery MS":            {managerID: 46, userIDs: []int64{46, 43, 42, 45, 27, 26, 25, 20, 23, 22, 21, 9, 10, 11, 44, 24}, groupNames: []string{"Tecnici", "Operations"}, enabled: true, usersNil: true},
		"Delivery TLC":           {managerID: 32, userIDs: []int64{32, 34, 37, 33, 36, 18, 4, 35, 39, 38, 46, 43, 27, 26, 25, 24}, groupNames: []string{"Tecnici", "Operations"}, enabled: true, usersNil: true},
		"Infrastruttura SYS-NET": {managerID: 5, userIDs: []int64{5, 34, 37, 33, 32, 36, 18, 4, 35, 39, 38, 46, 43}, detailUserIDs: []int64{42}, groupNames: []string{"Tecnici"}, enabled: true},
		"MeA":                    {managerID: 5, userIDs: []int64{12, 13, 14}, groupNames: []string{"MeA"}, enabled: true, usersNil: true},
		"Office - MI":            {managerID: 10, userIDs: []int64{10, 28}, groupNames: []string{"Office - MI"}, enabled: true, usersNil: true},
		"Office - TR":            {managerID: 44, userIDs: []int64{44, 24, 31}, groupNames: []string{"Office - TR"}, enabled: true, usersNil: true},
		"People":                 {managerID: 11, userIDs: []int64{11, 8, 9, 44}, groupNames: []string{"People"}, enabled: true},
		"Test-cc [PER TEST]":     {managerID: 40, userIDs: []int64{40, 28, 24, 31, 29, 30}, groupNames: []string{"Applications"}, enabled: true},
	},
	costCenterOrder: []string{
		"AI Process",
		"Amministrazione",
		"Applications",
		"Assurance",
		"Board",
		"CEO",
		"Communication",
		"Compliance",
		"CustXP",
		"Data Center",
		"Delivery CLOUD",
		"Delivery MS",
		"Delivery TLC",
		"Infrastruttura SYS-NET",
		"MeA",
		"Office - MI",
		"Office - TR",
		"People",
		"Test-cc [PER TEST]",
	},
	budgets: map[int64]*budgetData{
		11: {name: "Accessi", year: 2026, limit: "44154.000", current: "44154.000", userAllocations: nil, userAllocationsNil: true, ccAllocations: []costCenterBudgetAllocation{
			{BudgetID: 11, CostCenter: "Delivery TLC", Current: "44154.000", Enabled: true, Limit: "44154.000"},
		}},
		16: {name: "Associazioni/Abbonamenti", year: 2026, limit: "24000.000", current: "5860.760", userAllocations: []userBudgetAllocation{}, ccAllocations: []costCenterBudgetAllocation{}},
		6: {name: "Automezzi", year: 2026, limit: "20833.690", current: "1833.690", userAllocations: nil, userAllocationsNil: true, ccAllocations: []costCenterBudgetAllocation{
			{BudgetID: 6, CostCenter: "AI Process", Current: "0.000", Enabled: true, Limit: "1000.000"},
			{BudgetID: 6, CostCenter: "Board", Current: "0.000", Enabled: true, Limit: "1000.000"},
			{BudgetID: 6, CostCenter: "Compliance", Current: "0.000", Enabled: true, Limit: "1000.000"},
			{BudgetID: 6, CostCenter: "Data Center", Current: "0.000", Enabled: true, Limit: "2000.000"},
			{BudgetID: 6, CostCenter: "Office - TR", Current: "0.000", Enabled: true, Limit: "1000.000"},
			{BudgetID: 6, CostCenter: "Delivery TLC", Current: "0.000", Enabled: true, Limit: "1000.000"},
			{BudgetID: 6, CostCenter: "Delivery MS", Current: "0.000", Enabled: true, Limit: "5000.000"},
			{BudgetID: 6, CostCenter: "CustXP", Current: "0.000", Enabled: true, Limit: "7000.000"},
			{BudgetID: 6, CostCenter: "People", Current: "1833.690", Enabled: true, Limit: "1833.690"},
		}},
		24: {name: "Budget [PER TEST]", year: 2026, limit: "22300.000", current: "22300.000", userAllocations: []userBudgetAllocation{}, ccAllocations: []costCenterBudgetAllocation{}},
		17: {name: "CEO", year: 2026, limit: "100000.000", current: "0.000", userAllocations: []userBudgetAllocation{}, ccAllocations: []costCenterBudgetAllocation{}},
		3: {name: "Consulenza", year: 2026, limit: "95000.000", current: "10963.880", userAllocations: nil, userAllocationsNil: true, ccAllocations: []costCenterBudgetAllocation{
			{BudgetID: 3, CostCenter: "Amministrazione", Current: "0.000", Enabled: true, Limit: "10000.000"},
			{BudgetID: 3, CostCenter: "People", Current: "4662.000", Enabled: true, Limit: "10000.000"},
			{BudgetID: 3, CostCenter: "MeA", Current: "0.000", Enabled: true, Limit: "50000.000"},
			{BudgetID: 3, CostCenter: "Board", Current: "4130.880", Enabled: true, Limit: "20000.000"},
			{BudgetID: 3, CostCenter: "Compliance", Current: "2171.000", Enabled: true, Limit: "5000.000"},
		}},
		5: {name: "Eventi esterni", year: 2026, limit: "70000.000", current: "750.000", userAllocations: nil, userAllocationsNil: true, ccAllocations: []costCenterBudgetAllocation{
			{BudgetID: 5, CostCenter: "Board", Current: "0.000", Enabled: true, Limit: "20000.000"},
			{BudgetID: 5, CostCenter: "Communication", Current: "750.000", Enabled: true, Limit: "50000.000"},
		}},
		10: {name: "Eventi interni", year: 2026, limit: "27000.000", current: "5152.000", userAllocations: nil, userAllocationsNil: true, ccAllocations: []costCenterBudgetAllocation{
			{BudgetID: 10, CostCenter: "Communication", Current: "2460.000", Enabled: true, Limit: "10000.000"},
			{BudgetID: 10, CostCenter: "Office - MI", Current: "0.000", Enabled: true, Limit: "5000.000"},
			{BudgetID: 10, CostCenter: "Office - TR", Current: "0.000", Enabled: true, Limit: "2000.000"},
			{BudgetID: 10, CostCenter: "People", Current: "2692.000", Enabled: true, Limit: "10000.000"},
		}},
		18: {name: "Fonia", year: 2026, limit: "20000.000", current: "0.000", userAllocations: nil, userAllocationsNil: true, ccAllocations: []costCenterBudgetAllocation{
			{BudgetID: 18, CostCenter: "Delivery CLOUD", Current: "0.000", Enabled: true, Limit: "10000.000"},
			{BudgetID: 18, CostCenter: "Delivery MS", Current: "0.000", Enabled: true, Limit: "10000.000"},
		}},
		8: {name: "Formazione", year: 2026, limit: "46000.000", current: "1515.000", userAllocations: nil, userAllocationsNil: true, ccAllocations: []costCenterBudgetAllocation{
			{BudgetID: 8, CostCenter: "Applications", Current: "0.000", Enabled: true, Limit: "5000.000"},
			{BudgetID: 8, CostCenter: "Data Center", Current: "0.000", Enabled: true, Limit: "3000.000"},
			{BudgetID: 8, CostCenter: "Amministrazione", Current: "0.000", Enabled: true, Limit: "3000.000"},
			{BudgetID: 8, CostCenter: "Communication", Current: "0.000", Enabled: true, Limit: "3000.000"},
			{BudgetID: 8, CostCenter: "Compliance", Current: "0.000", Enabled: true, Limit: "2000.000"},
			{BudgetID: 8, CostCenter: "Office - MI", Current: "0.000", Enabled: true, Limit: "1000.000"},
			{BudgetID: 8, CostCenter: "Office - TR", Current: "0.000", Enabled: true, Limit: "1000.000"},
			{BudgetID: 8, CostCenter: "Delivery CLOUD", Current: "0.000", Enabled: true, Limit: "5000.000"},
			{BudgetID: 8, CostCenter: "Delivery MS", Current: "0.000", Enabled: true, Limit: "5000.000"},
			{BudgetID: 8, CostCenter: "Delivery TLC", Current: "0.000", Enabled: true, Limit: "5000.000"},
			{BudgetID: 8, CostCenter: "CustXP", Current: "0.000", Enabled: true, Limit: "5000.000"},
			{BudgetID: 8, CostCenter: "Assurance", Current: "0.000", Enabled: true, Limit: "5000.000"},
			{BudgetID: 8, CostCenter: "People", Current: "1515.000", Enabled: true, Limit: "3000.000"},
		}},
		22: {name: "Hardware", year: 2026, limit: "162398.500", current: "36623.980", userAllocations: nil, userAllocationsNil: true, ccAllocations: []costCenterBudgetAllocation{
			{BudgetID: 22, CostCenter: "Delivery MS", Current: "225.480", Enabled: true, Limit: "25000.000"},
			{BudgetID: 22, CostCenter: "Infrastruttura SYS-NET", Current: "0.000", Enabled: true, Limit: "50000.000"},
			{BudgetID: 22, CostCenter: "Delivery TLC", Current: "0.000", Enabled: true, Limit: "25000.000"},
			{BudgetID: 22, CostCenter: "AI Process", Current: "0.000", Enabled: true, Limit: "26000.000"},
			{BudgetID: 22, CostCenter: "Delivery CLOUD", Current: "36398.500", Enabled: true, Limit: "36398.500"},
		}},
		21: {name: "Manutenzioni", year: 2026, limit: "324060.000", current: "306060.000", userAllocations: nil, userAllocationsNil: true, ccAllocations: []costCenterBudgetAllocation{
			{BudgetID: 21, CostCenter: "Office - MI", Current: "0.000", Enabled: true, Limit: "5000.000"},
			{BudgetID: 21, CostCenter: "Office - TR", Current: "0.000", Enabled: true, Limit: "3000.000"},
			{BudgetID: 21, CostCenter: "Infrastruttura SYS-NET", Current: "0.000", Enabled: true, Limit: "10000.000"},
			{BudgetID: 21, CostCenter: "Data Center", Current: "306060.000", Enabled: true, Limit: "306060.000"},
		}},
		4: {name: "Merci/Attrezzature", year: 2026, limit: "102794.910", current: "17889.290", userAllocations: nil, userAllocationsNil: true, ccAllocations: []costCenterBudgetAllocation{
			{BudgetID: 4, CostCenter: "Delivery CLOUD", Current: "0.000", Enabled: true, Limit: "20000.000"},
			{BudgetID: 4, CostCenter: "Delivery MS", Current: "1765.000", Enabled: true, Limit: "20000.000"},
			{BudgetID: 4, CostCenter: "Amministrazione", Current: "0.000", Enabled: true, Limit: "1000.000"},
			{BudgetID: 4, CostCenter: "Applications", Current: "0.000", Enabled: true, Limit: "1000.000"},
			{BudgetID: 4, CostCenter: "Office - MI", Current: "674.030", Enabled: true, Limit: "5000.000"},
			{BudgetID: 4, CostCenter: "Communication", Current: "0.000", Enabled: true, Limit: "5000.000"},
			{BudgetID: 4, CostCenter: "Compliance", Current: "0.000", Enabled: true, Limit: "1000.000"},
			{BudgetID: 4, CostCenter: "CustXP", Current: "11794.910", Enabled: true, Limit: "11794.910"},
			{BudgetID: 4, CostCenter: "Delivery TLC", Current: "404.910", Enabled: true, Limit: "20000.000"},
			{BudgetID: 4, CostCenter: "Data Center", Current: "3196.100", Enabled: true, Limit: "15000.000"},
			{BudgetID: 4, CostCenter: "Office - TR", Current: "54.340", Enabled: true, Limit: "3000.000"},
		}},
		23: {name: "Pubblicità", year: 2026, limit: "25000.000", current: "1640.000", userAllocations: nil, userAllocationsNil: true, ccAllocations: []costCenterBudgetAllocation{
			{BudgetID: 23, CostCenter: "Communication", Current: "1640.000", Enabled: true, Limit: "25000.000"},
		}},
		15: {name: "Servizi Generali", year: 2026, limit: "20000.000", current: "0.000", userAllocations: nil, userAllocationsNil: true, ccAllocations: []costCenterBudgetAllocation{
			{BudgetID: 15, CostCenter: "Amministrazione", Current: "0.000", Enabled: true, Limit: "20000.000"},
		}},
		7: {name: "Servizi Infrastruttura", year: 2026, limit: "25000.000", current: "16399.710", userAllocations: nil, userAllocationsNil: true, ccAllocations: []costCenterBudgetAllocation{
			{BudgetID: 7, CostCenter: "Infrastruttura SYS-NET", Current: "16399.710", Enabled: true, Limit: "25000.000"},
		}},
		14: {name: "Software (per rivendita)", year: 2026, limit: "120000.000", current: "5891.710", userAllocations: nil, userAllocationsNil: true, ccAllocations: []costCenterBudgetAllocation{
			{BudgetID: 14, CostCenter: "Delivery MS", Current: "44.890", Enabled: true, Limit: "60000.000"},
			{BudgetID: 14, CostCenter: "Delivery CLOUD", Current: "5846.820", Enabled: true, Limit: "60000.000"},
		}},
		13: {name: "Software Uso Interno", year: 2026, limit: "57000.000", current: "3992.420", userAllocations: nil, userAllocationsNil: true, ccAllocations: []costCenterBudgetAllocation{
			{BudgetID: 13, CostCenter: "Communication", Current: "0.000", Enabled: true, Limit: "2000.000"},
			{BudgetID: 13, CostCenter: "Data Center", Current: "0.000", Enabled: true, Limit: "3000.000"},
			{BudgetID: 13, CostCenter: "Compliance", Current: "0.000", Enabled: true, Limit: "1000.000"},
			{BudgetID: 13, CostCenter: "Infrastruttura SYS-NET", Current: "0.000", Enabled: true, Limit: "20000.000"},
			{BudgetID: 13, CostCenter: "Board", Current: "0.000", Enabled: true, Limit: "5000.000"},
			{BudgetID: 13, CostCenter: "People", Current: "0.000", Enabled: true, Limit: "5000.000"},
			{BudgetID: 13, CostCenter: "AI Process", Current: "0.000", Enabled: true, Limit: "11000.000"},
			{BudgetID: 13, CostCenter: "Amministrazione", Current: "3159.920", Enabled: true, Limit: "5000.000"},
			{BudgetID: 13, CostCenter: "Applications", Current: "832.500", Enabled: true, Limit: "5000.000"},
		}},
		20: {name: "Tasse/Imposte", year: 2026, limit: "10000.000", current: "0.000", userAllocations: nil, userAllocationsNil: true, ccAllocations: []costCenterBudgetAllocation{
			{BudgetID: 20, CostCenter: "Amministrazione", Current: "0.000", Enabled: true, Limit: "10000.000"},
		}},
		25: {name: "Transito internet", year: 2026, limit: "17600.000", current: "10000.000", userAllocations: nil, userAllocationsNil: true, ccAllocations: []costCenterBudgetAllocation{
			{BudgetID: 25, CostCenter: "Delivery CLOUD", Current: "0.000", Enabled: true, Limit: "5000.000"},
			{BudgetID: 25, CostCenter: "Delivery TLC", Current: "7600.000", Enabled: true, Limit: "7600.000"},
			{BudgetID: 25, CostCenter: "Infrastruttura SYS-NET", Current: "2400.000", Enabled: true, Limit: "5000.000"},
		}},
		9: {name: "Trasferte", year: 2026, limit: "58000.000", current: "1822.920", userAllocations: nil, userAllocationsNil: true, ccAllocations: []costCenterBudgetAllocation{
			{BudgetID: 9, CostCenter: "CustXP", Current: "0.000", Enabled: true, Limit: "10000.000"},
			{BudgetID: 9, CostCenter: "Delivery CLOUD", Current: "0.000", Enabled: true, Limit: "1000.000"},
			{BudgetID: 9, CostCenter: "Delivery MS", Current: "0.000", Enabled: true, Limit: "5000.000"},
			{BudgetID: 9, CostCenter: "Delivery TLC", Current: "0.000", Enabled: true, Limit: "1000.000"},
			{BudgetID: 9, CostCenter: "Applications", Current: "0.000", Enabled: true, Limit: "1000.000"},
			{BudgetID: 9, CostCenter: "Board", Current: "0.000", Enabled: true, Limit: "5000.000"},
			{BudgetID: 9, CostCenter: "Compliance", Current: "0.000", Enabled: true, Limit: "1000.000"},
			{BudgetID: 9, CostCenter: "Communication", Current: "0.000", Enabled: true, Limit: "3000.000"},
			{BudgetID: 9, CostCenter: "Office - TR", Current: "0.000", Enabled: true, Limit: "5000.000"},
			{BudgetID: 9, CostCenter: "MeA", Current: "0.000", Enabled: true, Limit: "10000.000"},
			{BudgetID: 9, CostCenter: "AI Process", Current: "0.000", Enabled: true, Limit: "6000.000"},
			{BudgetID: 9, CostCenter: "Office - MI", Current: "721.650", Enabled: true, Limit: "5000.000"},
			{BudgetID: 9, CostCenter: "People", Current: "777.270", Enabled: true, Limit: "2000.000"},
			{BudgetID: 9, CostCenter: "Data Center", Current: "324.000", Enabled: true, Limit: "3000.000"},
		}},
		19: {name: "Welfare", year: 2026, limit: "60000.000", current: "510.000", userAllocations: nil, userAllocationsNil: true, ccAllocations: []costCenterBudgetAllocation{
			{BudgetID: 19, CostCenter: "People", Current: "510.000", Enabled: true, Limit: "60000.000"},
		}},
		12: {name: "Xconnect", year: 2026, limit: "20000.000", current: "4430.000", userAllocations: nil, userAllocationsNil: true, ccAllocations: []costCenterBudgetAllocation{
			{BudgetID: 12, CostCenter: "Data Center", Current: "4430.000", Enabled: true, Limit: "20000.000"},
		}},
	},
	budgetOrder:  []int64{11, 16, 6, 24, 17, 3, 5, 10, 18, 8, 22, 21, 4, 23, 15, 7, 14, 13, 20, 25, 9, 19, 12},
	nextBudgetID:   26,
	userRules:      map[int64]*userApprovalRule{},
	userRuleOrder:  []int64{},
	nextUserRuleID: 1,
	ccRules:        map[int64]*ccApprovalRule{},
	ccRuleOrder:    []int64{},
	nextCcRuleID:   1,
}

func (s *store) getUsers() []user {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.users
}

func (s *store) getUsersByIDs(ids []int64) []user {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getUsersByIDsLocked(ids)
}

func (s *store) listGroups() []group {
	s.mu.RLock()
	defer s.mu.RUnlock()
	groups := make([]group, 0, len(s.groupOrder))
	for _, name := range s.groupOrder {
		ids, ok := s.groups[name]
		if !ok {
			continue
		}
		groups = append(groups, group{Name: name, UserCount: len(ids)})
	}
	return groups
}

func (s *store) getGroupDetails(name string) (groupDetails, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids, ok := s.groups[name]
	if !ok {
		return groupDetails{}, false
	}
	users := s.getUsersByIDsLocked(ids)
	return groupDetails{Name: name, Users: users}, true
}

func (s *store) getUsersByIDsLocked(ids []int64) []user {
	byID := make(map[int64]user, len(s.users))
	for _, u := range s.users {
		byID[u.ID] = u
	}
	result := make([]user, 0, len(ids))
	for _, id := range ids {
		if u, ok := byID[id]; ok {
			result = append(result, u)
		}
	}
	return result
}

func (s *store) createGroup(name string, userIDs []int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.groups[name]; !exists {
		s.groupOrder = append(s.groupOrder, name)
	}
	s.groups[name] = userIDs
}

func (s *store) editGroup(name string, newName *string, userIDs *[]int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids, ok := s.groups[name]
	if !ok {
		return false
	}
	currentName := name
	if newName != nil && *newName != name {
		delete(s.groups, name)
		currentName = *newName
		for i, existingName := range s.groupOrder {
			if existingName == name {
				s.groupOrder[i] = currentName
				break
			}
		}
	}
	if userIDs != nil {
		ids = *userIDs
	}
	s.groups[currentName] = ids
	return true
}

func (s *store) deleteGroup(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.groups[name]
	if ok {
		delete(s.groups, name)
		for i, existingName := range s.groupOrder {
			if existingName == name {
				s.groupOrder = append(s.groupOrder[:i], s.groupOrder[i+1:]...)
				break
			}
		}
	}
	return ok
}

func (s *store) getUserByID(id int64) (user, bool) {
	for _, u := range s.users {
		if u.ID == id {
			return u, true
		}
	}
	return user{}, false
}

func (s *store) listCostCenters() []costCenter {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]costCenter, 0, len(s.costCenterOrder))
	for _, name := range s.costCenterOrder {
		cc, ok := s.costCenters[name]
		if !ok {
			continue
		}
		managerEmail := ""
		if mgr, found := s.getUserByID(cc.managerID); found {
			managerEmail = mgr.Email
		}
		groupUserCount := 0
		for _, gn := range cc.groupNames {
			if gids, exists := s.groups[gn]; exists {
				groupUserCount += len(gids)
			}
		}
		userCount := len(cc.userIDs)
		if cc.usersNil {
			userCount = 0
		} else if cc.detailUserIDs != nil {
			userCount = len(cc.detailUserIDs)
		}
		result = append(result, costCenter{
			Name:           name,
			ManagerEmail:   managerEmail,
			UserCount:      userCount,
			GroupCount:     len(cc.groupNames),
			GroupUserCount: groupUserCount,
			Enabled:        cc.enabled,
		})
	}
	return result
}

func (s *store) getCostCenterDetails(name string) (costCenterDetails, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cc, ok := s.costCenters[name]
	if !ok {
		return costCenterDetails{}, false
	}
	mgr, _ := s.getUserByID(cc.managerID)
	var users []user
	if !cc.usersNil {
		if cc.detailUserIDs != nil {
			users = s.getUsersByIDsLocked(cc.detailUserIDs)
		} else {
			users = s.getUsersByIDsLocked(cc.userIDs)
		}
	}
	groups := make([]groupDetails, 0, len(cc.groupNames))
	for _, gn := range cc.groupNames {
		if gids, exists := s.groups[gn]; exists {
			groups = append(groups, groupDetails{Name: gn, Users: s.getUsersByIDsLocked(gids)})
		}
	}
	return costCenterDetails{
		Name:    name,
		Manager: mgr,
		Users:   users,
		Groups:  groups,
		Enabled: cc.enabled,
	}, true
}

func (s *store) createCostCenter(name string, managerID int64, userIDs []int64, groupNames []string, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.costCenters[name]; !exists {
		s.costCenterOrder = append(s.costCenterOrder, name)
	}
	s.costCenters[name] = &costCenterData{
		managerID:     managerID,
		userIDs:       userIDs,
		detailUserIDs: userIDs,
		groupNames:    groupNames,
		enabled:       enabled,
		usersNil:      false,
	}
}

func (s *store) editCostCenter(name string, newName *string, managerID *int64, userIDs *[]int64, groupNames *[]string, enabled *bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	cc, ok := s.costCenters[name]
	if !ok {
		return false
	}
	currentName := name
	if newName != nil && *newName != name {
		delete(s.costCenters, name)
		currentName = *newName
		for i, n := range s.costCenterOrder {
			if n == name {
				s.costCenterOrder[i] = currentName
				break
			}
		}
	}
	if managerID != nil {
		cc.managerID = *managerID
	}
	if userIDs != nil {
		cc.userIDs = *userIDs
		cc.detailUserIDs = *userIDs
		cc.usersNil = false
	}
	if groupNames != nil {
		cc.groupNames = *groupNames
	}
	if enabled != nil {
		cc.enabled = *enabled
	}
	s.costCenters[currentName] = cc
	return true
}

// ── Budget store methods ──

func (s *store) listBudgets() []budget {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]budget, 0, len(s.budgetOrder))
	for _, id := range s.budgetOrder {
		bd, ok := s.budgets[id]
		if !ok {
			continue
		}
		result = append(result, budget{
			ID:      id,
			Name:    bd.name,
			Year:    bd.year,
			Limit:   bd.limit,
			Current: bd.current,
		})
	}
	return result
}

func (s *store) getBudgetDetails(id int64) (budgetDetails, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bd, ok := s.budgets[id]
	if !ok {
		return budgetDetails{}, false
	}
	ua := bd.userAllocations
	if !bd.userAllocationsNil && ua == nil {
		ua = []userBudgetAllocation{}
	}
	ca := bd.ccAllocations
	if ca == nil {
		ca = []costCenterBudgetAllocation{}
	}
	return budgetDetails{
		ID:                id,
		Name:              bd.name,
		Year:              bd.year,
		Limit:             bd.limit,
		Current:           bd.current,
		UserBudgets:       ua,
		CostCenterBudgets: ca,
	}, true
}

func (s *store) createBudget(name string, year int) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextBudgetID
	s.nextBudgetID++
	s.budgets[id] = &budgetData{
		name:               name,
		year:               year,
		limit:              "0.00",
		current:            "0.00",
		userAllocations:    []userBudgetAllocation{},
		ccAllocations:      []costCenterBudgetAllocation{},
		userAllocationsNil: false,
	}
	s.budgetOrder = append(s.budgetOrder, id)
	return id
}

func (s *store) editBudget(id int64, name *string, year *int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	bd, ok := s.budgets[id]
	if !ok {
		return false
	}
	if name != nil {
		bd.name = *name
	}
	if year != nil {
		bd.year = *year
	}
	return true
}

func (s *store) deleteBudget(id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.budgets[id]
	if !ok {
		return false
	}
	delete(s.budgets, id)
	for i, oid := range s.budgetOrder {
		if oid == id {
			s.budgetOrder = append(s.budgetOrder[:i], s.budgetOrder[i+1:]...)
			break
		}
	}
	return true
}

// ── Allocation store methods ──

func (s *store) createUserAllocation(budgetID int64, userID int64, limit string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	bd, ok := s.budgets[budgetID]
	if !ok {
		return false
	}
	email := ""
	if u, found := s.getUserByID(userID); found {
		email = u.Email
	}
	bd.userAllocations = append(bd.userAllocations, userBudgetAllocation{
		Limit: limit, Current: "0.00", UserID: userID, UserEmail: email, BudgetID: budgetID, Enabled: true,
	})
	return true
}

func (s *store) editUserAllocation(budgetID int64, userID int64, limit *string, enabled *bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	bd, ok := s.budgets[budgetID]
	if !ok {
		return false
	}
	for i := range bd.userAllocations {
		if bd.userAllocations[i].UserID == userID {
			if limit != nil {
				bd.userAllocations[i].Limit = *limit
			}
			if enabled != nil {
				bd.userAllocations[i].Enabled = *enabled
			}
			return true
		}
	}
	return false
}

func (s *store) createCcAllocation(budgetID int64, costCenter string, limit string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	bd, ok := s.budgets[budgetID]
	if !ok {
		return false
	}
	bd.ccAllocations = append(bd.ccAllocations, costCenterBudgetAllocation{
		Limit: limit, Current: "0.00", CostCenter: costCenter, BudgetID: budgetID, Enabled: true,
	})
	return true
}

func (s *store) editCcAllocation(budgetID int64, costCenter string, limit *string, enabled *bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	bd, ok := s.budgets[budgetID]
	if !ok {
		return false
	}
	for i := range bd.ccAllocations {
		if bd.ccAllocations[i].CostCenter == costCenter {
			if limit != nil {
				bd.ccAllocations[i].Limit = *limit
			}
			if enabled != nil {
				bd.ccAllocations[i].Enabled = *enabled
			}
			return true
		}
	}
	return false
}

// ── Approval rule store methods ──

func (s *store) listUserRules(budgetID int64, userID int64) []userApprovalRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []userApprovalRule
	for _, id := range s.userRuleOrder {
		r, ok := s.userRules[id]
		if !ok {
			continue
		}
		if r.BudgetID == budgetID && r.UserID == userID {
			result = append(result, *r)
		}
	}
	if result == nil {
		result = []userApprovalRule{}
	}
	return result
}

func (s *store) createUserRule(threshold string, approverID int64, budgetID int64, userID int64, level int, sendEmail bool) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextUserRuleID
	s.nextUserRuleID++
	email := ""
	if u, found := s.getUserByID(approverID); found {
		email = u.Email
	}
	s.userRules[id] = &userApprovalRule{
		ID: id, Threshold: threshold, ApproverID: approverID, ApproverEmail: email,
		BudgetID: budgetID, UserID: userID, Level: level, SendEmail: sendEmail,
	}
	s.userRuleOrder = append(s.userRuleOrder, id)
	return id
}

func (s *store) editUserRule(ruleID int64, threshold *string, approverID *int64, level *int, sendEmail *bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.userRules[ruleID]
	if !ok {
		return false
	}
	if threshold != nil {
		r.Threshold = *threshold
	}
	if approverID != nil {
		r.ApproverID = *approverID
		if u, found := s.getUserByID(*approverID); found {
			r.ApproverEmail = u.Email
		}
	}
	if level != nil {
		r.Level = *level
	}
	if sendEmail != nil {
		r.SendEmail = *sendEmail
	}
	return true
}

func (s *store) deleteUserRule(ruleID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.userRules[ruleID]
	if !ok {
		return false
	}
	delete(s.userRules, ruleID)
	for i, id := range s.userRuleOrder {
		if id == ruleID {
			s.userRuleOrder = append(s.userRuleOrder[:i], s.userRuleOrder[i+1:]...)
			break
		}
	}
	return true
}

func (s *store) listCcRules(budgetID int64, costCenter string) []ccApprovalRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []ccApprovalRule
	for _, id := range s.ccRuleOrder {
		r, ok := s.ccRules[id]
		if !ok {
			continue
		}
		if r.BudgetID == budgetID && r.CostCenter == costCenter {
			result = append(result, *r)
		}
	}
	if result == nil {
		result = []ccApprovalRule{}
	}
	return result
}

func (s *store) createCcRule(threshold string, approverID int64, budgetID int64, costCenter string, level int, sendEmail bool) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextCcRuleID
	s.nextCcRuleID++
	email := ""
	if u, found := s.getUserByID(approverID); found {
		email = u.Email
	}
	s.ccRules[id] = &ccApprovalRule{
		ID: id, Threshold: threshold, ApproverID: approverID, ApproverEmail: email,
		BudgetID: budgetID, CostCenter: costCenter, Level: level, SendEmail: sendEmail,
	}
	s.ccRuleOrder = append(s.ccRuleOrder, id)
	return id
}

func (s *store) editCcRule(ruleID int64, threshold *string, approverID *int64, level *int, sendEmail *bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.ccRules[ruleID]
	if !ok {
		return false
	}
	if threshold != nil {
		r.Threshold = *threshold
	}
	if approverID != nil {
		r.ApproverID = *approverID
		if u, found := s.getUserByID(*approverID); found {
			r.ApproverEmail = u.Email
		}
	}
	if level != nil {
		r.Level = *level
	}
	if sendEmail != nil {
		r.SendEmail = *sendEmail
	}
	return true
}

func (s *store) deleteCcRule(ruleID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.ccRules[ruleID]
	if !ok {
		return false
	}
	delete(s.ccRules, ruleID)
	for i, id := range s.ccRuleOrder {
		if id == ruleID {
			s.ccRuleOrder = append(s.ccRuleOrder[:i], s.ccRuleOrder[i+1:]...)
			break
		}
	}
	return true
}

// ── Report store methods ──

func (s *store) listBudgetsOverPercentage(pct float64) []budget {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []budget
	for _, id := range s.budgetOrder {
		bd, ok := s.budgets[id]
		if !ok {
			continue
		}
		limit, err := strconv.ParseFloat(bd.limit, 64)
		if err != nil || limit <= 0 {
			continue
		}
		current, err := strconv.ParseFloat(bd.current, 64)
		if err != nil {
			continue
		}
		if (current/limit)*100 > pct {
			result = append(result, budget{
				ID:      id,
				Name:    bd.name,
				Year:    bd.year,
				Limit:   bd.limit,
				Current: bd.current,
			})
		}
	}
	if result == nil {
		result = []budget{}
	}
	return result
}

func (s *store) listUnassignedUsers() []user {
	s.mu.RLock()
	defer s.mu.RUnlock()
	assigned := make(map[int64]bool)
	for _, bd := range s.budgets {
		if bd.userAllocationsNil {
			continue
		}
		for _, ua := range bd.userAllocations {
			assigned[ua.UserID] = true
		}
	}
	var result []user
	for _, u := range s.users {
		if u.Enabled && !assigned[u.ID] {
			result = append(result, u)
		}
	}
	if result == nil {
		result = []user{}
	}
	return result
}

// parseBudgetID parses budget_id from a path parameter.
func parseBudgetID(raw string) (int64, bool) {
	id, err := strconv.ParseInt(raw, 10, 64)
	return id, err == nil
}

// parseRuleID parses rule_id from a path parameter.
func parseRuleID(raw string) (int64, bool) {
	id, err := strconv.ParseInt(raw, 10, 64)
	return id, err == nil
}
