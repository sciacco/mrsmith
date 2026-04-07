package compliance

// BlockRequest represents a DNS block request header.
type BlockRequest struct {
	ID                int    `json:"id"`
	RequestDate       string `json:"request_date"`
	Reference         string `json:"reference"`
	MethodID          string `json:"method_id"`
	MethodDescription string `json:"method_description"`
}

// BlockDomain represents a domain in a block request.
type BlockDomain struct {
	ID     int    `json:"id"`
	Domain string `json:"domain"`
}

// ReleaseRequest represents a DNS release request header.
type ReleaseRequest struct {
	ID          int    `json:"id"`
	RequestDate string `json:"request_date"`
	Reference   string `json:"reference"`
}

// ReleaseDomain represents a domain in a release request.
type ReleaseDomain struct {
	ID     int    `json:"id"`
	Domain string `json:"domain"`
}

// Origin represents a block origin/method.
type Origin struct {
	MethodID    string `json:"method_id"`
	Description string `json:"description"`
	IsActive    bool   `json:"is_active"`
}

// DomainStatus represents computed block/release counts for a domain.
type DomainStatus struct {
	Domain       string `json:"domain"`
	BlockCount   int    `json:"block_count"`
	ReleaseCount int    `json:"release_count"`
}

// HistoryEntry represents a single domain event in the history.
type HistoryEntry struct {
	Domain      string `json:"domain"`
	RequestDate string `json:"request_date"`
	Reference   string `json:"reference"`
	RequestType string `json:"request_type"`
}

// CreateBlockRequest is the body for POST /blocks.
type CreateBlockRequest struct {
	RequestDate string   `json:"request_date"`
	Reference   string   `json:"reference"`
	MethodID    string   `json:"method_id"`
	Domains     []string `json:"domains"`
}

// UpdateBlockRequest is the body for PUT /blocks/:id.
type UpdateBlockRequest struct {
	RequestDate string `json:"request_date"`
	Reference   string `json:"reference"`
	MethodID    string `json:"method_id"`
}

// CreateReleaseRequest is the body for POST /releases.
type CreateReleaseRequest struct {
	RequestDate string   `json:"request_date"`
	Reference   string   `json:"reference"`
	Domains     []string `json:"domains"`
}

// UpdateReleaseRequest is the body for PUT /releases/:id.
type UpdateReleaseRequest struct {
	RequestDate string `json:"request_date"`
	Reference   string `json:"reference"`
}

// AddDomainsRequest is the body for POST /blocks/:id/domains and /releases/:id/domains.
type AddDomainsRequest struct {
	Domains []string `json:"domains"`
}

// UpdateDomainRequest is the body for PUT /blocks/:id/domains/:domainId and /releases/:id/domains/:domainId.
type UpdateDomainRequest struct {
	Domain string `json:"domain"`
}

// CreateOriginRequest is the body for POST /origins.
type CreateOriginRequest struct {
	MethodID    string `json:"method_id"`
	Description string `json:"description"`
}

// UpdateOriginRequest is the body for PUT /origins/:id.
type UpdateOriginRequest struct {
	Description string `json:"description"`
}
