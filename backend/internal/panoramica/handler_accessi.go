package panoramica

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// handleListConnectionTypes returns distinct connection types.
// GET /panoramica/v1/connection-types
func (h *Handler) handleListConnectionTypes(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	rows, err := h.mistraDB.QueryContext(r.Context(),
		`SELECT DISTINCT tipo_conn FROM loader.grappa_foglio_linee ORDER BY tipo_conn`)
	if err != nil {
		h.dbFailure(w, r, "list_connection_types", err)
		return
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			h.dbFailure(w, r, "list_connection_types_scan", err)
			return
		}
		result = append(result, s)
	}
	if !h.rowsDone(w, r, rows, "list_connection_types") {
		return
	}
	if result == nil {
		result = []string{}
	}

	httputil.JSON(w, http.StatusOK, result)
}

// handleListAccessLines returns access lines filtered by clients, stati, and connection types.
// GET /panoramica/v1/access-lines?clienti=1,2,3&stati=Attiva,Cessata&tipi=FTTH,FTTC
func (h *Handler) handleListAccessLines(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	clientiStr := r.URL.Query().Get("clienti")
	clientiParts := parseStringList(clientiStr)
	if len(clientiParts) == 0 {
		httputil.Error(w, http.StatusBadRequest, "missing_clienti_parameter")
		return
	}
	// Validate clienti as integers
	clienti := make([]int, 0, len(clientiParts))
	for _, p := range clientiParts {
		v, err := strconv.Atoi(p)
		if err != nil {
			httputil.Error(w, http.StatusBadRequest, "invalid_clienti_parameter")
			return
		}
		clienti = append(clienti, v)
	}

	statiStr := r.URL.Query().Get("stati")
	stati := parseStringList(statiStr)
	if len(stati) == 0 {
		httputil.Error(w, http.StatusBadRequest, "missing_stati_parameter")
		return
	}

	tipiStr := r.URL.Query().Get("tipi")
	tipi := parseStringList(tipiStr)
	if len(tipi) == 0 {
		httputil.Error(w, http.StatusBadRequest, "missing_tipi_parameter")
		return
	}

	// Build parameterized query with dynamic placeholders
	args := make([]any, 0)
	paramIdx := 1

	clientiPlaceholders := ""
	for i, c := range clienti {
		if i > 0 {
			clientiPlaceholders += ","
		}
		clientiPlaceholders += fmt.Sprintf("$%d", paramIdx)
		args = append(args, c)
		paramIdx++
	}

	statiPlaceholders := ""
	for i, s := range stati {
		if i > 0 {
			statiPlaceholders += ","
		}
		statiPlaceholders += fmt.Sprintf("$%d", paramIdx)
		args = append(args, s)
		paramIdx++
	}

	tipiPlaceholders := ""
	for i, t := range tipi {
		if i > 0 {
			tipiPlaceholders += ","
		}
		tipiPlaceholders += fmt.Sprintf("$%d", paramIdx)
		args = append(args, t)
		paramIdx++
	}

	query := fmt.Sprintf(`SELECT
    tipo_conn, fl.fornitore, provincia, comune, p.tipo, p.profilo_commerciale,
    cogn_rsoc_intest_linea AS intestatario, r.nome_testata_ordine AS ordine,
    r.data_ultima_fatt AS fatturato_fino_al, r.stato_riga, r.stato_ordine,
    fl.stato, fl.id, codice_ordine, fl.serialnumber, cf.codice_aggancio_gest AS id_anagrafica
FROM loader.grappa_foglio_linee fl
  JOIN loader.grappa_cli_fatturazione cf ON fl.id_anagrafica = cf.id
  LEFT JOIN loader.grappa_profili p ON fl.id_profilo = p.id
  LEFT JOIN (
    SELECT *, ROW_NUMBER() OVER(PARTITION BY serialnumber ORDER BY data_documento DESC, progressivo_riga) AS rn
    FROM loader.v_ordini_ricorrenti
  ) r ON fl.serialnumber = r.serialnumber AND r.numero_azienda = cf.codice_aggancio_gest AND r.rn = 1
WHERE fl.id_anagrafica IN (%s)
  AND fl.stato IN (%s)
  AND fl.tipo_conn IN (%s)
ORDER BY tipo_conn, fl.fornitore, provincia, comune, p.tipo, p.profilo_commerciale`,
		clientiPlaceholders, statiPlaceholders, tipiPlaceholders)

	rows, err := h.mistraDB.QueryContext(r.Context(), query, args...)
	if err != nil {
		h.dbFailure(w, r, "list_access_lines", err)
		return
	}
	defer rows.Close()

	type accessLine struct {
		TipoConn           string  `json:"tipo_conn"`
		Fornitore          *string `json:"fornitore"`
		Provincia          *string `json:"provincia"`
		Comune             *string `json:"comune"`
		Tipo               *string `json:"tipo"`
		ProfiloCommerciale *string `json:"profilo_commerciale"`
		Intestatario       *string `json:"intestatario"`
		Ordine             *string `json:"ordine"`
		FatturatoFinoAl    *string `json:"fatturato_fino_al"`
		StatoRiga          *string `json:"stato_riga"`
		StatoOrdine        *string `json:"stato_ordine"`
		Stato              string  `json:"stato"`
		ID                 int     `json:"id"`
		CodiceOrdine       *string `json:"codice_ordine"`
		Serialnumber       *string `json:"serialnumber"`
		IDAnagrafica       *int    `json:"id_anagrafica"`
	}

	var result []accessLine
	for rows.Next() {
		var a accessLine
		var (
			fornitore, provincia, comune, tipo, profComm     sql.NullString
			intestatario, ordine, fattFinoAl                 sql.NullString
			statoRiga, statoOrdine, codiceOrdine, serialnum  sql.NullString
			idAnagrafica                                     sql.NullInt64
		)

		if err := rows.Scan(
			&a.TipoConn, &fornitore, &provincia, &comune, &tipo, &profComm,
			&intestatario, &ordine, &fattFinoAl, &statoRiga, &statoOrdine,
			&a.Stato, &a.ID, &codiceOrdine, &serialnum, &idAnagrafica,
		); err != nil {
			h.dbFailure(w, r, "list_access_lines_scan", err)
			return
		}

		a.Fornitore = nullStringPtr(fornitore)
		a.Provincia = nullStringPtr(provincia)
		a.Comune = nullStringPtr(comune)
		a.Tipo = nullStringPtr(tipo)
		a.ProfiloCommerciale = nullStringPtr(profComm)
		a.Intestatario = nullStringPtr(intestatario)
		a.Ordine = nullStringPtr(ordine)
		a.FatturatoFinoAl = nullStringPtr(fattFinoAl)
		a.StatoRiga = nullStringPtr(statoRiga)
		a.StatoOrdine = nullStringPtr(statoOrdine)
		a.CodiceOrdine = nullStringPtr(codiceOrdine)
		a.Serialnumber = nullStringPtr(serialnum)
		if idAnagrafica.Valid {
			v := int(idAnagrafica.Int64)
			a.IDAnagrafica = &v
		}

		result = append(result, a)
	}
	if !h.rowsDone(w, r, rows, "list_access_lines") {
		return
	}
	if result == nil {
		result = []accessLine{}
	}

	httputil.JSON(w, http.StatusOK, result)
}
