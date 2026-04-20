package afctools

import (
	"net/http"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// DdtCespitoRow represents one row from the Alyante MSSQL view
// Tsmi_DDT_Verifica_Cespiti with its fixed schema.
type DdtCespitoRow struct {
	CodiceDocUscita string     `json:"Codice_doc_uscita"`
	TipoDocUscita   *string    `json:"Tipo_doc_uscita"`
	NumDocUscita    *string    `json:"Num_doc_uscita"`
	DataDocUscita   time.Time  `json:"Data_doc_uscita"`
	Quantita        float64    `json:"Quantita"`
	CodiceArticolo  *string    `json:"Codice_articolo"`
	Descrizione     *string    `json:"Descrizione"`
	ImportoUnitario float64    `json:"Importo_unitario"`
	ImportoTotale   *float64   `json:"Importo_totale"`
	Seriali         *string    `json:"Seriali"`
	NumDocIngresso  *string    `json:"Num_doc_ingresso"`
	DataDocIngresso *string    `json:"Data_doc_ingresso"`
}

func (h *Handler) listDdtCespiti(r *http.Request) ([]DdtCespitoRow, error) {
	const query = `SELECT Codice_doc_uscita,
       Tipo_doc_uscita,
       Num_doc_uscita,
       Data_doc_uscita,
       Quantita,
       Codice_articolo,
       Descrizione,
       Importo_unitario,
       Importo_totale,
       Seriali,
       Num_doc_ingresso,
       Data_doc_ingresso
FROM Tsmi_DDT_Verifica_Cespiti`

	rows, err := h.deps.Alyante.QueryContext(r.Context(), query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]DdtCespitoRow, 0)
	for rows.Next() {
		var row DdtCespitoRow
		if err := rows.Scan(
			&row.CodiceDocUscita,
			&row.TipoDocUscita,
			&row.NumDocUscita,
			&row.DataDocUscita,
			&row.Quantita,
			&row.CodiceArticolo,
			&row.Descrizione,
			&row.ImportoUnitario,
			&row.ImportoTotale,
			&row.Seriali,
			&row.NumDocIngresso,
			&row.DataDocIngresso,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
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
