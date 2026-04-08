package listini

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

var iaasValidation = map[string][2]float64{
	"charge_cpu":        {0.05, 0.1},
	"charge_ram_kvm":    {0.05, 0.2},
	"charge_ram_vmware": {0.18, 0.3},
	"charge_pstor":      {0.0005, 0.002},
	"charge_sstor":      {0.0005, 0.002},
	"charge_ip":         {0.02, 0}, // 0 = no max
}

// handleGetIaaSPricing returns IaaS pricing for a Grappa customer, with fallback to defaults.
func (h *Handler) handleGetIaaSPricing(w http.ResponseWriter, r *http.Request) {
	if !h.requireGrappa(w) {
		return
	}

	customerID, err := pathID(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_customer_id")
		return
	}

	type pricing struct {
		ChargeCPU       float64  `json:"charge_cpu"`
		ChargeRAMKVM    float64  `json:"charge_ram_kvm"`
		ChargeRAMVMware float64  `json:"charge_ram_vmware"`
		ChargePStor     float64  `json:"charge_pstor"`
		ChargeSStor     float64  `json:"charge_sstor"`
		ChargeIP        float64  `json:"charge_ip"`
		ChargePrefix24  *float64 `json:"charge_prefix24"`
		IsDefault       bool     `json:"is_default"`
	}

	// Try customer-specific first
	var p pricing
	err = h.grappaDB.QueryRowContext(r.Context(), `
		SELECT charge_cpu, charge_ram_kvm, charge_ram_vmware,
		       charge_pstor, charge_sstor, charge_ip, charge_prefix24
		FROM cdl_prezzo_risorse_iaas
		WHERE id_anagrafica = ?`, customerID).Scan(
		&p.ChargeCPU, &p.ChargeRAMKVM, &p.ChargeRAMVMware,
		&p.ChargePStor, &p.ChargeSStor, &p.ChargeIP, &p.ChargePrefix24)

	if err == nil {
		p.IsDefault = false
		httputil.JSON(w, http.StatusOK, p)
		return
	}

	// Fall back to default (id_anagrafica IS NULL)
	err = h.grappaDB.QueryRowContext(r.Context(), `
		SELECT charge_cpu, charge_ram_kvm, charge_ram_vmware,
		       charge_pstor, charge_sstor, charge_ip, charge_prefix24
		FROM cdl_prezzo_risorse_iaas
		WHERE id_anagrafica IS NULL
		LIMIT 1`).Scan(
		&p.ChargeCPU, &p.ChargeRAMKVM, &p.ChargeRAMVMware,
		&p.ChargePStor, &p.ChargeSStor, &p.ChargeIP, &p.ChargePrefix24)

	if h.rowError(w, r, "get_iaas_pricing_default", err) {
		return
	}
	p.IsDefault = true
	httputil.JSON(w, http.StatusOK, p)
}

// handleUpsertIaaSPricing creates or updates IaaS pricing for a Grappa customer.
func (h *Handler) handleUpsertIaaSPricing(w http.ResponseWriter, r *http.Request) {
	if !h.requireGrappa(w) {
		return
	}

	customerID, err := pathID(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_customer_id")
		return
	}

	var req IaaSPricingRequest
	if err := decodeBody(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_body")
		return
	}

	// Validate ranges
	fieldErrors := validateIaaSPricing(req)
	if len(fieldErrors) > 0 {
		httputil.JSON(w, http.StatusUnprocessableEntity, map[string]any{"errors": fieldErrors})
		return
	}

	// Fetch old values for diff detection
	var oldCPU, oldRAMKVM, oldRAMVMware, oldPStor, oldSStor, oldIP float64
	var hasOld bool
	err = h.grappaDB.QueryRowContext(r.Context(), `
		SELECT charge_cpu, charge_ram_kvm, charge_ram_vmware,
		       charge_pstor, charge_sstor, charge_ip
		FROM cdl_prezzo_risorse_iaas
		WHERE id_anagrafica = ?`, customerID).Scan(
		&oldCPU, &oldRAMKVM, &oldRAMVMware, &oldPStor, &oldSStor, &oldIP)
	if err == nil {
		hasOld = true
	}

	// Upsert
	_, err = h.grappaDB.ExecContext(r.Context(), `
		INSERT INTO cdl_prezzo_risorse_iaas
		  (id_anagrafica, charge_cpu, charge_ram_kvm, charge_ram_vmware,
		   charge_pstor, charge_sstor, charge_ip, charge_prefix24)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		  charge_cpu = VALUES(charge_cpu),
		  charge_ram_kvm = VALUES(charge_ram_kvm),
		  charge_ram_vmware = VALUES(charge_ram_vmware),
		  charge_pstor = VALUES(charge_pstor),
		  charge_sstor = VALUES(charge_sstor),
		  charge_ip = VALUES(charge_ip),
		  charge_prefix24 = VALUES(charge_prefix24)`,
		customerID, req.ChargeCPU, req.ChargeRAMKVM, req.ChargeRAMVMware,
		req.ChargePStor, req.ChargeSStor, req.ChargeIP, req.ChargePrefix24)
	if err != nil {
		h.dbFailure(w, r, "upsert_iaas_pricing", err)
		return
	}

	// HubSpot audit (async, fire-and-forget)
	if h.hubspot != nil && hasOld {
		hasChanges := oldCPU != req.ChargeCPU || oldRAMKVM != req.ChargeRAMKVM ||
			oldRAMVMware != req.ChargeRAMVMware || oldPStor != req.ChargePStor ||
			oldSStor != req.ChargeSStor || oldIP != req.ChargeIP

		if hasChanges {
			companyID, lookupErr := h.hubspot.LookupCompanyID(r.Context(), customerID)
			if lookupErr == nil {
				body := formatIaaSPricingNote(oldCPU, oldRAMKVM, oldRAMVMware, oldPStor, oldSStor, oldIP, req)
				h.hubspot.CreateNoteAsync(r.Context(), companyID, body)
			} else {
				logging.FromContext(r.Context()).Warn("hubspot company lookup failed",
					"component", "listini", "customer_id", customerID, "error", lookupErr)
			}
		}
	}

	httputil.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func validateIaaSPricing(req IaaSPricingRequest) map[string]string {
	errs := make(map[string]string)

	check := func(field string, value float64) {
		bounds, ok := iaasValidation[field]
		if !ok {
			return
		}
		min, max := bounds[0], bounds[1]
		if value < min {
			errs[field] = fmt.Sprintf("must be >= %g", min)
		} else if max > 0 && value > max {
			errs[field] = fmt.Sprintf("must be <= %g", max)
		}
	}

	check("charge_cpu", req.ChargeCPU)
	check("charge_ram_kvm", req.ChargeRAMKVM)
	check("charge_ram_vmware", req.ChargeRAMVMware)
	check("charge_pstor", req.ChargePStor)
	check("charge_sstor", req.ChargeSStor)
	check("charge_ip", req.ChargeIP)

	if len(errs) == 0 {
		return nil
	}
	return errs
}

func formatIaaSPricingNote(oldCPU, oldRAMKVM, oldRAMVMware, oldPStor, oldSStor, oldIP float64, req IaaSPricingRequest) string {
	var changes []string
	if oldCPU != req.ChargeCPU {
		changes = append(changes, fmt.Sprintf("CPU: %g → %g", oldCPU, req.ChargeCPU))
	}
	if oldRAMKVM != req.ChargeRAMKVM {
		changes = append(changes, fmt.Sprintf("RAM KVM: %g → %g", oldRAMKVM, req.ChargeRAMKVM))
	}
	if oldRAMVMware != req.ChargeRAMVMware {
		changes = append(changes, fmt.Sprintf("RAM VMware: %g → %g", oldRAMVMware, req.ChargeRAMVMware))
	}
	if oldPStor != req.ChargePStor {
		changes = append(changes, fmt.Sprintf("Disco Primario: %g → %g", oldPStor, req.ChargePStor))
	}
	if oldSStor != req.ChargeSStor {
		changes = append(changes, fmt.Sprintf("Disco Secondario: %g → %g", oldSStor, req.ChargeSStor))
	}
	if oldIP != req.ChargeIP {
		changes = append(changes, fmt.Sprintf("IP Pubblico: %g → %g", oldIP, req.ChargeIP))
	}
	return "Aggiornamento prezzi IaaS:\n" + strings.Join(changes, "\n")
}
