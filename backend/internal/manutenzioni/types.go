package manutenzioni

import (
	"encoding/json"
	"time"
)

const (
	StatusDraft      = "draft"
	StatusApproved   = "approved"
	StatusScheduled  = "scheduled"
	StatusAnnounced  = "announced"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusCancelled  = "cancelled"
	StatusSuperseded = "superseded"
)

type pagedResponse[T any] struct {
	Items    []T `json:"items"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}

type ReferenceItem struct {
	ID                  int64   `json:"id"`
	Code                string  `json:"code"`
	NameIT              string  `json:"name_it"`
	NameEN              *string `json:"name_en,omitempty"`
	Description         *string `json:"description,omitempty"`
	SortOrder           int     `json:"sort_order"`
	IsActive            bool    `json:"is_active"`
	City                *string `json:"city,omitempty"`
	CountryCode         *string `json:"country_code,omitempty"`
	TechnicalDomainID   *int64  `json:"technical_domain_id,omitempty"`
	TechnicalDomainName *string `json:"technical_domain_name,omitempty"`
	Scope               *string `json:"scope,omitempty"`
	OwnerMaintenanceID  *int64  `json:"owner_maintenance_id,omitempty"`
}

type ReferenceData struct {
	Sites            []ReferenceItem `json:"sites"`
	TechnicalDomains []ReferenceItem `json:"technical_domains"`
	MaintenanceKinds []ReferenceItem `json:"maintenance_kinds"`
	CustomerScopes   []ReferenceItem `json:"customer_scopes"`
	ServiceTaxonomy  []ReferenceItem `json:"service_taxonomy"`
	ReasonClasses    []ReferenceItem `json:"reason_classes"`
	ImpactEffects    []ReferenceItem `json:"impact_effects"`
	QualityFlags     []ReferenceItem `json:"quality_flags"`
	TargetTypes      []ReferenceItem `json:"target_types"`
	NoticeChannels   []ReferenceItem `json:"notice_channels"`
}

type StatusCount struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

type MaintenanceListItem struct {
	MaintenanceID      int64          `json:"maintenance_id"`
	Code               string         `json:"code"`
	TitleIT            string         `json:"title_it"`
	TitleEN            *string        `json:"title_en,omitempty"`
	Status             string         `json:"status"`
	MaintenanceKind    ReferenceItem  `json:"maintenance_kind"`
	TechnicalDomain    ReferenceItem  `json:"technical_domain"`
	CustomerScope      ReferenceItem  `json:"customer_scope"`
	Site               *ReferenceItem `json:"site,omitempty"`
	CurrentWindow      *WindowSummary `json:"current_window,omitempty"`
	PrimaryImpactLabel *string        `json:"primary_impact_label,omitempty"`
	NoticeStatuses     []StatusCount  `json:"notice_statuses"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type WindowSummary struct {
	MaintenanceWindowID     int64     `json:"maintenance_window_id"`
	SeqNo                   int       `json:"seq_no"`
	WindowStatus            string    `json:"window_status"`
	ScheduledStartAt        time.Time `json:"scheduled_start_at"`
	ScheduledEndAt          time.Time `json:"scheduled_end_at"`
	ExpectedDowntimeMinutes *int      `json:"expected_downtime_minutes,omitempty"`
}

type MaintenanceDetail struct {
	MaintenanceID     int64                `json:"maintenance_id"`
	Code              string               `json:"code"`
	TitleIT           string               `json:"title_it"`
	TitleEN           *string              `json:"title_en,omitempty"`
	DescriptionIT     *string              `json:"description_it,omitempty"`
	DescriptionEN     *string              `json:"description_en,omitempty"`
	Status            string               `json:"status"`
	MaintenanceKind   ReferenceItem        `json:"maintenance_kind"`
	TechnicalDomain   ReferenceItem        `json:"technical_domain"`
	CustomerScope     ReferenceItem        `json:"customer_scope"`
	Site              *ReferenceItem       `json:"site,omitempty"`
	ReasonIT          *string              `json:"reason_it,omitempty"`
	ReasonEN          *string              `json:"reason_en,omitempty"`
	ResidualServiceIT *string              `json:"residual_service_it,omitempty"`
	ResidualServiceEN *string              `json:"residual_service_en,omitempty"`
	CurrentWindow     *WindowSummary       `json:"current_window,omitempty"`
	Windows           []MaintenanceWindow  `json:"windows"`
	ServiceTaxonomy   []ClassificationItem `json:"service_taxonomy"`
	ReasonClasses     []ClassificationItem `json:"reason_classes"`
	ImpactEffects     []ClassificationItem `json:"impact_effects"`
	QualityFlags      []ClassificationItem `json:"quality_flags"`
	Targets           []MaintenanceTarget  `json:"targets"`
	ImpactedCustomers []ImpactedCustomer   `json:"impacted_customers"`
	Notices           []Notice             `json:"notices"`
	Events            []MaintenanceEvent   `json:"events"`
	CreatedAt         time.Time            `json:"created_at"`
	UpdatedAt         time.Time            `json:"updated_at"`
	Metadata          json.RawMessage      `json:"metadata,omitempty"`
}

type MaintenanceWindow struct {
	MaintenanceWindowID     int64      `json:"maintenance_window_id"`
	MaintenanceID           int64      `json:"maintenance_id"`
	SeqNo                   int        `json:"seq_no"`
	WindowStatus            string     `json:"window_status"`
	ScheduledStartAt        time.Time  `json:"scheduled_start_at"`
	ScheduledEndAt          time.Time  `json:"scheduled_end_at"`
	ExpectedDowntimeMinutes *int       `json:"expected_downtime_minutes,omitempty"`
	ActualStartAt           *time.Time `json:"actual_start_at,omitempty"`
	ActualEndAt             *time.Time `json:"actual_end_at,omitempty"`
	ActualDowntimeMinutes   *int       `json:"actual_downtime_minutes,omitempty"`
	CancellationReasonIT    *string    `json:"cancellation_reason_it,omitempty"`
	CancellationReasonEN    *string    `json:"cancellation_reason_en,omitempty"`
	AnnouncedAt             *time.Time `json:"announced_at,omitempty"`
	LastNoticeAt            *time.Time `json:"last_notice_at,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
}

type ClassificationItem struct {
	ID            int64           `json:"id"`
	MaintenanceID int64           `json:"maintenance_id"`
	Reference     ReferenceItem   `json:"reference"`
	Source        string          `json:"source"`
	Confidence    *float64        `json:"confidence,omitempty"`
	IsPrimary     bool            `json:"is_primary"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
}

type MaintenanceTarget struct {
	MaintenanceTargetID int64           `json:"maintenance_target_id"`
	MaintenanceID       int64           `json:"maintenance_id"`
	TargetType          ReferenceItem   `json:"target_type"`
	ReferenceTable      *string         `json:"reference_table,omitempty"`
	ReferenceID         *int64          `json:"reference_id,omitempty"`
	ExternalKey         *string         `json:"external_key,omitempty"`
	DisplayName         string          `json:"display_name"`
	Source              string          `json:"source"`
	Confidence          *float64        `json:"confidence,omitempty"`
	IsPrimary           bool            `json:"is_primary"`
	Metadata            json.RawMessage `json:"metadata,omitempty"`
}

type ImpactedCustomer struct {
	MaintenanceImpactedCustomerID int64           `json:"maintenance_impacted_customer_id"`
	MaintenanceID                 int64           `json:"maintenance_id"`
	CustomerID                    int64           `json:"customer_id"`
	CustomerName                  string          `json:"customer_name"`
	OrderID                       *int64          `json:"order_id,omitempty"`
	ServiceID                     *int64          `json:"service_id,omitempty"`
	ImpactScope                   string          `json:"impact_scope"`
	DerivationSource              string          `json:"derivation_source"`
	Confidence                    *float64        `json:"confidence,omitempty"`
	Reason                        *string         `json:"reason,omitempty"`
	Metadata                      json.RawMessage `json:"metadata,omitempty"`
	CreatedAt                     time.Time       `json:"created_at"`
}

type Notice struct {
	NoticeID            int64               `json:"notice_id"`
	MaintenanceID       int64               `json:"maintenance_id"`
	MaintenanceWindowID *int64              `json:"maintenance_window_id,omitempty"`
	NoticeType          string              `json:"notice_type"`
	Audience            string              `json:"audience"`
	NoticeChannel       ReferenceItem       `json:"notice_channel"`
	TemplateCode        *string             `json:"template_code,omitempty"`
	TemplateVersion     *int                `json:"template_version,omitempty"`
	GenerationSource    string              `json:"generation_source"`
	SendStatus          string              `json:"send_status"`
	ScheduledSendAt     *time.Time          `json:"scheduled_send_at,omitempty"`
	SentAt              *time.Time          `json:"sent_at,omitempty"`
	Locales             []NoticeLocale      `json:"locales"`
	QualityFlags        []NoticeQualityFlag `json:"quality_flags"`
	CreatedAt           time.Time           `json:"created_at"`
	Metadata            json.RawMessage     `json:"metadata,omitempty"`
}

type NoticeLocale struct {
	NoticeLocaleID int64   `json:"notice_locale_id"`
	NoticeID       int64   `json:"notice_id"`
	Locale         string  `json:"locale"`
	Subject        string  `json:"subject"`
	BodyHTML       *string `json:"body_html,omitempty"`
	BodyText       *string `json:"body_text,omitempty"`
}

type NoticeQualityFlag struct {
	ID         int64           `json:"id"`
	NoticeID   int64           `json:"notice_id"`
	Reference  ReferenceItem   `json:"reference"`
	Source     string          `json:"source"`
	Confidence *float64        `json:"confidence,omitempty"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
}

type MaintenanceEvent struct {
	MaintenanceEventID  int64           `json:"maintenance_event_id"`
	MaintenanceID       int64           `json:"maintenance_id"`
	MaintenanceWindowID *int64          `json:"maintenance_window_id,omitempty"`
	EventType           string          `json:"event_type"`
	ActorType           string          `json:"actor_type"`
	EventAt             time.Time       `json:"event_at"`
	Summary             *string         `json:"summary,omitempty"`
	Payload             json.RawMessage `json:"payload,omitempty"`
}

type CustomerSearchItem struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type createMaintenanceRequest struct {
	TitleIT                string                `json:"title_it"`
	TitleEN                *string               `json:"title_en"`
	DescriptionIT          *string               `json:"description_it"`
	DescriptionEN          *string               `json:"description_en"`
	MaintenanceKindID      int64                 `json:"maintenance_kind_id"`
	TechnicalDomainID      int64                 `json:"technical_domain_id"`
	CustomerScopeID        int64                 `json:"customer_scope_id"`
	SiteID                 *int64                `json:"site_id"`
	AdhocSite              *adhocSiteInput       `json:"adhoc_site"`
	ReasonIT               *string               `json:"reason_it"`
	ReasonEN               *string               `json:"reason_en"`
	ResidualServiceIT      *string               `json:"residual_service_it"`
	ResidualServiceEN      *string               `json:"residual_service_en"`
	FirstWindow            *windowRequest        `json:"first_window"`
	InitialTargets         []targetRequest       `json:"initial_targets"`
	InitialServiceTaxonomy []classificationInput `json:"initial_service_taxonomy"`
	InitialReasonClasses   []classificationInput `json:"initial_reason_classes"`
	InitialImpactEffects   []classificationInput `json:"initial_impact_effects"`
	InitialQualityFlags    []classificationInput `json:"initial_quality_flags"`
	Metadata               json.RawMessage       `json:"metadata"`
}

type adhocSiteInput struct {
	Name        string  `json:"name"`
	City        *string `json:"city"`
	CountryCode *string `json:"country_code"`
	Code        *string `json:"code"`
}

type updateMaintenanceRequest struct {
	TitleIT           *string         `json:"title_it"`
	TitleEN           *string         `json:"title_en"`
	DescriptionIT     *string         `json:"description_it"`
	DescriptionEN     *string         `json:"description_en"`
	MaintenanceKindID *int64          `json:"maintenance_kind_id"`
	TechnicalDomainID *int64          `json:"technical_domain_id"`
	CustomerScopeID   *int64          `json:"customer_scope_id"`
	SiteID            *int64          `json:"site_id"`
	AdhocSite         *adhocSiteInput `json:"adhoc_site"`
	ClearSite         bool            `json:"clear_site"`
	ReasonIT          *string         `json:"reason_it"`
	ReasonEN          *string         `json:"reason_en"`
	ResidualServiceIT *string         `json:"residual_service_it"`
	ResidualServiceEN *string         `json:"residual_service_en"`
	Metadata          json.RawMessage `json:"metadata"`
}

type statusActionRequest struct {
	Action   string  `json:"action"`
	ReasonIT *string `json:"reason_it"`
	ReasonEN *string `json:"reason_en"`
}

type windowRequest struct {
	ScheduledStartAt        string  `json:"scheduled_start_at"`
	ScheduledEndAt          string  `json:"scheduled_end_at"`
	ExpectedDowntimeMinutes *int    `json:"expected_downtime_minutes"`
	ActualStartAt           *string `json:"actual_start_at"`
	ActualEndAt             *string `json:"actual_end_at"`
	ActualDowntimeMinutes   *int    `json:"actual_downtime_minutes"`
	CancellationReasonIT    *string `json:"cancellation_reason_it"`
	CancellationReasonEN    *string `json:"cancellation_reason_en"`
	AnnouncedAt             *string `json:"announced_at"`
	LastNoticeAt            *string `json:"last_notice_at"`
}

type cancelWindowRequest struct {
	ReasonIT string  `json:"reason_it"`
	ReasonEN *string `json:"reason_en"`
}

type classificationInput struct {
	ReferenceID int64           `json:"reference_id"`
	Source      string          `json:"source"`
	Confidence  *float64        `json:"confidence"`
	IsPrimary   bool            `json:"is_primary"`
	Metadata    json.RawMessage `json:"metadata"`
}

type classificationRequest struct {
	Items []classificationInput `json:"items"`
}

type targetRequest struct {
	TargetTypeID   int64           `json:"target_type_id"`
	ReferenceTable *string         `json:"reference_table"`
	ReferenceID    *int64          `json:"reference_id"`
	ExternalKey    *string         `json:"external_key"`
	DisplayName    string          `json:"display_name"`
	Source         string          `json:"source"`
	Confidence     *float64        `json:"confidence"`
	IsPrimary      bool            `json:"is_primary"`
	Metadata       json.RawMessage `json:"metadata"`
}

type impactedCustomerRequest struct {
	CustomerID       int64           `json:"customer_id"`
	OrderID          *int64          `json:"order_id"`
	ServiceID        *int64          `json:"service_id"`
	ImpactScope      string          `json:"impact_scope"`
	DerivationSource string          `json:"derivation_source"`
	Confidence       *float64        `json:"confidence"`
	Reason           *string         `json:"reason"`
	Metadata         json.RawMessage `json:"metadata"`
}

type assistanceDraftRequest struct {
	Regenerate bool    `json:"regenerate"`
	Note       *string `json:"note"`
}

type assistanceTextProposal struct {
	TitleIT           *string `json:"title_it,omitempty"`
	TitleEN           *string `json:"title_en,omitempty"`
	DescriptionIT     *string `json:"description_it,omitempty"`
	DescriptionEN     *string `json:"description_en,omitempty"`
	ReasonEN          *string `json:"reason_en,omitempty"`
	ResidualServiceEN *string `json:"residual_service_en,omitempty"`
}

type assistanceClassificationProposal struct {
	ReferenceID int64    `json:"reference_id"`
	Label       string   `json:"label"`
	Source      string   `json:"source"`
	Confidence  *float64 `json:"confidence,omitempty"`
	IsPrimary   bool     `json:"is_primary"`
	Rationale   *string  `json:"rationale,omitempty"`
}

type assistanceAudit struct {
	GeneratedAt time.Time `json:"generated_at"`
	Model       string    `json:"model"`
	Summary     string    `json:"summary"`
}

type assistanceUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type assistanceDraftResponse struct {
	Texts           assistanceTextProposal             `json:"texts"`
	ServiceTaxonomy []assistanceClassificationProposal `json:"service_taxonomy"`
	ReasonClasses   []assistanceClassificationProposal `json:"reason_classes"`
	ImpactEffects   []assistanceClassificationProposal `json:"impact_effects"`
	QualityFlags    []assistanceClassificationProposal `json:"quality_flags"`
	Audit           assistanceAudit                    `json:"audit"`
	Usage           assistanceUsage                    `json:"usage"`
}

type assistancePreviewRequest struct {
	Brief             string `json:"brief"`
	MaintenanceKindID *int64 `json:"maintenance_kind_id,omitempty"`
	TechnicalDomainID *int64 `json:"technical_domain_id,omitempty"`
	CustomerScopeID   *int64 `json:"customer_scope_id,omitempty"`
}

type assistancePreviewResponse struct {
	Texts              assistanceTextProposal             `json:"texts"`
	ServiceTaxonomyIDs []int64                            `json:"service_taxonomy_ids"`
	ReasonClassIDs     []int64                            `json:"reason_class_ids"`
	ImpactEffectIDs    []int64                            `json:"impact_effect_ids"`
	QualityFlagIDs     []int64                            `json:"quality_flag_ids"`
	ServiceTaxonomy    []assistanceClassificationProposal `json:"service_taxonomy"`
	ReasonClasses      []assistanceClassificationProposal `json:"reason_classes"`
	ImpactEffects      []assistanceClassificationProposal `json:"impact_effects"`
	QualityFlags       []assistanceClassificationProposal `json:"quality_flags"`
	Audit              assistanceAudit                    `json:"audit"`
	Usage              assistanceUsage                    `json:"usage"`
}

type LLMModel struct {
	Scope string `json:"scope"`
	Model string `json:"model"`
}

type llmModelRequest struct {
	Scope string `json:"scope"`
	Model string `json:"model"`
}

type noticeRequest struct {
	MaintenanceWindowID *int64          `json:"maintenance_window_id"`
	NoticeType          string          `json:"notice_type"`
	Audience            string          `json:"audience"`
	NoticeChannelID     int64           `json:"notice_channel_id"`
	TemplateCode        *string         `json:"template_code"`
	TemplateVersion     *int            `json:"template_version"`
	GenerationSource    string          `json:"generation_source"`
	SendStatus          string          `json:"send_status"`
	ScheduledSendAt     *string         `json:"scheduled_send_at"`
	SentAt              *string         `json:"sent_at"`
	Locales             []localeRequest `json:"locales"`
	Metadata            json.RawMessage `json:"metadata"`
}

type localeRequest struct {
	Locale   string  `json:"locale"`
	Subject  string  `json:"subject"`
	BodyHTML *string `json:"body_html"`
	BodyText *string `json:"body_text"`
}

type noticeStatusRequest struct {
	SendStatus string  `json:"send_status"`
	SentAt     *string `json:"sent_at"`
}

type noticeQualityFlagsRequest struct {
	Items []classificationInput `json:"items"`
}

type configItemRequest struct {
	Code              string  `json:"code"`
	NameIT            string  `json:"name_it"`
	NameEN            *string `json:"name_en"`
	Description       *string `json:"description"`
	SortOrder         *int    `json:"sort_order"`
	IsActive          *bool   `json:"is_active"`
	City              *string `json:"city"`
	CountryCode       *string `json:"country_code"`
	TechnicalDomainID *int64  `json:"technical_domain_id"`
}

type configReorderRequest struct {
	Items []configReorderItem `json:"items"`
}

type configReorderItem struct {
	ID        int64 `json:"id"`
	SortOrder int   `json:"sort_order"`
}
