package grappadcim

type CameraItem struct {
	ID       int     `json:"id"`
	Code     string  `json:"code"`
	Model    string  `json:"model"`
	Brand    string  `json:"brand"`
	Position string  `json:"position"`
	IPAddr   *string `json:"ipaddr,omitempty"`
	Status   *string `json:"status,omitempty"`
	Serial   *string `json:"serial,omitempty"`
}

type CameraInput struct {
	Code     string  `json:"code"`
	Model    string  `json:"model"`
	Brand    string  `json:"brand"`
	Position string  `json:"position"`
	IPAddr   *string `json:"ipaddr,omitempty"`
	Status   *string `json:"status,omitempty"`
	Serial   *string `json:"serial,omitempty"`
}

type CameraPatch struct {
	Code     *string `json:"code,omitempty"`
	Model    *string `json:"model,omitempty"`
	Brand    *string `json:"brand,omitempty"`
	Position *string `json:"position,omitempty"`
	IPAddr   *string `json:"ipaddr,omitempty"`
	Status   *string `json:"status,omitempty"`
	Serial   *string `json:"serial,omitempty"`
}
