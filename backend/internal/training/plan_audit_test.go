package training

import (
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestPlanAuditEventRegistryIsComplete(t *testing.T) {
	want := []string{
		"adhoc_created",
		"bulk_plan_applied",
		"bulk_review_applied",
		"enrollment_cancelled",
		"enrollment_modified",
		"plan_budget_changed",
		"plan_created",
		"plan_deleted",
		"plan_notes_changed",
		"plan_status_changed",
		"suggestion_dismissed",
	}
	if got := planAuditEventTypes(); !reflect.DeepEqual(got, want) {
		t.Fatalf("planAuditEventTypes() = %#v, want %#v", got, want)
	}
	for _, eventType := range want {
		if !knownPlanAuditEvent(eventType) {
			t.Fatalf("knownPlanAuditEvent(%q) = false", eventType)
		}
	}
}

func TestBulkPlanAuditEventType(t *testing.T) {
	suggestionID := "suggestion-1"
	if got := bulkPlanAuditEventType(&suggestionID); got != PlanAuditBulkPlanApplied {
		t.Fatalf("bulkPlanAuditEventType(suggestion) = %q", got)
	}
	blank := "  "
	if got := bulkPlanAuditEventType(&blank); got != PlanAuditAdhocCreated {
		t.Fatalf("bulkPlanAuditEventType(blank) = %q", got)
	}
	if got := bulkPlanAuditEventType(nil); got != PlanAuditAdhocCreated {
		t.Fatalf("bulkPlanAuditEventType(nil) = %q", got)
	}
}

func TestValidateUpdatePlanInput(t *testing.T) {
	if err := validateUpdatePlanInput(UpdatePlanInput{}); err == nil {
		t.Fatal("empty update should fail")
	}
	negative := -1.0
	if err := validateUpdatePlanInput(UpdatePlanInput{BudgetTotal: &negative}); err == nil {
		t.Fatal("negative budget should fail")
	}
	zero := 0.0
	if err := validateUpdatePlanInput(UpdatePlanInput{BudgetTotal: &zero}); err != nil {
		t.Fatalf("zero budget should pass: %v", err)
	}
	notes := ""
	if err := validateUpdatePlanInput(UpdatePlanInput{Notes: &notes}); err != nil {
		t.Fatalf("notes-only update should pass: %v", err)
	}
}

func TestCanDeletePlan(t *testing.T) {
	cases := []struct {
		name        string
		status      string
		enrollments int
		want        bool
	}{
		{name: "draft empty", status: "draft", enrollments: 0, want: true},
		{name: "draft with enrollments", status: "draft", enrollments: 1, want: false},
		{name: "open empty", status: "open", enrollments: 0, want: false},
		{name: "closed empty", status: "closed", enrollments: 0, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := canDeletePlan(tc.status, tc.enrollments); got != tc.want {
				t.Fatalf("canDeletePlan(%q, %d) = %v, want %v", tc.status, tc.enrollments, got, tc.want)
			}
		})
	}
}

func TestOneOpenPlanUniqueViolation(t *testing.T) {
	err := &pgconn.PgError{Code: "23505", ConstraintName: "idx_training_one_open_plan"}
	if !isOneOpenPlanUniqueViolation(err) {
		t.Fatal("expected one-open-plan unique violation")
	}
	other := &pgconn.PgError{Code: "23505", ConstraintName: "training_plan_year_key"}
	if isOneOpenPlanUniqueViolation(other) {
		t.Fatal("year uniqueness should not be translated to another_plan_open")
	}
}
