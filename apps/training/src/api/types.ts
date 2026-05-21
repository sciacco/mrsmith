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
  mandatory: boolean;
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
  mandatory: boolean;
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

export type PersonComplianceStatus = 'a_norma' | 'con_gap' | 'senza_piano' | 'nuovo_assunto';

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
  compliance_status: PersonComplianceStatus;
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
  courses_per_person?: number;
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
    email: string;
    team_code: string;
  };
  compliance: PersonComplianceSection;
  enrollments_current_year: PlanEnrollment[];
  certifications: CertificationRow[];
  history_by_year: PersonHistoryYearRow[];
  skill_areas: PersonSkillArea[];
  suggestions: PersonSuggestion[];
}

export interface JobRunResponse {
  ok: boolean;
  expiredEnrollments: number;
  complianceNotifications: number;
  certificationNotifications: number;
}
