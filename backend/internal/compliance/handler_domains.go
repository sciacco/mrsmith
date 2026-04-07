package compliance

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleListDomainStatus(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	format := r.URL.Query().Get("format")
	search := r.URL.Query().Get("search")

	fullQuery := fmt.Sprintf(`
		SELECT domain, SUM(block_count) AS block_count, SUM(release_count) AS release_count
		FROM (
			SELECT domain, COUNT(*) AS block_count, 0 AS release_count
			FROM dns_bl_block_domain
			GROUP BY domain
			UNION ALL
			SELECT domain, 0, COUNT(*)
			FROM dns_bl_release_domain
			GROUP BY domain
		) agg
		%s
		GROUP BY domain
		ORDER BY domain`, searchWhere(search))

	args := searchArgs(search)

	rows, err := h.db.QueryContext(r.Context(), fullQuery, args...)
	if err != nil {
		h.dbFailure(w, r, "list_domain_status", err)
		return
	}
	defer rows.Close()

	domains := make([]DomainStatus, 0)
	for rows.Next() {
		var d DomainStatus
		if err := rows.Scan(&d.Domain, &d.BlockCount, &d.ReleaseCount); err != nil {
			h.dbFailure(w, r, "list_domain_status", err)
			return
		}
		domains = append(domains, d)
	}
	if !h.rowsDone(w, r, rows, "list_domain_status") {
		return
	}

	if format == "csv" || format == "xlsx" {
		headers := []string{"Dominio", "Blocchi", "Rilasci"}
		var exportRows [][]string
		for _, d := range domains {
			exportRows = append(exportRows, []string{
				d.Domain,
				fmt.Sprintf("%d", d.BlockCount),
				fmt.Sprintf("%d", d.ReleaseCount),
			})
		}
		filename := "domini-stato"
		if format == "csv" {
			writeCSV(w, filename+".csv", headers, exportRows)
		} else {
			writeXLSX(w, filename+".xlsx", headers, exportRows)
		}
		return
	}

	httputil.JSON(w, http.StatusOK, domains)
}

func (h *Handler) handleListHistory(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	format := r.URL.Query().Get("format")
	search := r.URL.Query().Get("search")

	baseQuery := `
		SELECT bd.domain, b.request_date, b.reference, 'block' AS request_type
		FROM dns_bl_block_domain bd
		JOIN dns_bl_block b ON b.id = bd.block_id
		UNION ALL
		SELECT rd.domain, r.request_date, r.reference, 'release'
		FROM dns_bl_release_domain rd
		JOIN dns_bl_release r ON r.id = rd.release_id`

	fullQuery := fmt.Sprintf(`
		SELECT domain, request_date, reference, request_type
		FROM (%s) history
		%s
		ORDER BY request_date DESC, domain ASC, request_type ASC`,
		baseQuery, searchWhere(search))

	args := searchArgs(search)

	rows, err := h.db.QueryContext(r.Context(), fullQuery, args...)
	if err != nil {
		h.dbFailure(w, r, "list_domain_history", err)
		return
	}
	defer rows.Close()

	entries := make([]HistoryEntry, 0)
	for rows.Next() {
		var e HistoryEntry
		if err := rows.Scan(&e.Domain, &e.RequestDate, &e.Reference, &e.RequestType); err != nil {
			h.dbFailure(w, r, "list_domain_history", err)
			return
		}
		entries = append(entries, e)
	}
	if !h.rowsDone(w, r, rows, "list_domain_history") {
		return
	}

	if format == "csv" || format == "xlsx" {
		headers := []string{"Dominio", "Data", "Riferimento", "Tipo"}
		var exportRows [][]string
		for _, e := range entries {
			typeLabel := "Blocco"
			if e.RequestType == "release" {
				typeLabel = "Rilascio"
			}
			exportRows = append(exportRows, []string{e.Domain, e.RequestDate, e.Reference, typeLabel})
		}
		filename := "riepilogo-domini"
		if format == "csv" {
			writeCSV(w, filename+".csv", headers, exportRows)
		} else {
			writeXLSX(w, filename+".xlsx", headers, exportRows)
		}
		return
	}

	httputil.JSON(w, http.StatusOK, entries)
}

// insertDomains batch-inserts domains into the given table within a transaction.
func insertDomains(ctx context.Context, tx *sql.Tx, table, fkCol string, parentID int, domains []string) error {
	if len(domains) == 0 {
		return nil
	}

	// Build batch insert: INSERT INTO table (fk_col, domain) VALUES ($1, $2), ($1, $3), ...
	var b strings.Builder
	b.WriteString("INSERT INTO ")
	b.WriteString(table)
	b.WriteString(" (")
	b.WriteString(fkCol)
	b.WriteString(", domain) VALUES ")

	args := make([]any, 0, 1+len(domains))
	args = append(args, parentID)

	for i, d := range domains {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(fmt.Sprintf("($1, $%d)", i+2))
		args = append(args, d)
	}

	_, err := tx.ExecContext(ctx, b.String(), args...)
	return err
}

// searchWhere returns a WHERE clause for search filtering, or empty string.
func searchWhere(search string) string {
	if strings.TrimSpace(search) == "" {
		return ""
	}
	return "WHERE domain ILIKE '%' || $1 || '%'"
}

// searchArgs returns the args for the search filter.
func searchArgs(search string) []any {
	s := strings.TrimSpace(search)
	if s == "" {
		return nil
	}
	return []any{s}
}
