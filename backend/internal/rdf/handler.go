package rdf

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
	"github.com/sciacco/mrsmith/internal/platform/openrouter"
)

const (
	standardPipelineID = "255768766"
	iaasPipelineID     = "255768768"
)

var (
	errAIUnavailable = errors.New("ai unavailable")

	allowedRichiestaStates = map[string]struct{}{
		"nuova":      {},
		"in corso":   {},
		"completata": {},
		"annullata":  {},
	}
	allowedFattibilitaStates = map[string]struct{}{
		"bozza":       {},
		"inviata":     {},
		"sollecitata": {},
		"completata":  {},
		"annullata":   {},
	}
)

type Handler struct {
	anisettaDB                *sql.DB
	mistraDB                  *sql.DB
	ai                        *openrouter.Client
	teamsWebhookURL           string
	teamsNotificationsEnabled bool
	httpCli                   *http.Client
}

type createRichiestaRequest struct {
	DealID             int64  `json:"deal_id"`
	Indirizzo          string `json:"indirizzo"`
	Descrizione        string `json:"descrizione"`
	FornitoriPreferiti []int  `json:"fornitori_preferiti"`
}

type updateRichiestaStatoRequest struct {
	Stato string `json:"stato"`
}

type createFattibilitaItem struct {
	FornitoreID  int `json:"fornitore_id"`
	TecnologiaID int `json:"tecnologia_id"`
}

type createFattibilitaRequest struct {
	Items []createFattibilitaItem `json:"items"`
}

type updateFattibilitaRequest struct {
	Descrizione          *string  `json:"descrizione"`
	ContattoFornitore    *string  `json:"contatto_fornitore"`
	RiferimentoFornitore *string  `json:"riferimento_fornitore"`
	Stato                *string  `json:"stato"`
	Annotazioni          *string  `json:"annotazioni"`
	EsitoRicevutoIl      *string  `json:"esito_ricevuto_il"`
	DaOrdinare           *bool    `json:"da_ordinare"`
	ProfiloFornitore     *string  `json:"profilo_fornitore"`
	NRC                  *float64 `json:"nrc"`
	MRC                  *float64 `json:"mrc"`
	DurataMesi           *int     `json:"durata_mesi"`
	AderenzaBudget       *int     `json:"aderenza_budget"`
	Copertura            *bool    `json:"copertura"`
	GiorniRilascio       *int     `json:"giorni_rilascio"`
}

func RegisterRoutes(
	mux *http.ServeMux,
	anisettaDB, mistraDB *sql.DB,
	ai *openrouter.Client,
	teamsWebhookURL string,
	teamsNotificationsEnabled bool,
) {
	h := &Handler{
		anisettaDB:                anisettaDB,
		mistraDB:                  mistraDB,
		ai:                        ai,
		teamsWebhookURL:           strings.TrimSpace(teamsWebhookURL),
		teamsNotificationsEnabled: teamsNotificationsEnabled,
		httpCli:                   &http.Client{Timeout: 20 * time.Second},
	}

	accessProtect := acl.RequireRole(applaunch.RichiesteFattibilitaAccessRoles()...)
	managerProtect := acl.RequireRole(applaunch.RichiesteFattibilitaManagerRoles()...)
	access := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, accessProtect(http.HandlerFunc(handler)))
	}
	manager := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, managerProtect(http.HandlerFunc(handler)))
	}

	access("GET /rdf/v1/capabilities", h.handleCapabilities)
	access("GET /rdf/v1/deals", h.handleListDeals)
	access("GET /rdf/v1/deals/{id}", h.handleGetDeal)
	access("GET /rdf/v1/fornitori", h.handleListFornitori)
	access("GET /rdf/v1/tecnologie", h.handleListTecnologie)
	access("GET /rdf/v1/richieste/summary", h.handleListRichiesteSummary)
	access("GET /rdf/v1/richieste/{id}/full", h.handleGetRichiestaFull)
	access("GET /rdf/v1/fattibilita/{id}", h.handleGetFattibilita)
	access("POST /rdf/v1/richieste", h.handleCreateRichiesta)
	access("POST /rdf/v1/richieste/{id}/analisi", h.handleAnalyzeRichiesta)
	access("POST /rdf/v1/richieste/{id}/analisi-json", h.handleAnalyzeRichiestaJSON)
	access("GET /rdf/v1/richieste/{id}/pdf", h.handleRenderRichiestaPDF)

	manager("POST /rdf/v1/richieste/{id}/fattibilita", h.handleCreateFattibilita)
	manager("PATCH /rdf/v1/richieste/{id}/stato", h.handleUpdateRichiestaStato)
	manager("PATCH /rdf/v1/fattibilita/{id}", h.handleUpdateFattibilita)
}

func (h *Handler) handleListDeals(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	cliente := strings.TrimSpace(r.URL.Query().Get("cliente"))
	page := queryPositiveInt(r, "page", 1)
	pageSize := queryPositiveInt(r, "page_size", 12)
	offset := (page - 1) * pageSize

	whereParts := []string{eligibleDealCondition("d", "s")}
	args := []any{}
	if q != "" {
		pattern := "%" + q + "%"
		whereParts = append(whereParts, "(COALESCE(d.codice, '') ILIKE "+placeholder(&args, pattern)+" OR COALESCE(d.name, '') ILIKE "+placeholder(&args, pattern)+" OR COALESCE(c.name, '') ILIKE "+placeholder(&args, pattern)+" OR COALESCE(o.email, '') ILIKE "+placeholder(&args, pattern)+")")
	}
	if cliente != "" {
		whereParts = append(whereParts, "COALESCE(c.name, '') ILIKE "+placeholder(&args, "%"+cliente+"%"))
	}
	whereClause := " WHERE " + strings.Join(whereParts, " AND ")

	baseFrom := ` FROM loader.hubs_deal d
		LEFT JOIN loader.hubs_company c ON c.id = d.company_id
		LEFT JOIN loader.hubs_pipeline p ON p.id = d.pipeline
		LEFT JOIN loader.hubs_stages s ON s.id = d.dealstage
		LEFT JOIN loader.hubs_owner o ON o.id = d.hubspot_owner_id`

	var total int
	countQuery := "SELECT COUNT(*)" + baseFrom + whereClause
	if err := h.mistraDB.QueryRowContext(r.Context(), countQuery, args...).Scan(&total); err != nil {
		h.dbFailure(w, r, "list_deals_count", err)
		return
	}

	listArgs := append([]any{}, args...)
	limitPlaceholder := placeholder(&listArgs, pageSize)
	offsetPlaceholder := placeholder(&listArgs, offset)
	listQuery := `SELECT d.id, COALESCE(d.codice, ''), COALESCE(d.name, ''), c.name, o.email, p.label, s.label, s.display_order` +
		baseFrom + whereClause +
		` ORDER BY d.id DESC LIMIT ` + limitPlaceholder + ` OFFSET ` + offsetPlaceholder

	rows, err := h.mistraDB.QueryContext(r.Context(), listQuery, listArgs...)
	if err != nil {
		h.dbFailure(w, r, "list_deals", err)
		return
	}
	defer rows.Close()

	items := make([]Deal, 0)
	for rows.Next() {
		item, err := scanDeal(rows)
		if err != nil {
			h.dbFailure(w, r, "list_deals_scan", err)
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		h.dbFailure(w, r, "list_deals_rows", err)
		return
	}

	httputil.JSON(w, http.StatusOK, pagedResponse[Deal]{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	})
}

func (h *Handler) handleGetDeal(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	id, err := pathInt64(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_deal_id")
		return
	}

	deal, err := h.fetchDeal(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "deal_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "get_deal", err, "deal_id", id)
		return
	}

	httputil.JSON(w, http.StatusOK, deal)
}

func (h *Handler) handleListFornitori(w http.ResponseWriter, r *http.Request) {
	if !h.requireAnisetta(w) {
		return
	}

	rows, err := h.anisettaDB.QueryContext(r.Context(), `SELECT id, COALESCE(nome, '') FROM public.rdf_fornitori ORDER BY nome NULLS LAST, id`)
	if err != nil {
		h.dbFailure(w, r, "list_fornitori", err)
		return
	}
	defer rows.Close()

	items := make([]LookupItem, 0)
	for rows.Next() {
		var item LookupItem
		if err := rows.Scan(&item.ID, &item.Nome); err != nil {
			h.dbFailure(w, r, "list_fornitori_scan", err)
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		h.dbFailure(w, r, "list_fornitori_rows", err)
		return
	}

	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleListTecnologie(w http.ResponseWriter, r *http.Request) {
	if !h.requireAnisetta(w) {
		return
	}

	rows, err := h.anisettaDB.QueryContext(r.Context(), `SELECT id, nome FROM public.rdf_tecnologie ORDER BY nome, id`)
	if err != nil {
		h.dbFailure(w, r, "list_tecnologie", err)
		return
	}
	defer rows.Close()

	items := make([]LookupItem, 0)
	for rows.Next() {
		var item LookupItem
		if err := rows.Scan(&item.ID, &item.Nome); err != nil {
			h.dbFailure(w, r, "list_tecnologie_scan", err)
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		h.dbFailure(w, r, "list_tecnologie_rows", err)
		return
	}

	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleListRichiesteSummary(w http.ResponseWriter, r *http.Request) {
	if !h.requireAnisetta(w) || !h.requireMistra(w) {
		return
	}

	stati := queryCSV(r, "stato")
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	dataDa := strings.TrimSpace(r.URL.Query().Get("data_da"))
	dataA := strings.TrimSpace(r.URL.Query().Get("data_a"))
	page := queryPositiveInt(r, "page", 1)
	pageSize := queryPositiveInt(r, "page_size", 12)
	offset := (page - 1) * pageSize

	var dealIDsForCompany []int64
	if q != "" {
		matchedIDs, err := h.fetchDealIDsByCompany(r.Context(), q)
		if err != nil {
			h.dbFailure(w, r, "list_richieste_summary_company_search", err)
			return
		}
		dealIDsForCompany = matchedIDs
	}

	args := []any{}
	whereClause := buildRichiestaWhereClause(&args, stati, q, dealIDsForCompany, dataDa, dataA)

	query := `WITH counts AS (
		SELECT richiesta_id,
		       COUNT(*) AS totale,
		       COUNT(*) FILTER (WHERE stato = 'bozza') AS bozza,
		       COUNT(*) FILTER (WHERE stato = 'inviata') AS inviata,
		       COUNT(*) FILTER (WHERE stato = 'sollecitata') AS sollecitata,
		       COUNT(*) FILTER (WHERE stato = 'completata') AS completata,
		       COUNT(*) FILTER (WHERE stato = 'annullata') AS annullata
		FROM public.rdf_fattibilita_fornitori
		GROUP BY richiesta_id
	)
		SELECT r.id, r.deal_id, r.data_richiesta, r.descrizione, r.indirizzo, r.stato,
		       r.annotazioni_richiedente, r.annotazioni_carrier, r.created_by, r.created_at, r.updated_at,
		       r.fornitori_preferiti, COALESCE(r.codice_deal, ''),
		       COALESCE(c.bozza, 0), COALESCE(c.inviata, 0), COALESCE(c.sollecitata, 0), COALESCE(c.completata, 0), COALESCE(c.annullata, 0), COALESCE(c.totale, 0)
		FROM public.rdf_richieste r
		LEFT JOIN counts c ON c.richiesta_id = r.id` + whereClause +
		` ORDER BY r.data_richiesta DESC, r.id DESC`

	rows, err := h.anisettaDB.QueryContext(r.Context(), query, args...)
	if err != nil {
		h.dbFailure(w, r, "list_richieste_summary", err)
		return
	}
	defer rows.Close()

	items := make([]RichiestaSummary, 0)
	dealIDs := make([]int64, 0)
	for rows.Next() {
		item, err := scanRichiestaSummary(rows)
		if err != nil {
			h.dbFailure(w, r, "list_richieste_summary_scan", err)
			return
		}
		if item.DealID != nil {
			dealIDs = append(dealIDs, *item.DealID)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		h.dbFailure(w, r, "list_richieste_summary_rows", err)
		return
	}

	dealsByID, err := h.fetchDealsMap(r.Context(), dealIDs)
	if err != nil {
		h.dbFailure(w, r, "list_richieste_summary_deals", err)
		return
	}
	for i := range items {
		dealID := items[i].DealID
		if dealID == nil {
			continue
		}
		deal, ok := dealsByID[*dealID]
		if !ok {
			continue
		}
		items[i].DealName = stringPointer(deal.DealName)
		items[i].CompanyName = deal.CompanyName
		items[i].OwnerEmail = deal.OwnerEmail
	}

	total := len(items)
	if offset >= total {
		items = []RichiestaSummary{}
	} else {
		end := offset + pageSize
		if end > total {
			end = total
		}
		items = items[offset:end]
	}

	httputil.JSON(w, http.StatusOK, pagedResponse[RichiestaSummary]{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	})
}

func (h *Handler) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	httputil.JSON(w, http.StatusOK, map[string]bool{
		"ai_enabled": h.ai != nil,
	})
}

func (h *Handler) handleGetRichiestaFull(w http.ResponseWriter, r *http.Request) {
	if !h.requireAnisetta(w) || !h.requireMistra(w) {
		return
	}

	id, err := pathInt(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_richiesta_id")
		return
	}

	full, err := h.loadRichiestaFull(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "richiesta_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "get_richiesta_full", err, "richiesta_id", id)
		return
	}

	httputil.JSON(w, http.StatusOK, full)
}

func (h *Handler) handleGetFattibilita(w http.ResponseWriter, r *http.Request) {
	if !h.requireAnisetta(w) {
		return
	}

	id, err := pathInt(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_fattibilita_id")
		return
	}

	item, err := h.fetchFattibilita(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "fattibilita_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "get_fattibilita", err, "fattibilita_id", id)
		return
	}

	httputil.JSON(w, http.StatusOK, item)
}

func (h *Handler) handleCreateRichiesta(w http.ResponseWriter, r *http.Request) {
	if !h.requireAnisetta(w) || !h.requireMistra(w) {
		return
	}

	var body createRichiestaRequest
	if err := decodeBody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}

	body.Indirizzo = strings.TrimSpace(body.Indirizzo)
	body.Descrizione = strings.TrimSpace(body.Descrizione)
	if body.DealID <= 0 {
		httputil.Error(w, http.StatusBadRequest, "deal_id_required")
		return
	}
	if body.Indirizzo == "" {
		httputil.Error(w, http.StatusBadRequest, "indirizzo_required")
		return
	}
	if body.Descrizione == "" {
		httputil.Error(w, http.StatusBadRequest, "descrizione_required")
		return
	}

	deal, err := h.fetchEligibleDeal(r.Context(), body.DealID)
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusBadRequest, "deal_not_eligible")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "create_richiesta_fetch_deal", err, "deal_id", body.DealID)
		return
	}

	claims, _ := auth.GetClaims(r.Context())
	createdBy := strings.TrimSpace(claims.Email)
	if createdBy == "" {
		createdBy = strings.TrimSpace(claims.Name)
	}

	var richiestaID int
	err = h.anisettaDB.QueryRowContext(
		r.Context(),
		`INSERT INTO public.rdf_richieste (deal_id, descrizione, indirizzo, created_by, fornitori_preferiti, codice_deal)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id`,
		body.DealID,
		body.Descrizione,
		body.Indirizzo,
		nullIfEmpty(createdBy),
		formatIntArrayLiteral(body.FornitoriPreferiti),
		deal.Codice,
	).Scan(&richiestaID)
	if err != nil {
		h.dbFailure(w, r, "create_richiesta_insert", err, "deal_id", body.DealID)
		return
	}

	full, err := h.loadRichiestaFull(r.Context(), richiestaID)
	if err != nil {
		h.dbFailure(w, r, "create_richiesta_load", err, "richiesta_id", richiestaID)
		return
	}
	if err := h.notifyCreate(r.Context(), full); err != nil {
		logging.FromContext(r.Context()).Warn("rdf create notification failed", "component", "rdf", "richiesta_id", richiestaID, "error", err)
	}

	httputil.JSON(w, http.StatusCreated, full)
}

func (h *Handler) handleCreateFattibilita(w http.ResponseWriter, r *http.Request) {
	if !h.requireAnisetta(w) {
		return
	}

	richiestaID, err := pathInt(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_richiesta_id")
		return
	}
	if _, err := h.fetchRichiesta(r.Context(), richiestaID); errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "richiesta_not_found")
		return
	} else if err != nil {
		h.dbFailure(w, r, "create_fattibilita_fetch_richiesta", err, "richiesta_id", richiestaID)
		return
	}

	var body createFattibilitaRequest
	if err := decodeBody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if len(body.Items) == 0 {
		httputil.Error(w, http.StatusBadRequest, "items_required")
		return
	}

	tx, err := h.anisettaDB.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "create_fattibilita_begin", err, "richiesta_id", richiestaID)
		return
	}
	defer h.rollbackTx(r.Context(), tx, "create_fattibilita_rollback", "richiesta_id", richiestaID)

	createdIDs := make([]int, 0, len(body.Items))
	for _, item := range body.Items {
		if item.FornitoreID <= 0 || item.TecnologiaID <= 0 {
			httputil.Error(w, http.StatusBadRequest, "invalid_batch_item")
			return
		}
		var createdID int
		if err := tx.QueryRowContext(
			r.Context(),
			`INSERT INTO public.rdf_fattibilita_fornitori (richiesta_id, fornitore_id, tecnologia_id)
			 VALUES ($1, $2, $3)
			 RETURNING id`,
			richiestaID,
			item.FornitoreID,
			item.TecnologiaID,
		).Scan(&createdID); err != nil {
			h.dbFailure(w, r, "create_fattibilita_insert", err, "richiesta_id", richiestaID)
			return
		}
		createdIDs = append(createdIDs, createdID)
	}

	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "create_fattibilita_commit", err, "richiesta_id", richiestaID)
		return
	}

	full, err := h.loadRichiestaFull(r.Context(), richiestaID)
	if err != nil {
		h.dbFailure(w, r, "create_fattibilita_load", err, "richiesta_id", richiestaID)
		return
	}

	createdSet := make(map[int]struct{}, len(createdIDs))
	for _, id := range createdIDs {
		createdSet[id] = struct{}{}
	}
	items := make([]Fattibilita, 0, len(createdIDs))
	for _, item := range full.Fattibilita {
		if _, ok := createdSet[item.ID]; ok {
			items = append(items, item)
		}
	}

	httputil.JSON(w, http.StatusCreated, items)
}

func (h *Handler) handleUpdateRichiestaStato(w http.ResponseWriter, r *http.Request) {
	if !h.requireAnisetta(w) {
		return
	}

	id, err := pathInt(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_richiesta_id")
		return
	}

	var body updateRichiestaStatoRequest
	if err := decodeBody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	body.Stato = strings.TrimSpace(body.Stato)
	if _, ok := allowedRichiestaStates[body.Stato]; !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid_stato")
		return
	}

	var updatedID int
	err = h.anisettaDB.QueryRowContext(
		r.Context(),
		`UPDATE public.rdf_richieste
		 SET stato = $1, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $2
		 RETURNING id`,
		body.Stato,
		id,
	).Scan(&updatedID)
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "richiesta_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "update_richiesta_stato", err, "richiesta_id", id)
		return
	}

	richiesta, err := h.fetchRichiesta(r.Context(), updatedID)
	if err != nil {
		h.dbFailure(w, r, "update_richiesta_stato_load", err, "richiesta_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, richiesta)
}

func (h *Handler) handleUpdateFattibilita(w http.ResponseWriter, r *http.Request) {
	if !h.requireAnisetta(w) {
		return
	}

	id, err := pathInt(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_fattibilita_id")
		return
	}

	current, err := h.fetchFattibilita(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "fattibilita_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "update_fattibilita_fetch", err, "fattibilita_id", id)
		return
	}

	var body updateFattibilitaRequest
	if err := decodeBody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}

	setClauses := make([]string, 0, 16)
	args := []any{}
	setString := func(column string, value *string) {
		if value == nil {
			return
		}
		setClauses = append(setClauses, column+" = "+placeholder(&args, nullableTrimmedString(*value)))
	}

	setString("descrizione", body.Descrizione)
	setString("contatto_fornitore", body.ContattoFornitore)
	setString("riferimento_fornitore", body.RiferimentoFornitore)
	setString("annotazioni", body.Annotazioni)
	setString("profilo_fornitore", body.ProfiloFornitore)
	if body.Stato != nil {
		body.Stato = stringPointer(strings.TrimSpace(*body.Stato))
		if _, ok := allowedFattibilitaStates[*body.Stato]; !ok {
			httputil.Error(w, http.StatusBadRequest, "invalid_stato")
			return
		}
		setClauses = append(setClauses, "stato = "+placeholder(&args, *body.Stato))
	}
	if body.EsitoRicevutoIl != nil {
		value, err := parseNullableDate(*body.EsitoRicevutoIl)
		if err != nil {
			httputil.Error(w, http.StatusBadRequest, "invalid_esito_ricevuto_il")
			return
		}
		setClauses = append(setClauses, "esito_ricevuto_il = "+placeholder(&args, value))
	}
	if body.DaOrdinare != nil {
		setClauses = append(setClauses, "da_ordinare = "+placeholder(&args, *body.DaOrdinare))
	}
	if body.NRC != nil {
		setClauses = append(setClauses, "nrc = "+placeholder(&args, *body.NRC))
	}
	if body.MRC != nil {
		setClauses = append(setClauses, "mrc = "+placeholder(&args, *body.MRC))
	}
	if body.DurataMesi != nil {
		setClauses = append(setClauses, "durata_mesi = "+placeholder(&args, *body.DurataMesi))
	}
	if body.AderenzaBudget != nil {
		if *body.AderenzaBudget < 0 || *body.AderenzaBudget > 5 {
			httputil.Error(w, http.StatusBadRequest, "invalid_aderenza_budget")
			return
		}
		setClauses = append(setClauses, "aderenza_budget = "+placeholder(&args, *body.AderenzaBudget))
	}
	if body.Copertura != nil {
		setClauses = append(setClauses, "copertura = "+placeholder(&args, boolToInt(*body.Copertura)))
	}
	if body.GiorniRilascio != nil {
		setClauses = append(setClauses, "giorni_rilascio = "+placeholder(&args, *body.GiorniRilascio))
	}

	if len(setClauses) == 0 {
		httputil.Error(w, http.StatusBadRequest, "no_fields_to_update")
		return
	}

	query := `UPDATE public.rdf_fattibilita_fornitori SET ` + strings.Join(setClauses, ", ") + ` WHERE id = ` + placeholder(&args, id) + ` RETURNING id`
	var updatedID int
	if err := h.anisettaDB.QueryRowContext(r.Context(), query, args...).Scan(&updatedID); errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "fattibilita_not_found")
		return
	} else if err != nil {
		h.dbFailure(w, r, "update_fattibilita_exec", err, "fattibilita_id", id)
		return
	}

	updated, err := h.fetchFattibilita(r.Context(), updatedID)
	if err != nil {
		h.dbFailure(w, r, "update_fattibilita_load", err, "fattibilita_id", id)
		return
	}
	if shouldNotifyDiff(current, updated) {
		full, loadErr := h.loadRichiestaFull(r.Context(), updated.RichiestaID)
		if loadErr == nil {
			if err := h.notifyFattibilitaUpdate(r.Context(), full, updated); err != nil {
				logging.FromContext(r.Context()).Warn("rdf update notification failed", "component", "rdf", "fattibilita_id", id, "error", err)
			}
		}
	}

	httputil.JSON(w, http.StatusOK, updated)
}

func (h *Handler) handleAnalyzeRichiesta(w http.ResponseWriter, r *http.Request) {
	if !h.requireAnisetta(w) || !h.requireMistra(w) {
		return
	}

	id, err := pathInt(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_richiesta_id")
		return
	}

	full, err := h.loadRichiestaFull(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "richiesta_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "analyze_richiesta_load", err, "richiesta_id", id)
		return
	}

	analysis, err := h.analyzeText(r.Context(), id, full)
	if errors.Is(err, errAIUnavailable) {
		httputil.Error(w, http.StatusServiceUnavailable, "ai_unavailable")
		return
	}
	if err != nil {
		logging.FromContext(r.Context()).Error("rdf ai text failed", "component", "rdf", "richiesta_id", id, "error", err)
		httputil.Error(w, http.StatusBadGateway, "ai_request_failed")
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]string{"analysis": analysis})
}

func (h *Handler) handleAnalyzeRichiestaJSON(w http.ResponseWriter, r *http.Request) {
	if !h.requireAnisetta(w) || !h.requireMistra(w) {
		return
	}

	id, err := pathInt(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_richiesta_id")
		return
	}

	full, err := h.loadRichiestaFull(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "richiesta_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "analyze_richiesta_json_load", err, "richiesta_id", id)
		return
	}

	analysis, err := h.analyzeJSON(r.Context(), id, full)
	if errors.Is(err, errAIUnavailable) {
		httputil.Error(w, http.StatusServiceUnavailable, "ai_unavailable")
		return
	}
	if err != nil {
		logging.FromContext(r.Context()).Error("rdf ai json failed", "component", "rdf", "richiesta_id", id, "error", err)
		httputil.Error(w, http.StatusBadGateway, "ai_request_failed")
		return
	}

	httputil.JSON(w, http.StatusOK, analysis)
}

func (h *Handler) handleRenderRichiestaPDF(w http.ResponseWriter, r *http.Request) {
	if !h.requireAnisetta(w) || !h.requireMistra(w) {
		return
	}

	id, err := pathInt(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_richiesta_id")
		return
	}

	full, err := h.loadRichiestaFull(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "richiesta_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "render_pdf_load", err, "richiesta_id", id)
		return
	}

	analysis, err := h.analyzeText(r.Context(), id, full)
	if errors.Is(err, errAIUnavailable) {
		analysis = ""
	} else if err != nil {
		logging.FromContext(r.Context()).Error("rdf pdf analysis failed", "component", "rdf", "richiesta_id", id, "error", err)
		httputil.Error(w, http.StatusBadGateway, "ai_request_failed")
		return
	}

	pdfBytes, err := renderRichiestaPDF(full, analysis)
	if err != nil {
		h.dbFailure(w, r, "render_pdf_build", err, "richiesta_id", id)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="rdf-%d.pdf"`, id))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(pdfBytes); err != nil {
		logging.FromContext(r.Context()).Warn("rdf pdf write failed", "component", "rdf", "richiesta_id", id, "error", err)
	}
}

func (h *Handler) loadRichiestaFull(ctx context.Context, id int) (RichiestaFull, error) {
	richiesta, err := h.fetchRichiesta(ctx, id)
	if err != nil {
		return RichiestaFull{}, err
	}

	full := RichiestaFull{Richiesta: richiesta}
	if richiesta.DealID != nil && h.mistraDB != nil {
		deal, err := h.fetchDeal(ctx, *richiesta.DealID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return RichiestaFull{}, err
		}
		if err == nil {
			full.Deal = &deal
			full.DealName = stringPointer(deal.DealName)
			full.CompanyName = deal.CompanyName
			full.OwnerEmail = deal.OwnerEmail
		}
	}

	full.PreferredSupplierNames, err = h.lookupSupplierNames(ctx, richiesta.FornitoriPreferiti)
	if err != nil {
		return RichiestaFull{}, err
	}
	full.Fattibilita, err = h.listFattibilitaByRichiesta(ctx, id)
	if err != nil {
		return RichiestaFull{}, err
	}
	full.Counts = countFattibilita(full.Fattibilita)
	return full, nil
}

func (h *Handler) fetchRichiesta(ctx context.Context, id int) (Richiesta, error) {
	var (
		item                   Richiesta
		dealID                 sql.NullInt64
		dataRichiesta          time.Time
		annotazioniRichiedente sql.NullString
		annotazioniCarrier     sql.NullString
		createdBy              sql.NullString
		createdAt              time.Time
		updatedAt              sql.NullTime
		fornitoriPreferitiRaw  sql.NullString
	)

	err := h.anisettaDB.QueryRowContext(
		ctx,
		`SELECT id, deal_id, data_richiesta, descrizione, indirizzo, stato,
		        annotazioni_richiedente, annotazioni_carrier, created_by, created_at, updated_at,
		        fornitori_preferiti, COALESCE(codice_deal, '')
		 FROM public.rdf_richieste
		 WHERE id = $1`,
		id,
	).Scan(
		&item.ID,
		&dealID,
		&dataRichiesta,
		&item.Descrizione,
		&item.Indirizzo,
		&item.Stato,
		&annotazioniRichiedente,
		&annotazioniCarrier,
		&createdBy,
		&createdAt,
		&updatedAt,
		&fornitoriPreferitiRaw,
		&item.CodiceDeal,
	)
	if err != nil {
		return Richiesta{}, err
	}

	item.DealID = nullInt64Ptr(dealID)
	item.DataRichiesta = formatDate(dataRichiesta)
	item.AnnotazioniRichiedente = nullStringPtr(annotazioniRichiedente)
	item.AnnotazioniCarrier = nullStringPtr(annotazioniCarrier)
	item.CreatedBy = nullStringPtr(createdBy)
	item.CreatedAt = formatTimestamp(createdAt)
	item.UpdatedAt = nullTimePtr(updatedAt)
	item.FornitoriPreferiti = parseNullableIntArrayLiteral(fornitoriPreferitiRaw)
	return item, nil
}

func (h *Handler) fetchDeal(ctx context.Context, id int64) (Deal, error) {
	row := h.mistraDB.QueryRowContext(
		ctx,
		`SELECT d.id, COALESCE(d.codice, ''), COALESCE(d.name, ''), c.name, o.email, p.label, s.label, s.display_order
		 FROM loader.hubs_deal d
		 LEFT JOIN loader.hubs_company c ON c.id = d.company_id
		 LEFT JOIN loader.hubs_owner o ON o.id = d.hubspot_owner_id
		 LEFT JOIN loader.hubs_pipeline p ON p.id = d.pipeline
		 LEFT JOIN loader.hubs_stages s ON s.id = d.dealstage
		 WHERE d.id = $1`,
		id,
	)
	return scanDeal(row)
}

func (h *Handler) fetchDealsMap(ctx context.Context, ids []int64) (map[int64]Deal, error) {
	result := make(map[int64]Deal)
	if len(ids) == 0 {
		return result, nil
	}

	seen := make(map[int64]struct{}, len(ids))
	args := make([]any, 0, len(ids))
	holders := make([]string, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		holders = append(holders, placeholder(&args, id))
	}
	if len(holders) == 0 {
		return result, nil
	}

	query := `SELECT d.id, COALESCE(d.codice, ''), COALESCE(d.name, ''), c.name, o.email, p.label, s.label, s.display_order
		FROM loader.hubs_deal d
		LEFT JOIN loader.hubs_company c ON c.id = d.company_id
		LEFT JOIN loader.hubs_owner o ON o.id = d.hubspot_owner_id
		LEFT JOIN loader.hubs_pipeline p ON p.id = d.pipeline
		LEFT JOIN loader.hubs_stages s ON s.id = d.dealstage
		WHERE d.id IN (` + strings.Join(holders, ", ") + `)`
	rows, err := h.mistraDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		item, err := scanDeal(rows)
		if err != nil {
			return nil, err
		}
		result[item.ID] = item
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (h *Handler) fetchEligibleDeal(ctx context.Context, id int64) (Deal, error) {
	row := h.mistraDB.QueryRowContext(
		ctx,
		`SELECT d.id, COALESCE(d.codice, ''), COALESCE(d.name, ''), c.name, o.email, p.label, s.label, s.display_order
		 FROM loader.hubs_deal d
		 LEFT JOIN loader.hubs_company c ON c.id = d.company_id
		 LEFT JOIN loader.hubs_owner o ON o.id = d.hubspot_owner_id
		 LEFT JOIN loader.hubs_pipeline p ON p.id = d.pipeline
		 LEFT JOIN loader.hubs_stages s ON s.id = d.dealstage
		 WHERE d.id = $1 AND `+eligibleDealCondition("d", "s"),
		id,
	)
	return scanDeal(row)
}

type dealScanner interface {
	Scan(dest ...any) error
}

func scanDeal(scanner dealScanner) (Deal, error) {
	var (
		item       Deal
		company    sql.NullString
		owner      sql.NullString
		pipeline   sql.NullString
		stage      sql.NullString
		stageOrder sql.NullInt64
	)
	if err := scanner.Scan(&item.ID, &item.Codice, &item.DealName, &company, &owner, &pipeline, &stage, &stageOrder); err != nil {
		return Deal{}, err
	}
	item.CompanyName = nullStringPtr(company)
	item.OwnerEmail = nullStringPtr(owner)
	item.PipelineLabel = nullStringPtr(pipeline)
	item.StageLabel = nullStringPtr(stage)
	if stageOrder.Valid {
		v := int(stageOrder.Int64)
		item.StageOrder = &v
	}
	return item, nil
}

func (h *Handler) listFattibilitaByRichiesta(ctx context.Context, richiestaID int) ([]Fattibilita, error) {
	rows, err := h.anisettaDB.QueryContext(
		ctx,
		`SELECT ff.id, ff.richiesta_id, ff.fornitore_id, COALESCE(f.nome, ''), ff.data_richiesta,
		        ff.tecnologia_id, t.nome, ff.descrizione, ff.contatto_fornitore, ff.riferimento_fornitore,
		        ff.stato, ff.annotazioni, ff.esito_ricevuto_il, COALESCE(ff.da_ordinare, false), ff.profilo_fornitore,
		        ff.nrc, ff.mrc, ff.durata_mesi, COALESCE(ff.aderenza_budget, 0), COALESCE(ff.copertura, 0), ff.giorni_rilascio
		 FROM public.rdf_fattibilita_fornitori ff
		 JOIN public.rdf_fornitori f ON f.id = ff.fornitore_id
		 JOIN public.rdf_tecnologie t ON t.id = ff.tecnologia_id
		 WHERE ff.richiesta_id = $1
		 ORDER BY f.nome, t.nome, ff.id`,
		richiestaID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Fattibilita, 0)
	for rows.Next() {
		item, err := scanFattibilita(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (h *Handler) fetchFattibilita(ctx context.Context, id int) (Fattibilita, error) {
	row := h.anisettaDB.QueryRowContext(
		ctx,
		`SELECT ff.id, ff.richiesta_id, ff.fornitore_id, COALESCE(f.nome, ''), ff.data_richiesta,
		        ff.tecnologia_id, t.nome, ff.descrizione, ff.contatto_fornitore, ff.riferimento_fornitore,
		        ff.stato, ff.annotazioni, ff.esito_ricevuto_il, COALESCE(ff.da_ordinare, false), ff.profilo_fornitore,
		        ff.nrc, ff.mrc, ff.durata_mesi, COALESCE(ff.aderenza_budget, 0), COALESCE(ff.copertura, 0), ff.giorni_rilascio
		 FROM public.rdf_fattibilita_fornitori ff
		 JOIN public.rdf_fornitori f ON f.id = ff.fornitore_id
		 JOIN public.rdf_tecnologie t ON t.id = ff.tecnologia_id
		 WHERE ff.id = $1`,
		id,
	)
	return scanFattibilita(row)
}

type fattibilitaScanner interface {
	Scan(dest ...any) error
}

func scanFattibilita(scanner fattibilitaScanner) (Fattibilita, error) {
	var (
		item           Fattibilita
		dataRichiesta  time.Time
		descrizione    sql.NullString
		contatto       sql.NullString
		riferimento    sql.NullString
		annotazioni    sql.NullString
		esitoRicevuto  sql.NullTime
		profilo        sql.NullString
		nrc            sql.NullFloat64
		mrc            sql.NullFloat64
		durata         sql.NullInt64
		copertura      sql.NullInt64
		giorniRilascio sql.NullInt64
	)
	if err := scanner.Scan(
		&item.ID,
		&item.RichiestaID,
		&item.FornitoreID,
		&item.FornitoreNome,
		&dataRichiesta,
		&item.TecnologiaID,
		&item.TecnologiaNome,
		&descrizione,
		&contatto,
		&riferimento,
		&item.Stato,
		&annotazioni,
		&esitoRicevuto,
		&item.DaOrdinare,
		&profilo,
		&nrc,
		&mrc,
		&durata,
		&item.AderenzaBudget,
		&copertura,
		&giorniRilascio,
	); err != nil {
		return Fattibilita{}, err
	}
	item.DataRichiesta = formatDate(dataRichiesta)
	item.Descrizione = nullStringPtr(descrizione)
	item.ContattoFornitore = nullStringPtr(contatto)
	item.RiferimentoFornitore = nullStringPtr(riferimento)
	item.Annotazioni = nullStringPtr(annotazioni)
	item.EsitoRicevutoIl = nullDatePtr(esitoRicevuto)
	item.ProfiloFornitore = nullStringPtr(profilo)
	item.NRC = nullFloat64Ptr(nrc)
	item.MRC = nullFloat64Ptr(mrc)
	item.DurataMesi = nullIntPtr(durata)
	item.Copertura = copertura.Valid && copertura.Int64 == 1
	item.GiorniRilascio = nullIntPtr(giorniRilascio)
	return item, nil
}

func (h *Handler) lookupSupplierNames(ctx context.Context, ids []int) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	args := make([]any, 0, len(ids))
	placeholders := make([]string, 0, len(ids))
	for _, id := range ids {
		placeholders = append(placeholders, placeholder(&args, id))
	}
	query := `SELECT COALESCE(nome, '') FROM public.rdf_fornitori WHERE id IN (` + strings.Join(placeholders, ", ") + `) ORDER BY nome`
	rows, err := h.anisettaDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]string, 0, len(ids))
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if strings.TrimSpace(name) != "" {
			result = append(result, name)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (h *Handler) notifyCreate(ctx context.Context, full RichiestaFull) error {
	if !h.notificationsEnabled() {
		return nil
	}

	payload := map[string]any{
		"type": "MessageCard",
		"attachments": []map[string]any{
			{
				"contentType": "application/vnd.microsoft.card.adaptive",
				"content": map[string]any{
					"type":    "AdaptiveCard",
					"version": "1.2",
					"body": []map[string]any{
						{"type": "TextBlock", "text": fmt.Sprintf("Richiesta Nuova Fattibilita - Deal %s", firstNonEmpty(full.CodiceDeal, fmt.Sprintf("#%d", full.ID))), "weight": "Bolder", "size": "Medium"},
						{"type": "TextBlock", "text": fmt.Sprintf("Da %s", derefString(full.CreatedBy, "utente non disponibile")), "weight": "Normal", "size": "Small"},
						{"type": "TextBlock", "text": "Dettagli della richiesta", "weight": "Bolder", "size": "Small"},
						{"type": "TextBlock", "text": fmt.Sprintf("Cliente: %s", derefString(full.CompanyName, "Non disponibile")), "wrap": true},
						{"type": "TextBlock", "text": fmt.Sprintf("Deal: %s", derefString(full.DealName, "Non disponibile")), "wrap": true},
						{"type": "TextBlock", "text": fmt.Sprintf("Indirizzo: %s", full.Indirizzo), "wrap": true},
						{"type": "TextBlock", "text": fmt.Sprintf("Descrizione: %s", full.Descrizione), "wrap": true},
					},
					"actions": []map[string]any{
						{"type": "Action.OpenUrl", "title": "Apri Smartapp", "url": applaunch.RichiesteFattibilitaAppHref},
					},
				},
			},
		},
	}
	return h.sendTeamsPayload(ctx, payload)
}

func (h *Handler) notifyFattibilitaUpdate(ctx context.Context, full RichiestaFull, item Fattibilita) error {
	if !h.notificationsEnabled() {
		return nil
	}

	text := fmt.Sprintf(
		"Aggiornamento RDF *%s* (_%s_)\n- %s / %s\n- Stato: *%s*",
		firstNonEmpty(full.CodiceDeal, fmt.Sprintf("#%d", full.ID)),
		derefString(full.DealName, "Deal non disponibile"),
		item.FornitoreNome,
		item.TecnologiaNome,
		item.Stato,
	)
	if item.Copertura {
		text += " / Copertura: *SI*"
	}
	return h.sendTeamsPayload(ctx, map[string]string{"text": text})
}

func (h *Handler) sendTeamsPayload(ctx context.Context, payload any) error {
	if !h.notificationsEnabled() {
		return nil
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal teams payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.teamsWebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create teams request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpCli.Do(req)
	if err != nil {
		return fmt.Errorf("teams request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("teams request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func (h *Handler) notificationsEnabled() bool {
	return h.teamsNotificationsEnabled && h.teamsWebhookURL != ""
}

func buildRichiestaWhereClause(args *[]any, stati []string, q string, dealIDsForCompany []int64, dataDa, dataA string) string {
	parts := make([]string, 0, 4)
	if len(stati) > 0 {
		holders := make([]string, 0, len(stati))
		for _, stato := range stati {
			holders = append(holders, placeholder(args, stato))
		}
		parts = append(parts, "r.stato IN ("+strings.Join(holders, ", ")+")")
	}
	if q != "" {
		pattern := "%" + q + "%"
		orParts := []string{
			"COALESCE(r.codice_deal, '') ILIKE " + placeholder(args, pattern),
			"COALESCE(r.indirizzo, '') ILIKE " + placeholder(args, pattern),
			"COALESCE(r.created_by, '') ILIKE " + placeholder(args, pattern),
		}
		if len(dealIDsForCompany) > 0 {
			idHolders := make([]string, 0, len(dealIDsForCompany))
			for _, id := range dealIDsForCompany {
				idHolders = append(idHolders, placeholder(args, id))
			}
			orParts = append(orParts, "r.deal_id IN ("+strings.Join(idHolders, ", ")+")")
		}
		parts = append(parts, "("+strings.Join(orParts, " OR ")+")")
	}
	if dataDa != "" {
		parts = append(parts, "r.data_richiesta >= "+placeholder(args, dataDa))
	}
	if dataA != "" {
		parts = append(parts, "r.data_richiesta <= "+placeholder(args, dataA))
	}
	if len(parts) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(parts, " AND ")
}

func (h *Handler) fetchDealIDsByCompany(ctx context.Context, q string) ([]int64, error) {
	if h.mistraDB == nil {
		return nil, nil
	}
	rows, err := h.mistraDB.QueryContext(ctx, `
		SELECT d.id
		FROM loader.hubs_deal d
		LEFT JOIN loader.hubs_company c ON c.id = d.company_id
		WHERE COALESCE(c.name, '') ILIKE $1
		LIMIT 5000
	`, "%"+q+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func scanRichiestaSummary(scanner interface{ Scan(dest ...any) error }) (RichiestaSummary, error) {
	var (
		item                   RichiestaSummary
		dealID                 sql.NullInt64
		dataRichiesta          time.Time
		annotazioniRichiedente sql.NullString
		annotazioniCarrier     sql.NullString
		createdBy              sql.NullString
		createdAt              time.Time
		updatedAt              sql.NullTime
		fornitoriPreferitiRaw  sql.NullString
	)

	if err := scanner.Scan(
		&item.ID,
		&dealID,
		&dataRichiesta,
		&item.Descrizione,
		&item.Indirizzo,
		&item.Stato,
		&annotazioniRichiedente,
		&annotazioniCarrier,
		&createdBy,
		&createdAt,
		&updatedAt,
		&fornitoriPreferitiRaw,
		&item.CodiceDeal,
		&item.Counts.Bozza,
		&item.Counts.Inviata,
		&item.Counts.Sollecitata,
		&item.Counts.Completata,
		&item.Counts.Annullata,
		&item.Counts.Totale,
	); err != nil {
		return RichiestaSummary{}, err
	}

	item.DealID = nullInt64Ptr(dealID)
	item.DataRichiesta = formatDate(dataRichiesta)
	item.AnnotazioniRichiedente = nullStringPtr(annotazioniRichiedente)
	item.AnnotazioniCarrier = nullStringPtr(annotazioniCarrier)
	item.CreatedBy = nullStringPtr(createdBy)
	item.CreatedAt = formatTimestamp(createdAt)
	item.UpdatedAt = nullTimePtr(updatedAt)
	item.FornitoriPreferiti = parseNullableIntArrayLiteral(fornitoriPreferitiRaw)
	return item, nil
}

func countFattibilita(items []Fattibilita) FattibilitaCounts {
	var counts FattibilitaCounts
	for _, item := range items {
		counts.Totale++
		switch item.Stato {
		case "bozza":
			counts.Bozza++
		case "inviata":
			counts.Inviata++
		case "sollecitata":
			counts.Sollecitata++
		case "completata":
			counts.Completata++
		case "annullata":
			counts.Annullata++
		}
	}
	return counts
}

func shouldNotifyDiff(before, after Fattibilita) bool {
	if before.Stato != after.Stato {
		return true
	}
	if before.Copertura != after.Copertura {
		return true
	}
	return !equalFloatPointers(before.NRC, after.NRC) || !equalFloatPointers(before.MRC, after.MRC)
}

func eligibleDealCondition(dealAlias, stageAlias string) string {
	return fmt.Sprintf(`(
		(%s.pipeline = '%s' AND COALESCE(%s.display_order, 0) BETWEEN 1 AND 5)
		OR
		(%s.pipeline = '%s' AND COALESCE(%s.display_order, 0) BETWEEN 3 AND 8)
	) AND COALESCE(%s.codice, '') <> ''`,
		dealAlias, standardPipelineID, stageAlias,
		dealAlias, iaasPipelineID, stageAlias,
		dealAlias,
	)
}

func (h *Handler) requireAnisetta(w http.ResponseWriter) bool {
	if h.anisettaDB == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "anisetta_database_not_configured")
		return false
	}
	return true
}

func (h *Handler) requireMistra(w http.ResponseWriter) bool {
	if h.mistraDB == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "mistra_database_not_configured")
		return false
	}
	return true
}

func (h *Handler) dbFailure(w http.ResponseWriter, r *http.Request, operation string, err error, attrs ...any) {
	args := []any{"component", "rdf", "operation", operation}
	args = append(args, attrs...)
	httputil.InternalError(w, r, err, "database operation failed", args...)
}

func (h *Handler) rollbackTx(ctx context.Context, tx *sql.Tx, operation string, attrs ...any) {
	if tx == nil {
		return
	}
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		args := []any{"component", "rdf", "operation", operation, "error", err}
		args = append(args, attrs...)
		logging.FromContext(ctx).Warn("transaction rollback failed", args...)
	}
}

func decodeBody(r *http.Request, dst any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(dst)
}

func queryPositiveInt(r *http.Request, name string, fallback int) int {
	value := strings.TrimSpace(r.URL.Query().Get(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	if parsed > 100 {
		return 100
	}
	return parsed
}

func queryCSV(r *http.Request, name string) []string {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func pathInt(r *http.Request, name string) (int, error) {
	return strconv.Atoi(r.PathValue(name))
}

func pathInt64(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}

func placeholder(args *[]any, value any) string {
	*args = append(*args, value)
	return fmt.Sprintf("$%d", len(*args))
}

func parseIntArrayLiteral(raw string) []int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []int{}
	}
	raw = strings.TrimPrefix(raw, "{")
	raw = strings.TrimSuffix(raw, "}")
	if strings.TrimSpace(raw) == "" {
		return []int{}
	}
	parts := strings.Split(raw, ",")
	result := make([]int, 0, len(parts))
	seen := make(map[int]struct{}, len(parts))
	for _, part := range parts {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func parseNullableIntArrayLiteral(raw sql.NullString) []int {
	if !raw.Valid {
		return []int{}
	}
	return parseIntArrayLiteral(raw.String)
}

func formatIntArrayLiteral(ids []int) string {
	if len(ids) == 0 {
		return ""
	}
	seen := make(map[int]struct{}, len(ids))
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		parts = append(parts, strconv.Itoa(id))
	}
	if len(parts) == 0 {
		return ""
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func parseNullableDate(raw string) (*time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func formatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

func formatTimestamp(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func nullTimePtr(value sql.NullTime) *string {
	if !value.Valid {
		return nil
	}
	formatted := formatTimestamp(value.Time)
	return &formatted
}

func nullDatePtr(value sql.NullTime) *string {
	if !value.Valid {
		return nil
	}
	formatted := formatDate(value.Time)
	return &formatted
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	trimmed := strings.TrimSpace(value.String)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func nullInt64Ptr(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	v := value.Int64
	return &v
}

func nullIntPtr(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	v := int(value.Int64)
	return &v
}

func nullFloat64Ptr(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	v := value.Float64
	return &v
}

func nullableTrimmedString(value string) any {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func nullIfEmpty(value string) any {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func equalFloatPointers(left, right *float64) bool {
	if left == nil || right == nil {
		return left == right
	}
	return math.Abs(*left-*right) < 0.000001
}

func stringPointer(value string) *string {
	return &value
}

func derefString(value *string, fallback string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return fallback
	}
	return *value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func yesNo(value bool) string {
	if value {
		return "SI"
	}
	return "NO"
}

func budgetLabel(score int) string {
	if score <= 0 || score >= len(scoreBudgetLabels) {
		return ""
	}
	return scoreBudgetLabels[score]
}
