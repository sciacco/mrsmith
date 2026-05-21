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

export interface ImportWarning {
  sheet: string;
  row: number;
  code: string;
  message: string;
}

export interface ImportSheet {
  name: string;
  rows: number;
}

export interface ImportRow {
  sheet: string;
  row: number;
  employeeName?: string;
  employeeEmail?: string;
  courseTitle?: string;
  year?: number;
  status: string;
}

export interface ImportDryRunResponse {
  ok: boolean;
  dryRun: boolean;
  fileName: string;
  sheets: ImportSheet[];
  summary: {
    parsedRows: number;
    candidateRows: number;
    skippedRows: number;
    ambiguousRows: number;
    createdEnrollments?: number;
    updatedEnrollments?: number;
  };
  warnings: ImportWarning[];
  rows: ImportRow[];
}

export interface JobRunResponse {
  ok: boolean;
  expiredEnrollments: number;
  complianceNotifications: number;
  certificationNotifications: number;
}

export interface HRSyncResponse {
  ok: boolean;
  created: number;
  updated: number;
  skipped: number;
}
