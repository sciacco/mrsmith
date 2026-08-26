package training

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

const arakRDARoot = "/arak/rda/v1"

// EconomicState is Training's fail-closed interpretation of an Arak PO state.
type EconomicState string

const (
	EconomicStateApproved EconomicState = "approved"
	EconomicStatePending  EconomicState = "pending"
	EconomicStateRejected EconomicState = "rejected"
)

// PurchaseOrder is the read-only subset of an Arak PO required by Training.
type PurchaseOrder struct {
	ID            int64
	Code          string
	RawState      string
	EconomicState EconomicState
	TotalPrice    string
	Currency      string
	Budget        PurchaseOrderBudget
}

// PurchaseOrderBudget preserves the budget and its association as returned by
// Arak. Exactly one association is normally present upstream.
type PurchaseOrderBudget struct {
	ID           int64
	Name         string
	Year         int
	CostCenter   *string
	BudgetUserID *int64
}

type arakPurchaseOrder struct {
	ID         int64                   `json:"id"`
	Code       string                  `json:"code"`
	State      string                  `json:"state"`
	TotalPrice string                  `json:"total_price"`
	Currency   string                  `json:"currency"`
	Budget     arakPurchaseOrderBudget `json:"budget"`
}

type arakPurchaseOrderBudget struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Year         int     `json:"year"`
	CostCenter   *string `json:"cost_center"`
	BudgetUserID *int64  `json:"budget_user_id"`
}

// classifyEconomicState deliberately recognizes only explicitly approved
// states; every unknown state remains pending until Training is updated.
func classifyEconomicState(rawState string) EconomicState {
	switch rawState {
	case "APPROVED", "PENDING_SEND", "SENT", "PENDING_VERIFICATION", "PENDING_DISPUTE", "DELIVERED_AND_COMPLIANT", "CLOSED":
		return EconomicStateApproved
	case "REJECTED":
		return EconomicStateRejected
	default:
		return EconomicStatePending
	}
}

// resolvePurchaseOrder resolves a user-entered PO ID or code using the
// principal's Arak visibility. It never writes to Arak.
func (h *handler) resolvePurchaseOrder(ctx context.Context, principal Principal, poReference string) (PurchaseOrder, error) {
	if h.arak == nil {
		return PurchaseOrder{}, serviceUnavailableError("arak_not_configured", "servizio PO temporaneamente non disponibile")
	}

	reference := strings.TrimSpace(poReference)
	if reference == "" {
		return PurchaseOrder{}, validationError("invalid_po_reference", "riferimento PO obbligatorio")
	}
	if isDigits(reference) {
		id, err := strconv.ParseInt(reference, 10, 64)
		if err != nil || id <= 0 {
			return PurchaseOrder{}, validationError("invalid_po_reference", "ID PO non valido")
		}
		return h.getPurchaseOrder(ctx, principal.Email, id)
	}
	return h.findPurchaseOrderByCode(ctx, principal.Email, reference)
}

func (h *handler) getPurchaseOrder(ctx context.Context, email string, id int64) (PurchaseOrder, error) {
	resp, err := h.arak.DoWithHeadersContext(ctx, http.MethodGet, arakRDARoot+"/po/"+strconv.FormatInt(id, 10), "", nil, requesterEmailHeaders(email))
	if err != nil {
		return PurchaseOrder{}, fmt.Errorf("get Arak PO %d: %w", id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		return PurchaseOrder{}, notFoundError("po_not_found", "PO non trovato")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		return PurchaseOrder{}, fmt.Errorf("get Arak PO %d: upstream status %d", id, resp.StatusCode)
	}

	var po arakPurchaseOrder
	if err := json.NewDecoder(resp.Body).Decode(&po); err != nil {
		return PurchaseOrder{}, fmt.Errorf("decode Arak PO %d: %w", id, err)
	}
	return purchaseOrderFromArak(po), nil
}

func (h *handler) findPurchaseOrderByCode(ctx context.Context, email, code string) (PurchaseOrder, error) {
	query := url.Values{"search_string": {code}, "disable_pagination": {"true"}}
	resp, err := h.arak.DoWithHeadersContext(ctx, http.MethodGet, arakRDARoot+"/po", query.Encode(), nil, requesterEmailHeaders(email))
	if err != nil {
		return PurchaseOrder{}, fmt.Errorf("search Arak PO %q: %w", code, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		return PurchaseOrder{}, notFoundError("po_not_found", "PO non trovato")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		return PurchaseOrder{}, fmt.Errorf("search Arak PO %q: upstream status %d", code, resp.StatusCode)
	}

	var result struct {
		Items []arakPurchaseOrder `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return PurchaseOrder{}, fmt.Errorf("decode Arak PO search %q: %w", code, err)
	}

	matches := make([]arakPurchaseOrder, 0, 1)
	for _, po := range result.Items {
		if po.Code == code {
			matches = append(matches, po)
		}
	}
	switch len(matches) {
	case 0:
		return PurchaseOrder{}, notFoundError("po_not_found", "PO non trovato")
	case 1:
		return purchaseOrderFromArak(matches[0]), nil
	default:
		return PurchaseOrder{}, conflictError("po_reference_ambiguous", "riferimento PO ambiguo: usare l'ID")
	}
}

func purchaseOrderFromArak(po arakPurchaseOrder) PurchaseOrder {
	return PurchaseOrder{
		ID:            po.ID,
		Code:          po.Code,
		RawState:      po.State,
		EconomicState: classifyEconomicState(po.State),
		TotalPrice:    po.TotalPrice,
		Currency:      po.Currency,
		Budget: PurchaseOrderBudget{
			ID:           po.Budget.ID,
			Name:         po.Budget.Name,
			Year:         po.Budget.Year,
			CostCenter:   po.Budget.CostCenter,
			BudgetUserID: po.Budget.BudgetUserID,
		},
	}
}

// enrollmentStartGate returns the PATCH-only callback run by reconciliation
// while its SQL transaction remains open. Empty coverage deliberately avoids
// even checking Arak configuration, so uncovered enrollments start normally.
func (h *handler) enrollmentStartGate(email string) enrollmentStartGate {
	return func(ctx context.Context, purchaseOrderIDs []int64) error {
		return h.requireApprovedEnrollmentPurchaseOrders(ctx, email, purchaseOrderIDs)
	}
}

// requireApprovedEnrollmentPurchaseOrders reads every distinct covered PO live
// with the authenticated principal's visibility. It collects all economic
// blockers before returning a conflict, but lets a technical Arak failure stop
// the current transaction immediately.
func (h *handler) requireApprovedEnrollmentPurchaseOrders(ctx context.Context, email string, purchaseOrderIDs []int64) error {
	ids := distinctSortedPurchaseOrderIDs(purchaseOrderIDs)
	if len(ids) == 0 {
		return nil
	}
	if h.arak == nil {
		return serviceUnavailableError("arak_not_configured", "servizio PO temporaneamente non disponibile")
	}

	blockers := make([]PurchaseOrder, 0)
	for _, id := range ids {
		po, err := h.getPurchaseOrder(ctx, email, id)
		if err != nil {
			return err
		}
		if po.EconomicState != EconomicStateApproved {
			blockers = append(blockers, po)
		}
	}
	if len(blockers) == 0 {
		return nil
	}

	parts := make([]string, 0, len(blockers))
	for _, po := range blockers {
		parts = append(parts, fmt.Sprintf("PO %d (%s): stato %s, economico %s", po.ID, po.Code, po.RawState, po.EconomicState))
	}
	return conflictError("event_expense_po_not_approved", "avvio iscrizione bloccato: "+strings.Join(parts, "; "))
}

func distinctSortedPurchaseOrderIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func requesterEmailHeaders(email string) http.Header {
	return http.Header{"Requester-Email": []string{email}}
}

func isDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return value != ""
}
