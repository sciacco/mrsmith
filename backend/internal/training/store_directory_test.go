package training

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestListPeopleDirectoryFlagsFiltersAndPriority(t *testing.T) {
	store := NewSQLStore(openTrainingDirectoryTestDB(t, "directory"))

	people, err := store.ListPeopleDirectory(context.Background(), Principal{IsPeopleAdmin: true}, PeopleDirectoryFilters{Year: 2026})
	if err != nil {
		t.Fatalf("ListPeopleDirectory returned error: %v", err)
	}
	if len(people) != 7 {
		t.Fatalf("expected 7 people, got %d", len(people))
	}

	byID := peopleByID(people)
	zero := byID["zero"]
	if dominantPersonFlag(zero.Flags) != "" {
		t.Fatalf("zero flag row dominant flag = %q", dominantPersonFlag(zero.Flags))
	}

	toPlan := byID["to-plan"]
	if !toPlan.Flags.DaPianificare || !toPlan.Flags.ComplianceGap {
		t.Fatalf("da_pianificare must cascade into compliance_gap, got %+v", toPlan.Flags)
	}

	expiring := byID["expiring"]
	if expiring.NextDeadline == nil || expiring.NextDeadline.Type != "cert" || expiring.NextDeadline.Date != "2026-06-19" {
		t.Fatalf("unexpected expiring deadline: %+v", expiring.NextDeadline)
	}

	if !(toPlan.PriorityScore > byID["gap"].PriorityScore &&
		byID["gap"].PriorityScore > byID["expiring"].PriorityScore &&
		byID["expiring"].PriorityScore > byID["failed"].PriorityScore &&
		byID["failed"].PriorityScore > byID["no-active"].PriorityScore &&
		byID["no-active"].PriorityScore > zero.PriorityScore) {
		t.Fatalf("priority scores not in dominant flag order: %+v", map[string]float64{
			"to-plan":   toPlan.PriorityScore,
			"gap":       byID["gap"].PriorityScore,
			"expiring":  byID["expiring"].PriorityScore,
			"failed":    byID["failed"].PriorityScore,
			"no-active": byID["no-active"].PriorityScore,
			"zero":      zero.PriorityScore,
		})
	}

	filterCases := []struct {
		filter string
		want   []string
	}{
		{filter: "da_pianificare", want: []string{"multi", "to-plan"}},
		{filter: "compliance_gap", want: []string{"gap", "multi", "to-plan"}},
		{filter: "scadenze_imminenti", want: []string{"expiring", "multi"}},
		{filter: "failed_recente", want: []string{"failed", "multi"}},
		{filter: "senza_formazione_attiva", want: []string{"multi", "no-active"}},
		{filter: "legacy_status", want: []string{"expiring", "failed", "gap", "multi", "no-active", "to-plan", "zero"}},
	}

	for _, tc := range filterCases {
		t.Run(tc.filter, func(t *testing.T) {
			got, err := store.ListPeopleDirectory(context.Background(), Principal{IsPeopleAdmin: true}, PeopleDirectoryFilters{Year: 2026, Filter: tc.filter})
			if err != nil {
				t.Fatalf("ListPeopleDirectory filter %q returned error: %v", tc.filter, err)
			}
			gotIDs := peopleIDs(got)
			sort.Strings(gotIDs)
			sort.Strings(tc.want)
			if !reflect.DeepEqual(gotIDs, tc.want) {
				t.Fatalf("filter %q ids = %#v, want %#v", tc.filter, gotIDs, tc.want)
			}
		})
	}
}

func TestEffectiveDirectoryYear(t *testing.T) {
	now := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	if got := effectiveDirectoryYear(2025, now); got != 2025 {
		t.Fatalf("requested year = %d, want 2025", got)
	}
	if got := effectiveDirectoryYear(0, now); got != 2026 {
		t.Fatalf("default year = %d, want 2026", got)
	}
}

func peopleByID(people []PersonSummary) map[string]PersonSummary {
	result := make(map[string]PersonSummary, len(people))
	for _, person := range people {
		result[person.ID] = person
	}
	return result
}

func peopleIDs(people []PersonSummary) []string {
	result := make([]string, 0, len(people))
	for _, person := range people {
		result = append(result, person.ID)
	}
	return result
}

func openTrainingDirectoryTestDB(t *testing.T, mode string) *sql.DB {
	t.Helper()
	registerTrainingDirectoryTestDriver()

	db, err := sql.Open(trainingDirectoryTestDriverName, mode)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

const trainingDirectoryTestDriverName = "training_directory_test_driver"

var registerTrainingDirectoryDriverOnce sync.Once

func registerTrainingDirectoryTestDriver() {
	registerTrainingDirectoryDriverOnce.Do(func() {
		sql.Register(trainingDirectoryTestDriverName, trainingDirectoryTestDriver{})
	})
}

type trainingDirectoryTestDriver struct{}

func (trainingDirectoryTestDriver) Open(name string) (driver.Conn, error) {
	return trainingDirectoryTestConn{mode: name}, nil
}

type trainingDirectoryTestConn struct {
	mode string
}

func (c trainingDirectoryTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c trainingDirectoryTestConn) Close() error { return nil }

func (c trainingDirectoryTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func (c trainingDirectoryTestConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if c.mode == "directory" && strings.Contains(query, "WITH active_emp AS") {
		return &trainingDirectoryTestRows{
			columns: []string{
				"id", "name", "email", "team_code",
				"gaps_open", "active_enrollments_count", "expiring_certs_count", "hist_count",
				"da_pianificare", "compliance_gap", "scadenze_imminenti", "failed_recente", "senza_formazione_attiva",
				"deadline_type", "deadline_date", "deadline_label",
			},
			values: [][]driver.Value{
				{"to-plan", "Verdi Ada", "ada.verdi@example.com", "CLOUD", int64(1), int64(1), int64(0), int64(4), true, true, false, false, false, nil, nil, nil},
				{"gap", "Rossi Marco", "marco.rossi@example.com", "APP", int64(1), int64(1), int64(0), int64(6), false, true, false, false, false, nil, nil, nil},
				{"expiring", "Bianchi Laura", "laura.bianchi@example.com", "CLOUD", int64(0), int64(1), int64(1), int64(5), false, false, true, false, false, "cert", "2026-06-19", "Cert in scadenza"},
				{"failed", "Neri Federico", "federico.neri@example.com", "SEC", int64(0), int64(1), int64(0), int64(5), false, false, false, true, false, nil, nil, nil},
				{"no-active", "Gallo Giulia", "giulia.gallo@example.com", "PEOPLE", int64(0), int64(0), int64(0), int64(2), false, false, false, false, true, nil, nil, nil},
				{"multi", "Conti Marta", "marta.conti@example.com", "APP", int64(2), int64(0), int64(1), int64(3), true, true, true, true, true, "mandatory_due", "2026-06-04", "Ricorrenza obbligatoria"},
				{"zero", "Ferri Nadia", "nadia.ferri@example.com", "FIN", int64(0), int64(2), int64(0), int64(9), false, false, false, false, false, "course_end", "2026-07-10", "Fine corso prevista"},
			},
		}, nil
	}
	return nil, errors.New("unexpected query")
}

func (c trainingDirectoryTestConn) Ping(context.Context) error { return nil }

var _ driver.QueryerContext = trainingDirectoryTestConn{}
var _ driver.Pinger = trainingDirectoryTestConn{}

type trainingDirectoryTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *trainingDirectoryTestRows) Columns() []string {
	return r.columns
}

func (r *trainingDirectoryTestRows) Close() error { return nil }

func (r *trainingDirectoryTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}
