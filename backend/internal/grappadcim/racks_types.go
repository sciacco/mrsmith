package grappadcim

type RackListItem struct {
	ID              int      `json:"id"`
	Name            string   `json:"name"`
	UnitCount       int      `json:"unitCount"`
	CustomerID      *int     `json:"customerId,omitempty"`
	DatacenterID    int      `json:"datacenterId"`
	DatacenterName  *string  `json:"datacenterName,omitempty"`
	BuildingName    *string  `json:"buildingName,omitempty"`
	Status          *string  `json:"status,omitempty"`
	Magnetotermico  *string  `json:"magnetotermico,omitempty"`
	Ampere          *int     `json:"ampere,omitempty"`
	Floor           *int     `json:"floor,omitempty"`
	Island          *int     `json:"island,omitempty"`
	Type            *string  `json:"type,omitempty"`
	Position        *string  `json:"position,omitempty"`
	RackNumber      *int     `json:"rackNumber,omitempty"`
	PositionID      *int     `json:"positionId,omitempty"`
	IsletID         *int     `json:"isletId,omitempty"`
	Shared          *string  `json:"shared,omitempty"`
	Reserved        *string  `json:"reserved,omitempty"`
	Note            *string  `json:"note,omitempty"`
	ActivatedAt     *string  `json:"activatedAt,omitempty"`
	CeasedAt        *string  `json:"ceasedAt,omitempty"`
	OrderCode       *string  `json:"orderCode,omitempty"`
	SoldPower       *float64 `json:"soldPower,omitempty"`
	SerialNumber    *string  `json:"serialNumber,omitempty"`
	CommittedPower  *float64 `json:"committedPower,omitempty"`
	VariableBilling *int     `json:"variableBilling,omitempty"`
	SocketCount     int      `json:"socketCount"`
}

type RackDetail struct {
	RackListItem
	Units   []RackUnit   `json:"units"`
	Sockets []RackSocket `json:"sockets"`
	Media   []RackMedia  `json:"media"`
}

type RackInput struct {
	Name            string   `json:"name"`
	UnitCount       int      `json:"unitCount"`
	CustomerID      *int     `json:"customerId,omitempty"`
	DatacenterID    int      `json:"datacenterId"`
	Status          *string  `json:"status,omitempty"`
	Magnetotermico  *string  `json:"magnetotermico,omitempty"`
	Ampere          *int     `json:"ampere,omitempty"`
	Floor           *int     `json:"floor,omitempty"`
	Island          *int     `json:"island,omitempty"`
	Type            string   `json:"type"`
	Position        string   `json:"position"`
	RackNumber      *int     `json:"rackNumber,omitempty"`
	PositionID      *int     `json:"positionId,omitempty"`
	IsletID         *int     `json:"isletId,omitempty"`
	Shared          *string  `json:"shared,omitempty"`
	Reserved        *string  `json:"reserved,omitempty"`
	Note            *string  `json:"note,omitempty"`
	OrderCode       *string  `json:"orderCode,omitempty"`
	SoldPower       *float64 `json:"soldPower,omitempty"`
	SerialNumber    *string  `json:"serialNumber,omitempty"`
	CommittedPower  *float64 `json:"committedPower,omitempty"`
	VariableBilling *int     `json:"variableBilling,omitempty"`
	SocketCount     *int     `json:"socketCount,omitempty"`
}

type RackPatch struct {
	Name            *string  `json:"name,omitempty"`
	UnitCount       *int     `json:"unitCount,omitempty"`
	CustomerID      *int     `json:"customerId,omitempty"`
	Status          *string  `json:"status,omitempty"`
	Magnetotermico  *string  `json:"magnetotermico,omitempty"`
	Ampere          *int     `json:"ampere,omitempty"`
	Shared          *string  `json:"shared,omitempty"`
	Reserved        *string  `json:"reserved,omitempty"`
	Note            *string  `json:"note,omitempty"`
	OrderCode       *string  `json:"orderCode,omitempty"`
	SoldPower       *float64 `json:"soldPower,omitempty"`
	SerialNumber    *string  `json:"serialNumber,omitempty"`
	CommittedPower  *float64 `json:"committedPower,omitempty"`
	VariableBilling *int     `json:"variableBilling,omitempty"`
}

type RackMoveInput struct {
	DatacenterID int    `json:"datacenterId"`
	PositionID   int    `json:"positionId"`
	IsletID      *int   `json:"isletId,omitempty"`
	Type         string `json:"type"`
	Position     string `json:"position"`
}

type RackUnit struct {
	ID       int  `json:"id"`
	Num      *int `json:"num,omitempty"`
	RackID   *int `json:"rackId,omitempty"`
	DeviceID *int `json:"deviceId,omitempty"`
}

type RackMedia struct {
	ID        int     `json:"id"`
	Path      *string `json:"path,omitempty"`
	UnitID    *int    `json:"unitId,omitempty"`
	Side      *string `json:"side,omitempty"`
	UpdatedAt *string `json:"updatedAt,omitempty"`
}

type RackMediaInput struct {
	Items []RackMediaWrite `json:"items"`
}

type RackMediaWrite struct {
	UnitID int    `json:"unitId"`
	Side   string `json:"side"`
	Path   string `json:"path"`
}
