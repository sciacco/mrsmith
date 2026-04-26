export interface PagedResponse<T> {
  items: T[];
  page: number;
  page_size: number;
  total: number;
}

export type JsonValue =
  | string
  | number
  | boolean
  | null
  | JsonValue[]
  | { [key: string]: JsonValue };

export type JsonObject = { [key: string]: JsonValue };

export interface ReferenceItem {
  id: number;
  code: string;
  name_it: string;
  name_en?: string | null;
  description?: string | null;
  sort_order: number;
  is_active: boolean;
  city?: string | null;
  country_code?: string | null;
  technical_domain_id?: number | null;
  technical_domain_name?: string | null;
  target_type_id?: number | null;
  target_type_name?: string | null;
  audience?: string | null;
  /** Solo per sites: 'global' (anagrafica condivisa) o 'scoped' (ad-hoc per una manutenzione). */
  scope?: string | null;
  /** Solo per sites scoped: id della manutenzione proprietaria. */
  owner_maintenance_id?: number | null;
}

export interface AdhocSiteInput {
  name: string;
  city?: string | null;
  country_code?: string | null;
  code?: string | null;
}

export interface ReferenceData {
  sites: ReferenceItem[];
  technical_domains: ReferenceItem[];
  maintenance_kinds: ReferenceItem[];
  customer_scopes: ReferenceItem[];
  service_taxonomy: ReferenceItem[];
  reason_classes: ReferenceItem[];
  impact_effects: ReferenceItem[];
  quality_flags: ReferenceItem[];
  target_types: ReferenceItem[];
  notice_channels: ReferenceItem[];
}

export interface StatusCount {
  status: string;
  count: number;
}

export interface WindowSummary {
  maintenance_window_id: number;
  seq_no: number;
  window_status: string;
  scheduled_start_at: string;
  scheduled_end_at: string;
  expected_downtime_minutes?: number | null;
}

export interface MaintenanceListItem {
  maintenance_id: number;
  code: string;
  title_it: string;
  title_en?: string | null;
  status: string;
  maintenance_kind: ReferenceItem;
  technical_domain: ReferenceItem;
  customer_scope: ReferenceItem | null;
  site?: ReferenceItem | null;
  current_window?: WindowSummary | null;
  primary_impact_label?: string | null;
  notice_statuses: StatusCount[];
  created_at: string;
  updated_at: string;
}

export interface MaintenanceRadarResponse {
  items: MaintenanceListItem[];
  today: string;
  next_7_days_to: string;
  next_45_days_from: string;
  next_45_days_to: string;
  six_months_to: string;
}

export interface MaintenanceWindow extends WindowSummary {
  maintenance_id: number;
  actual_start_at?: string | null;
  actual_end_at?: string | null;
  actual_downtime_minutes?: number | null;
  cancellation_reason_it?: string | null;
  cancellation_reason_en?: string | null;
  announced_at?: string | null;
  last_notice_at?: string | null;
  created_at: string;
}

export interface ClassificationItem {
  id: number;
  maintenance_id: number;
  reference: ReferenceItem;
  source: string;
  confidence?: number | null;
  is_primary: boolean;
  role?: 'operated' | 'dependent' | null;
  expected_severity?: SeverityValue | null;
  expected_audience?: AudienceOverride | null;
}

export interface MaintenanceTarget {
  maintenance_target_id: number;
  maintenance_id: number;
  target_type: ReferenceItem;
  service_taxonomy_id?: number | null;
  service_taxonomy?: ReferenceItem | null;
  reference_table?: string | null;
  reference_id?: number | null;
  external_key?: string | null;
  display_name: string;
  source: string;
  confidence?: number | null;
  is_primary: boolean;
}

export interface ImpactedCustomer {
  maintenance_impacted_customer_id: number;
  maintenance_id: number;
  customer_id: number;
  customer_name: string;
  order_id?: number | null;
  service_id?: number | null;
  impact_scope: string;
  derivation_source: string;
  confidence?: number | null;
  reason?: string | null;
  created_at: string;
}

export interface NoticeLocale {
  notice_locale_id: number;
  notice_id: number;
  locale: 'it' | 'en';
  subject: string;
  body_html?: string | null;
  body_text?: string | null;
}

export interface NoticeQualityFlag {
  id: number;
  notice_id: number;
  reference: ReferenceItem;
  source: string;
  confidence?: number | null;
}

export interface Notice {
  notice_id: number;
  maintenance_id: number;
  maintenance_window_id?: number | null;
  notice_type: string;
  audience: string;
  notice_channel: ReferenceItem;
  template_code?: string | null;
  template_version?: number | null;
  generation_source: string;
  send_status: string;
  scheduled_send_at?: string | null;
  sent_at?: string | null;
  locales: NoticeLocale[];
  quality_flags: NoticeQualityFlag[];
  created_at: string;
}

export interface MaintenanceEvent {
  maintenance_event_id: number;
  maintenance_id: number;
  maintenance_window_id?: number | null;
  event_type: string;
  actor_type: string;
  event_at: string;
  summary?: string | null;
}

export interface MaintenanceDetail {
  maintenance_id: number;
  code: string;
  title_it: string;
  title_en?: string | null;
  description_it?: string | null;
  description_en?: string | null;
  status: string;
  maintenance_kind: ReferenceItem;
  technical_domain: ReferenceItem;
  customer_scope: ReferenceItem | null;
  site?: ReferenceItem | null;
  reason_it?: string | null;
  reason_en?: string | null;
  residual_service_it?: string | null;
  residual_service_en?: string | null;
  current_window?: WindowSummary | null;
  windows: MaintenanceWindow[];
  service_taxonomy: ClassificationItem[];
  reason_classes: ClassificationItem[];
  impact_effects: ClassificationItem[];
  quality_flags: ClassificationItem[];
  targets: MaintenanceTarget[];
  impacted_customers: ImpactedCustomer[];
  notices: Notice[];
  events: MaintenanceEvent[];
  created_at: string;
  updated_at: string;
  metadata?: JsonObject;
}

export interface MaintenanceFilters {
  q?: string;
  status?: string[];
  scheduled_from?: string;
  scheduled_to?: string;
  technical_domain_id?: string;
  maintenance_kind_id?: string;
  customer_scope_id?: string;
  site_id?: string;
  page?: number;
  page_size?: number;
}

export interface WindowBody {
  scheduled_start_at: string;
  scheduled_end_at: string;
  expected_downtime_minutes?: number | null;
  actual_start_at?: string | null;
  actual_end_at?: string | null;
  actual_downtime_minutes?: number | null;
  cancellation_reason_it?: string | null;
  cancellation_reason_en?: string | null;
}

export interface MaintenanceFormBody {
  title_it: string;
  title_en?: string | null;
  description_it?: string | null;
  description_en?: string | null;
  maintenance_kind_id: number;
  technical_domain_id: number;
  customer_scope_id: number | null;
  site_id?: number | null;
  adhoc_site?: AdhocSiteInput | null;
  reason_it?: string | null;
  reason_en?: string | null;
  residual_service_it?: string | null;
  residual_service_en?: string | null;
  first_window?: WindowBody | null;
  initial_targets?: TargetBody[];
  initial_service_taxonomy?: ClassificationInput[];
  initial_reason_classes?: ClassificationInput[];
  initial_impact_effects?: ClassificationInput[];
  initial_quality_flags?: ClassificationInput[];
  metadata?: JsonObject;
}

export interface MaintenancePatchBody extends Partial<MaintenanceFormBody> {
  clear_site?: boolean;
  clear_customer_scope?: boolean;
}

export interface ClassificationInput {
  reference_id: number;
  service_taxonomy_id?: number;
  source?: string;
  confidence?: number | null;
  is_primary?: boolean;
  role?: 'operated' | 'dependent';
  expected_severity?: SeverityValue;
  expected_audience?: AudienceOverride | null;
  metadata?: JsonObject | null;
}

export interface TargetBody {
  target_type_id: number;
  service_taxonomy_id?: number | null;
  reference_table?: string | null;
  reference_id?: number | null;
  external_key?: string | null;
  display_name: string;
  source?: string;
  confidence?: number | null;
  is_primary?: boolean;
}

export interface ImpactedCustomerBody {
  customer_id: number;
  order_id?: number | null;
  service_id?: number | null;
  impact_scope: string;
  derivation_source: string;
  confidence?: number | null;
  reason?: string | null;
}

export interface MaintenanceAssistanceDraftBody {
  regenerate?: boolean;
  note?: string | null;
}

export interface AssistanceTextProposal {
  title_it?: string | null;
  title_en?: string | null;
  description_it?: string | null;
  description_en?: string | null;
  reason_en?: string | null;
  residual_service_en?: string | null;
}

export interface AssistanceClassificationProposal {
  reference_id: number;
  label: string;
  source: string;
  confidence?: number | null;
  is_primary: boolean;
  rationale?: string | null;
}

export interface AssistanceAudit {
  generated_at: string;
  model: string;
  summary: string;
}

export interface AssistanceUsage {
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
}

export interface MaintenanceAssistanceDraft {
  texts: AssistanceTextProposal;
  service_taxonomy: AssistanceClassificationProposal[];
  reason_classes: AssistanceClassificationProposal[];
  impact_effects: AssistanceClassificationProposal[];
  quality_flags: AssistanceClassificationProposal[];
  audit: AssistanceAudit;
  usage: AssistanceUsage;
}

export interface LLMModel {
  scope: string;
  model: string;
}

export interface NoticeBody {
  maintenance_window_id?: number | null;
  notice_type: string;
  audience: string;
  notice_channel_id: number;
  generation_source?: string;
  send_status?: string;
  scheduled_send_at?: string | null;
  sent_at?: string | null;
  locales?: Array<{
    locale: 'it' | 'en';
    subject: string;
    body_text?: string | null;
  }>;
}

export interface CustomerSearchItem {
  id: number;
  name: string;
}

export type SeverityValue = 'none' | 'degraded' | 'unavailable';
export type AudienceOverride = 'internal' | 'external' | 'both';

export interface ServiceDependency {
  service_dependency_id: number;
  upstream_service_id: number;
  upstream_service: ReferenceItem;
  downstream_service_id: number;
  downstream_service: ReferenceItem;
  dependency_type: 'runs_on' | 'connects_through' | 'consumes' | 'depends_on';
  is_redundant: boolean;
  default_severity: SeverityValue;
  source: string;
  is_active: boolean;
  metadata?: JsonObject;
  created_at: string;
  updated_at: string;
}

export interface ServiceDependencyBody {
  upstream_service_id: number;
  downstream_service_id: number;
  dependency_type: ServiceDependency['dependency_type'];
  is_redundant: boolean;
  default_severity: SeverityValue;
  metadata?: JsonObject | null;
}
