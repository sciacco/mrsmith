package training

import (
	"strings"
	"time"
)

type Actor string

const (
	ActorEmployee    Actor = "employee"
	ActorManager     Actor = "manager"
	ActorPeopleAdmin Actor = "people_admin"
	ActorSystem      Actor = "system"
)

type TransitionContext struct {
	Actor                  Actor
	Reason                 string
	PlanStatus             string
	ActualStart            *time.Time
	HasLinkedCertification *bool
	TargetPlanIsOpenable   bool
}

type TransitionResult struct {
	OK      bool   `json:"ok"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Target  string `json:"target,omitempty"`
}

func allow(target string) TransitionResult {
	return TransitionResult{OK: true, Target: target}
}

func deny(code, message string) TransitionResult {
	return TransitionResult{OK: false, Code: code, Message: message}
}

func requireActor(ctx TransitionContext, allowed ...Actor) *TransitionResult {
	for _, actor := range allowed {
		if ctx.Actor == actor {
			return nil
		}
	}
	return ptr(deny("UNAUTHORIZED_ACTOR", "attore non autorizzato per questa transizione"))
}

func requireReason(ctx TransitionContext) *TransitionResult {
	if len(strings.TrimSpace(ctx.Reason)) < 3 {
		return ptr(deny("REASON_REQUIRED", "una motivazione di almeno 3 caratteri e obbligatoria"))
	}
	return nil
}

func ptr[T any](value T) *T {
	return &value
}

type EnrollmentState string
type EnrollmentTransition string

const (
	EnrollmentProposed   EnrollmentState = "proposed"
	EnrollmentApproved   EnrollmentState = "approved"
	EnrollmentInProgress EnrollmentState = "in_progress"
	EnrollmentCompleted  EnrollmentState = "completed"
	EnrollmentFailed     EnrollmentState = "failed"
	EnrollmentCancelled  EnrollmentState = "cancelled"
	EnrollmentExpired    EnrollmentState = "expired"

	EnrollmentApprove          EnrollmentTransition = "approve"
	EnrollmentRevertToProposed EnrollmentTransition = "revert_to_proposed"
	EnrollmentStart            EnrollmentTransition = "start"
	EnrollmentComplete         EnrollmentTransition = "complete"
	EnrollmentFail             EnrollmentTransition = "fail"
	EnrollmentCancel           EnrollmentTransition = "cancel"
	EnrollmentExpire           EnrollmentTransition = "expire"
	EnrollmentReopen           EnrollmentTransition = "reopen"
)

var enrollmentTargets = map[EnrollmentState]map[EnrollmentTransition]EnrollmentState{
	EnrollmentProposed: {
		EnrollmentApprove: EnrollmentApproved,
		EnrollmentCancel:  EnrollmentCancelled,
		EnrollmentExpire:  EnrollmentExpired,
	},
	EnrollmentApproved: {
		EnrollmentRevertToProposed: EnrollmentProposed,
		EnrollmentStart:            EnrollmentInProgress,
		EnrollmentCancel:           EnrollmentCancelled,
		EnrollmentExpire:           EnrollmentExpired,
	},
	EnrollmentInProgress: {
		EnrollmentComplete: EnrollmentCompleted,
		EnrollmentFail:     EnrollmentFailed,
		EnrollmentCancel:   EnrollmentCancelled,
	},
	EnrollmentCompleted: {EnrollmentReopen: EnrollmentInProgress},
	EnrollmentFailed:    {EnrollmentReopen: EnrollmentInProgress},
	EnrollmentCancelled: {EnrollmentReopen: EnrollmentInProgress},
	EnrollmentExpired:   {EnrollmentReopen: EnrollmentInProgress},
}

func IsEnrollmentTerminal(state EnrollmentState) bool {
	switch state {
	case EnrollmentCompleted, EnrollmentFailed, EnrollmentCancelled, EnrollmentExpired:
		return true
	default:
		return false
	}
}

func AttemptEnrollmentTransition(
	current EnrollmentState,
	transition EnrollmentTransition,
	ctx TransitionContext,
) TransitionResult {
	target, ok := enrollmentTargets[current][transition]
	if !ok {
		return deny("INVALID_TRANSITION", "transizione non consentita dallo stato corrente")
	}

	switch transition {
	case EnrollmentApprove:
		if r := requireActor(ctx, ActorPeopleAdmin); r != nil {
			return *r
		}
		if ctx.PlanStatus == "closed" {
			return deny("PLAN_CLOSED", "impossibile modificare iscrizioni di un piano chiuso")
		}
	case EnrollmentRevertToProposed:
		if r := requireActor(ctx, ActorPeopleAdmin); r != nil {
			return *r
		}
		if ctx.PlanStatus == "closed" {
			return deny("PLAN_CLOSED", "impossibile modificare iscrizioni di un piano chiuso")
		}
		if r := requireReason(ctx); r != nil {
			return *r
		}
	case EnrollmentStart:
		if r := requireActor(ctx, ActorPeopleAdmin, ActorEmployee); r != nil {
			return *r
		}
		if ctx.ActualStart != nil && ctx.ActualStart.After(time.Now()) {
			return deny("ACTUAL_START_IN_FUTURE", "la data di inizio effettivo non puo essere nel futuro")
		}
	case EnrollmentComplete:
		if r := requireActor(ctx, ActorPeopleAdmin, ActorEmployee); r != nil {
			return *r
		}
	case EnrollmentFail:
		if r := requireActor(ctx, ActorPeopleAdmin); r != nil {
			return *r
		}
		if ctx.HasLinkedCertification != nil && !*ctx.HasLinkedCertification {
			return deny("FAIL_REQUIRES_CERT", "stato non superato applicabile solo a corsi collegati a certificazione")
		}
	case EnrollmentCancel:
		if r := requireActor(ctx, ActorPeopleAdmin, ActorManager); r != nil {
			return *r
		}
		if r := requireReason(ctx); r != nil {
			return *r
		}
	case EnrollmentExpire:
		if r := requireActor(ctx, ActorSystem); r != nil {
			return *r
		}
		if ctx.PlanStatus != "closed" {
			return deny("PLAN_NOT_CLOSED", "scadenza consentita solo alla chiusura del piano")
		}
	case EnrollmentReopen:
		if r := requireActor(ctx, ActorPeopleAdmin); r != nil {
			return *r
		}
		if r := requireReason(ctx); r != nil {
			return *r
		}
	}

	return allow(string(target))
}

type AwardOutcome string
type AwardTransition string

const (
	AwardInProgress     AwardOutcome    = "in_progress"
	AwardPassedExam     AwardOutcome    = "passed_exam"
	AwardFailedExam     AwardOutcome    = "failed_exam"
	AwardAttendance     AwardOutcome    = "attendance_only"
	AwardIssue          AwardTransition = "issue"
	AwardMarkPassed     AwardTransition = "mark_passed"
	AwardMarkFailed     AwardTransition = "mark_failed"
	AwardMarkAttendance AwardTransition = "mark_attendance"
	AwardCorrect        AwardTransition = "correct"
)

var awardTargets = map[AwardOutcome]map[AwardTransition]AwardOutcome{
	AwardInProgress: {
		AwardMarkPassed:     AwardPassedExam,
		AwardMarkFailed:     AwardFailedExam,
		AwardMarkAttendance: AwardAttendance,
	},
	AwardPassedExam: {AwardCorrect: AwardPassedExam},
	AwardFailedExam: {AwardCorrect: AwardFailedExam},
	AwardAttendance: {AwardCorrect: AwardAttendance},
}

func AttemptAwardTransition(
	current AwardOutcome,
	transition AwardTransition,
	ctx TransitionContext,
) TransitionResult {
	if transition == AwardIssue {
		return allow("")
	}

	target, ok := awardTargets[current][transition]
	if !ok {
		return deny("INVALID_TRANSITION", "transizione non consentita dall'esito corrente")
	}

	if transition == AwardCorrect {
		if r := requireActor(ctx, ActorPeopleAdmin); r != nil {
			return *r
		}
		if r := requireReason(ctx); r != nil {
			return *r
		}
		return allow(string(target))
	}

	if r := requireActor(ctx, ActorPeopleAdmin, ActorEmployee); r != nil {
		return *r
	}
	return allow(string(target))
}

type RequestState string
type RequestTransition string

const (
	RequestSubmitted   RequestState = "submitted"
	RequestUnderReview RequestState = "under_review"
	RequestAccepted    RequestState = "accepted"
	RequestRejected    RequestState = "rejected"
	RequestConverted   RequestState = "converted"

	RequestSubmit      RequestTransition = "submit"
	RequestStartReview RequestTransition = "start_review"
	RequestAccept      RequestTransition = "accept"
	RequestReject      RequestTransition = "reject"
	RequestConvert     RequestTransition = "convert"
	RequestWithdraw    RequestTransition = "withdraw"
)

var requestTargets = map[RequestState]map[RequestTransition]RequestState{
	RequestSubmitted: {
		RequestStartReview: RequestUnderReview,
		RequestWithdraw:    RequestRejected,
	},
	RequestUnderReview: {
		RequestAccept:   RequestAccepted,
		RequestReject:   RequestRejected,
		RequestWithdraw: RequestRejected,
	},
	RequestAccepted:  {RequestConvert: RequestConverted},
	RequestRejected:  {},
	RequestConverted: {},
}

func AttemptRequestTransition(
	current RequestState,
	transition RequestTransition,
	ctx TransitionContext,
) TransitionResult {
	if transition == RequestSubmit {
		if r := requireActor(ctx, ActorEmployee); r != nil {
			return *r
		}
		return allow("")
	}

	target, ok := requestTargets[current][transition]
	if !ok {
		return deny("INVALID_TRANSITION", "transizione non consentita dallo stato corrente")
	}

	switch transition {
	case RequestStartReview, RequestAccept:
		if r := requireActor(ctx, ActorPeopleAdmin); r != nil {
			return *r
		}
	case RequestReject:
		if r := requireActor(ctx, ActorPeopleAdmin); r != nil {
			return *r
		}
		if r := requireReason(ctx); r != nil {
			return *r
		}
	case RequestConvert:
		if r := requireActor(ctx, ActorPeopleAdmin); r != nil {
			return *r
		}
		if !ctx.TargetPlanIsOpenable {
			return deny("PLAN_NOT_OPENABLE", "serve un piano apribile per convertire la richiesta")
		}
	case RequestWithdraw:
		if r := requireActor(ctx, ActorEmployee); r != nil {
			return *r
		}
	}

	return allow(string(target))
}
