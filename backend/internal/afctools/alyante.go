package afctools

import (
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// DdtCespitoRow: the MSSQL view Tsmi_DDT_Verifica_Cespiti exposes an open-ended
// set of columns; for a strict 1:1 port we scan them generically as map[string]any
// so the frontend can render whatever the view returns, matching Appsmith's
// dynamic table binding. Preserves the `SELECT *` semantics (decision A.5.1e).
type DdtCespitoRow = map[string]any

func (h *Handler) listDdtCespiti(r *http.Request) ([]DdtCespitoRow, error) {
	const query = `SELECT * FROM Tsmi_DDT_Verifica_Cespiti`

	rows, err := h.deps.Alyante.QueryContext(r.Context(), query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	out := make([]DdtCespitoRow, 0)
	for rows.Next() {
		values := make([]any, len(cols))
		scanTargets := make([]any, len(cols))
		for i := range values {
			scanTargets[i] = &values[i]
		}
		if err := rows.Scan(scanTargets...); err != nil {
			return nil, err
		}

		row := make(DdtCespitoRow, len(cols))
		for i, c := range cols {
			row[c] = normalizeScanValue(values[i])
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// normalizeScanValue coerces driver-specific Go types (byte slices for text,
// time.Time for datetime) into JSON-friendly values. Nil remains nil.
func normalizeScanValue(v any) any {
	switch x := v.(type) {
	case []byte:
		return string(x)
	default:
		return x
	}
}

func (h *Handler) handleDdtCespiti(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w, h.deps.Alyante, "alyante") {
		return
	}
	rowsOut, err := h.listDdtCespiti(r)
	if err != nil {
		h.dbFailure(w, r, "list_ddt_cespiti", err)
		return
	}
	httputil.JSON(w, http.StatusOK, rowsOut)
}
