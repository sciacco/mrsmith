package afctools

import (
	"net/http"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// MistraMissingArticle mirrors the "articoli_non_in_alyante" query shape.
type MistraMissingArticle struct {
	Code          string   `json:"code"`
	Categoria     *string  `json:"categoria"`
	NRC           *float64 `json:"nrc"`
	MRC           *float64 `json:"mrc"`
	DescrizioneIT *string  `json:"descrizione_it"`
	DescrizioneEN *string  `json:"descrizione_en"`
}

// XConnectOrder mirrors the "All_orders_xcon" query shape.
// id_ordine comes from orders.order.id (int). data_creazione is ISO-8601.
type XConnectOrder struct {
	IDOrdine      int64     `json:"id_ordine"`
	CodiceOrdine  *string   `json:"codice_ordine"`
	Cliente       *string   `json:"cliente"`
	DataCreazione time.Time `json:"data_creazione"`
}

func (h *Handler) listMissingArticles(r *http.Request) ([]MistraMissingArticle, error) {
	const query = `
SELECT p.code,
       pc.name AS categoria,
       p.nrc,
       p.mrc,
       max(CASE WHEN t.language = 'it' THEN short END) AS descrizione_it,
       max(CASE WHEN t.language = 'en' THEN short END) AS descrizione_en
FROM loader.erp_anagrafica_articoli_vendita a
    RIGHT JOIN products.product p ON trim(a.cod_articolo) = p.code
    JOIN products.product_category pc ON pc.id = p.category_id
    LEFT JOIN common.translation t ON p.translation_uuid = t.translation_uuid
WHERE a.cod_articolo IS NULL AND p.erp_sync = true
GROUP BY 1, 2, 3, 4
`
	rows, err := h.deps.Mistra.QueryContext(r.Context(), query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]MistraMissingArticle, 0)
	for rows.Next() {
		var a MistraMissingArticle
		if err := rows.Scan(&a.Code, &a.Categoria, &a.NRC, &a.MRC, &a.DescrizioneIT, &a.DescrizioneEN); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (h *Handler) handleMissingArticles(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w, h.deps.Mistra, "mistra") {
		return
	}
	rowsOut, err := h.listMissingArticles(r)
	if err != nil {
		h.dbFailure(w, r, "list_missing_articles", err)
		return
	}
	httputil.JSON(w, http.StatusOK, rowsOut)
}

func (h *Handler) listXConnectOrders(r *http.Request) ([]XConnectOrder, error) {
	const query = `
SELECT o.id           AS id_ordine,
       hd.codice      AS codice_ordine,
       c.name         AS cliente,
       o.created_at   AS data_creazione
FROM loader.hubs_deal hd
JOIN loader.cp_ordini cpo    ON hd.id = cpo.hs_deal_id
JOIN orders.order o          ON o.order_number = cpo.order_number
JOIN customers.customer c    ON c.id = o.customer_id
JOIN orders.order_state os   ON os.id = o.state_id
WHERE o.kit_category = 'XCONNECT' AND os.name = 'EVASO'
ORDER BY o.created_at DESC
`
	rows, err := h.deps.Mistra.QueryContext(r.Context(), query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]XConnectOrder, 0)
	for rows.Next() {
		var o XConnectOrder
		if err := rows.Scan(&o.IDOrdine, &o.CodiceOrdine, &o.Cliente, &o.DataCreazione); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (h *Handler) handleXConnectOrders(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w, h.deps.Mistra, "mistra") {
		return
	}
	rowsOut, err := h.listXConnectOrders(r)
	if err != nil {
		h.dbFailure(w, r, "list_xconnect_orders", err)
		return
	}
	httputil.JSON(w, http.StatusOK, rowsOut)
}
