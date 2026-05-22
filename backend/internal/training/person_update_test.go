package training

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestNormalizePersonUpdateInputValidation(t *testing.T) {
	teamID := "team-cloud"
	valid := PersonUpdateInput{
		FirstName: " Ada ",
		LastName:  " Lovelace ",
		Email:     " ADA@example.COM ",
		Status:    "active",
		TeamID:    &teamID,
	}

	normalized, err := normalizePersonUpdateInput(valid)
	if err != nil {
		t.Fatalf("normalize valid input: %v", err)
	}
	if normalized.FirstName != "Ada" || normalized.LastName != "Lovelace" || normalized.Email != "ada@example.com" || normalized.TeamID != "team-cloud" {
		t.Fatalf("unexpected normalization: %+v", normalized)
	}

	cases := []struct {
		name string
		in   PersonUpdateInput
		code string
	}{
		{
			name: "required fields",
			in:   PersonUpdateInput{FirstName: "", LastName: "Rossi", Email: "mario.rossi@example.com", Status: "active"},
			code: "person_required_fields_missing",
		},
		{
			name: "email",
			in:   PersonUpdateInput{FirstName: "Mario", LastName: "Rossi", Email: "not an email", Status: "active"},
			code: "person_email_invalid",
		},
		{
			name: "status",
			in:   PersonUpdateInput{FirstName: "Mario", LastName: "Rossi", Email: "mario.rossi@example.com", Status: "archived"},
			code: "person_status_invalid",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := normalizePersonUpdateInput(tc.in)
			appErr, ok := asAppError(err)
			if !ok {
				t.Fatalf("expected appError, got %T %v", err, err)
			}
			if appErr.code != tc.code {
				t.Fatalf("error code = %q, want %q", appErr.code, tc.code)
			}
		})
	}
}

func TestNormalizePersonCreateInputValidation(t *testing.T) {
	teamID := "team-cloud"
	valid := PersonCreateInput{
		FirstName: " Ada ",
		LastName:  " Lovelace ",
		Email:     " ADA@example.COM ",
		Status:    "active",
		TeamID:    &teamID,
	}

	normalized, err := normalizePersonCreateInput(valid)
	if err != nil {
		t.Fatalf("normalize valid input: %v", err)
	}
	if normalized.FirstName != "Ada" || normalized.LastName != "Lovelace" || normalized.Email != "ada@example.com" || normalized.TeamID != "team-cloud" {
		t.Fatalf("unexpected normalization: %+v", normalized)
	}

	_, err = normalizePersonCreateInput(PersonCreateInput{
		FirstName: "Ada",
		LastName:  "Lovelace",
		Email:     "invalid",
		Status:    "active",
	})
	appErr, ok := asAppError(err)
	if !ok {
		t.Fatalf("expected appError, got %T %v", err, err)
	}
	if appErr.code != "person_email_invalid" {
		t.Fatalf("error code = %q, want person_email_invalid", appErr.code)
	}
}

func TestCreatePersonCreatesEmployeeMembershipAndAudit(t *testing.T) {
	state := newPersonUpdateTestState()
	store := NewSQLStore(openPersonUpdateTestDB(t, state))

	teamID := "team-app"
	response, err := store.CreatePerson(context.Background(), Principal{IsPeopleAdmin: true, Email: state.actorEmail}, PersonCreateInput{
		FirstName: "Laura",
		LastName:  "Bianchi",
		Email:     "laura.bianchi@example.com",
		Status:    "active",
		TeamID:    &teamID,
		Notes:     "Inserita dal team People",
	})
	if err != nil {
		t.Fatalf("CreatePerson returned error: %v", err)
	}
	if !response.OK || response.ID != state.nextEmployeeID || response.Status != "active" {
		t.Fatalf("unexpected response: %+v", response)
	}
	if !state.committed || state.rolledBack {
		t.Fatalf("transaction state committed=%v rolledBack=%v, want commit only", state.committed, state.rolledBack)
	}
	if state.createdEmployee == nil {
		t.Fatal("employee was not created")
	}
	if state.createdEmployee.FirstName != "Laura" || state.createdEmployee.LastName != "Bianchi" || state.createdEmployee.Email != "laura.bianchi@example.com" || state.createdEmployee.Notes != "Inserita dal team People" {
		t.Fatalf("employee not created correctly: %+v", state.createdEmployee)
	}
	active := state.activeMemberships()
	wantActive := []personUpdateMembership{{ID: "membership-new", EmployeeID: state.nextEmployeeID, TeamID: "team-app"}}
	if !reflect.DeepEqual(active, wantActive) {
		t.Fatalf("active memberships = %+v, want %+v", active, wantActive)
	}
	if got := state.auditActions(); !reflect.DeepEqual(got, []string{
		"employee:create",
		"team_membership:create",
	}) {
		t.Fatalf("audit actions = %#v", got)
	}
}

func TestCreatePersonRejectsDuplicateEmailBeforeInsert(t *testing.T) {
	state := newPersonUpdateTestState()
	store := NewSQLStore(openPersonUpdateTestDB(t, state))

	_, err := store.CreatePerson(context.Background(), Principal{IsPeopleAdmin: true, Email: state.actorEmail}, PersonCreateInput{
		FirstName: "Mario",
		LastName:  "Rossi",
		Email:     state.employee.Email,
		Status:    "active",
	})
	appErr, ok := asAppError(err)
	if !ok {
		t.Fatalf("expected appError, got %T %v", err, err)
	}
	if appErr.code != "person_email_duplicate" {
		t.Fatalf("error code = %q, want person_email_duplicate", appErr.code)
	}
	if state.employeeCreated {
		t.Fatal("employee was inserted after duplicate email validation failed")
	}
	if !state.rolledBack || state.committed {
		t.Fatalf("transaction state committed=%v rolledBack=%v, want rollback only", state.committed, state.rolledBack)
	}
}

func TestCreatePersonRejectsInactiveTeamBeforeInsert(t *testing.T) {
	state := newPersonUpdateTestState()
	state.teams["team-app"] = false
	store := NewSQLStore(openPersonUpdateTestDB(t, state))

	teamID := "team-app"
	_, err := store.CreatePerson(context.Background(), Principal{IsPeopleAdmin: true, Email: state.actorEmail}, PersonCreateInput{
		FirstName: "Laura",
		LastName:  "Bianchi",
		Email:     "laura.bianchi@example.com",
		Status:    "active",
		TeamID:    &teamID,
	})
	appErr, ok := asAppError(err)
	if !ok {
		t.Fatalf("expected appError, got %T %v", err, err)
	}
	if appErr.code != "team_inactive" {
		t.Fatalf("error code = %q, want team_inactive", appErr.code)
	}
	if state.employeeCreated {
		t.Fatal("employee was inserted after inactive team validation failed")
	}
	if !state.rolledBack || state.committed {
		t.Fatalf("transaction state committed=%v rolledBack=%v, want rollback only", state.committed, state.rolledBack)
	}
}

func TestCreatePersonRejectsUnknownTeamBeforeInsert(t *testing.T) {
	state := newPersonUpdateTestState()
	store := NewSQLStore(openPersonUpdateTestDB(t, state))

	teamID := "team-missing"
	_, err := store.CreatePerson(context.Background(), Principal{IsPeopleAdmin: true, Email: state.actorEmail}, PersonCreateInput{
		FirstName: "Laura",
		LastName:  "Bianchi",
		Email:     "laura.bianchi@example.com",
		Status:    "active",
		TeamID:    &teamID,
	})
	appErr, ok := asAppError(err)
	if !ok {
		t.Fatalf("expected appError, got %T %v", err, err)
	}
	if appErr.code != "team_not_found" {
		t.Fatalf("error code = %q, want team_not_found", appErr.code)
	}
	if state.employeeCreated {
		t.Fatal("employee was inserted after unknown team validation failed")
	}
	if !state.rolledBack || state.committed {
		t.Fatalf("transaction state committed=%v rolledBack=%v, want rollback only", state.committed, state.rolledBack)
	}
}

func TestUpdatePersonRejectsDuplicateEmailBeforeUpdate(t *testing.T) {
	state := newPersonUpdateTestState()
	state.duplicateEmailID = "employee-duplicate"
	store := NewSQLStore(openPersonUpdateTestDB(t, state))

	teamID := "team-cloud"
	_, err := store.UpdatePerson(context.Background(), Principal{IsPeopleAdmin: true, Email: state.actorEmail}, state.employee.ID, PersonUpdateInput{
		FirstName: "Mario",
		LastName:  "Rossi",
		Email:     "duplicate@example.com",
		Status:    "active",
		TeamID:    &teamID,
	})
	appErr, ok := asAppError(err)
	if !ok {
		t.Fatalf("expected appError, got %T %v", err, err)
	}
	if appErr.code != "person_email_duplicate" {
		t.Fatalf("error code = %q, want person_email_duplicate", appErr.code)
	}
	if state.employeeUpdated {
		t.Fatal("employee was updated after duplicate email validation failed")
	}
	if !state.rolledBack || state.committed {
		t.Fatalf("transaction state committed=%v rolledBack=%v, want rollback only", state.committed, state.rolledBack)
	}
}

func TestUpdatePersonReplacesCurrentTeamMembership(t *testing.T) {
	state := newPersonUpdateTestState()
	state.memberships["membership-cloud"] = &personUpdateMembership{ID: "membership-cloud", EmployeeID: state.employee.ID, TeamID: "team-cloud"}
	state.memberships["membership-sec"] = &personUpdateMembership{ID: "membership-sec", EmployeeID: state.employee.ID, TeamID: "team-sec"}
	state.membershipOrder = []string{"membership-cloud", "membership-sec"}
	store := NewSQLStore(openPersonUpdateTestDB(t, state))

	teamID := "team-app"
	response, err := store.UpdatePerson(context.Background(), Principal{IsPeopleAdmin: true, Email: state.actorEmail}, state.employee.ID, PersonUpdateInput{
		FirstName: "Laura",
		LastName:  "Bianchi",
		Email:     "laura.bianchi@example.com",
		Status:    "on_leave",
		TeamID:    &teamID,
		Notes:     "Rientro previsto Q4",
	})
	if err != nil {
		t.Fatalf("UpdatePerson returned error: %v", err)
	}
	if !response.OK || response.ID != state.employee.ID || response.Status != "on_leave" {
		t.Fatalf("unexpected response: %+v", response)
	}
	if !state.committed || state.rolledBack {
		t.Fatalf("transaction state committed=%v rolledBack=%v, want commit only", state.committed, state.rolledBack)
	}
	if state.employee.FirstName != "Laura" || state.employee.LastName != "Bianchi" || state.employee.Email != "laura.bianchi@example.com" || state.employee.Status != "on_leave" || state.employee.Notes != "Rientro previsto Q4" {
		t.Fatalf("employee not updated correctly: %+v", state.employee)
	}
	if !state.memberships["membership-cloud"].Ended || !state.memberships["membership-sec"].Ended {
		t.Fatalf("previous memberships not closed: %+v", state.memberships)
	}
	active := state.activeMemberships()
	wantActive := []personUpdateMembership{{ID: "membership-new", EmployeeID: state.employee.ID, TeamID: "team-app"}}
	if !reflect.DeepEqual(active, wantActive) {
		t.Fatalf("active memberships = %+v, want %+v", active, wantActive)
	}
	if got := state.auditActions(); !reflect.DeepEqual(got, []string{
		"employee:update",
		"team_membership:close",
		"team_membership:close",
		"team_membership:create",
	}) {
		t.Fatalf("audit actions = %#v", got)
	}
}

type personUpdateEmployee struct {
	ID        string
	FirstName string
	LastName  string
	Email     string
	Status    string
	Notes     string
}

type personUpdateMembership struct {
	ID         string
	EmployeeID string
	TeamID     string
	Ended      bool
}

type personUpdateAudit struct {
	EntityType string
	Action     string
}

type personUpdateTestState struct {
	mu               sync.Mutex
	employee         personUpdateEmployee
	createdEmployee  *personUpdateEmployee
	actorEmail       string
	actorID          string
	teams            map[string]bool
	memberships      map[string]*personUpdateMembership
	membershipOrder  []string
	nextEmployeeID   string
	nextMembershipID string
	duplicateEmailID string
	employeeCreated  bool
	employeeUpdated  bool
	committed        bool
	rolledBack       bool
	audits           []personUpdateAudit
}

func newPersonUpdateTestState() *personUpdateTestState {
	return &personUpdateTestState{
		employee: personUpdateEmployee{
			ID:        "employee-1",
			FirstName: "Mario",
			LastName:  "Rossi",
			Email:     "mario.rossi@example.com",
			Status:    "active",
		},
		actorEmail:       "people@example.com",
		actorID:          "actor-1",
		teams:            map[string]bool{"team-cloud": true, "team-sec": true, "team-app": true},
		memberships:      map[string]*personUpdateMembership{},
		nextEmployeeID:   "employee-created",
		nextMembershipID: "membership-new",
	}
}

func (s *personUpdateTestState) activeMemberships() []personUpdateMembership {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := []personUpdateMembership{}
	for _, id := range s.membershipOrder {
		membership := s.memberships[id]
		if membership != nil && !membership.Ended {
			result = append(result, *membership)
		}
	}
	return result
}

func (s *personUpdateTestState) auditActions() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]string, 0, len(s.audits))
	for _, audit := range s.audits {
		result = append(result, audit.EntityType+":"+audit.Action)
	}
	return result
}

func openPersonUpdateTestDB(t *testing.T, state *personUpdateTestState) *sql.DB {
	t.Helper()
	registerPersonUpdateTestDriver()

	dsn := fmt.Sprintf("%s-%p", t.Name(), state)
	personUpdateTestStates.Store(dsn, state)
	db, err := sql.Open(personUpdateTestDriverName, dsn)
	if err != nil {
		t.Fatalf("open person update test db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = db.Close()
		personUpdateTestStates.Delete(dsn)
	})
	return db
}

const personUpdateTestDriverName = "training_person_update_test_driver"

var (
	registerPersonUpdateDriverOnce sync.Once
	personUpdateTestStates         sync.Map
)

func registerPersonUpdateTestDriver() {
	registerPersonUpdateDriverOnce.Do(func() {
		sql.Register(personUpdateTestDriverName, personUpdateTestDriver{})
	})
}

type personUpdateTestDriver struct{}

func (personUpdateTestDriver) Open(name string) (driver.Conn, error) {
	value, ok := personUpdateTestStates.Load(name)
	if !ok {
		return nil, fmt.Errorf("unknown person update test db %q", name)
	}
	return &personUpdateTestConn{state: value.(*personUpdateTestState)}, nil
}

type personUpdateTestConn struct {
	state *personUpdateTestState
}

func (c *personUpdateTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *personUpdateTestConn) Close() error { return nil }

func (c *personUpdateTestConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *personUpdateTestConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return personUpdateTestTx{state: c.state}, nil
}

func (c *personUpdateTestConn) Ping(context.Context) error { return nil }

func (c *personUpdateTestConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()

	switch {
	case strings.Contains(query, "SELECT to_jsonb(row)") && strings.Contains(query, "FROM training.employee"):
		id := namedString(args, 0)
		switch {
		case id == c.state.employee.ID:
			return newPersonUpdateJSONRows(c.state.employee)
		case c.state.createdEmployee != nil && id == c.state.createdEmployee.ID:
			return newPersonUpdateJSONRows(c.state.createdEmployee)
		default:
			return newPersonUpdateRows([]string{"to_jsonb"}, nil), nil
		}

	case strings.Contains(query, "SELECT to_jsonb(row)") && strings.Contains(query, "FROM training.team_membership"):
		id := namedString(args, 0)
		membership := c.state.memberships[id]
		if membership == nil {
			return newPersonUpdateRows([]string{"to_jsonb"}, nil), nil
		}
		return newPersonUpdateJSONRows(membership)

	case strings.Contains(query, "AND id <> $2::uuid"):
		if c.state.duplicateEmailID == "" {
			return newPersonUpdateRows([]string{"id"}, nil), nil
		}
		return newPersonUpdateRows([]string{"id"}, [][]driver.Value{{c.state.duplicateEmailID}}), nil

	case strings.Contains(query, "SELECT is_active") && strings.Contains(query, "FROM training.team"):
		teamID := namedString(args, 0)
		active, ok := c.state.teams[teamID]
		if !ok {
			return newPersonUpdateRows([]string{"is_active"}, nil), nil
		}
		return newPersonUpdateRows([]string{"is_active"}, [][]driver.Value{{active}}), nil

	case strings.Contains(query, "FROM training.team_membership") && strings.Contains(query, "ORDER BY start_date DESC"):
		values := [][]driver.Value{}
		employeeID := namedString(args, 0)
		for _, id := range c.state.membershipOrder {
			membership := c.state.memberships[id]
			if membership != nil && membership.EmployeeID == employeeID && !membership.Ended {
				values = append(values, []driver.Value{membership.ID, membership.TeamID})
			}
		}
		return newPersonUpdateRows([]string{"id", "team_id"}, values), nil

	case strings.Contains(query, "UPDATE training.employee") && strings.Contains(query, "RETURNING id::text, status::text"):
		id := namedString(args, 0)
		if id != c.state.employee.ID {
			return newPersonUpdateRows([]string{"id", "status"}, nil), nil
		}
		c.state.employee.FirstName = namedString(args, 1)
		c.state.employee.LastName = namedString(args, 2)
		c.state.employee.Email = namedString(args, 3)
		c.state.employee.Status = namedString(args, 4)
		c.state.employee.Notes = namedString(args, 5)
		c.state.employeeUpdated = true
		return newPersonUpdateRows([]string{"id", "status"}, [][]driver.Value{{c.state.employee.ID, c.state.employee.Status}}), nil

	case strings.Contains(query, "INSERT INTO training.employee"):
		employee := &personUpdateEmployee{
			ID:        c.state.nextEmployeeID,
			FirstName: namedString(args, 0),
			LastName:  namedString(args, 1),
			Email:     namedString(args, 2),
			Status:    namedString(args, 3),
			Notes:     namedString(args, 4),
		}
		c.state.createdEmployee = employee
		c.state.employeeCreated = true
		return newPersonUpdateRows([]string{"id", "status"}, [][]driver.Value{{employee.ID, employee.Status}}), nil

	case strings.Contains(query, "INSERT INTO training.team_membership"):
		employeeID := namedString(args, 0)
		teamID := namedString(args, 1)
		id := c.state.nextMembershipID
		c.state.memberships[id] = &personUpdateMembership{ID: id, EmployeeID: employeeID, TeamID: teamID}
		c.state.membershipOrder = append(c.state.membershipOrder, id)
		return newPersonUpdateRows([]string{"id"}, [][]driver.Value{{id}}), nil

	case strings.Contains(query, "FROM training.employee") && strings.Contains(query, "WHERE email = $1") && strings.Contains(query, "LIMIT 1"):
		email := namedString(args, 0)
		switch email {
		case c.state.actorEmail:
			return newPersonUpdateRows([]string{"id"}, [][]driver.Value{{c.state.actorID}}), nil
		case c.state.employee.Email:
			return newPersonUpdateRows([]string{"id"}, [][]driver.Value{{c.state.employee.ID}}), nil
		case func() string {
			if c.state.createdEmployee == nil {
				return ""
			}
			return c.state.createdEmployee.Email
		}():
			return newPersonUpdateRows([]string{"id"}, [][]driver.Value{{c.state.createdEmployee.ID}}), nil
		default:
			return newPersonUpdateRows([]string{"id"}, nil), nil
		}
	}
	return nil, fmt.Errorf("unexpected query: %s", query)
}

func (c *personUpdateTestConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()

	switch {
	case strings.Contains(query, "UPDATE training.team_membership") && strings.Contains(query, "SET end_date = now()"):
		id := namedString(args, 0)
		membership := c.state.memberships[id]
		if membership != nil {
			membership.Ended = true
		}
		return driver.RowsAffected(1), nil

	case strings.Contains(query, "INSERT INTO training.audit_log"):
		c.state.audits = append(c.state.audits, personUpdateAudit{
			EntityType: namedString(args, 1),
			Action:     namedString(args, 3),
		})
		return driver.RowsAffected(1), nil
	}
	return nil, fmt.Errorf("unexpected exec: %s", query)
}

type personUpdateTestTx struct {
	state *personUpdateTestState
}

func (tx personUpdateTestTx) Commit() error {
	tx.state.mu.Lock()
	defer tx.state.mu.Unlock()
	tx.state.committed = true
	return nil
}

func (tx personUpdateTestTx) Rollback() error {
	tx.state.mu.Lock()
	defer tx.state.mu.Unlock()
	tx.state.rolledBack = true
	return nil
}

type personUpdateRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func newPersonUpdateRows(columns []string, values [][]driver.Value) *personUpdateRows {
	return &personUpdateRows{columns: columns, values: values}
}

func newPersonUpdateJSONRows(value any) (driver.Rows, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return newPersonUpdateRows([]string{"to_jsonb"}, [][]driver.Value{{raw}}), nil
}

func (r *personUpdateRows) Columns() []string {
	return r.columns
}

func (r *personUpdateRows) Close() error { return nil }

func (r *personUpdateRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func namedString(args []driver.NamedValue, index int) string {
	if index >= len(args) || args[index].Value == nil {
		return ""
	}
	return fmt.Sprint(args[index].Value)
}

var _ driver.Driver = personUpdateTestDriver{}
var _ driver.Conn = (*personUpdateTestConn)(nil)
var _ driver.ConnBeginTx = (*personUpdateTestConn)(nil)
var _ driver.QueryerContext = (*personUpdateTestConn)(nil)
var _ driver.ExecerContext = (*personUpdateTestConn)(nil)
var _ driver.Pinger = (*personUpdateTestConn)(nil)
