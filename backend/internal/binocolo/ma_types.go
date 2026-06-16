package binocolo

import (
	"encoding/json"
	"time"
)

const (
	maStrategyTypeATECO    = "ateco"
	maStrategyTypeExpanded = "expanded"

	maSessionStatusDraft     = "draft"
	maSessionStatusEstimated = "estimated"
	maSessionStatusRunning   = "running"
	maSessionStatusCompleted = "completed"
	maSessionStatusFailed    = "failed"

	maRunStatusRunning   = "running"
	maRunStatusCompleted = "completed"
	maRunStatusFailed    = "failed"

	maMatchStateMatch   = "match"
	maMatchStatePartial = "match_parziale"
	maMatchStateOutside = "fuori_criterio"

	maEvidenceMatch   = "match"
	maEvidencePartial = "match_parziale"
	maEvidenceMissing = "criterio_mancante"
	maEvidenceOutside = "fuori_criterio"

	maModelScopeStrategy             = "ma_strategy"
	maModelScopeSectorClassification = "ma_sector_classification"

	maATECOSuccessThreshold = 10
	maDefaultSearchLimit    = 100
	maVendorLimit           = 1000
)

type MACreateSessionRequest struct {
	Prompt   string `json:"prompt"`
	ModelID  string `json:"modelId,omitempty"`
	PromptID string `json:"promptId,omitempty"`
}

type MAEstimateSessionRequest struct {
	Strategy *MAStrategySpec `json:"strategy,omitempty"`
}

type MAExecuteSessionRequest struct {
	Strategy     *MAStrategySpec `json:"strategy,omitempty"`
	StrategyType string          `json:"strategyType,omitempty"`
	Limit        int             `json:"limit,omitempty"`
}

type MAExportRequest struct {
	Format string `json:"format,omitempty"`
}

type MALLMOptionsResponse struct {
	Models  []MALLMModelOption  `json:"models"`
	Prompts []MALLMPromptOption `json:"prompts"`
}

type MALLMModelOption struct {
	ID        string `json:"id"`
	Scope     string `json:"scope"`
	Name      string `json:"name"`
	Model     string `json:"model"`
	IsDefault bool   `json:"isDefault"`
}

type MALLMPromptOption struct {
	ID        string `json:"id"`
	Scope     string `json:"scope"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
}

type MASessionSummary struct {
	ID               string     `json:"id"`
	Title            string     `json:"title"`
	Prompt           string     `json:"prompt"`
	Status           string     `json:"status"`
	SelectedStrategy string     `json:"selectedStrategy,omitempty"`
	EstimatedCount   int        `json:"estimatedCount"`
	EstimatedCost    float64    `json:"estimatedCost"`
	ResultCount      int        `json:"resultCount"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	LastRunAt        *time.Time `json:"lastRunAt,omitempty"`
}

type MASessionDetail struct {
	Session   MASession          `json:"session"`
	Strategy  *MAStrategyVersion `json:"strategy,omitempty"`
	Estimates []MAEstimate       `json:"estimates"`
	Runs      []MAExecutionRun   `json:"runs"`
	Targets   []MATarget         `json:"targets"`
}

type MASession struct {
	ID               string     `json:"id"`
	Title            string     `json:"title"`
	Prompt           string     `json:"prompt"`
	Status           string     `json:"status"`
	SelectedStrategy string     `json:"selectedStrategy,omitempty"`
	ActiveStrategyID string     `json:"activeStrategyId,omitempty"`
	CreatedBySubject string     `json:"createdBySubject,omitempty"`
	CreatedByEmail   string     `json:"createdByEmail,omitempty"`
	LastEstimatedAt  *time.Time `json:"lastEstimatedAt,omitempty"`
	LastExecutedAt   *time.Time `json:"lastExecutedAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type MAStrategyVersion struct {
	ID             string         `json:"id"`
	SessionID      string         `json:"sessionId"`
	Version        int            `json:"version"`
	Strategy       MAStrategySpec `json:"strategy"`
	CreatedByEmail string         `json:"createdByEmail,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
}

type MAStrategySpec struct {
	Title                  string               `json:"title,omitempty"`
	SectorDescription      string               `json:"sectorDescription"`
	TerritoryLabel         string               `json:"territoryLabel,omitempty"`
	Provinces              []string             `json:"provinces"`
	ActivityStatus         string               `json:"activityStatus"`
	TurnoverAround         *int                 `json:"turnoverAround,omitempty"`
	TurnoverMin            *int                 `json:"turnoverMin,omitempty"`
	TurnoverMax            *int                 `json:"turnoverMax,omitempty"`
	EmployeeMin            *int                 `json:"employeeMin,omitempty"`
	EmployeeMax            *int                 `json:"employeeMax,omitempty"`
	SearchLimit            int                  `json:"searchLimit"`
	AtecoCandidates        []MAAtecoCandidate   `json:"atecoCandidates"`
	Keywords               []string             `json:"keywords"`
	ScoringCriteria        []MAScoringCriterion `json:"scoringCriteria,omitempty"`
	Rationale              string               `json:"rationale"`
	MissingCriteria        []string             `json:"missingCriteria"`
	SelectedStrategy       string               `json:"selectedStrategy,omitempty"`
	ExpandedClassification string               `json:"expandedClassification,omitempty"`
}

type MAAtecoCandidate struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Rationale   string `json:"rationale"`
	SearchCode  string `json:"-"`
}

type MAScoringCriterion struct {
	ID          string              `json:"id"`
	Label       string              `json:"label"`
	Description string              `json:"description,omitempty"`
	Weight      int                 `json:"weight"`
	Evaluation  MAScoringEvaluation `json:"evaluation"`
	Source      string              `json:"source,omitempty"`
}

type MAScoringEvaluation struct {
	SourcePath string   `json:"sourcePath,omitempty"`
	Operator   string   `json:"operator,omitempty"`
	Value      any      `json:"value,omitempty"`
	Min        *float64 `json:"min,omitempty"`
	Max        *float64 `json:"max,omitempty"`
	Tolerance  *float64 `json:"tolerance,omitempty"`
	Match      string   `json:"match,omitempty"`
}

type MAEstimate struct {
	ID                string          `json:"id"`
	SessionID         string          `json:"sessionId"`
	StrategyVersionID string          `json:"strategyVersionId"`
	StrategyType      string          `json:"strategyType"`
	AtecoCode         string          `json:"atecoCode,omitempty"`
	AtecoDescription  string          `json:"atecoDescription,omitempty"`
	Province          string          `json:"province,omitempty"`
	EstimatedCount    int             `json:"estimatedCount"`
	EstimatedCost     float64         `json:"estimatedCost"`
	Selected          bool            `json:"selected"`
	Params            json.RawMessage `json:"params,omitempty"`
	VendorResponse    json.RawMessage `json:"vendorResponse,omitempty"`
	CreatedAt         time.Time       `json:"createdAt"`
}

type MAExecutionRun struct {
	ID                string     `json:"id"`
	SessionID         string     `json:"sessionId"`
	StrategyVersionID string     `json:"strategyVersionId"`
	StrategyType      string     `json:"strategyType"`
	Status            string     `json:"status"`
	EstimatedCount    int        `json:"estimatedCount"`
	ResultCount       int        `json:"resultCount"`
	ErrorCode         string     `json:"errorCode,omitempty"`
	StartedAt         time.Time  `json:"startedAt"`
	CompletedAt       *time.Time `json:"completedAt,omitempty"`
}

type MATarget struct {
	ID               string             `json:"id"`
	SessionID        string             `json:"sessionId"`
	RunID            string             `json:"runId"`
	VendorID         string             `json:"vendorId,omitempty"`
	CompanyName      string             `json:"companyName"`
	VATCode          string             `json:"vatCode,omitempty"`
	TaxCode          string             `json:"taxCode,omitempty"`
	Province         string             `json:"province,omitempty"`
	Town             string             `json:"town,omitempty"`
	ActivityStatus   string             `json:"activityStatus,omitempty"`
	Turnover         *int               `json:"turnover,omitempty"`
	TurnoverYear     *int               `json:"turnoverYear,omitempty"`
	Employees        *int               `json:"employees,omitempty"`
	AtecoCode        string             `json:"atecoCode,omitempty"`
	AtecoDescription string             `json:"atecoDescription,omitempty"`
	Score            int                `json:"score"`
	MatchState       string             `json:"matchState"`
	Rationale        string             `json:"rationale"`
	MissingCriteria  []string           `json:"missingCriteria"`
	Evidence         []MATargetEvidence `json:"evidence"`
	VendorPayload    json.RawMessage    `json:"vendorPayload,omitempty"`
	CreatedAt        time.Time          `json:"createdAt"`
}

type MATargetEvidence struct {
	Criterion  string `json:"criterion"`
	Status     string `json:"status"`
	Label      string `json:"label"`
	Value      string `json:"value,omitempty"`
	SourcePath string `json:"sourcePath,omitempty"`
}

type maStrategyDraftEnvelope struct {
	Strategy MAStrategySpec `json:"strategy"`
}

type maModelAuditWrite struct {
	SessionID         string
	StrategyVersionID string
	Scope             string
	ModelID           string
	PromptID          string
	Model             string
	Prompt            json.RawMessage
	Response          json.RawMessage
	Usage             json.RawMessage
}

type maLLMModel struct {
	ID        string
	Scope     string
	Name      string
	Model     string
	IsDefault bool
}

type maLLMPrompt struct {
	ID        string
	Scope     string
	Name      string
	Prompt    string
	IsDefault bool
}

type maSessionCreate struct {
	Session  MASession
	Strategy MAStrategySpec
}

type maExecutionRunCreate struct {
	ID                string
	SessionID         string
	StrategyVersionID string
	StrategyType      string
	EstimatedCount    int
}
