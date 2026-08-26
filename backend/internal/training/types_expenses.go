package training

// EventExpenseInput creates a local link to a live Arak purchase order.
type EventExpenseInput struct {
	POReference   string   `json:"poReference"`
	EnrollmentIDs []string `json:"enrollmentIds"`
}

// EventExpenseReplaceInput replaces only the purchase order; coverage remains
// unchanged.
type EventExpenseReplaceInput struct {
	POReference string `json:"poReference"`
}

// EventExpenseEnrollmentsInput replaces the complete explicit coverage set.
type EventExpenseEnrollmentsInput struct {
	EnrollmentIDs *[]string `json:"enrollmentIds"`
}

// EventExpenseBudget is the live budget association returned by Arak.
type EventExpenseBudget struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Year         int     `json:"year"`
	CostCenter   *string `json:"costCenter"`
	BudgetUserID *int64  `json:"budgetUserId"`
}

// EventExpense is a local event/PO link hydrated with the current Arak PO.
type EventExpense struct {
	ID            string             `json:"id"`
	EventID       string             `json:"eventId"`
	POID          int64              `json:"poId"`
	POCode        string             `json:"poCode"`
	TotalPrice    string             `json:"totalPrice"`
	Currency      string             `json:"currency"`
	RawState      string             `json:"rawState"`
	EconomicState EconomicState      `json:"economicState"`
	Budget        EventExpenseBudget `json:"budget"`
	EnrollmentIDs []string           `json:"enrollmentIds"`
	CreatedAt     string             `json:"createdAt"`
	UpdatedAt     string             `json:"updatedAt"`
}

type eventExpenseLocal struct {
	ID            string
	EventID       string
	POID          int64
	EnrollmentIDs []string
	CreatedAt     string
	UpdatedAt     string
}

func hydrateEventExpense(local eventExpenseLocal, po PurchaseOrder) EventExpense {
	return EventExpense{
		ID:            local.ID,
		EventID:       local.EventID,
		POID:          local.POID,
		POCode:        po.Code,
		TotalPrice:    po.TotalPrice,
		Currency:      po.Currency,
		RawState:      po.RawState,
		EconomicState: po.EconomicState,
		Budget: EventExpenseBudget{
			ID:           po.Budget.ID,
			Name:         po.Budget.Name,
			Year:         po.Budget.Year,
			CostCenter:   po.Budget.CostCenter,
			BudgetUserID: po.Budget.BudgetUserID,
		},
		EnrollmentIDs: local.EnrollmentIDs,
		CreatedAt:     local.CreatedAt,
		UpdatedAt:     local.UpdatedAt,
	}
}
