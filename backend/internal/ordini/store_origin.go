package ordini

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/applaunch"
)

func (h *Handler) loadOrigin(r *http.Request, orderID int64) (*OrderOrigin, error) {
	if h.deps.Mistra == nil {
		return nil, nil
	}
	start := time.Now()
	var quoteID int64
	err := h.deps.Mistra.QueryRowContext(r.Context(), `
SELECT quote_id
FROM orders.legacy_orders
WHERE vodka_id = $1
LIMIT 1`, orderID).Scan(&quoteID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	origin := &OrderOrigin{
		Type:     "quote",
		QuoteID:  quoteID,
		QuoteURL: h.deps.AppURLs.Link(applaunch.QuotesAppID, "quotes/"+strconv.FormatInt(quoteID, 10)),
	}
	var quoteCode sql.NullString
	err = h.deps.Mistra.QueryRowContext(r.Context(), `
SELECT quote_number
FROM quotes.quote
WHERE id = $1`, quoteID).Scan(&quoteCode)
	if errors.Is(err, sql.ErrNoRows) {
		h.logFailure(r, slog.LevelWarn, "origin quote not found", "origin_quote_lookup", start, "order_id", orderID, "quote_id", quoteID)
		return origin, nil
	}
	if err != nil {
		return nil, err
	}
	if quoteCode.Valid && quoteCode.String != "" {
		origin.QuoteCode = &quoteCode.String
	}
	return origin, nil
}
