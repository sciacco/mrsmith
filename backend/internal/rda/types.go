package rda

import (
	"database/sql"
	"encoding/json"
	"log/slog"

	"github.com/sciacco/mrsmith/internal/platform/arak"
)

type Deps struct {
	Arak   *arak.Client
	ArakDB *sql.DB
	Logger *slog.Logger
}

type Handler struct {
	arak   *arak.Client
	arakDB *sql.DB
	logger *slog.Logger
}

type userRef struct {
	ID        int64  `json:"id,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Email     string `json:"email,omitempty"`
}

type approverRef struct {
	Level any `json:"level,omitempty"`
	User  struct {
		Email string `json:"email,omitempty"`
	} `json:"user,omitempty"`
}

type poDetail struct {
	ID                   any               `json:"id,omitempty"`
	Code                 string            `json:"code,omitempty"`
	State                string            `json:"state,omitempty"`
	CurrentApprovalLevel any               `json:"current_approval_level,omitempty"`
	TotalPrice           string            `json:"total_price,omitempty"`
	Requester            userRef           `json:"requester,omitempty"`
	Provider             providerDetail    `json:"provider,omitempty"`
	PaymentMethod        json.RawMessage   `json:"payment_method,omitempty"`
	Rows                 []json.RawMessage `json:"rows,omitempty"`
	Attachments          []json.RawMessage `json:"attachments,omitempty"`
	Approvers            []approverRef     `json:"approvers,omitempty"`
}

type paymentMethod struct {
	Code         string `json:"code"`
	Description  string `json:"description"`
	RDAAvailable bool   `json:"rda_available"`
}

type defaultPaymentMethod struct {
	Code string `json:"code"`
}

type rdaPermissions struct {
	IsApprover            bool `json:"is_approver"`
	IsAFC                 bool `json:"is_afc"`
	IsApproverNoLeasing   bool `json:"is_approver_no_leasing"`
	IsApproverExtraBudget bool `json:"is_approver_extra_budget"`
}

type providerDetail struct {
	ID                   int64           `json:"id"`
	Language             string          `json:"language"`
	VATNumber            string          `json:"vat_number"`
	PostalCode           string          `json:"postal_code"`
	CAP                  string          `json:"cap"`
	DefaultPaymentMethod json.RawMessage `json:"default_payment_method"`
}

type createPORequest struct {
	Type              string `json:"type"`
	BudgetID          int64  `json:"budget_id"`
	CostCenter        string `json:"cost_center,omitempty"`
	BudgetUserID      int64  `json:"budget_user_id,omitempty"`
	ProviderID        int64  `json:"provider_id"`
	PaymentMethod     string `json:"payment_method,omitempty"`
	Currency          string `json:"currency,omitempty"`
	Project           string `json:"project"`
	Object            string `json:"object"`
	Description       string `json:"description,omitempty"`
	Note              string `json:"note,omitempty"`
	ProviderOfferCode string `json:"provider_offer_code,omitempty"`
	ProviderOfferDate string `json:"provider_offer_date,omitempty"`
}
