package smartpassive

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const component = "smartpassive"

const alyanteInvoicesQuery = `SELECT
    t.DO11_DITTA_CG18,
    t.DO11_NUMREG_CO99,
    t.DO11_DOCUM_MG36,
    d.MG36_DESCDOCUM,
    t.DO11_NUMDOC,
    t.DO11_SEZDOC,
    t.DO11_DATADOC,
    t.DO11_CLIFOR_CG44,
    t.DO11_NUMDOCORIG,
    t.DO11_NOTEDOCUM,
    tot.DO13_TOTDOCUMENTO,
    tot.DO13_TOTAPAGARE,
    s.NUM_RATE_APERTE,
    s.TOTRATE,
    s.EF01_SCADE_S,
    s.EF01_IMPEFFORIG,
    ISNULL(s.RESIDUO, tot.DO13_TOTAPAGARE) AS RESIDUO,
    CASE
        WHEN s.EF01_NUMREG_CO99 IS NULL THEN CAST(0 AS decimal(18,2))
        ELSE ISNULL(s.EF01_IMPEFFORIG, 0) - ISNULL(s.RESIDUO, 0)
    END AS PAGATO_SU_RESIDUO,
    IIF(s.EF01_NUMREG_CO99 IS NOT NULL, 1, 0) AS IN_SCADENZIARIO,
    r.DO30_PROGRIGA,
    r.DO30_PROGVISUASTA,
    r.DO30_INDTIPORIGA,
    r.DO30_CODART_MG66,
    r.DO30_DESCART,
    r.DO30_UM1,
    r.DO30_QTA1,
    r.DO30_PREZZO1,
    r.DO30_SCPER1,
    r.DO30_SCPER2,
    r.DO30_SCPER3,
    r.DO30_SCIMP,
    r.DO30_IMPORTO,
    r.DO30_IMPNETSCP,
    r.DO30_ALIVA_CG28,
    r.DO30_IMPORTOIVA
FROM dbo.DO11_DOCTESTATA AS t
INNER JOIN dbo.MG36_DOCUMENTI AS d
    ON d.MG36_CODDOCUM = t.DO11_DOCUM_MG36
LEFT JOIN dbo.DO13_DOCTOTALI AS tot
    ON tot.DO13_DITTA_CG18 = t.DO11_DITTA_CG18
   AND tot.DO13_NUMREG_CO99 = t.DO11_NUMREG_CO99
LEFT JOIN (
    SELECT
        EF01_DITTA_CG18,
        EF01_NUMREG_CO99,
        MAX(EF01_TOTRATE) AS TOTRATE,
        SUM(CASE
                WHEN EF01_INDSTATO_S = 0
                 AND ABS(ISNULL(EF01_IMPEFF_S, 0)) > 0.02
                THEN 1 ELSE 0
            END) AS NUM_RATE_APERTE,
        MIN(CASE
                WHEN EF01_INDSTATO_S = 0
                 AND ABS(ISNULL(EF01_IMPEFF_S, 0)) > 0.02
                THEN EF01_SCADE_S
            END) AS EF01_SCADE_S,
        SUM(ISNULL(EF01_IMPEFFORIG, 0)) AS EF01_IMPEFFORIG,
        SUM(CASE
                WHEN EF01_INDSTATO_S = 0
                 AND ABS(ISNULL(EF01_IMPEFF_S, 0)) > 0.02
                THEN ISNULL(EF01_IMPEFF_S, 0)
                ELSE CAST(0 AS decimal(18,2))
            END) AS RESIDUO
    FROM dbo.EF01_SCADENZE
    WHERE EF01_DITTA_CG18 = 1
    GROUP BY
        EF01_DITTA_CG18,
        EF01_NUMREG_CO99
) AS s
    ON s.EF01_DITTA_CG18 = t.DO11_DITTA_CG18
   AND s.EF01_NUMREG_CO99 = t.DO11_NUMREG_CO99
LEFT JOIN dbo.DO30_DOCCORPO AS r
    ON r.DO30_DITTA_CG18 = t.DO11_DITTA_CG18
   AND r.DO30_NUMREG_CO99 = t.DO11_NUMREG_CO99
WHERE d.MG36_INDCLIFOR = 2
  AND d.MG36_INDLISACQVEN = 1
  AND d.MG36_TIPODOC IN (3, 4, 5)
  AND t.DO11_DITTA_CG18 = 1
  AND t.DO11_ANNODOC > 2025
  AND (
        s.EF01_NUMREG_CO99 IS NULL
     OR ABS(ISNULL(s.RESIDUO, 0)) > 0.02
  )
ORDER BY
    t.DO11_DATADOC,
    t.DO11_NUMDOC,
    r.DO30_PROGVISUASTA,
    r.DO30_PROGRIGA;`

type Handler struct {
	alyanteDB *sql.DB
}

type AlyanteInvoiceRow struct {
	DO11DittaCG18    *int       `json:"DO11_DITTA_CG18"`
	DO11NumregCO99   *int64     `json:"DO11_NUMREG_CO99"`
	DO11DocumMG36    *string    `json:"DO11_DOCUM_MG36"`
	MG36Descdocum    *string    `json:"MG36_DESCDOCUM"`
	DO11Numdoc       *string    `json:"DO11_NUMDOC"`
	DO11Sezdoc       *string    `json:"DO11_SEZDOC"`
	DO11Datadoc      *time.Time `json:"DO11_DATADOC"`
	DO11CliforCG44   *int64     `json:"DO11_CLIFOR_CG44"`
	DO11Numdocorig   *string    `json:"DO11_NUMDOCORIG"`
	DO11Notedocum    *string    `json:"DO11_NOTEDOCUM"`
	DO13Totdocumento *float64   `json:"DO13_TOTDOCUMENTO"`
	DO13Totapagare   *float64   `json:"DO13_TOTAPAGARE"`
	NumRateAperte    *int64     `json:"NUM_RATE_APERTE"`
	Totrate          *int64     `json:"TOTRATE"`
	EF01ScadeS       *time.Time `json:"EF01_SCADE_S"`
	EF01Impefforig   *float64   `json:"EF01_IMPEFFORIG"`
	Residuo          *float64   `json:"RESIDUO"`
	PagatoSuResiduo  *float64   `json:"PAGATO_SU_RESIDUO"`
	InScadenzario    bool       `json:"IN_SCADENZIARIO"`
	DO30Progriga     *int64     `json:"DO30_PROGRIGA"`
	DO30Progvisuasta *int64     `json:"DO30_PROGVISUASTA"`
	DO30Indtiporiga  *int64     `json:"DO30_INDTIPORIGA"`
	DO30CodartMG66   *string    `json:"DO30_CODART_MG66"`
	DO30Descart      *string    `json:"DO30_DESCART"`
	DO30UM1          *string    `json:"DO30_UM1"`
	DO30Qta1         *float64   `json:"DO30_QTA1"`
	DO30Prezzo1      *float64   `json:"DO30_PREZZO1"`
	DO30Scper1       *float64   `json:"DO30_SCPER1"`
	DO30Scper2       *float64   `json:"DO30_SCPER2"`
	DO30Scper3       *float64   `json:"DO30_SCPER3"`
	DO30Scimp        *float64   `json:"DO30_SCIMP"`
	DO30Importo      *float64   `json:"DO30_IMPORTO"`
	DO30Impnetscp    *float64   `json:"DO30_IMPNETSCP"`
	DO30AlivaCG28    *string    `json:"DO30_ALIVA_CG28"`
	DO30Importoiva   *float64   `json:"DO30_IMPORTOIVA"`
}

func RegisterRoutes(mux *http.ServeMux, alyanteDB *sql.DB) {
	h := &Handler{alyanteDB: alyanteDB}
	protect := acl.RequireRole(applaunch.SmartPassiveAccessRoles()...)
	mux.Handle("GET /smart-passive/v1/alyante-invoices", protect(http.HandlerFunc(h.handleAlyanteInvoices)))
}

func (h *Handler) handleAlyanteInvoices(w http.ResponseWriter, r *http.Request) {
	if h.alyanteDB == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "Connessione Alyante non configurata")
		return
	}

	rows, err := h.alyanteDB.QueryContext(r.Context(), alyanteInvoicesQuery)
	if err != nil {
		httputil.InternalError(w, r, err, "alyante invoices query failed", "component", component, "operation", "list_alyante_invoices")
		return
	}
	defer rows.Close()

	out := make([]AlyanteInvoiceRow, 0)
	for rows.Next() {
		var row AlyanteInvoiceRow
		var ditta sql.NullInt64
		var numreg sql.NullInt64
		var docum sql.NullString
		var descdocum sql.NullString
		var numdoc sql.NullString
		var sezdoc sql.NullString
		var datadoc sql.NullTime
		var clifor sql.NullInt64
		var numdocorig sql.NullString
		var notedocum sql.NullString
		var totdocumento sql.NullFloat64
		var totapagare sql.NullFloat64
		var numRateAperte sql.NullInt64
		var totrate sql.NullInt64
		var scade sql.NullTime
		var impefforig sql.NullFloat64
		var residuo sql.NullFloat64
		var pagatoSuResiduo sql.NullFloat64
		var inScadenzario sql.NullInt64
		var progriga sql.NullInt64
		var progvisuasta sql.NullInt64
		var indtiporiga sql.NullInt64
		var codart sql.NullString
		var descart sql.NullString
		var um1 sql.NullString
		var qta1 sql.NullFloat64
		var prezzo1 sql.NullFloat64
		var scper1 sql.NullFloat64
		var scper2 sql.NullFloat64
		var scper3 sql.NullFloat64
		var scimp sql.NullFloat64
		var importo sql.NullFloat64
		var impnetscp sql.NullFloat64
		var aliva sql.NullString
		var importoiva sql.NullFloat64

		if err := rows.Scan(
			&ditta,
			&numreg,
			&docum,
			&descdocum,
			&numdoc,
			&sezdoc,
			&datadoc,
			&clifor,
			&numdocorig,
			&notedocum,
			&totdocumento,
			&totapagare,
			&numRateAperte,
			&totrate,
			&scade,
			&impefforig,
			&residuo,
			&pagatoSuResiduo,
			&inScadenzario,
			&progriga,
			&progvisuasta,
			&indtiporiga,
			&codart,
			&descart,
			&um1,
			&qta1,
			&prezzo1,
			&scper1,
			&scper2,
			&scper3,
			&scimp,
			&importo,
			&impnetscp,
			&aliva,
			&importoiva,
		); err != nil {
			httputil.InternalError(w, r, err, "alyante invoices scan failed", "component", component, "operation", "list_alyante_invoices")
			return
		}

		row.DO11DittaCG18 = intPtr(ditta)
		row.DO11NumregCO99 = int64Ptr(numreg)
		row.DO11DocumMG36 = stringPtr(docum)
		row.MG36Descdocum = stringPtr(descdocum)
		row.DO11Numdoc = stringPtr(numdoc)
		row.DO11Sezdoc = stringPtr(sezdoc)
		row.DO11Datadoc = timePtr(datadoc)
		row.DO11CliforCG44 = int64Ptr(clifor)
		row.DO11Numdocorig = stringPtr(numdocorig)
		row.DO11Notedocum = stringPtr(notedocum)
		row.DO13Totdocumento = float64Ptr(totdocumento)
		row.DO13Totapagare = float64Ptr(totapagare)
		row.NumRateAperte = int64Ptr(numRateAperte)
		row.Totrate = int64Ptr(totrate)
		row.EF01ScadeS = timePtr(scade)
		row.EF01Impefforig = float64Ptr(impefforig)
		row.Residuo = float64Ptr(residuo)
		row.PagatoSuResiduo = float64Ptr(pagatoSuResiduo)
		row.InScadenzario = inScadenzario.Valid && inScadenzario.Int64 == 1
		row.DO30Progriga = int64Ptr(progriga)
		row.DO30Progvisuasta = int64Ptr(progvisuasta)
		row.DO30Indtiporiga = int64Ptr(indtiporiga)
		row.DO30CodartMG66 = stringPtr(codart)
		row.DO30Descart = stringPtr(descart)
		row.DO30UM1 = stringPtr(um1)
		row.DO30Qta1 = float64Ptr(qta1)
		row.DO30Prezzo1 = float64Ptr(prezzo1)
		row.DO30Scper1 = float64Ptr(scper1)
		row.DO30Scper2 = float64Ptr(scper2)
		row.DO30Scper3 = float64Ptr(scper3)
		row.DO30Scimp = float64Ptr(scimp)
		row.DO30Importo = float64Ptr(importo)
		row.DO30Impnetscp = float64Ptr(impnetscp)
		row.DO30AlivaCG28 = stringPtr(aliva)
		row.DO30Importoiva = float64Ptr(importoiva)

		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		httputil.InternalError(w, r, err, "alyante invoices rows failed", "component", component, "operation", "list_alyante_invoices")
		return
	}

	httputil.JSON(w, http.StatusOK, out)
}

func stringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func int64Ptr(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	return &value.Int64
}

func intPtr(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	v := int(value.Int64)
	return &v
}

func float64Ptr(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	return &value.Float64
}

func timePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}
