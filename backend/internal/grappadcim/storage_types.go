package grappadcim

type StorageItem struct {
	ID           int     `json:"id"`
	Protocol     *string `json:"protocol,omitempty"`
	Size         *int    `json:"size,omitempty"`
	CustomerID   int     `json:"customerId"`
	EquipmentID  int     `json:"equipmentId"`
	Equipment    *string `json:"equipment,omitempty"`
	Note         *string `json:"note,omitempty"`
	SizeType     *string `json:"sizeType,omitempty"`
	Status       string  `json:"status"`
	CreatedAt    *string `json:"createdAt,omitempty"`
	ClosedAt     *string `json:"closedAt,omitempty"`
	OrderCode    *string `json:"orderCode,omitempty"`
	SerialNumber *string `json:"serialNumber,omitempty"`
	ReadOnly     bool    `json:"readOnly"`
}

type StorageInput struct {
	Protocol     *string `json:"protocol,omitempty"`
	Size         *int    `json:"size,omitempty"`
	CustomerID   int     `json:"customerId"`
	EquipmentID  int     `json:"equipmentId"`
	Note         *string `json:"note,omitempty"`
	SizeType     *string `json:"sizeType,omitempty"`
	Status       *string `json:"status,omitempty"`
	OrderCode    *string `json:"orderCode,omitempty"`
	SerialNumber *string `json:"serialNumber,omitempty"`
}

type StoragePatch struct {
	Protocol     *string `json:"protocol,omitempty"`
	Size         *int    `json:"size,omitempty"`
	CustomerID   *int    `json:"customerId,omitempty"`
	EquipmentID  *int    `json:"equipmentId,omitempty"`
	Note         *string `json:"note,omitempty"`
	SizeType     *string `json:"sizeType,omitempty"`
	Status       *string `json:"status,omitempty"`
	OrderCode    *string `json:"orderCode,omitempty"`
	SerialNumber *string `json:"serialNumber,omitempty"`
}
