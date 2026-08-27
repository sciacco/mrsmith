package training

import (
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// registerQueueRoutes registra le route delle code operative derivate (#140,
// §4-Code). Tutte in sola lettura: le code sono query sui fatti, nessun
// endpoint scrive.
func (h *handler) registerQueueRoutes(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	mux.Handle("GET /training/v1/queues/requests-without-tl-opinion", protect(h.requireStore(http.HandlerFunc(h.handleQueueRequestsWithoutTLOpinion))))
	mux.Handle("GET /training/v1/queues/requests-awaiting-decision", protect(h.requireStore(http.HandlerFunc(h.handleQueueRequestsAwaitingDecision))))
	mux.Handle("GET /training/v1/queues/seat-rule-coverage", protect(h.requireStore(http.HandlerFunc(h.handleQueueSeatRuleCoverage))))
	mux.Handle("GET /training/v1/queues/expiring-coverage", protect(h.requireStore(http.HandlerFunc(h.handleQueueExpiringCoverage))))
	mux.Handle("GET /training/v1/queues/unfed-population", protect(h.requireStore(http.HandlerFunc(h.handleQueueUnfedPopulation))))
	mux.Handle("GET /training/v1/queues/rounds-without-event", protect(h.requireStore(http.HandlerFunc(h.handleQueueRoundsWithoutEvent))))
	mux.Handle("GET /training/v1/queues/unapproved-event-expenses", protect(h.requireStore(http.HandlerFunc(h.handleQueueUnapprovedEventExpenses))))
	mux.Handle("GET /training/v1/queues/stale-enrollments", protect(h.requireStore(http.HandlerFunc(h.handleQueueStaleEnrollments))))
}

func (h *handler) handleQueueRequestsWithoutTLOpinion(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.principalOrUnauthorized(w, r); !ok {
		return
	}
	requests, err := h.store.QueueRequestsWithoutTLOpinion(r.Context())
	if err != nil {
		h.writeActionError(w, r, err, "training.queue_requests_without_tl_opinion")
		return
	}
	httputil.JSON(w, http.StatusOK, RequestsWithoutTLOpinionResponse{Requests: requests})
}

func (h *handler) handleQueueRequestsAwaitingDecision(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.principalOrUnauthorized(w, r); !ok {
		return
	}
	requests, err := h.store.QueueRequestsAwaitingDecision(r.Context())
	if err != nil {
		h.writeActionError(w, r, err, "training.queue_requests_awaiting_decision")
		return
	}
	httputil.JSON(w, http.StatusOK, RequestsAwaitingDecisionResponse{Requests: requests})
}

func (h *handler) handleQueueSeatRuleCoverage(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.principalOrUnauthorized(w, r); !ok {
		return
	}
	rules, err := h.store.QueueSeatRuleCoverage(r.Context())
	if err != nil {
		h.writeActionError(w, r, err, "training.queue_seat_rule_coverage")
		return
	}
	httputil.JSON(w, http.StatusOK, SeatRuleCoverageResponse{Rules: rules})
}

func (h *handler) handleQueueExpiringCoverage(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.principalOrUnauthorized(w, r); !ok {
		return
	}
	response, err := h.store.QueueExpiringCoverage(r.Context(), r.URL.Query().Get("withinDays"))
	if err != nil {
		h.writeActionError(w, r, err, "training.queue_expiring_coverage")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleQueueUnfedPopulation(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.principalOrUnauthorized(w, r); !ok {
		return
	}
	rules, err := h.store.QueueUnfedPopulation(r.Context())
	if err != nil {
		h.writeActionError(w, r, err, "training.queue_unfed_population")
		return
	}
	httputil.JSON(w, http.StatusOK, UnfedPopulationResponse{Rules: rules})
}

func (h *handler) handleQueueUnapprovedEventExpenses(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	locals, err := h.store.QueueUnapprovedEventExpenses(r.Context())
	if err != nil {
		h.writeActionError(w, r, err, "training.queue_unapproved_event_expenses")
		return
	}
	expenseLocals := make([]eventExpenseLocal, 0, len(locals))
	for _, local := range locals {
		expenseLocals = append(expenseLocals, local.Expense)
	}
	hydrated, err := h.hydrateEventExpenses(r.Context(), principal.Email, expenseLocals)
	if err != nil {
		h.writeActionError(w, r, err, "training.hydrate_unapproved_event_expenses")
		return
	}
	response := UnapprovedEventExpensesResponse{Expenses: make([]UnapprovedEventExpenseRow, 0, len(locals))}
	for index, expense := range hydrated {
		if expense.EconomicState == EconomicStateApproved {
			continue
		}
		local := locals[index]
		response.Expenses = append(response.Expenses, UnapprovedEventExpenseRow{
			EventID:         local.Expense.EventID,
			CourseID:        local.CourseID,
			CourseTitle:     local.CourseTitle,
			ExpenseID:       expense.ID,
			POID:            expense.POID,
			POCode:          expense.POCode,
			RawState:        expense.RawState,
			EconomicState:   expense.EconomicState,
			TotalPrice:      expense.TotalPrice,
			Currency:        expense.Currency,
			Budget:          expense.Budget,
			EnrollmentCount: local.EnrollmentCount,
		})
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleQueueRoundsWithoutEvent(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.principalOrUnauthorized(w, r); !ok {
		return
	}
	response, err := h.store.QueueRoundsWithoutEvent(r.Context(), r.URL.Query().Get("withinDays"))
	if err != nil {
		h.writeActionError(w, r, err, "training.queue_rounds_without_event")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleQueueStaleEnrollments(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.principalOrUnauthorized(w, r); !ok {
		return
	}
	response, err := h.store.QueueStaleEnrollments(r.Context(), r.URL.Query().Get("olderThanDays"))
	if err != nil {
		h.writeActionError(w, r, err, "training.queue_stale_enrollments")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}
