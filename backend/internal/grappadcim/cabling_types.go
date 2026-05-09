package grappadcim

type Plenum struct {
	ID              int     `json:"id"`
	Name            *string `json:"name,omitempty"`
	Isle            *string `json:"isle,omitempty"`
	Type            *string `json:"type,omitempty"`
	DatacenterID    int     `json:"datacenterId"`
	DatacenterName  *string `json:"datacenterName,omitempty"`
	Status          string  `json:"status"`
	SlotCount       int     `json:"slotCount"`
	LinkedPortCount int     `json:"linkedPortCount"`
}

type PlenumInput struct {
	Name         *string `json:"name,omitempty"`
	Isle         *string `json:"isle,omitempty"`
	Type         *string `json:"type,omitempty"`
	DatacenterID int     `json:"datacenterId"`
	Status       string  `json:"status"`
}

type PlenumPatch struct {
	Name   *string `json:"name,omitempty"`
	Isle   *string `json:"isle,omitempty"`
	Type   *string `json:"type,omitempty"`
	Status *string `json:"status,omitempty"`
}

type PlenumMatrix struct {
	Plenum         Plenum             `json:"plenum"`
	Slots          []PlenumMatrixSlot `json:"slots"`
	Incomplete     bool               `json:"incomplete"`
	ExpectedSlots  int                `json:"expectedSlots"`
	ExpectedCells  int                `json:"expectedCells"`
	FreeCells      int                `json:"freeCells"`
	AssignedCells  int                `json:"assignedCells"`
	MissingCells   int                `json:"missingCells"`
	MapOnlyRecords int                `json:"mapOnlyRecords"`
}

type PlenumMatrixSlot struct {
	ID      *int               `json:"id,omitempty"`
	Cable   int                `json:"cable"`
	Number  int                `json:"number"`
	Type    *string            `json:"type,omitempty"`
	Status  *string            `json:"status,omitempty"`
	Missing bool               `json:"missing"`
	Cells   []PlenumMatrixCell `json:"cells"`
}

type PlenumMatrixCell struct {
	Cable      int     `json:"cable"`
	SlotNumber int     `json:"slotNumber"`
	Fiber      int     `json:"fiber"`
	Status     string  `json:"status"`
	PortID     *int    `json:"portId,omitempty"`
	PortLabel  *string `json:"portLabel,omitempty"`
	FiberID    *int    `json:"fiberId,omitempty"`
}

type Cable struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	FibersNum      int    `json:"fibersNum"`
	Status         string `json:"status"`
	AssignedFibers int    `json:"assignedFibers"`
}

type CableInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	FibersNum   int    `json:"fibersNum"`
	Status      string `json:"status"`
}

type CablePatch struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Status      *string `json:"status,omitempty"`
}

type Fiber struct {
	ID          int     `json:"id"`
	Number      int     `json:"number"`
	Status      string  `json:"status"`
	CableID     int     `json:"cableId"`
	LeftPortID  *int    `json:"leftPortId,omitempty"`
	RightPortID *int    `json:"rightPortId,omitempty"`
	LeftLabel   *string `json:"leftLabel,omitempty"`
	RightLabel  *string `json:"rightLabel,omitempty"`
}

type FiberAssignmentInput struct {
	LeftPortID  *int `json:"leftPortId,omitempty"`
	RightPortID *int `json:"rightPortId,omitempty"`
}

type PortItem struct {
	ID           int     `json:"id"`
	SlotID       int     `json:"slotId"`
	Number       *int    `json:"number,omitempty"`
	Status       string  `json:"status"`
	PlSlotID     *int    `json:"plSlotId,omitempty"`
	PlPortNumber *int    `json:"plPortNumber,omitempty"`
	RackID       *int    `json:"rackId,omitempty"`
	RackName     *string `json:"rackName,omitempty"`
	PlenumID     *int    `json:"plenumId,omitempty"`
	DeviceID     *int    `json:"deviceId,omitempty"`
	Name         *string `json:"name,omitempty"`
	CableFiberID *int    `json:"cableFiberId,omitempty"`
	Label        string  `json:"label"`
}

type Xcon struct {
	ID             int       `json:"id"`
	Ticket         string    `json:"ticket"`
	PA             *string   `json:"pa,omitempty"`
	CustomerID     int       `json:"customerId"`
	Status         string    `json:"status"`
	OrderCode      *string   `json:"orderCode,omitempty"`
	SerialNumber   *string   `json:"serialNumber,omitempty"`
	Type           string    `json:"type"`
	ActivatedAt    *string   `json:"activatedAt,omitempty"`
	CeasedAt       *string   `json:"ceasedAt,omitempty"`
	AEndUnit       string    `json:"aEndUnit"`
	AEndSlot       *string   `json:"aEndSlot,omitempty"`
	AEndFibers     string    `json:"aEndFibers"`
	AEndEquipment  string    `json:"aEndEquipment"`
	ZEndUnit       string    `json:"zEndUnit"`
	ZEndSlot       *string   `json:"zEndSlot,omitempty"`
	ZEndFibers     string    `json:"zEndFibers"`
	ZEndEquipment  string    `json:"zEndEquipment"`
	Note           *string   `json:"note,omitempty"`
	ExtendedTicket *string   `json:"extendedTicket,omitempty"`
	CustomerNote   *string   `json:"customerNote,omitempty"`
	Source         *string   `json:"source,omitempty"`
	CreatedAt      *string   `json:"createdAt,omitempty"`
	AEndRackID     *int      `json:"aEndRackId,omitempty"`
	ZEndRackID     *int      `json:"zEndRackId,omitempty"`
	LoaName        *string   `json:"loaName,omitempty"`
	LoaID          *int      `json:"loaId,omitempty"`
	MMRPort        *string   `json:"mmrPort,omitempty"`
	Hops           []XconHop `json:"hops,omitempty"`
}

type XconInput struct {
	Ticket         string  `json:"ticket"`
	PA             *string `json:"pa,omitempty"`
	CustomerID     int     `json:"customerId"`
	Status         string  `json:"status"`
	OrderCode      *string `json:"orderCode,omitempty"`
	SerialNumber   *string `json:"serialNumber,omitempty"`
	Type           string  `json:"type"`
	ActivatedAt    *string `json:"activatedAt,omitempty"`
	CeasedAt       *string `json:"ceasedAt,omitempty"`
	AEndUnit       string  `json:"aEndUnit"`
	AEndSlot       *string `json:"aEndSlot,omitempty"`
	AEndFibers     string  `json:"aEndFibers"`
	AEndEquipment  string  `json:"aEndEquipment"`
	ZEndUnit       string  `json:"zEndUnit"`
	ZEndSlot       *string `json:"zEndSlot,omitempty"`
	ZEndFibers     string  `json:"zEndFibers"`
	ZEndEquipment  string  `json:"zEndEquipment"`
	Note           *string `json:"note,omitempty"`
	ExtendedTicket *string `json:"extendedTicket,omitempty"`
	CustomerNote   *string `json:"customerNote,omitempty"`
	Source         *string `json:"source,omitempty"`
	AEndRackID     *int    `json:"aEndRackId,omitempty"`
	ZEndRackID     *int    `json:"zEndRackId,omitempty"`
	LoaName        *string `json:"loaName,omitempty"`
	LoaID          *int    `json:"loaId,omitempty"`
	MMRPort        *string `json:"mmrPort,omitempty"`
}

type XconPatch = XconInput

type XconHop struct {
	ID     int     `json:"id,omitempty"`
	XconID int     `json:"xconId,omitempty"`
	Room   string  `json:"room"`
	Rack   string  `json:"rack"`
	Unit   string  `json:"unit"`
	Slot   *string `json:"slot,omitempty"`
	Fibers string  `json:"fibers"`
	Order  int     `json:"order"`
	RackID int     `json:"rackId"`
}

type XconHopsInput struct {
	Items []XconHop `json:"items"`
}
