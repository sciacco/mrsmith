package listini

import (
	"fmt"
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

// handleListIaaSAccounts returns all active, billable IaaS accounts.
func (h *Handler) handleListIaaSAccounts(w http.ResponseWriter, r *http.Request) {
	if !h.requireGrappa(w) {
		return
	}

	rows, err := h.grappaDB.QueryContext(r.Context(), `
		SELECT ca.domainuuid, ca.id_cli_fatturazione,
		       COALESCE(cf.intestazione, '') AS intestazione,
		       COALESCE(ca.abbreviazione, '') AS abbreviazione,
		       COALESCE(ca.serialnumber, '') AS serialnumber,
		       COALESCE(ca.codice_ordine, '') AS codice_ordine,
		       COALESCE(ca.data_attivazione, '') AS data_attivazione,
		       ca.credito,
		       COALESCE(cs.infrastructure_platform, '') AS infrastructure_platform
		FROM cdl_accounts ca
		JOIN cli_fatturazione cf ON cf.id = ca.id_cli_fatturazione
		JOIN cdl_services cs ON cs.name = ca.cdl_service
		WHERE ca.id_cli_fatturazione > 0
		  AND ca.attivo = 1
		  AND ca.fatturazione = 1
		  AND cf.codice_aggancio_gest NOT IN (385, 485)
		ORDER BY cf.intestazione, ca.abbreviazione`)
	if err != nil {
		h.dbFailure(w, r, "list_iaas_accounts", err)
		return
	}
	defer rows.Close()

	type account struct {
		DomainUUID             string  `json:"domainuuid"`
		IDCliFatturazione      int     `json:"id_cli_fatturazione"`
		Intestazione           string  `json:"intestazione"`
		Abbreviazione          string  `json:"abbreviazione"`
		Serialnumber           string  `json:"serialnumber"`
		CodiceOrdine           string  `json:"codice_ordine"`
		DataAttivazione        string  `json:"data_attivazione"`
		Credito                float64 `json:"credito"`
		InfrastructurePlatform string  `json:"infrastructure_platform"`
	}

	var result []account
	for rows.Next() {
		var a account
		if err := rows.Scan(
			&a.DomainUUID, &a.IDCliFatturazione,
			&a.Intestazione, &a.Abbreviazione,
			&a.Serialnumber, &a.CodiceOrdine,
			&a.DataAttivazione, &a.Credito,
			&a.InfrastructurePlatform,
		); err != nil {
			h.dbFailure(w, r, "list_iaas_accounts_scan", err)
			return
		}
		result = append(result, a)
	}
	if !h.rowsDone(w, r, rows, "list_iaas_accounts") {
		return
	}
	if result == nil {
		result = []account{}
	}

	httputil.JSON(w, http.StatusOK, result)
}

// handleBatchUpdateIaaSCredits updates credits for multiple IaaS accounts in a transaction.
func (h *Handler) handleBatchUpdateIaaSCredits(w http.ResponseWriter, r *http.Request) {
	if !h.requireGrappa(w) {
		return
	}

	var req BatchIaaSCreditRequest
	if err := decodeBody(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_body")
		return
	}

	if len(req.Items) == 0 {
		httputil.Error(w, http.StatusBadRequest, "empty_items")
		return
	}

	logger := logging.FromContext(r.Context())

	// Fetch old values for diff detection
	type oldRow struct {
		DomainUUID        string
		IDCliFatturazione int
		Abbreviazione     string
		OldCredito        float64
	}
	oldMap := make(map[string]oldRow) // keyed by domainuuid

	for _, item := range req.Items {
		var abbr string
		var oldCredito float64
		err := h.grappaDB.QueryRowContext(r.Context(), `
			SELECT abbreviazione, credito FROM cdl_accounts
			WHERE domainuuid = ? AND id_cli_fatturazione = ?`,
			item.DomainUUID, item.IDCliFatturazione).Scan(&abbr, &oldCredito)
		if err == nil {
			oldMap[item.DomainUUID] = oldRow{
				DomainUUID:        item.DomainUUID,
				IDCliFatturazione: item.IDCliFatturazione,
				Abbreviazione:     abbr,
				OldCredito:        oldCredito,
			}
		}
	}

	tx, err := h.grappaDB.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "batch_update_iaas_credits_begin", err)
		return
	}
	defer h.rollbackTx(r, tx, "batch_update_iaas_credits")

	for _, item := range req.Items {
		_, err := tx.ExecContext(r.Context(),
			`UPDATE cdl_accounts SET credito = ? WHERE domainuuid = ? AND id_cli_fatturazione = ?`,
			item.Credito, item.DomainUUID, item.IDCliFatturazione)
		if err != nil {
			h.dbFailure(w, r, "batch_update_iaas_credits_exec", err)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "batch_update_iaas_credits_commit", err)
		return
	}

	// HubSpot audit (async, per changed row)
	if h.hubspot != nil {
		for _, item := range req.Items {
			old, ok := oldMap[item.DomainUUID]
			if !ok || old.OldCredito == item.Credito {
				continue
			}
			companyID, lookupErr := h.hubspot.LookupCompanyID(r.Context(), item.IDCliFatturazione)
			if lookupErr != nil {
				logger.Warn("hubspot company lookup failed",
					"component", "listini", "customer_id", item.IDCliFatturazione, "error", lookupErr)
				continue
			}
			body := fmt.Sprintf("Credito IaaS aggiornato: %g → %g (account: %s)",
				old.OldCredito, item.Credito, old.Abbreviazione)
			h.hubspot.CreateNoteAsync(r.Context(), companyID, body)
		}
	}

	httputil.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
