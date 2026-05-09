package grappadcim

type FiberRing struct {
	ID                 int                `json:"id"`
	Name               string             `json:"name"`
	CustomerID         *int               `json:"customerId,omitempty"`
	NodeCount          int                `json:"nodeCount"`
	Note               *string            `json:"note,omitempty"`
	SerialNumber       *string            `json:"serialNumber,omitempty"`
	OrderCode          *string            `json:"orderCode,omitempty"`
	Status             string             `json:"status"`
	KMLFilePresent     bool               `json:"kmlFilePresent"`
	NodeTotal          int                `json:"nodeTotal"`
	ArcTotal           int                `json:"arcTotal"`
	RouteTotal         int                `json:"routeTotal"`
	KMLArtifactTotal   int                `json:"kmlArtifactTotal"`
	DeleteCheck        *DependencySummary `json:"deleteCheck,omitempty"`
	TopologyConsistent bool               `json:"topologyConsistent"`
}

type FiberRingInput struct {
	Name         string  `json:"name"`
	CustomerID   *int    `json:"customerId,omitempty"`
	NodeCount    int     `json:"nodeCount"`
	Note         *string `json:"note,omitempty"`
	SerialNumber *string `json:"serialNumber,omitempty"`
	OrderCode    *string `json:"orderCode,omitempty"`
	Status       *string `json:"status,omitempty"`
}

type FiberRingPatch struct {
	Name         *string `json:"name,omitempty"`
	CustomerID   *int    `json:"customerId,omitempty"`
	NodeCount    *int    `json:"nodeCount,omitempty"`
	Note         *string `json:"note,omitempty"`
	SerialNumber *string `json:"serialNumber,omitempty"`
	OrderCode    *string `json:"orderCode,omitempty"`
	Status       *string `json:"status,omitempty"`
}

type IncreaseFiberRingNodesRequest struct {
	NodeCount int `json:"nodeCount"`
}

type FiberRingTopology struct {
	Ring  FiberRing       `json:"ring"`
	Nodes []FiberRingNode `json:"nodes"`
	Arcs  []FiberRingArc  `json:"arcs"`
}

type FiberRingNode struct {
	ID                  int      `json:"id"`
	Identifier          string   `json:"identifier"`
	Address             string   `json:"address"`
	LineSheetID         *int     `json:"lineSheetId,omitempty"`
	CustomerID          *int     `json:"customerId,omitempty"`
	RingID              int      `json:"ringId"`
	Longitude           *float64 `json:"longitude,omitempty"`
	Latitude            *float64 `json:"latitude,omitempty"`
	Position            *int     `json:"position,omitempty"`
	SwitchModel         *string  `json:"switchModel,omitempty"`
	SwitchSerialNumber  *string  `json:"switchSerialNumber,omitempty"`
	SwitchMacAddress    *string  `json:"switchMacAddress,omitempty"`
	IPAddress           *string  `json:"ipAddress,omitempty"`
	UPSIPAddress        *string  `json:"upsIpAddress,omitempty"`
	EAPSMasterNode      *string  `json:"eapsMasterNode,omitempty"`
	EastNodeID          *int     `json:"eastNodeId,omitempty"`
	EastPort            *string  `json:"eastPort,omitempty"`
	PrimaryEastPort     *string  `json:"primaryEastPort,omitempty"`
	SecondaryEastPort   *string  `json:"secondaryEastPort,omitempty"`
	EastTransceiverType *string  `json:"eastTransceiverType,omitempty"`
	WestNodeID          *int     `json:"westNodeId,omitempty"`
	WestPort            *string  `json:"westPort,omitempty"`
	PrimaryWestPort     *string  `json:"primaryWestPort,omitempty"`
	SecondaryWestPort   *string  `json:"secondaryWestPort,omitempty"`
	WestTransceiverType *string  `json:"westTransceiverType,omitempty"`
	Note                *string  `json:"note,omitempty"`
}

type FiberRingNodePatch struct {
	Identifier          *string  `json:"identifier,omitempty"`
	Address             *string  `json:"address,omitempty"`
	LineSheetID         *int     `json:"lineSheetId,omitempty"`
	CustomerID          *int     `json:"customerId,omitempty"`
	Longitude           *float64 `json:"longitude,omitempty"`
	Latitude            *float64 `json:"latitude,omitempty"`
	Position            *int     `json:"position,omitempty"`
	SwitchModel         *string  `json:"switchModel,omitempty"`
	SwitchSerialNumber  *string  `json:"switchSerialNumber,omitempty"`
	SwitchMacAddress    *string  `json:"switchMacAddress,omitempty"`
	IPAddress           *string  `json:"ipAddress,omitempty"`
	UPSIPAddress        *string  `json:"upsIpAddress,omitempty"`
	EAPSMasterNode      *string  `json:"eapsMasterNode,omitempty"`
	EastNodeID          *int     `json:"eastNodeId,omitempty"`
	EastPort            *string  `json:"eastPort,omitempty"`
	PrimaryEastPort     *string  `json:"primaryEastPort,omitempty"`
	SecondaryEastPort   *string  `json:"secondaryEastPort,omitempty"`
	EastTransceiverType *string  `json:"eastTransceiverType,omitempty"`
	WestNodeID          *int     `json:"westNodeId,omitempty"`
	WestPort            *string  `json:"westPort,omitempty"`
	PrimaryWestPort     *string  `json:"primaryWestPort,omitempty"`
	SecondaryWestPort   *string  `json:"secondaryWestPort,omitempty"`
	WestTransceiverType *string  `json:"westTransceiverType,omitempty"`
	Note                *string  `json:"note,omitempty"`
}

type FiberRingArc struct {
	ID                int              `json:"id"`
	RingID            int              `json:"ringId"`
	FromNodeID        int              `json:"fromNodeId"`
	ToNodeID          int              `json:"toNodeId"`
	FromIdentifier    *string          `json:"fromIdentifier,omitempty"`
	ToIdentifier      *string          `json:"toIdentifier,omitempty"`
	Distance          *float64         `json:"distance,omitempty"`
	Attenuation       *float64         `json:"attenuation,omitempty"`
	Reference         *string          `json:"reference,omitempty"`
	MetrowebReference *string          `json:"metrowebReference,omitempty"`
	ReleasedAt        *string          `json:"releasedAt,omitempty"`
	Routes            []FiberRingRoute `json:"routes,omitempty"`
}

type FiberRingArcPatch struct {
	Distance          *float64 `json:"distance,omitempty"`
	Attenuation       *float64 `json:"attenuation,omitempty"`
	Reference         *string  `json:"reference,omitempty"`
	MetrowebReference *string  `json:"metrowebReference,omitempty"`
	ReleasedAt        *string  `json:"releasedAt,omitempty"`
}

type FiberRingRoute struct {
	ID                        int      `json:"id,omitempty"`
	ArcID                     int      `json:"arcId,omitempty"`
	Identifier                *string  `json:"identifier,omitempty"`
	SourceCabinet             *string  `json:"sourceCabinet,omitempty"`
	SourceLevel               *string  `json:"sourceLevel,omitempty"`
	SourceCable               *string  `json:"sourceCable,omitempty"`
	SourceFibers              *string  `json:"sourceFibers,omitempty"`
	SourceOpticalSegment      *string  `json:"sourceOpticalSegment,omitempty"`
	DestinationCabinet        *string  `json:"destinationCabinet,omitempty"`
	DestinationLevel          *string  `json:"destinationLevel,omitempty"`
	DestinationCable          *string  `json:"destinationCable,omitempty"`
	DestinationFibers         *string  `json:"destinationFibers,omitempty"`
	DestinationOpticalSegment *string  `json:"destinationOpticalSegment,omitempty"`
	RouteLengthMeters         *float64 `json:"routeLengthMeters,omitempty"`
	DropLengthMeters          *float64 `json:"dropLengthMeters,omitempty"`
}

type FiberRingRoutesInput struct {
	ArcID  int              `json:"arcId"`
	Routes []FiberRingRoute `json:"routes"`
}

type FiberRingKML struct {
	RingID    int        `json:"ringId"`
	Artifacts []Artifact `json:"artifacts"`
}

type Artifact struct {
	ID          int     `json:"id"`
	Kind        string  `json:"kind"`
	Name        string  `json:"name"`
	FileName    string  `json:"fileName"`
	RingName    *string `json:"ringName,omitempty"`
	Detail      *string `json:"detail,omitempty"`
	Available   bool    `json:"available"`
	DownloadURL *string `json:"downloadUrl,omitempty"`
}
