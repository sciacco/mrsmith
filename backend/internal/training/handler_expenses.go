package training

import (
	"context"
	"fmt"
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// hydrateEventExpenses resolves each distinct local PO once for this request,
// then restores the local ordering in the response. It has no cross-request
// cache: Arak remains the source of truth for economic data.
func (h *handler) hydrateEventExpenses(ctx context.Context, email string, locals []eventExpenseLocal) ([]EventExpense, error) {
	if len(locals) == 0 {
		return make([]EventExpense, 0), nil
	}
	if h.arak == nil {
		return nil, serviceUnavailableError("arak_not_configured", "servizio PO temporaneamente non disponibile")
	}

	purchaseOrders := make(map[int64]PurchaseOrder, len(locals))
	result := make([]EventExpense, 0, len(locals))
	for _, local := range locals {
		po, ok := purchaseOrders[local.POID]
		if !ok {
			var err error
			po, err = h.getPurchaseOrder(ctx, email, local.POID)
			if err != nil {
				return nil, fmt.Errorf("hydrate training event expense PO %d: %w", local.POID, err)
			}
			purchaseOrders[local.POID] = po
		}
		result = append(result, hydrateEventExpense(local, po))
	}
	return result, nil
}

func (h *handler) handleCreateEventExpense(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[EventExpenseInput](w, r)
	if !ok {
		return
	}
	po, err := h.resolvePurchaseOrder(r.Context(), principal, input.POReference)
	if err != nil {
		h.writeActionError(w, r, err, "training.resolve_event_expense_po")
		return
	}
	expense, err := h.store.CreateEventExpense(r.Context(), principal, r.PathValue("id"), po.ID, input.EnrollmentIDs)
	if err != nil {
		h.writeActionError(w, r, err, "training.create_event_expense")
		return
	}
	httputil.JSON(w, http.StatusCreated, hydrateEventExpense(expense, po))
}

func (h *handler) handleReplaceEventExpensePO(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[EventExpenseReplaceInput](w, r)
	if !ok {
		return
	}
	po, err := h.resolvePurchaseOrder(r.Context(), principal, input.POReference)
	if err != nil {
		h.writeActionError(w, r, err, "training.resolve_event_expense_po")
		return
	}
	expense, err := h.store.ReplaceEventExpensePO(r.Context(), principal, r.PathValue("id"), po.ID)
	if err != nil {
		h.writeActionError(w, r, err, "training.replace_event_expense_po")
		return
	}
	httputil.JSON(w, http.StatusOK, hydrateEventExpense(expense, po))
}

func (h *handler) handleReplaceEventExpenseEnrollments(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[EventExpenseEnrollmentsInput](w, r)
	if !ok {
		return
	}
	if input.EnrollmentIDs == nil {
		h.writeActionError(w, r, validationError("enrollment_ids_required", "elenco iscrizioni obbligatorio"), "training.replace_event_expense_enrollments")
		return
	}
	response, err := h.store.ReplaceEventExpenseEnrollments(r.Context(), principal, r.PathValue("id"), *input.EnrollmentIDs)
	if err != nil {
		h.writeActionError(w, r, err, "training.replace_event_expense_enrollments")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleDeleteEventExpense(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	response, err := h.store.DeleteEventExpense(r.Context(), principal, r.PathValue("id"))
	if err != nil {
		h.writeActionError(w, r, err, "training.delete_event_expense")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}
