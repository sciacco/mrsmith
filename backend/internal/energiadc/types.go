package energiadc

type lookupItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type rackDetailResponse struct {
	ID               int      `json:"id"`
	Name             string   `json:"name"`
	CustomerID       int      `json:"customerId"`
	CustomerName     string   `json:"customerName"`
	BuildingName     string   `json:"buildingName"`
	RoomName         string   `json:"roomName"`
	Floor            *int     `json:"floor,omitempty"`
	Island           *int     `json:"island,omitempty"`
	RackType         string   `json:"rackType,omitempty"`
	Position         string   `json:"position,omitempty"`
	OrderCode        string   `json:"orderCode,omitempty"`
	SerialNumber     string   `json:"serialNumber,omitempty"`
	CommittedPower   *float64 `json:"committedPower,omitempty"`
	VariableBilling  bool     `json:"variableBilling"`
	BillingStartDate string   `json:"billingStartDate,omitempty"`
}

type rackSocketStatusResponse struct {
	SocketID     int      `json:"socketId"`
	Label        string   `json:"label"`
	Ampere       float64  `json:"ampere"`
	MaxAmpere    float64  `json:"maxAmpere"`
	UsagePercent float64  `json:"usagePercent"`
	PowerMeter   string   `json:"powerMeter"`
	DetectorIP   string   `json:"detectorIp"`
	Breaker      string   `json:"breaker"`
	Positions    []string `json:"positions"`
	Position1    string   `json:"position1"`
	Position2    string   `json:"position2"`
	Position3    string   `json:"position3"`
	Position4    string   `json:"position4"`
}

type powerReadingRowResponse struct {
	ID          int     `json:"id"`
	SocketID    int     `json:"socketId"`
	SocketLabel string  `json:"socketLabel"`
	OID         string  `json:"oid"`
	Date        string  `json:"date"`
	Ampere      float64 `json:"ampere"`
}

type powerReadingsPageResponse struct {
	Items []powerReadingRowResponse `json:"items"`
	Total int                       `json:"total"`
	Page  int                       `json:"page"`
	Size  int                       `json:"size"`
}

type rackStatPointResponse struct {
	Bucket   string  `json:"bucket"`
	Ampere   float64 `json:"ampere"`
	Kilowatt float64 `json:"kilowatt"`
}

type kwPointResponse struct {
	Bucket     string  `json:"bucket"`
	Label      string  `json:"label"`
	RangeLabel string  `json:"rangeLabel"`
	Kilowatt   float64 `json:"kilowatt"`
}

type billingChargeResponse struct {
	ID               int      `json:"id"`
	StartPeriod      string   `json:"startPeriod,omitempty"`
	EndPeriod        string   `json:"endPeriod,omitempty"`
	Ampere           float64  `json:"ampere"`
	Eccedenti        float64  `json:"eccedenti"`
	Amount           *float64 `json:"amount,omitempty"`
	PUN              float64  `json:"pun"`
	Coefficiente     float64  `json:"coefficiente"`
	FissoCU          float64  `json:"fissoCu"`
	ImportoEccedenti float64  `json:"importoEccedenti"`
}

type noVariableRackResponse struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	BuildingName    string `json:"buildingName"`
	RoomName        string `json:"roomName"`
	Floor           *int   `json:"floor,omitempty"`
	Island          *int   `json:"island,omitempty"`
	RackType        string `json:"rackType,omitempty"`
	Position        string `json:"position,omitempty"`
	OrderCode       string `json:"orderCode,omitempty"`
	SerialNumber    string `json:"serialNumber,omitempty"`
	VariableBilling bool   `json:"variableBilling"`
}

type lowConsumptionRowResponse struct {
	CustomerID   int      `json:"customerId"`
	CustomerName string   `json:"customerName"`
	BuildingName string   `json:"buildingName"`
	RoomName     string   `json:"roomName"`
	RackName     string   `json:"rackName"`
	SocketID     int      `json:"socketId"`
	SocketLabel  string   `json:"socketLabel"`
	Ampere       float64  `json:"ampere"`
	PowerMeter   string   `json:"powerMeter"`
	Breaker      string   `json:"breaker"`
	Positions    []string `json:"positions"`
}
