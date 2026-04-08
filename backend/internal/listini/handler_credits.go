package listini

import (
	"database/sql"
	"net/http"

	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// handleGetCreditBalance returns the credit balance for a customer.
func (h *Handler) handleGetCreditBalance(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	customerID, err := pathID(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_customer_id")
		return
	}

	var balance float64
	err = h.mistraDB.QueryRowContext(r.Context(),
		`SELECT credit FROM customers.customer_credits WHERE customer_id = $1`,
		customerID).Scan(&balance)
	if err != nil && err != sql.ErrNoRows {
		h.dbFailure(w, r, "get_credit_balance", err)
		return
	}
	// ErrNoRows means balance = 0 (default)

	httputil.JSON(w, http.StatusOK, map[string]float64{"balance": balance})
}

// handleListTransactions returns credit transactions for a customer.
func (h *Handler) handleListTransactions(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	customerID, err := pathID(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_customer_id")
		return
	}

	rows, err := h.mistraDB.QueryContext(r.Context(), `
		SELECT id, transaction_date, amount, operation_sign, description, operated_by
		FROM customers.customer_credit_transaction
		WHERE customer_id = $1
		ORDER BY transaction_date DESC, id DESC`, customerID)
	if err != nil {
		h.dbFailure(w, r, "list_transactions", err)
		return
	}
	defer rows.Close()

	type transaction struct {
		ID              int     `json:"id"`
		TransactionDate string  `json:"transaction_date"`
		Amount          float64 `json:"amount"`
		OperationSign   string  `json:"operation_sign"`
		Description     string  `json:"description"`
		OperatedBy      string  `json:"operated_by"`
	}

	var result []transaction
	for rows.Next() {
		var t transaction
		if err := rows.Scan(
			&t.ID, &t.TransactionDate, &t.Amount,
			&t.OperationSign, &t.Description, &t.OperatedBy,
		); err != nil {
			h.dbFailure(w, r, "list_transactions_scan", err)
			return
		}
		result = append(result, t)
	}
	if !h.rowsDone(w, r, rows, "list_transactions") {
		return
	}
	if result == nil {
		result = []transaction{}
	}

	httputil.JSON(w, http.StatusOK, result)
}

// handleCreateTransaction inserts a new credit transaction (immutable ledger — INSERT only).
func (h *Handler) handleCreateTransaction(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	customerID, err := pathID(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_customer_id")
		return
	}

	var req TransactionRequest
	if err := decodeBody(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_body")
		return
	}

	// Validate
	fieldErrors := make(map[string]string)
	if req.Amount <= 0 || req.Amount > 10000 {
		fieldErrors["amount"] = "must be between 0 and 10000 (exclusive of 0)"
	}
	if req.OperationSign != "+" && req.OperationSign != "-" {
		fieldErrors["operation_sign"] = "must be '+' or '-'"
	}
	if req.Description == "" {
		fieldErrors["description"] = "required"
	} else if len(req.Description) > 255 {
		fieldErrors["description"] = "max 255 characters"
	}
	if len(fieldErrors) > 0 {
		httputil.JSON(w, http.StatusUnprocessableEntity, map[string]any{"errors": fieldErrors})
		return
	}

	// Extract operator identity from JWT
	claims, ok := auth.GetClaims(r.Context())
	operatedBy := "unknown"
	if ok {
		operatedBy = claims.Email
		if operatedBy == "" {
			operatedBy = claims.Name
		}
		if operatedBy == "" {
			operatedBy = claims.Subject
		}
	}

	var id int
	var transactionDate string
	err = h.mistraDB.QueryRowContext(r.Context(), `
		INSERT INTO customers.customer_credit_transaction
		  (customer_id, amount, operation_sign, description, operated_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, transaction_date`,
		customerID, req.Amount, req.OperationSign, req.Description, operatedBy,
	).Scan(&id, &transactionDate)
	if err != nil {
		h.dbFailure(w, r, "create_transaction", err)
		return
	}

	httputil.JSON(w, http.StatusCreated, map[string]any{
		"id":               id,
		"transaction_date": transactionDate,
	})
}
