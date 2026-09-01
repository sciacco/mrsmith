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

const arakRDAsQuery = `SELECT
    po.id,
    po.type,
    po.project,
    po.description,
    po.note,
    po.object,
    po.requester_id,
    po.code,
    po.payment_method,
    po.budget_id,
    po.cost_center,
    po.budget_user_id,
    po.provider_id,
    po.currency,
    po.leasing,
    po.total_price,
    po.state,
    po.created_document,
    po.created,
    po.updated,
    po.deleted,
    po.budget_increment_id,
    po.subtracted_from_budget,
    po.provider_offer_date,
    po.provider_offer_code,
    po.reference_warehouse,
    po.advance_payment,
    p.company_name AS provider_company_name,
    p.erp_id,
    p.state AS provider_state,
    b.name AS budget_name,
    b.year AS budget_year,
    u.email AS requester_email,
    pm.code AS payment_method_code,
    pm.description AS payment_method_description,
    (
        SELECT min(a.level) AS min
        FROM rda.approval a
        WHERE a.order_id = po.id
          AND po.state::text = 'PENDING_APPROVAL'::text
          AND NOT (
              EXISTS (
                  SELECT 1
                  FROM rda.approval a2
                  WHERE a2.order_id = po.id
                    AND a2.level = a.level
                    AND (a2.state::text = ANY (ARRAY['APPROVED'::character varying, 'REJECTED'::character varying]::text[]))
              )
          )
    ) AS current_approval_level,
    por.id AS row_id,
    por.product_code,
    por."type" AS row_type,
    por.qty,
    por.nrc,
    por.mrc,
    por.product_description,
    por.description AS row_description,
    por.total,
    por.price
FROM rda.purchase_order po
    LEFT JOIN provider_qualifications.provider p ON p.id = po.provider_id
    LEFT JOIN budgets.budget b ON b.id = po.budget_id
    LEFT JOIN users_int."user" u ON u.id = po.requester_id
    LEFT JOIN provider_qualifications.payment_method pm ON pm.code = po.payment_method
    LEFT JOIN rda.reference_warehouse wh ON wh.name::text = po.reference_warehouse::text
    LEFT JOIN rda.purchase_order_row por ON por.order_id = po.id
WHERE po."state" NOT IN ('DRAFT','CANCELED')
  AND po.deleted IS NULL
ORDER BY po.id, por.id;`

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
	arakDB     *sql.DB
}

// ArakRDARow is one row of the RDA Arak extraction: header fields are repeated
// for each purchase_order_row. Null join results (provider, budget,
// payment method, rows) are represented as nil pointers.
type ArakRDARow struct {
	ID                       *int64     `json:"id"`
	Type                     *string    `json:"type"`
	Project                  *string    `json:"project"`
	Description              *string    `json:"description"`
	Note                     *string    `json:"note"`
	Object                   *string    `json:"object"`
	RequesterID              *int64     `json:"requester_id"`
	Code                     *string    `json:"code"`
	PaymentMethod            *string    `json:"payment_method"`
	BudgetID                 *int64     `json:"budget_id"`
	CostCenter               *string    `json:"cost_center"`
	BudgetUserID             *int64     `json:"budget_user_id"`
	ProviderID               *int64     `json:"provider_id"`
	Currency                 *string    `json:"currency"`
	Leasing                  *bool      `json:"leasing"`
	TotalPrice               *float64   `json:"total_price"`
	State                    *string    `json:"state"`
	CreatedDocument          *string    `json:"created_document"`
	Created                  *time.Time `json:"created"`
	Updated                  *time.Time `json:"updated"`
	Deleted                  *time.Time `json:"deleted"`
	BudgetIncrementID        *int64     `json:"budget_increment_id"`
	SubtractedFromBudget     *bool      `json:"subtracted_from_budget"`
	ProviderOfferDate        *time.Time `json:"provider_offer_date"`
	ProviderOfferCode        *string    `json:"provider_offer_code"`
	ReferenceWarehouse       *string    `json:"reference_warehouse"`
	AdvancePayment           *bool      `json:"advance_payment"`
	ProviderCompanyName      *string    `json:"provider_company_name"`
	ErpID                    *string    `json:"erp_id"`
	ProviderState            *string    `json:"provider_state"`
	BudgetName               *string    `json:"budget_name"`
	BudgetYear               *int       `json:"budget_year"`
	RequesterEmail           *string    `json:"requester_email"`
	PaymentMethodCode        *string    `json:"payment_method_code"`
	PaymentMethodDescription *string    `json:"payment_method_description"`
	CurrentApprovalLevel     *int64     `json:"current_approval_level"`
	RowID                    *int64     `json:"row_id"`
	ProductCode              *string    `json:"product_code"`
	RowType                  *string    `json:"row_type"`
	Qty                      *float64   `json:"qty"`
	NRC                      *float64   `json:"nrc"`
	MRC                      *float64   `json:"mrc"`
	ProductDescription       *string    `json:"product_description"`
	RowDescription           *string    `json:"row_description"`
	Total                    *float64   `json:"total"`
	Price                    *float64   `json:"price"`
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

func RegisterRoutes(mux *http.ServeMux, alyanteDB, arakDB *sql.DB) {
	h := &Handler{alyanteDB: alyanteDB, arakDB: arakDB}
	protect := acl.RequireRole(applaunch.SmartPassiveAccessRoles()...)
	mux.Handle("GET /smart-passive/v1/alyante-invoices", protect(http.HandlerFunc(h.handleAlyanteInvoices)))
	mux.Handle("GET /smart-passive/v1/arak-rdas", protect(http.HandlerFunc(h.handleArakRDAs)))
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

func (h *Handler) handleArakRDAs(w http.ResponseWriter, r *http.Request) {
	if h.arakDB == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "Connessione Arak non configurata")
		return
	}

	rows, err := h.arakDB.QueryContext(r.Context(), arakRDAsQuery)
	if err != nil {
		httputil.InternalError(w, r, err, "arak rdas query failed", "component", component, "operation", "list_arak_rdas")
		return
	}
	defer rows.Close()

	out := make([]ArakRDARow, 0)
	for rows.Next() {
		var row ArakRDARow
		var id sql.NullInt64
		var typ sql.NullString
		var project sql.NullString
		var description sql.NullString
		var note sql.NullString
		var object sql.NullString
		var requesterID sql.NullInt64
		var code sql.NullString
		var paymentMethod sql.NullString
		var budgetID sql.NullInt64
		var costCenter sql.NullString
		var budgetUserID sql.NullInt64
		var providerID sql.NullInt64
		var currency sql.NullString
		var leasing sql.NullBool
		var totalPrice sql.NullFloat64
		var state sql.NullString
		var createdDocument sql.NullString
		var created sql.NullTime
		var updated sql.NullTime
		var deleted sql.NullTime
		var budgetIncrementID sql.NullInt64
		var subtractedFromBudget sql.NullBool
		var providerOfferDate sql.NullTime
		var providerOfferCode sql.NullString
		var referenceWarehouse sql.NullString
		var advancePayment sql.NullBool
		var providerCompanyName sql.NullString
		var erpID sql.NullString
		var providerState sql.NullString
		var budgetName sql.NullString
		var budgetYear sql.NullInt64
		var requesterEmail sql.NullString
		var paymentMethodCode sql.NullString
		var paymentMethodDescription sql.NullString
		var currentApprovalLevel sql.NullInt64
		var rowID sql.NullInt64
		var productCode sql.NullString
		var rowType sql.NullString
		var qty sql.NullFloat64
		var nrc sql.NullFloat64
		var mrc sql.NullFloat64
		var productDescription sql.NullString
		var rowDescription sql.NullString
		var total sql.NullFloat64
		var price sql.NullFloat64

		if err := rows.Scan(
			&id,
			&typ,
			&project,
			&description,
			&note,
			&object,
			&requesterID,
			&code,
			&paymentMethod,
			&budgetID,
			&costCenter,
			&budgetUserID,
			&providerID,
			&currency,
			&leasing,
			&totalPrice,
			&state,
			&createdDocument,
			&created,
			&updated,
			&deleted,
			&budgetIncrementID,
			&subtractedFromBudget,
			&providerOfferDate,
			&providerOfferCode,
			&referenceWarehouse,
			&advancePayment,
			&providerCompanyName,
			&erpID,
			&providerState,
			&budgetName,
			&budgetYear,
			&requesterEmail,
			&paymentMethodCode,
			&paymentMethodDescription,
			&currentApprovalLevel,
			&rowID,
			&productCode,
			&rowType,
			&qty,
			&nrc,
			&mrc,
			&productDescription,
			&rowDescription,
			&total,
			&price,
		); err != nil {
			httputil.InternalError(w, r, err, "arak rdas scan failed", "component", component, "operation", "list_arak_rdas")
			return
		}

		row.ID = int64Ptr(id)
		row.Type = stringPtr(typ)
		row.Project = stringPtr(project)
		row.Description = stringPtr(description)
		row.Note = stringPtr(note)
		row.Object = stringPtr(object)
		row.RequesterID = int64Ptr(requesterID)
		row.Code = stringPtr(code)
		row.PaymentMethod = stringPtr(paymentMethod)
		row.BudgetID = int64Ptr(budgetID)
		row.CostCenter = stringPtr(costCenter)
		row.BudgetUserID = int64Ptr(budgetUserID)
		row.ProviderID = int64Ptr(providerID)
		row.Currency = stringPtr(currency)
		row.Leasing = boolPtr(leasing)
		row.TotalPrice = float64Ptr(totalPrice)
		row.State = stringPtr(state)
		row.CreatedDocument = stringPtr(createdDocument)
		row.Created = timePtr(created)
		row.Updated = timePtr(updated)
		row.Deleted = timePtr(deleted)
		row.BudgetIncrementID = int64Ptr(budgetIncrementID)
		row.SubtractedFromBudget = boolPtr(subtractedFromBudget)
		row.ProviderOfferDate = timePtr(providerOfferDate)
		row.ProviderOfferCode = stringPtr(providerOfferCode)
		row.ReferenceWarehouse = stringPtr(referenceWarehouse)
		row.AdvancePayment = boolPtr(advancePayment)
		row.ProviderCompanyName = stringPtr(providerCompanyName)
		row.ErpID = stringPtr(erpID)
		row.ProviderState = stringPtr(providerState)
		row.BudgetName = stringPtr(budgetName)
		row.BudgetYear = intPtr(budgetYear)
		row.RequesterEmail = stringPtr(requesterEmail)
		row.PaymentMethodCode = stringPtr(paymentMethodCode)
		row.PaymentMethodDescription = stringPtr(paymentMethodDescription)
		row.CurrentApprovalLevel = int64Ptr(currentApprovalLevel)
		row.RowID = int64Ptr(rowID)
		row.ProductCode = stringPtr(productCode)
		row.RowType = stringPtr(rowType)
		row.Qty = float64Ptr(qty)
		row.NRC = float64Ptr(nrc)
		row.MRC = float64Ptr(mrc)
		row.ProductDescription = stringPtr(productDescription)
		row.RowDescription = stringPtr(rowDescription)
		row.Total = float64Ptr(total)
		row.Price = float64Ptr(price)

		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		httputil.InternalError(w, r, err, "arak rdas rows failed", "component", component, "operation", "list_arak_rdas")
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

func boolPtr(value sql.NullBool) *bool {
	if !value.Valid {
		return nil
	}
	return &value.Bool
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
