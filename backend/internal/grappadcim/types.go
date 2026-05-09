package grappadcim

type MetaResponse struct {
	CanRead            bool   `json:"canRead"`
	CanOperate         bool   `json:"canOperate"`
	CanViewCredentials bool   `json:"canViewCredentials"`
	AppVersion         string `json:"appVersion,omitempty"`
}

type LookupItem struct {
	ID    any    `json:"id"`
	Label string `json:"label"`
}

type LookupResponse struct {
	Infrastructure []LookupItem `json:"infrastructure"`
	Assets         []LookupItem `json:"assets"`
	Connectivity   []LookupItem `json:"connectivity"`
	Topology       []LookupItem `json:"topology"`
}

type DestructiveActionRequest struct {
	ConfirmPrimary     bool    `json:"confirmPrimary"`
	ConfirmSecondary   bool    `json:"confirmSecondary"`
	ConfirmationPhrase *string `json:"confirmationPhrase,omitempty"`
	Reason             *string `json:"reason,omitempty"`
}

type MutationResponse struct {
	ID      int    `json:"id,omitempty"`
	Message string `json:"message"`
}

type DependencySummary struct {
	Allowed bool               `json:"allowed"`
	Counts  map[string]int     `json:"counts"`
	Message string             `json:"message,omitempty"`
	Details []DependencyDetail `json:"details,omitempty"`
}

type DependencyDetail struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}
