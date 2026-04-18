package simulatorivendita

import (
	"context"
	"encoding/json"
	"math"
	"net/http"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

type pdfRenderer interface {
	GeneratePDF(ctx context.Context, payload any) ([]byte, error)
}

type Handler struct {
	renderer pdfRenderer
}

type resourceValueFields struct {
	VCPU       *float64 `json:"vcpu"`
	RAMVMware  *float64 `json:"ram_vmware"`
	RAMOS      *float64 `json:"ram_os"`
	StoragePri *float64 `json:"storage_pri"`
	StorageSec *float64 `json:"storage_sec"`
	FWStd      *float64 `json:"fw_std"`
	FWAdv      *float64 `json:"fw_adv"`
	PrivNet    *float64 `json:"priv_net"`
	OSWindows  *float64 `json:"os_windows"`
	MSSQLStd   *float64 `json:"ms_sql_std"`
}

type dailyTotalsFields struct {
	Computing *float64 `json:"computing"`
	Storage   *float64 `json:"storage"`
	Sicurezza *float64 `json:"sicurezza"`
	AddOn     *float64 `json:"addon"`
	Totale    *float64 `json:"totale"`
	Mese      *float64 `json:"mese"`
}

type quoteRequestFields struct {
	Quantities resourceValueFields `json:"qta"`
	Prices     resourceValueFields `json:"prezzi"`
	DailyTotal dailyTotalsFields   `json:"totale_giornaliero"`
}

type resourceValues struct {
	VCPU       float64 `json:"vcpu"`
	RAMVMware  float64 `json:"ram_vmware"`
	RAMOS      float64 `json:"ram_os"`
	StoragePri float64 `json:"storage_pri"`
	StorageSec float64 `json:"storage_sec"`
	FWStd      float64 `json:"fw_std"`
	FWAdv      float64 `json:"fw_adv"`
	PrivNet    float64 `json:"priv_net"`
	OSWindows  float64 `json:"os_windows"`
	MSSQLStd   float64 `json:"ms_sql_std"`
}

type dailyTotals struct {
	Computing float64 `json:"computing"`
	Storage   float64 `json:"storage"`
	Sicurezza float64 `json:"sicurezza"`
	AddOn     float64 `json:"addon"`
	Totale    float64 `json:"totale"`
	Mese      float64 `json:"mese"`
}

type quotePayload struct {
	Quantities resourceValues `json:"qta"`
	Prices     resourceValues `json:"prezzi"`
	DailyTotal dailyTotals    `json:"totale_giornaliero"`
}

type renderPayload struct {
	ConvertTo string       `json:"convertTo"`
	Data      quotePayload `json:"data"`
}

func RegisterRoutes(mux *http.ServeMux, renderer pdfRenderer) {
	h := &Handler{renderer: renderer}
	protect := acl.RequireRole(applaunch.SimulatoriVenditaAccessRoles()...)
	mux.Handle(
		"POST /simulatori-vendita/v1/iaas/quote",
		protect(http.HandlerFunc(h.handleGenerateQuote)),
	)
}

func (h *Handler) handleGenerateQuote(w http.ResponseWriter, r *http.Request) {
	if h.renderer == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "simulatori_vendita_pdf_not_configured")
		return
	}

	var input quoteRequestFields
	if err := decodeJSON(r, &input); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_quote_payload")
		return
	}

	quantities, ok := normalizeResourceValues(input.Quantities)
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid_quote_payload")
		return
	}
	prices, ok := normalizeResourceValues(input.Prices)
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid_quote_payload")
		return
	}
	totals, ok := normalizeDailyTotals(input.DailyTotal)
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid_quote_payload")
		return
	}

	payload := renderPayload{
		ConvertTo: "pdf",
		Data: quotePayload{
			Quantities: quantities,
			Prices:     prices,
			DailyTotal: totals,
		},
	}

	pdfBytes, err := h.renderer.GeneratePDF(r.Context(), payload)
	if err != nil {
		httputil.InternalError(
			w,
			r,
			err,
			"simulatori vendita pdf generation failed",
			"component",
			"simulatori-vendita",
			"operation",
			"generate_quote",
		)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="calcolatore-iaas.pdf"`)
	_, _ = w.Write(pdfBytes)
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func normalizeResourceValues(fields resourceValueFields) (resourceValues, bool) {
	vcpu, ok := normalizeNumber(fields.VCPU)
	if !ok {
		return resourceValues{}, false
	}
	ramVMware, ok := normalizeNumber(fields.RAMVMware)
	if !ok {
		return resourceValues{}, false
	}
	ramOS, ok := normalizeNumber(fields.RAMOS)
	if !ok {
		return resourceValues{}, false
	}
	storagePri, ok := normalizeNumber(fields.StoragePri)
	if !ok {
		return resourceValues{}, false
	}
	storageSec, ok := normalizeNumber(fields.StorageSec)
	if !ok {
		return resourceValues{}, false
	}
	fwStd, ok := normalizeNumber(fields.FWStd)
	if !ok {
		return resourceValues{}, false
	}
	fwAdv, ok := normalizeNumber(fields.FWAdv)
	if !ok {
		return resourceValues{}, false
	}
	privNet, ok := normalizeNumber(fields.PrivNet)
	if !ok {
		return resourceValues{}, false
	}
	osWindows, ok := normalizeNumber(fields.OSWindows)
	if !ok {
		return resourceValues{}, false
	}
	msSQLStd, ok := normalizeNumber(fields.MSSQLStd)
	if !ok {
		return resourceValues{}, false
	}

	return resourceValues{
		VCPU:       vcpu,
		RAMVMware:  ramVMware,
		RAMOS:      ramOS,
		StoragePri: storagePri,
		StorageSec: storageSec,
		FWStd:      fwStd,
		FWAdv:      fwAdv,
		PrivNet:    privNet,
		OSWindows:  osWindows,
		MSSQLStd:   msSQLStd,
	}, true
}

func normalizeDailyTotals(fields dailyTotalsFields) (dailyTotals, bool) {
	computing, ok := normalizeNumber(fields.Computing)
	if !ok {
		return dailyTotals{}, false
	}
	storage, ok := normalizeNumber(fields.Storage)
	if !ok {
		return dailyTotals{}, false
	}
	sicurezza, ok := normalizeNumber(fields.Sicurezza)
	if !ok {
		return dailyTotals{}, false
	}
	addOn, ok := normalizeNumber(fields.AddOn)
	if !ok {
		return dailyTotals{}, false
	}
	totale, ok := normalizeNumber(fields.Totale)
	if !ok {
		return dailyTotals{}, false
	}
	mese, ok := normalizeNumber(fields.Mese)
	if !ok {
		return dailyTotals{}, false
	}

	return dailyTotals{
		Computing: computing,
		Storage:   storage,
		Sicurezza: sicurezza,
		AddOn:     addOn,
		Totale:    totale,
		Mese:      mese,
	}, true
}

func normalizeNumber(value *float64) (float64, bool) {
	if value == nil {
		return 0, false
	}
	if math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 {
		return 0, false
	}
	return *value, true
}
