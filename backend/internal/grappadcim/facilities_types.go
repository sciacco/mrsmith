package grappadcim

type Building struct {
	ID              int     `json:"id"`
	Name            string  `json:"name"`
	Address         string  `json:"address"`
	Status          string  `json:"status"`
	PortalEnabled   bool    `json:"portalEnabled"`
	RackCapacity    int     `json:"rackCapacity"`
	CreatedAt       *string `json:"createdAt,omitempty"`
	UpdatedAt       *string `json:"updatedAt,omitempty"`
	CeasedAt        *string `json:"ceasedAt,omitempty"`
	DatacenterCount int     `json:"datacenterCount"`
	RackCount       int     `json:"rackCount"`
}

type BuildingInput struct {
	Name          string `json:"name"`
	Address       string `json:"address"`
	Status        string `json:"status"`
	PortalEnabled bool   `json:"portalEnabled"`
	RackCapacity  int    `json:"rackCapacity"`
}

type BuildingPatch struct {
	Name          *string `json:"name,omitempty"`
	Address       *string `json:"address,omitempty"`
	Status        *string `json:"status,omitempty"`
	PortalEnabled *bool   `json:"portalEnabled,omitempty"`
	RackCapacity  *int    `json:"rackCapacity,omitempty"`
}

type Datacenter struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	Address       string  `json:"address"`
	Note          *string `json:"note,omitempty"`
	RackCapacity  int     `json:"rackCapacity"`
	Status        *string `json:"status,omitempty"`
	CustomerID    *int    `json:"customerId,omitempty"`
	PortalEnabled bool    `json:"portalEnabled"`
	ActivatedAt   *string `json:"activatedAt,omitempty"`
	CeasedAt      *string `json:"ceasedAt,omitempty"`
	OrderCode     *string `json:"orderCode,omitempty"`
	BuildingID    *int    `json:"buildingId,omitempty"`
	BuildingName  *string `json:"buildingName,omitempty"`
	IsMMR         bool    `json:"isMmr"`
	SetOrder      *int    `json:"setOrder,omitempty"`
	MMRType       *string `json:"mmrType,omitempty"`
	SerialNumber  *string `json:"serialNumber,omitempty"`
	Floor         *string `json:"floor,omitempty"`
	IsletCount    int     `json:"isletCount"`
	RackCount     int     `json:"rackCount"`
}

type DatacenterInput struct {
	Name          string  `json:"name"`
	Address       string  `json:"address"`
	Note          *string `json:"note,omitempty"`
	RackCapacity  int     `json:"rackCapacity"`
	Status        *string `json:"status,omitempty"`
	CustomerID    *int    `json:"customerId,omitempty"`
	PortalEnabled bool    `json:"portalEnabled"`
	OrderCode     *string `json:"orderCode,omitempty"`
	BuildingID    *int    `json:"buildingId,omitempty"`
	IsMMR         bool    `json:"isMmr"`
	SetOrder      *int    `json:"setOrder,omitempty"`
	MMRType       *string `json:"mmrType,omitempty"`
	SerialNumber  *string `json:"serialNumber,omitempty"`
	Floor         *string `json:"floor,omitempty"`
}

type DatacenterPatch struct {
	Name          *string `json:"name,omitempty"`
	Address       *string `json:"address,omitempty"`
	Note          *string `json:"note,omitempty"`
	RackCapacity  *int    `json:"rackCapacity,omitempty"`
	Status        *string `json:"status,omitempty"`
	CustomerID    *int    `json:"customerId,omitempty"`
	PortalEnabled *bool   `json:"portalEnabled,omitempty"`
	OrderCode     *string `json:"orderCode,omitempty"`
	BuildingID    *int    `json:"buildingId,omitempty"`
	IsMMR         *bool   `json:"isMmr,omitempty"`
	SetOrder      *int    `json:"setOrder,omitempty"`
	MMRType       *string `json:"mmrType,omitempty"`
	SerialNumber  *string `json:"serialNumber,omitempty"`
	Floor         *string `json:"floor,omitempty"`
}

type DatacenterMap struct {
	Datacenter Datacenter     `json:"datacenter"`
	Islets     []Islet        `json:"islets"`
	Positions  []Position     `json:"positions"`
	Racks      []RackListItem `json:"racks"`
	Incomplete bool           `json:"incomplete"`
}

type LayoutGridResponse struct {
	Datacenter Datacenter        `json:"datacenter"`
	Blocks     []LayoutGridBlock `json:"blocks"`
	Positions  []Position        `json:"positions"`
	Racks      []RackListItem    `json:"racks"`
	Incomplete bool              `json:"incomplete"`
	Warnings   []string          `json:"warnings"`
}

type LayoutGridBlock struct {
	ID            int                `json:"id"`
	DatacenterID  int                `json:"datacenterId"`
	IsletID       *int               `json:"isletId,omitempty"`
	IsletName     string             `json:"isletName"`
	Title         string             `json:"title"`
	LayoutWidth   *string            `json:"layoutWidth,omitempty"`
	DisplayOrder  int                `json:"displayOrder"`
	SchemaVersion string             `json:"schemaVersion"`
	Grid          [][]LayoutGridCell `json:"grid"`
}

type LayoutGridCell struct {
	Type       string  `json:"type"`
	Pos        *int    `json:"pos,omitempty"`
	Text       *string `json:"text,omitempty"`
	PlenumType *string `json:"plenumType,omitempty"`

	PositionID     *int    `json:"positionId,omitempty"`
	PositionStatus *string `json:"positionStatus,omitempty"`
	PositionType   *string `json:"positionType,omitempty"`
	RackID         *int    `json:"rackId,omitempty"`
	RackName       *string `json:"rackName,omitempty"`
	RackType       *string `json:"rackType,omitempty"`
	RackPos        *string `json:"rackPos,omitempty"`

	PlenumID     *int    `json:"plenumId,omitempty"`
	PlenumName   *string `json:"plenumName,omitempty"`
	PlenumStatus *string `json:"plenumStatus,omitempty"`
}

type DatacenterPort struct {
	ID         int     `json:"id"`
	RackID     *int    `json:"rackId,omitempty"`
	RackName   *string `json:"rackName,omitempty"`
	SlotID     *int    `json:"slotId,omitempty"`
	PortNumber *int    `json:"portNumber,omitempty"`
	Status     *string `json:"status,omitempty"`
	PortType   *string `json:"portType,omitempty"`
	Label      string  `json:"label"`
}

type DatacenterPortInput struct {
	RackID     *int    `json:"rackId,omitempty"`
	SlotID     *int    `json:"slotId,omitempty"`
	PortNumber *int    `json:"portNumber,omitempty"`
	Status     *string `json:"status,omitempty"`
	PortType   *string `json:"portType,omitempty"`
	Label      *string `json:"label,omitempty"`
}
