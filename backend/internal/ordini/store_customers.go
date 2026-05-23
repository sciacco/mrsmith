package ordini

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const alyanteCustomersQuery = `
SELECT NUMERO_AZIENDA, RAGIONE_SOCIALE
FROM Tsmi_Anagrafiche_clienti
WHERE DATA_DISMISSIONE IS NULL
  AND RAGGRUPPAMENTO_3 <> 'Ecommerce'
  AND TIPOLOGIA_AZIENDA <> 'DIPENDENTE'
GROUP BY NUMERO_AZIENDA, RAGIONE_SOCIALE
ORDER BY RAGIONE_SOCIALE ASC`

const alyanteCustomerByIDQuery = `
SELECT TOP 1 NUMERO_AZIENDA, RAGIONE_SOCIALE
FROM Tsmi_Anagrafiche_clienti
WHERE NUMERO_AZIENDA = @p1
  AND DATA_DISMISSIONE IS NULL
  AND RAGGRUPPAMENTO_3 <> 'Ecommerce'
  AND TIPOLOGIA_AZIENDA <> 'DIPENDENTE'`

func (h *Handler) listCustomers(ctx context.Context) ([]CustomerRef, error) {
	rows, err := h.deps.Alyante.QueryContext(ctx, alyanteCustomersQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]CustomerRef, 0)
	for rows.Next() {
		var id sql.NullInt64
		var name sql.NullString
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		if !id.Valid || !name.Valid || strings.TrimSpace(name.String) == "" {
			continue
		}
		items = append(items, CustomerRef{ID: id.Int64, Name: strings.TrimSpace(name.String)})
	}
	return items, rows.Err()
}

func (h *Handler) getCustomerByID(ctx context.Context, id int64) (*CustomerRef, error) {
	var rawID sql.NullInt64
	var rawName sql.NullString
	err := h.deps.Alyante.QueryRowContext(ctx, alyanteCustomerByIDQuery, id).Scan(&rawID, &rawName)
	if err != nil {
		return nil, err
	}
	if !rawID.Valid || !rawName.Valid || strings.TrimSpace(rawName.String) == "" {
		return nil, sql.ErrNoRows
	}
	return &CustomerRef{ID: rawID.Int64, Name: strings.TrimSpace(rawName.String)}, nil
}

func (h *Handler) handleListCustomers(w http.ResponseWriter, r *http.Request) {
	if !h.requireAlyante(w) {
		return
	}
	items, err := h.listCustomers(r.Context())
	if err != nil {
		h.dbFailure(w, r, "list_customers", err)
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}
