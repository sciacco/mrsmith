package grappadcim

type EquipmentItem struct {
	ID                 int     `json:"id"`
	Name               string  `json:"name"`
	RackID             *int    `json:"rackId,omitempty"`
	RackName           *string `json:"rackName,omitempty"`
	DatacenterName     *string `json:"datacenterName,omitempty"`
	UnitPosition       *int    `json:"unitPosition,omitempty"`
	Unit               *int    `json:"unit,omitempty"`
	ManagementIP       *string `json:"managementIp,omitempty"`
	Note               *string `json:"note,omitempty"`
	Type               string  `json:"type"`
	Serial             *string `json:"serial,omitempty"`
	OS                 *string `json:"os,omitempty"`
	Model              *string `json:"model,omitempty"`
	CustomerID         *int    `json:"customerId,omitempty"`
	Status             *string `json:"status,omitempty"`
	Bandwidth          *int    `json:"bandwidth,omitempty"`
	PortCount          *int    `json:"portCount,omitempty"`
	PortName           *string `json:"portName,omitempty"`
	PortType           *string `json:"portType,omitempty"`
	PortLayer          *string `json:"portLayer,omitempty"`
	ActivatedAt        *string `json:"activatedAt,omitempty"`
	CeasedAt           *string `json:"ceasedAt,omitempty"`
	InstallAddress     *string `json:"installAddress,omitempty"`
	ShippingAddress    *string `json:"shippingAddress,omitempty"`
	CdlanOwned         *string `json:"cdlanOwned,omitempty"`
	ClusterName        *string `json:"clusterName,omitempty"`
	EndCustomer        *string `json:"endCustomer,omitempty"`
	ConfigurationType  *string `json:"configurationType,omitempty"`
	Shipping           *string `json:"shipping,omitempty"`
	OnsiteInstallation *string `json:"onsiteInstallation,omitempty"`
	MonitoringActive   *string `json:"monitoringActive,omitempty"`
	FirewallType       *string `json:"firewallType,omitempty"`
	SerialNumber       *string `json:"serialNumber,omitempty"`
	OrderCode          *string `json:"orderCode,omitempty"`
	LastNotificationAt *string `json:"lastNotificationAt,omitempty"`
	NICCount           int     `json:"nicCount"`
}

type EquipmentInput struct {
	Name               string  `json:"name"`
	RackID             *int    `json:"rackId,omitempty"`
	UnitPosition       *int    `json:"unitPosition,omitempty"`
	Unit               *int    `json:"unit,omitempty"`
	ManagementIP       *string `json:"managementIp,omitempty"`
	Note               *string `json:"note,omitempty"`
	Type               string  `json:"type"`
	Serial             *string `json:"serial,omitempty"`
	OS                 *string `json:"os,omitempty"`
	Model              *string `json:"model,omitempty"`
	CustomerID         *int    `json:"customerId,omitempty"`
	Status             *string `json:"status,omitempty"`
	Bandwidth          *int    `json:"bandwidth,omitempty"`
	PortCount          *int    `json:"portCount,omitempty"`
	PortName           *string `json:"portName,omitempty"`
	PortType           *string `json:"portType,omitempty"`
	PortLayer          *string `json:"portLayer,omitempty"`
	ActivatedAt        *string `json:"activatedAt,omitempty"`
	InstallAddress     *string `json:"installAddress,omitempty"`
	ShippingAddress    *string `json:"shippingAddress,omitempty"`
	CdlanOwned         *string `json:"cdlanOwned,omitempty"`
	ClusterName        *string `json:"clusterName,omitempty"`
	EndCustomer        *string `json:"endCustomer,omitempty"`
	ConfigurationType  *string `json:"configurationType,omitempty"`
	Shipping           *string `json:"shipping,omitempty"`
	OnsiteInstallation *string `json:"onsiteInstallation,omitempty"`
	MonitoringActive   *string `json:"monitoringActive,omitempty"`
	FirewallType       *string `json:"firewallType,omitempty"`
	SerialNumber       *string `json:"serialNumber,omitempty"`
	OrderCode          *string `json:"orderCode,omitempty"`
}

type EquipmentPatch struct {
	Name               *string `json:"name,omitempty"`
	RackID             *int    `json:"rackId,omitempty"`
	UnitPosition       *int    `json:"unitPosition,omitempty"`
	Unit               *int    `json:"unit,omitempty"`
	ManagementIP       *string `json:"managementIp,omitempty"`
	Note               *string `json:"note,omitempty"`
	Type               *string `json:"type,omitempty"`
	Serial             *string `json:"serial,omitempty"`
	OS                 *string `json:"os,omitempty"`
	Model              *string `json:"model,omitempty"`
	CustomerID         *int    `json:"customerId,omitempty"`
	Status             *string `json:"status,omitempty"`
	Bandwidth          *int    `json:"bandwidth,omitempty"`
	PortCount          *int    `json:"portCount,omitempty"`
	PortName           *string `json:"portName,omitempty"`
	PortType           *string `json:"portType,omitempty"`
	PortLayer          *string `json:"portLayer,omitempty"`
	ActivatedAt        *string `json:"activatedAt,omitempty"`
	InstallAddress     *string `json:"installAddress,omitempty"`
	ShippingAddress    *string `json:"shippingAddress,omitempty"`
	CdlanOwned         *string `json:"cdlanOwned,omitempty"`
	ClusterName        *string `json:"clusterName,omitempty"`
	EndCustomer        *string `json:"endCustomer,omitempty"`
	ConfigurationType  *string `json:"configurationType,omitempty"`
	Shipping           *string `json:"shipping,omitempty"`
	OnsiteInstallation *string `json:"onsiteInstallation,omitempty"`
	MonitoringActive   *string `json:"monitoringActive,omitempty"`
	FirewallType       *string `json:"firewallType,omitempty"`
	SerialNumber       *string `json:"serialNumber,omitempty"`
	OrderCode          *string `json:"orderCode,omitempty"`
}

type NICItem struct {
	ID                int     `json:"id"`
	EquipmentID       *int    `json:"equipmentId,omitempty"`
	Identifier        string  `json:"identifier"`
	Name              string  `json:"name"`
	CustomerID        *int    `json:"customerId,omitempty"`
	Note              *string `json:"note,omitempty"`
	Type              *string `json:"type,omitempty"`
	LinkedEquipmentID *int    `json:"linkedEquipmentId,omitempty"`
	LinkedNICID       *int    `json:"linkedNicId,omitempty"`
	Layer             *string `json:"layer,omitempty"`
	LinkedServerID    *int    `json:"linkedServerId,omitempty"`
	Status            *string `json:"status,omitempty"`
}
