package training

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// registerReportRoutes registra i due prospetti di report ratificati (#163,
// slice 4 del task 7). Sola lettura: ogni prospetto e una proiezione sui
// fatti gia persistiti, nessuna tabella propria.
func (h *handler) registerReportRoutes(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	mux.Handle("GET /training/v1/reports/economic", protect(h.requireStore(http.HandlerFunc(h.handleEconomicReport))))
	mux.Handle("GET /training/v1/reports/delivered", protect(h.requireStore(http.HandlerFunc(h.handleDeliveredReport))))
}

func (h *handler) handleEconomicReport(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	rows, err := h.economicReportRows(r.Context(), principal)
	if err != nil {
		h.writeActionError(w, r, err, "training.economic_report")
		return
	}
	httputil.JSON(w, http.StatusOK, EconomicReportResponse{Rows: rows})
}

func (h *handler) handleDeliveredReport(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.principalOrUnauthorized(w, r); !ok {
		return
	}
	from, to, err := parseReportPeriod(r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		h.writeActionError(w, r, err, "training.delivered_report")
		return
	}
	rows, err := h.store.DeliveredReportRows(r.Context(), from, to)
	if err != nil {
		h.writeActionError(w, r, err, "training.delivered_report")
		return
	}
	httputil.JSON(w, http.StatusOK, DeliveredReportResponse{
		From: from.Format("2006-01-02"),
		To:   to.Format("2006-01-02"),
		Rows: rows,
	})
}

// economicReportRows carica ogni voce e la idrata col suo PO vivo, letture
// sequenziali per PO distinto (scala verificata: decine/anno). Un errore su
// un PO non blocca il prospetto: la riga resta con i soli campi locali e
// poError valorizzato (#163 punto 1) — a differenza di hydrateEventExpenses
// (handler_expenses.go), che abortisce l'intera richiesta al primo errore:
// qui ogni riga e indipendente, come richiesto dal contratto di slice.
// L'unica condizione che interrompe l'intero prospetto e Arak non
// configurato, dove nessun PO e comunque risolvibile.
func (h *handler) economicReportRows(ctx context.Context, principal Principal) ([]EconomicReportRow, error) {
	locals, err := h.store.EconomicReportLocals(ctx)
	if err != nil {
		return nil, err
	}
	if len(locals) == 0 {
		return make([]EconomicReportRow, 0), nil
	}
	if h.arak == nil {
		return nil, serviceUnavailableError("arak_not_configured", "servizio PO temporaneamente non disponibile")
	}

	purchaseOrders := make(map[int64]PurchaseOrder, len(locals))
	poErrors := make(map[int64]string, len(locals))
	rows := make([]EconomicReportRow, 0, len(locals))
	for _, local := range locals {
		row := EconomicReportRow{
			ExpenseID:          local.ExpenseID,
			EventID:            local.EventID,
			CourseTitle:        local.CourseTitle,
			EventCancelled:     local.EventCancelled,
			CreatedAt:          local.CreatedAt,
			CoveredEnrollments: local.CoveredEnrollments,
			POID:               local.POID,
		}
		if po, ok := purchaseOrders[local.POID]; ok {
			rows = append(rows, hydrateEconomicReportRow(row, po))
			continue
		}
		if message, ok := poErrors[local.POID]; ok {
			row.POError = message
			rows = append(rows, row)
			continue
		}
		po, err := h.getPurchaseOrder(ctx, principal.Email, local.POID)
		if err != nil {
			poErrors[local.POID] = err.Error()
			row.POError = err.Error()
			rows = append(rows, row)
			continue
		}
		purchaseOrders[local.POID] = po
		rows = append(rows, hydrateEconomicReportRow(row, po))
	}
	return rows, nil
}

func hydrateEconomicReportRow(row EconomicReportRow, po PurchaseOrder) EconomicReportRow {
	row.POCode = po.Code
	row.Amount = po.TotalPrice
	row.Currency = po.Currency
	row.EconomicState = po.EconomicState
	row.BudgetName = po.Budget.Name
	row.BudgetYear = po.Budget.Year
	return row
}

// parseReportPeriod normalizza from/to del prospetto erogato: entrambe
// obbligatorie, formato data pura, periodo non invertito (#163 punto 2).
func parseReportPeriod(fromRaw, toRaw string) (time.Time, time.Time, error) {
	fromRaw = strings.TrimSpace(fromRaw)
	toRaw = strings.TrimSpace(toRaw)
	if fromRaw == "" || toRaw == "" {
		return time.Time{}, time.Time{}, validationError("period_required", "periodo obbligatorio: from e to")
	}
	from, err := time.Parse("2006-01-02", fromRaw)
	if err != nil {
		return time.Time{}, time.Time{}, validationError("invalid_period", "data from non valida")
	}
	to, err := time.Parse("2006-01-02", toRaw)
	if err != nil {
		return time.Time{}, time.Time{}, validationError("invalid_period", "data to non valida")
	}
	if to.Before(from) {
		return time.Time{}, time.Time{}, validationError("invalid_period", "il periodo e invertito")
	}
	return dateOnly(from), dateOnly(to), nil
}
