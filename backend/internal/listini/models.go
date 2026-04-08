package listini

// ── Request/Response types ──

// IaaSPricingRequest is the body for IaaS pricing upsert.
type IaaSPricingRequest struct {
	ChargeCPU       float64  `json:"charge_cpu"`
	ChargeRAMKVM    float64  `json:"charge_ram_kvm"`
	ChargeRAMVMware float64  `json:"charge_ram_vmware"`
	ChargePStor     float64  `json:"charge_pstor"`
	ChargeSStor     float64  `json:"charge_sstor"`
	ChargeIP        float64  `json:"charge_ip"`
	ChargePrefix24  *float64 `json:"charge_prefix24"`
}

// TimooPricingRequest is the body for Timoo pricing upsert.
type TimooPricingRequest struct {
	UserMonth float64 `json:"user_month"`
	SeMonth   float64 `json:"se_month"`
}

// TransactionRequest is the body for creating a credit transaction.
type TransactionRequest struct {
	Amount        float64 `json:"amount"`
	OperationSign string  `json:"operation_sign"`
	Description   string  `json:"description"`
}

// GroupSyncRequest is the body for syncing customer group associations.
type GroupSyncRequest struct {
	GroupIDs []int `json:"groupIds"`
}

// IaaSCreditUpdateItem represents a single account credit update.
type IaaSCreditUpdateItem struct {
	DomainUUID        string  `json:"domainuuid"`
	IDCliFatturazione int     `json:"id_cli_fatturazione"`
	Credito           float64 `json:"credito"`
}

// BatchIaaSCreditRequest is the body for batch credit updates.
type BatchIaaSCreditRequest struct {
	Items []IaaSCreditUpdateItem `json:"items"`
}

// RackDiscountUpdateItem represents a single rack discount update.
type RackDiscountUpdateItem struct {
	IDRack int     `json:"id_rack"`
	Sconto float64 `json:"sconto"`
}

// BatchRackDiscountRequest is the body for batch rack discount updates.
type BatchRackDiscountRequest struct {
	Items []RackDiscountUpdateItem `json:"items"`
}
