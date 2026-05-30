package grappadcim

type Islet struct {
	ID            int     `json:"id"`
	DatacenterID  int     `json:"datacenterId"`
	Name          string  `json:"name"`
	RackNum       int     `json:"rackNum"`
	Type          string  `json:"type"`
	Floor         int     `json:"floor"`
	Serial        *string `json:"serial,omitempty"`
	Order         *string `json:"order,omitempty"`
	CustomerID    *int    `json:"customerId,omitempty"`
	PositionCount int     `json:"positionCount"`
	OccupiedCount int     `json:"occupiedCount"`
}

type IsletInput struct {
	DatacenterID int     `json:"datacenterId"`
	Name         string  `json:"name"`
	RackNum      int     `json:"rackNum"`
	Type         string  `json:"type"`
	Floor        int     `json:"floor"`
	Serial       *string `json:"serial,omitempty"`
	Order        *string `json:"order,omitempty"`
	CustomerID   *int    `json:"customerId,omitempty"`
}

type IsletPatch struct {
	Name       *string `json:"name,omitempty"`
	RackNum    *int    `json:"rackNum,omitempty"`
	Type       *string `json:"type,omitempty"`
	Floor      *int    `json:"floor,omitempty"`
	Serial     *string `json:"serial,omitempty"`
	Order      *string `json:"order,omitempty"`
	CustomerID *int    `json:"customerId,omitempty"`
}

type Position struct {
	ID      int            `json:"id"`
	Status  string         `json:"status"`
	Type    string         `json:"type"`
	Num     int            `json:"num"`
	IsletID int            `json:"isletId"`
	Racks   []PositionRack `json:"racks"`
}

// PositionRack is an active rack placed on a position (mattonella).
// A Full position holds at most one rack (pos "F"); a Half position holds
// up to two (pos "A" = mezzo alto, pos "B" = mezzo basso).
type PositionRack struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Pos    string `json:"pos"`
	Shared bool   `json:"shared"` // condiviso: cabinet hosts equipment from multiple customers — never free
}

type PositionPatch struct {
	Status *string `json:"status,omitempty"`
	Type   *string `json:"type,omitempty"`
	Num    *int    `json:"num,omitempty"`
}

type PositionBatchInput struct {
	Count int    `json:"count"`
	Type  string `json:"type"`
}
