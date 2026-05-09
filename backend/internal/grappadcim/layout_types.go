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
	ID       int     `json:"id"`
	Status   string  `json:"status"`
	Type     string  `json:"type"`
	Num      int     `json:"num"`
	IsletID  int     `json:"isletId"`
	RackID   *int    `json:"rackId,omitempty"`
	RackName *string `json:"rackName,omitempty"`
	RackType *string `json:"rackType,omitempty"`
	RackPos  *string `json:"rackPos,omitempty"`
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
