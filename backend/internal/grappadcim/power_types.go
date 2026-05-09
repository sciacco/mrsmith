package grappadcim

type RackSocket struct {
	ID                   int      `json:"id"`
	RackID               *int     `json:"rackId,omitempty"`
	Magnetotermico       string   `json:"magnetotermico"`
	SNMPMonitoringDevice string   `json:"snmpMonitoringDevice"`
	DetectorIP           string   `json:"detectorIp"`
	OID                  string   `json:"oid"`
	OID2                 string   `json:"oid2"`
	OID3                 string   `json:"oid3"`
	OID4                 string   `json:"oid4"`
	Position             string   `json:"position"`
	Position2            string   `json:"position2"`
	Position3            string   `json:"position3"`
	Position4            string   `json:"position4"`
	Status               string   `json:"status"`
	LatestAmpere         *float64 `json:"latestAmpere,omitempty"`
	LatestReadingAt      *string  `json:"latestReadingAt,omitempty"`
}

type RackSocketInput struct {
	Magnetotermico       *string `json:"magnetotermico,omitempty"`
	SNMPMonitoringDevice *string `json:"snmpMonitoringDevice,omitempty"`
	DetectorIP           *string `json:"detectorIp,omitempty"`
	OID                  *string `json:"oid,omitempty"`
	OID2                 *string `json:"oid2,omitempty"`
	OID3                 *string `json:"oid3,omitempty"`
	OID4                 *string `json:"oid4,omitempty"`
	Position             *string `json:"position,omitempty"`
	Position2            *string `json:"position2,omitempty"`
	Position3            *string `json:"position3,omitempty"`
	Position4            *string `json:"position4,omitempty"`
	Status               *string `json:"status,omitempty"`
}

type RackPowerReading struct {
	ID           int     `json:"id"`
	OID          string  `json:"oid"`
	Date         string  `json:"date"`
	Ampere       float64 `json:"ampere"`
	RackSocketID int     `json:"rackSocketId"`
}

type RackPowerReadingsResponse struct {
	Items []RackPowerReading `json:"items"`
	Total int                `json:"total"`
	Page  int                `json:"page"`
	Size  int                `json:"size"`
}

type RackPowerSummaryPoint struct {
	Day      *string  `json:"day,omitempty"`
	Kilowatt *float64 `json:"kilowatt,omitempty"`
}
