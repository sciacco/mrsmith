package training

import "testing"

func TestEnrollmentTransitionsMatchApprovedMatrix(t *testing.T) {
	cases := []struct {
		name       string
		current    EnrollmentState
		transition EnrollmentTransition
		ctx        TransitionContext
		wantOK     bool
		wantTarget string
		wantCode   string
	}{
		{
			name:       "people approves proposed enrollment",
			current:    EnrollmentProposed,
			transition: EnrollmentApprove,
			ctx:        TransitionContext{Actor: ActorPeopleAdmin, PlanStatus: "open"},
			wantOK:     true,
			wantTarget: string(EnrollmentApproved),
		},
		{
			name:       "employee cannot approve enrollment",
			current:    EnrollmentProposed,
			transition: EnrollmentApprove,
			ctx:        TransitionContext{Actor: ActorEmployee, PlanStatus: "open"},
			wantCode:   "UNAUTHORIZED_ACTOR",
		},
		{
			name:       "revert requires reason",
			current:    EnrollmentApproved,
			transition: EnrollmentRevertToProposed,
			ctx:        TransitionContext{Actor: ActorPeopleAdmin, PlanStatus: "open"},
			wantCode:   "REASON_REQUIRED",
		},
		{
			name:       "closed plan blocks people changes",
			current:    EnrollmentProposed,
			transition: EnrollmentApprove,
			ctx:        TransitionContext{Actor: ActorPeopleAdmin, PlanStatus: "closed"},
			wantCode:   "PLAN_CLOSED",
		},
		{
			name:       "system expires approved enrollment only on closed plan",
			current:    EnrollmentApproved,
			transition: EnrollmentExpire,
			ctx:        TransitionContext{Actor: ActorSystem, PlanStatus: "closed"},
			wantOK:     true,
			wantTarget: string(EnrollmentExpired),
		},
		{
			name:       "complete cannot jump back to proposed",
			current:    EnrollmentCompleted,
			transition: EnrollmentRevertToProposed,
			ctx:        TransitionContext{Actor: ActorPeopleAdmin, Reason: "correzione"},
			wantCode:   "INVALID_TRANSITION",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := AttemptEnrollmentTransition(tc.current, tc.transition, tc.ctx)
			if got.OK != tc.wantOK {
				t.Fatalf("OK = %v, want %v (%+v)", got.OK, tc.wantOK, got)
			}
			if got.Target != tc.wantTarget {
				t.Fatalf("Target = %q, want %q", got.Target, tc.wantTarget)
			}
			if got.Code != tc.wantCode {
				t.Fatalf("Code = %q, want %q", got.Code, tc.wantCode)
			}
		})
	}
}

func TestRequestConvertRequiresOpenablePlan(t *testing.T) {
	got := AttemptRequestTransition(
		RequestAccepted,
		RequestConvert,
		TransitionContext{Actor: ActorPeopleAdmin},
	)
	if got.OK {
		t.Fatalf("convert without openable plan should fail")
	}
	if got.Code != "PLAN_NOT_OPENABLE" {
		t.Fatalf("Code = %q, want PLAN_NOT_OPENABLE", got.Code)
	}

	got = AttemptRequestTransition(
		RequestAccepted,
		RequestConvert,
		TransitionContext{Actor: ActorPeopleAdmin, TargetPlanIsOpenable: true},
	)
	if !got.OK || got.Target != string(RequestConverted) {
		t.Fatalf("convert result = %+v, want converted", got)
	}
}
