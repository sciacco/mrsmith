package openapiit

import "encoding/json"

// CompanyDataset preserves vendor Company payloads without forcing every
// downstream caller to depend on a hand-maintained copy of the whole spec.
type CompanyDataset = json.RawMessage

type CompanyRequest struct {
	State string `json:"state"`
	ID    string `json:"id"`
}

type CompanyPostBody struct {
	Callback *CompanyCallback `json:"callback,omitempty"`
}

type CompanyCallback struct {
	Method  string            `json:"method,omitempty"`
	Field   string            `json:"field,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Data    map[string]any    `json:"data,omitempty"`
}

type CompanyITSearchParams struct {
	DryRun              *int
	DataEnrichment      string
	Lat                 *float64
	Long                *float64
	Radius              *int
	CompanyName         string
	Autocomplete        string
	Province            string
	TownCode            string
	AtecoCode           string
	CCIAA               string
	REACode             string
	MinTurnover         *int
	MaxTurnover         *int
	MinEmployees        *int
	MaxEmployees        *int
	SDICode             string
	LegalFormCode       string
	ShareHolderTaxCode  string
	ActivityStatus      string
	PEC                 string
	CreationTimestamp   *int
	LastUpdateTimestamp *int
	Skip                *int
	Limit               *int
}

type CompanyFRSearchParams struct {
	DryRun              *int
	DataEnrichment      string
	Lat                 *float64
	Long                *float64
	Radius              *int
	CompanyName         string
	Autocomplete        string
	Town                string
	TownCode            string
	NAFCode             string
	DepartmentCode      string
	RegionCode          string
	MinTurnover         *int
	MaxTurnover         *int
	MinEmployees        *int
	MaxEmployees        *int
	CreditWorthy        *int
	LegalFormCode       string
	ActivityStatus      string
	CreationTimestamp   *int
	LastUpdateTimestamp *int
	Skip                *int
	Limit               *int
}

type CompanyClosedParams struct {
	Date string
}

type CompanyVATGroupParams struct {
	TaxCode string
}

type CompanyMonitorGetParams struct {
	Callback bool
}

type CompanyMonitorPayload struct {
	Code            string           `json:"code"`
	Dataset         string           `json:"dataset"`
	MonitoredFields []string         `json:"monitoredFields,omitempty"`
	Callback        *CompanyCallback `json:"callback,omitempty"`
	AutoRenew       *bool            `json:"autorenew,omitempty"`
	CCIAA           string           `json:"cciaa,omitempty"`
	Country         string           `json:"country,omitempty"`
}

type CompanyMonitorRequest struct {
	Code            string           `json:"code"`
	Dataset         string           `json:"dataset"`
	MonitoredFields []string         `json:"monitoredFields"`
	Callback        *CompanyCallback `json:"callback"`
	Owner           string           `json:"owner"`
	CreatedOn       string           `json:"createdOn"`
	CompanyID       string           `json:"companyId"`
	VATCode         *string          `json:"vatCode"`
	TaxCode         *string          `json:"taxCode"`
	CompanyName     string           `json:"companyName"`
	UpdatedFields   map[string]any   `json:"updatedFields"`
	LatestDataset   CompanyDataset   `json:"latestDataset"`
	PreviousDataset CompanyDataset   `json:"previousDataset"`
	CheckedOn       *string          `json:"checkedOn"`
	NextOn          string           `json:"nextOn"`
	ExpiresOn       string           `json:"expiresOn"`
	AutoRenew       bool             `json:"autorenew"`
	ID              string           `json:"id"`
}

type CompanyMonitorSummary struct {
	CompanyName string  `json:"companyName"`
	CheckedOn   *string `json:"checkedOn"`
	NextOn      string  `json:"nextOn"`
	ExpiresOn   string  `json:"expiresOn"`
	ID          string  `json:"id"`
}

type CompanyLegalForm struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}
