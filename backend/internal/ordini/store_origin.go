package ordini

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
)

func (h *Handler) loadOrigin(ctx context.Context, orderID int64) (*OrderOrigin, error) {
	if h.deps.Mistra == nil {
		return nil, nil
	}
	var quoteID int64
	err := h.deps.Mistra.QueryRowContext(ctx, `
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
		QuoteURL: "/apps/quotes/quotes/" + strconv.FormatInt(quoteID, 10),
	}
	var quoteCode sql.NullString
	err = h.deps.Mistra.QueryRowContext(ctx, `
SELECT quote_number
FROM quotes.quote
WHERE id = $1`, quoteID).Scan(&quoteCode)
	if errors.Is(err, sql.ErrNoRows) {
		h.logger.Warn("origin quote not found", "operation", "origin_quote_lookup", "order_id", orderID, "quote_id", quoteID)
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
