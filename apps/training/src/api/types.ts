// Vocabolario nuovo del dominio Training (#151/#155): stati di erogazione
// planned|in_progress|completed|partially_completed|not_attended|cancelled;
// stati di partecipazione assigned|in_progress|completed|not_attended. Copre
// solo gli endpoint di questa slice: /me, /lookups, le 8 code operative,
// /events. Gli altri endpoint arrivano con le slice che li usano.

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
}

// ── Code operative (#140, sola lettura) ──

export interface QueueLeadRef {
  employeeId: string;
  name: string;
  email: string;
}

export interface RequestWithoutTLOpinionRow {
  requestId: string;
  employeeId: string;
  employeeName: string;
  selectedTeamId: string;
  selectedTeamName: string;
  teamLeads: QueueLeadRef[];
  courseId?: string;
  courseTitle?: string;
  freeTextTitle?: string;
  ageDays: number;
  createdAt: string;
}

export interface RequestsWithoutTLOpinionResponse {
  requests: RequestWithoutTLOpinionRow[];
}

export type TLOpinion = 'favorable' | 'unfavorable';

export interface RequestAwaitingDecisionRow {
  requestId: string;
  employeeId: string;
  employeeName: string;
  selectedTeamId: string;
  selectedTeamName: string;
  courseId?: string;
  courseTitle?: string;
  freeTextTitle?: string;
  tlOpinion: TLOpinion;
  tlOpinionById?: string;
  tlOpinionByName?: string;
  tlOpinionAt?: string;
  tlOpinionReason?: string;
  ageDays: number;
  createdAt: string;
}

export interface RequestsAwaitingDecisionResponse {
  requests: RequestAwaitingDecisionRow[];
}

export type QueueNeed = 'attendance' | 'certification';

export interface SeatRuleInTrainingRow {
  enrollmentId: string;
  employeeId: string;
  employeeName: string;
  eventId: string;
  deliveryStatus: DeliveryStatus;
}

export interface SeatRuleCoverageRow {
  ruleId: string;
  ruleName: string;
  courseId: string;
  courseTitle: string;
  need: QueueNeed;
  certificationId?: string;
  deadline: string;
  seatCount: number;
  covered: number;
  missing: number;
  inTraining: SeatRuleInTrainingRow[];
}

export interface SeatRuleCoverageResponse {
  rules: SeatRuleCoverageRow[];
}

export type ExpiringPersonReason =
  | 'uncovered'
  | 'never_completed'
  | 'personal_deadline'
  | 'award_expiring'
  | 'award_expired'
  | 'never_awarded';

export interface ExpiringPersonRow {
  ruleId: string;
  ruleName: string;
  courseId: string;
  courseTitle: string;
  need: QueueNeed;
  employeeId: string;
  employeeName: string;
  deadline: string;
  daysUntil: number;
  reason: ExpiringPersonReason;
}

export interface ExpiringSeatRuleRow {
  ruleId: string;
  ruleName: string;
  courseId: string;
  courseTitle: string;
  certificationId: string;
  seatCount: number;
  validToday: number;
  validAtHorizon: number;
}

export interface ExpiringCoverageResponse {
  withinDays: number;
  people: ExpiringPersonRow[];
  seatRules: ExpiringSeatRuleRow[];
}

export interface QueueMemberRef {
  employeeId: string;
  name: string;
}

export interface UnfedPopulationRow {
  ruleId: string;
  ruleName: string;
  courseId: string;
  courseTitle: string;
  eventId: string;
  roundDeadline: string;
  members: QueueMemberRef[];
}

export interface UnfedPopulationResponse {
  rules: UnfedPopulationRow[];
}

export interface RoundWithoutEventRow {
  ruleId: string;
  ruleName: string;
  courseId: string;
  courseTitle: string;
  need: QueueNeed;
  recurrenceMonths?: number;
  recurrenceAnchor?: string;
  nextRoundDeadline: string;
  daysUntil: number;
  firstRound: boolean;
}

export interface RoundsWithoutEventResponse {
  withinDays: number;
  rules: RoundWithoutEventRow[];
}

export type EconomicState = 'approved' | 'pending' | 'rejected';

export interface EventExpenseBudget {
  id: number;
  name: string;
  year: number;
  costCenter: string | null;
  budgetUserId: number | null;
}

export interface UnapprovedEventExpenseRow {
  eventId: string;
  courseId: string;
  courseTitle: string;
  expenseId: string;
  poId: number;
  poCode: string;
  rawState: string;
  economicState: EconomicState;
  totalPrice: string;
  currency: string;
  budget: EventExpenseBudget;
  enrollmentCount: number;
}

export interface UnapprovedEventExpensesResponse {
  expenses: UnapprovedEventExpenseRow[];
}

export interface StaleEnrollmentRow {
  enrollmentId: string;
  employeeId: string;
  employeeName: string;
  eventId: string;
  courseTitle: string;
  ageDays: number;
  createdAt: string;
  lastSessionDate: string | null;
}

export interface StaleEnrollmentsResponse {
  olderThanDays: number;
  enrollments: StaleEnrollmentRow[];
}

// ── Eventi (letture event-centric; #155 usa solo i flag di condizione) ──

export type DeliveryStatus =
  | 'planned'
  | 'in_progress'
  | 'completed'
  | 'partially_completed'
  | 'not_attended'
  | 'cancelled';

export type ParticipationStatus = 'assigned' | 'in_progress' | 'completed' | 'not_attended';

export type LearningOutcome = 'passed' | 'failed' | 'not_taken' | 'not_required';

export interface EventFlags {
  cancelled: boolean;
  withoutSessions: boolean;
  unassignedEnrollments: boolean;
  inProgress: boolean;
  needsReconciliation: boolean;
  concluded: boolean;
}

export interface EventListRow {
  id: string;
  courseId: string;
  courseTitle: string;
  vendorId?: string;
  vendorName?: string;
  agreedPrice?: number;
  origin: string;
  cancelledAt?: string;
  sessionsCount: number;
  enrollmentsCount: number;
  cancelledEnrollmentsCount: number;
  flags: EventFlags;
  createdAt: string;
  updatedAt: string;
}

export interface EventListResponse {
  events: EventListRow[];
}
