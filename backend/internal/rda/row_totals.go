package rda

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
)

type rowEconomics struct {
	Price float64
	MRC   float64
	NRC   float64
	Total float64
}

func (h *Handler) fetchPORowEconomics(ctx context.Context, poID string) (map[string]rowEconomics, error) {
	if h.arakDB == nil {
		return nil, nil
	}
	id, err := strconv.ParseInt(strings.TrimSpace(poID), 10, 64)
	if err != nil {
		return nil, err
	}

	rows, err := h.arakDB.QueryContext(ctx, `
		SELECT id, price, mrc, nrc, total
		FROM rda.purchase_order_row
		WHERE order_id = $1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	economics := make(map[string]rowEconomics)
	for rows.Next() {
		var rowID int64
		var price, mrc, nrc, total sql.NullFloat64
		if err := rows.Scan(&rowID, &price, &mrc, &nrc, &total); err != nil {
			return nil, err
		}
		economics[strconv.FormatInt(rowID, 10)] = rowEconomics{
			Price: nullFloat(price),
			MRC:   nullFloat(mrc),
			NRC:   nullFloat(nrc),
			Total: nullFloat(total),
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return economics, nil
}

func normalizePODetailRows(body []byte, economics map[string]rowEconomics) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}

	rows, ok := payload["rows"].([]any)
	if !ok {
		return body, nil
	}
	for _, item := range rows {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		normalizeRowEconomics(row, economics[rowIDKey(row["id"])])
		normalizeRowTotal(row)
	}
	normalizePOTotal(payload, rows)
	return json.Marshal(payload)
}

func normalizeRowEconomics(row map[string]any, economics rowEconomics) {
	rowType := strings.ToLower(strings.TrimSpace(stringValue(row["type"])))

	if moneyValue(row["montly_fee"]) <= 0 && moneyValue(row["monthly_fee"]) <= 0 && moneyValue(row["mrc"]) > 0 {
		row["montly_fee"] = decimalString(row["mrc"])
	}
	if moneyValue(row["activation_fee"]) <= 0 && moneyValue(row["activation_price"]) <= 0 && moneyValue(row["nrc"]) > 0 {
		row["activation_fee"] = decimalString(row["nrc"])
	}

	if economics.MRC > 0 && moneyValue(row["montly_fee"]) <= 0 && moneyValue(row["monthly_fee"]) <= 0 {
		row["montly_fee"] = decimalString(economics.MRC)
	}
	if rowType == "service" && economics.Price > 0 && moneyValue(row["montly_fee"]) <= 0 && moneyValue(row["monthly_fee"]) <= 0 {
		row["montly_fee"] = decimalString(economics.Price)
	}
	if economics.NRC > 0 && moneyValue(row["activation_fee"]) <= 0 && moneyValue(row["activation_price"]) <= 0 {
		row["activation_fee"] = decimalString(economics.NRC)
	}
	if economics.Total > 0 && moneyValue(row["total"]) <= 0 && moneyValue(row["total_price"]) <= 0 {
		row["total"] = decimalString(economics.Total)
	}

	if rowType != "good" || moneyValue(row["price"]) > 0 {
		return
	}
	if economics.Price > 0 {
		row["price"] = decimalString(economics.Price)
		return
	}
	if total := firstPositiveMoney(row["total"], row["total_price"], economics.Total); total > 0 {
		if qty := moneyValue(row["qty"]); qty > 0 {
			row["price"] = decimalString(total / qty)
		}
	}
}

func normalizeRowTotal(row map[string]any) {
	if moneyValue(row["total_price"]) > 0 {
		return
	}
	if total := moneyValue(row["total"]); total > 0 {
		row["total_price"] = decimalString(total)
		return
	}
	total := rowCalculatedTotal(row)
	if total > 0 {
		row["total_price"] = decimalString(total)
	}
}

func rowCalculatedTotal(row map[string]any) float64 {
	qty := moneyValue(row["qty"])
	if qty <= 0 {
		return 0
	}

	switch strings.ToLower(strings.TrimSpace(stringValue(row["type"]))) {
	case "good":
		return moneyValue(row["price"]) * qty
	case "service":
		mrc := firstPositiveMoney(row["monthly_fee"], row["montly_fee"], row["mrc"], row["price"])
		nrc := firstPositiveMoney(row["activation_fee"], row["activation_price"], row["nrc"])
		duration := nestedMoneyValue(row, "renew_detail", "initial_subscription_months")
		return (mrc * qty * duration) + (nrc * qty)
	default:
		return 0
	}
}

func normalizePOTotal(payload map[string]any, rows []any) {
	if moneyValue(payload["total_price"]) > 0 {
		return
	}
	total := 0.0
	for _, item := range rows {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		total += moneyValue(row["total_price"])
	}
	if total > 0 {
		payload["total_price"] = decimalString(total)
	}
}

func firstPositiveMoney(values ...any) float64 {
	for _, value := range values {
		if amount := moneyValue(value); amount > 0 {
			return amount
		}
	}
	return 0
}

func nestedMoneyValue(row map[string]any, objectKey string, valueKey string) float64 {
	object, ok := row[objectKey].(map[string]any)
	if !ok {
		return 0
	}
	return moneyValue(object[valueKey])
}

func rowIDKey(value any) string {
	switch v := value.(type) {
	case json.Number:
		return v.String()
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return ""
	}
}

func moneyValue(value any) float64 {
	switch v := value.(type) {
	case string:
		return parseTotalPrice(v)
	default:
		return numberValue(v)
	}
}

func nullFloat(value sql.NullFloat64) float64 {
	if !value.Valid {
		return 0
	}
	return value.Float64
}
