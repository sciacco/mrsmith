export interface Principal {
  subject: string;
  email: string;
  name: string;
  roles: string[];
  isPeopleAdmin: boolean;
}

export interface Employee {
  id: string;
  firstName: string;
  lastName: string;
  email: string;
  status: string;
}

export interface MeResponse {
  principal: Principal;
  employee: Employee | null;
  onboardingPending: boolean;
}

export interface PlanEnrollment {
  id: string;
  employeeName: string;
  employeeEmail: string;
  teamCode?: string;
  teamName?: string;
  courseTitle: string;
  vendorName?: string;
  skillAreaName?: string;
  status: string;
  year: number;
  priority?: number;
  levelAsIs?: number;
  levelToBe?: number;
  plannedStart?: string;
  plannedEnd?: string;
  hoursPlanned?: number;
  costPlanned?: number;
  motivation?: string;
  objective?: string;
  notes?: string;
  documentId?: string;
  documentFilename?: string;
  documentValidated: boolean;
  complianceRelated: boolean;
  complianceFramework?: string;
  requiredByRule: boolean;
  mandatoryRuleId?: string;
  mandatoryRuleName?: string;
}

export interface TrainingRequest {
  id: string;
  employeeName: string;
  employeeEmail: string;
  courseId?: string;
  courseTitle?: string;
  freeTextTitle?: string;
  skillAreaName?: string;
  motivation: string;
  desiredYear?: number;
  status: string;
  createdAt: string;
}

export interface CatalogCourse {
  id: string;
  title: string;
  vendorId?: string;
  vendorName?: string;
  skillAreaId?: string;
  skillAreaName?: string;
  leadsToCertId?: string;
  certificationName?: string;
  deliveryMode: string;
  providerKind: string;
  defaultHours?: number;
  defaultCost?: number;
  courseUrl?: string;
  description?: string;
  complianceRelated: boolean;
  recurrenceMonths?: number;
  complianceFramework?: string;
  active: boolean;
}

export interface CertificationRow {
  awardId: string;
  employeeName: string;
  employeeEmail?: string;
  certificationCode: string;
  certificationName: string;
  outcome: string;
  awardedOn: string;
  expiresOn?: string;
  currentStatus: string;
  validationSource: string;
  documentId?: string;
  documentFilename?: string;
  documentValidated: boolean;
}

export interface PlanBudgetRow {
  year: number;
  teamCode?: string;
  enrollmentsCount: number;
  costTotal?: number;
  hoursTotal?: number;
}

export interface ExpiringCertificationRow {
  employeeName: string;
  employeeEmail: string;
  certificationCode: string;
  certificationName: string;
  expiresOn: string;
  daysToExpiry: number;
}

export interface ComplianceGapRow {
  employeeName: string;
  courseTitle: string;
  complianceFramework?: string;
  lastValidAwardedOn?: string;
  complianceStatus: string;
}

export interface CatalogMasterData {
  vendors: VendorRow[];
  teams: TeamRow[];
  skillAreas: SkillAreaRow[];
  certifications: CatalogCertificationRow[];
  plans: TrainingPlanRow[];
  mandatoryRules: MandatoryRuleRow[];
}

export interface VendorRow {
  id: string;
  name: string;
  website?: string;
  notes?: string;
  active: boolean;
}

export interface TeamRow {
  id: string;
  code: string;
  name: string;
  description?: string;
  active: boolean;
}

export interface SkillAreaRow {
  id: string;
  code: string;
  name: string;
  parentId?: string;
  parentLabel?: string;
  description?: string;
  active: boolean;
}

export interface CatalogCertificationRow {
  id: string;
  code: string;
  name: string;
  issuerVendorId?: string;
  issuerVendorName?: string;
  skillAreaId?: string;
  skillAreaLabel?: string;
  typicalValidityMonths?: number;
  description?: string;
  active: boolean;
}

export interface TrainingPlanRow {
  id: string;
  year: number;
  status: string;
  budgetTotal?: number;
  notes?: string;
}

export interface MandatoryRuleRow {
  id: string;
  courseId: string;
  courseTitle: string;
  teamId?: string;
  teamLabel?: string;
  roleFilter?: string;
  notes?: string;
  active: boolean;
}

export interface WorkspaceResponse {
  me: MeResponse;
  plan: PlanEnrollment[];
  requests: TrainingRequest[];
  catalog: CatalogCourse[];
  certifications: CertificationRow[];
  planBudget: PlanBudgetRow[];
  expiringCertifications: ExpiringCertificationRow[];
  mandatoryComplianceGaps: ComplianceGapRow[];
  masterData?: CatalogMasterData;
}

export interface LookupItem {
  id: string;
  label: string;
  active: boolean;
  complianceRelated?: boolean;
  complianceFramework?: string;
}

export interface LookupResponse {
  employees: LookupItem[];
  teams: LookupItem[];
  vendors: LookupItem[];
  skillAreas: LookupItem[];
  courses: LookupItem[];
  certifications: LookupItem[];
  plans: LookupItem[];
}

export interface ActionResponse {
  ok: boolean;
  id?: string;
  status?: string;
}

export type BulkTargetState = 'approved' | 'in_progress' | 'completed' | 'cancelled';

export interface BulkTransitionFailure {
  enrollment_id: string;
  code?: string;
  message?: string;
}

export interface BulkTransitionResponse {
  succeeded: number;
  failed: number;
  failures?: BulkTransitionFailure[];
}

export type PersonFlagKey =
  | 'da_pianificare'
  | 'compliance_gap'
  | 'scadenze_imminenti'
  | 'failed_recente'
  | 'senza_formazione_attiva';

export interface PersonFlags {
  da_pianificare: boolean;
  compliance_gap: boolean;
  scadenze_imminenti: boolean;
  failed_recente: boolean;
  senza_formazione_attiva: boolean;
}

export interface PersonNextDeadline {
  type: 'cert' | 'course_end' | 'mandatory_due';
  date: string;
  label: string;
}

export interface PersonSummary {
  id: string;
  name: string;
  email: string;
  team_code: string;
  team_name?: string;
  flags: PersonFlags;
  active_enrollments_count: number;
  next_deadline: PersonNextDeadline | null;
  priority_score: number;
  gaps_open: number;
  expiring_certs_count: number;
  historical_enrollments: number;
}

export interface BulkAssignResponse {
  created: number;
  failed: number;
  failures?: Array<{ employee_id: string; code?: string; message?: string }>;
}

export interface PersonComplianceMandatoryRule {
  course_id: string;
  course_title: string;
  compliance_framework?: string;
  status: string;
  last_valid_awarded_on?: string;
}

export interface PersonComplianceSection {
  mandatory_rules: PersonComplianceMandatoryRule[];
  coverage_pct: number;
  open_gaps: PersonComplianceMandatoryRule[];
  expiring_certs: ExpiringCertificationRow[];
}

export interface PersonHistoryYearRow {
  year: number;
  completed_count: number;
  failed_count: number;
  hours_total: number;
  cost_total: number;
}

export interface PersonSkillEvidence {
  courses_completed: string[];
  certs: string[];
}

export interface PersonSkillArea {
  skill_area_id: string;
  name: string;
  derived_level: string;
  evidence: PersonSkillEvidence;
}

export interface PersonGap {
  type: string;
  description: string;
}

export interface PersonSuggestion {
  gap: PersonGap;
  recommended_courses: CatalogCourse[];
}

export interface OverviewException {
  id: string;
  severity: 'critical' | 'warning' | 'info';
  title: string;
  drilldown_url: string;
}

export interface OverviewTrend {
  vs_previous_year?: string;
  vs_target?: string | null;
}

export interface OverviewFamily {
  value: string;
  trend: OverviewTrend;
  exceptions: OverviewException[];
  spent_pct?: number;
  calendar_alignment?: 'in_linea' | 'in_ritardo' | 'in_anticipo';
  min_courses_per_person?: number;
  max_courses_per_person?: number;
}

export interface OverviewResponse {
  year: number;
  team_scope: string;
  esecuzione: OverviewFamily;
  compliance: OverviewFamily;
  budget: OverviewFamily;
  engagement: OverviewFamily;
}

export interface PersonProfile {
  identity_min: {
    id: string;
    name: string;
    first_name: string;
    last_name: string;
    email: string;
    status: PersonStatus;
    team_id?: string;
    team_name?: string;
    team_code: string;
    notes?: string;
  };
  compliance: PersonComplianceSection;
  enrollments_current_year: PlanEnrollment[];
  certifications: CertificationRow[];
  history_by_year: PersonHistoryYearRow[];
  skill_areas: PersonSkillArea[];
  suggestions: PersonSuggestion[];
}

export type PersonStatus = 'active' | 'on_leave' | 'terminated';

export interface PersonUpdateInput {
  firstName: string;
  lastName: string;
  email: string;
  status: PersonStatus;
  teamId: string | null;
  notes?: string;
}

export interface PersonCreateInput {
  firstName: string;
  lastName: string;
  email: string;
  status: PersonStatus;
  teamId: string | null;
  notes?: string;
}

export interface JobRunResponse {
  ok: boolean;
  expiredEnrollments: number;
  complianceNotifications: number;
  certificationNotifications: number;
}

export type SuggestionSeverity = 'critical' | 'warning' | 'info';
export type SuggestionOrigin = 'compliance' | 'expiring' | 'skill_gap' | 'employee_request';

export interface PlanningSuggestion {
  id: string;
  severity: SuggestionSeverity;
  origin: SuggestionOrigin;
  title: string;
  description?: string;
  affected_count: number;
  affected_employee_ids: string[];
  suggested_course_id?: string;
  suggested_course_name?: string;
  suggested_course_hours?: number;
  suggested_course_cost?: number;
  alternative_course_ids?: string[];
  estimated_cost: number;
  dismissed: boolean;
  rule_id?: string;
  source_custom_group_id?: string;
}

export type PlanStatus = 'draft' | 'open' | 'frozen' | 'closed' | 'missing';

export interface PlanningSummary {
  plan_id: string;
  year: number;
  status: PlanStatus;
  budget_total: number;
  budget_spent: number;
  budget_residual: number;
  budget_pct: number;
  calendar_alignment: 'in_linea' | 'in_ritardo' | 'in_anticipo';
  enrollments_planned: number;
  has_prev_year_plan: boolean;
  notes?: string;
}

export interface PlanningResponse {
  year: number;
  team_scope: string;
  plan: PlanningSummary | null;
  suggestions: PlanningSuggestion[];
}

export interface CreatePlanInput {
  year: number;
  budget_total?: number;
  duplicate_from?: number;
}

export interface UpdatePlanInput {
  budget_total?: number;
  notes?: string;
}

export interface UpdatePlanResponse {
  ok: boolean;
  plan_id: string;
  warnings?: string[];
}

export interface TrainingPlanListRow {
  id: string;
  year: number;
  status: Exclude<PlanStatus, 'missing'>;
  budget_total: number;
  created_at: string;
}

export interface TrainingPlansResponse {
  plans: TrainingPlanListRow[];
}

export type PlanTransition = 'open' | 'closed' | 'reopened' | 'frozen';

export interface TransitionPlanResponse {
  ok: boolean;
  plan_id: string;
  status: string;
  expired_enrollments_count?: number;
}

export interface BulkPlanFromSuggestionInput {
  suggestion_id: string | null;
  employee_ids: string[];
  course_id: string;
  plan_params: {
    year: number;
    planned_start?: string;
    planned_end?: string;
    hours_planned?: number;
    cost_planned?: number;
  };
  mandatory_rule_id?: string;
  source_custom_group_id?: string;
}

export interface BulkReviewEmployeeRequestsInput {
  request_ids: string[];
  target: 'approved' | 'rejected';
  motivation?: string;
  course_id?: string;
  year?: number;
}

export interface BulkReviewEmployeeRequestsResponse {
  succeeded: number;
  failed: number;
  failures?: Array<{ employee_id: string; code?: string; message?: string }>;
}

export interface ComplianceExpiringRow {
  employee_id: string;
  employee_name: string;
  rule_id: string;
  rule_title: string;
  expires_in_days: number;
  severity: 'critical' | 'warning' | 'info';
}

export interface ComplianceRuleGap {
  employee_id: string;
  employee_name: string;
  status: 'never_covered' | 'expired' | 'expiring_soon';
  detail?: string;
}

export interface ComplianceRule {
  id: string;
  title: string;
  cadence_label?: string;
  population_target?: string;
  coverage_pct: number;
  covered_count: number;
  target_count: number;
  gaps: ComplianceRuleGap[];
  severity: 'critical' | 'warning' | 'ok';
  suggested_course_ids: string[];
}

export interface ComplianceOverviewResponse {
  year: number;
  team_scope: string;
  deadline_days: number;
  expiring_deadlines: ComplianceExpiringRow[];
  rules: ComplianceRule[];
}

export type PopulationKind = 'all' | 'team' | 'skill_area' | 'custom_group';

export interface PopulationTarget {
  kind: PopulationKind;
  id?: string;
  label?: string;
  count?: number;
}

export interface RuleUsage {
  kind: 'enrollment' | 'rule';
  id?: string;
  label: string;
  count?: number;
}

export interface RuleImpact {
  target_count: number;
  covered_count: number;
  gap_count: number;
  coverage_pct: number;
  severity: 'critical' | 'warning' | 'ok';
  gaps?: ComplianceRuleGap[];
}

export interface MandatoryRule {
  id: string;
  name: string;
  course_id: string;
  course_title: string;
  compliance_framework?: string;
  cadence_label?: string;
  population_target: PopulationTarget;
  active: boolean;
  notes?: string;
  coverage_pct: number;
  covered_count: number;
  target_count: number;
  gap_count: number;
  gaps?: ComplianceRuleGap[];
  severity: 'critical' | 'warning' | 'ok';
  used_by?: RuleUsage[];
  created_at?: string;
  updated_at?: string;
}

export interface MandatoryRuleInput {
  name: string;
  course_id: string;
  population_target: PopulationTarget;
  active?: boolean;
  notes?: string;
}

export interface MandatoryRulesResponse {
  rules: MandatoryRule[];
}

export interface MandatoryRuleMutationResponse {
  rule: MandatoryRule;
  warnings?: string[];
  impact: RuleImpact;
}

export interface GroupMember {
  id: string;
  name: string;
  email: string;
  team_code?: string;
  team_name?: string;
}

export interface CustomGroupUsage {
  kind: 'rule' | 'enrollment';
  id?: string;
  label: string;
  count?: number;
}

export interface CustomGroup {
  id: string;
  name: string;
  description?: string;
  active: boolean;
  member_count: number;
  members?: GroupMember[];
  used_by?: CustomGroupUsage[];
  created_at?: string;
  updated_at?: string;
}

export interface CustomGroupInput {
  name: string;
  description?: string;
  active?: boolean;
  member_ids: string[];
}

export interface CustomGroupsResponse {
  groups: CustomGroup[];
}

export interface CatalogCourseWithCounts extends CatalogCourse {
  enrollments_current_year: number;
  enrollments_completed_historical: number;
}

export interface CatalogListResponse {
  courses: CatalogCourseWithCounts[];
}

export interface PlanAuditActor {
  id: string;
  display_name: string;
}

export interface PlanAuditEvent {
  id: number;
  plan_id: string;
  event_type:
    | 'plan_created'
    | 'plan_status_changed'
    | 'plan_budget_changed'
    | 'plan_notes_changed'
    | 'plan_deleted'
    | 'bulk_plan_applied'
    | 'suggestion_dismissed'
    | 'adhoc_created'
    | 'enrollment_modified'
    | 'enrollment_cancelled'
    | 'bulk_review_applied';
  actor: PlanAuditActor;
  payload: Record<string, unknown>;
  created_at: string;
}

export interface PlanAuditResponse {
  events: PlanAuditEvent[];
  next_cursor?: string;
}
