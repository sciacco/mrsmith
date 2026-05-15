package cpbackoffice

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// listBiometricRequestsQuery is the locked SQL used by
// GET /cp-backoffice/v1/biometric-requests. The four source anchors
// (`customers.biometric_request`, `customers.user_struct`,
// `customers.customer`, `customers.user_entrance_detail`) and the
// `ORDER BY data_richiesta DESC` tail are part of the Slice S4 contract
// in apps/customer-portal/FINAL.md and must not drift.
const listBiometricRequestsQuery = `SELECT
    br.id,
    us.first_name AS nome,
    us.last_name AS cognome,
    us.primary_email AS email,
    c.name AS azienda,
    br.request_type::text AS tipo_richiesta,
    br.request_completed AS stato_richiesta,
    br.request_date AS data_richiesta,
    br.request_approval_date AS data_approvazione,
    COALESCE(ued.is_biometric, false) AS is_biometric_lenel
FROM customers.biometric_request br
JOIN customers.user_struct us ON us.id = br.user_struct_id
LEFT JOIN customers.customer c ON c.id = br.customer_id
LEFT JOIN customers.user_entrance_detail ued ON ued.email = us.primary_email
ORDER BY data_richiesta DESC`

// setBiometricCompletedQuery updates the tri-state request_completed value:
// true=confirmed, false=pending, null=cancelled. The approval timestamp mirrors
// the old stored-function behavior for true/false and is cleared for cancelled.
const setBiometricCompletedQuery = `UPDATE customers.biometric_request
SET
    request_completed = $2::boolean,
    request_approval_date = CASE
        WHEN $2::boolean IS NULL THEN NULL
        ELSE now()
    END
WHERE id = $1::bigint
RETURNING id`

// handleListBiometricRequests returns every biometric request, ordered by
// data_richiesta DESC. No pagination and no filters in v1.
func handleListBiometricRequests(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireMistra(deps) {
			writeDatabaseUnavailable(w)
			return
		}

		rows, err := deps.Mistra.QueryContext(r.Context(), listBiometricRequestsQuery)
		if err != nil {
			dbFailure(w, r, err, "biometric.list")
			return
		}
		defer rows.Close()

		out := make([]BiometricRequestRow, 0)
		for rows.Next() {
			var (
				row       BiometricRequestRow
				email     sql.NullString
				azienda   sql.NullString
				completed sql.NullBool
				approval  sql.NullTime
			)
			if err := rows.Scan(
				&row.ID,
				&row.Nome,
				&row.Cognome,
				&email,
				&azienda,
				&row.TipoRichiesta,
				&completed,
				&row.DataRichiesta,
				&approval,
				&row.IsBiometricLenel,
			); err != nil {
				dbFailure(w, r, err, "biometric.list.scan")
				return
			}
			if email.Valid {
				row.Email = email.String
			}
			if azienda.Valid {
				row.Azienda = azienda.String
			}
			if completed.Valid {
				v := completed.Bool
				row.StatoRichiesta = &v
			}
			if approval.Valid {
				t := approval.Time
				row.DataApprovazione = &t
			}
			out = append(out, row)
		}
		if err := rows.Err(); err != nil {
			dbFailure(w, r, err, "biometric.list.rows")
			return
		}

		httputil.JSON(w, http.StatusOK, out)
	}
}

// handleSetBiometricCompleted sets `request_completed` for a single biometric
// request row. The field is nullable: true=confirmed, false=pending,
// null=cancelled. The response body is the locked { "ok": true } shape.
func handleSetBiometricCompleted(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireMistra(deps) {
			writeDatabaseUnavailable(w)
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			httputil.Error(w, http.StatusBadRequest, "invalid_id")
			return
		}

		var body CompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.Error(w, http.StatusBadRequest, "invalid_body")
			return
		}

		var completed any
		if body.Completed != nil {
			completed = *body.Completed
		}

		var updatedID int64
		if err := deps.Mistra.QueryRowContext(r.Context(),
			setBiometricCompletedQuery, id, completed).Scan(&updatedID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				httputil.Error(w, http.StatusNotFound, "not_found")
				return
			}
			dbFailure(w, r, err, "biometric.setCompleted")
			return
		}

		httputil.JSON(w, http.StatusOK, CompletionResponse{Ok: true})
	}
}
