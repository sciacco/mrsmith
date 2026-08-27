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

// ── Eventi: dettaglio e mutazioni (#156) ──

export interface ActionResponse {
  ok: boolean;
  id?: string;
  status?: string;
}

export interface EventInput {
  courseId: string;
  vendorId?: string;
  agreedPrice?: number;
  agreedConditions?: string;
  notes?: string;
}

export interface ReasonInput {
  reason: string;
}

export type ScheduleType = 'scheduled' | 'self_paced';

// SessionInput copre creazione e modifica: sostituzione completa, il campo
// omesso azzera (capienza compresa).
export interface SessionInput {
  scheduleType: ScheduleType;
  startsAt?: string;
  endsAt?: string;
  dueAt?: string;
  maxCapacity?: number;
  notes?: string;
}

export interface SessionDetail {
  id: string;
  scheduleType?: ScheduleType;
  startsAt?: string;
  endsAt?: string;
  dueAt?: string;
  maxCapacity?: number;
  occupancy: number;
  notes?: string;
  factorialSessionId?: string;
  createdAt: string;
  updatedAt: string;
}

// EnrollmentFactsInput e l'update unico dei fatti senza effetti di stato:
// sostituzione completa, il campo omesso azzera.
export interface EnrollmentFactsInput {
  objective?: string;
  notes?: string;
  actualStart?: string;
  actualEnd?: string;
  hoursActual?: number;
  learningOutcome?: LearningOutcome | '';
}

export interface EnrollmentDetail {
  id: string;
  employeeId: string;
  employeeName: string;
  employeeEmail: string;
  deliveryStatus: DeliveryStatus;
  learningOutcome?: LearningOutcome;
  origin: string;
  objective?: string;
  notes?: string;
  actualStart?: string;
  actualEnd?: string;
  hoursActual?: number;
  cancellationReason?: string;
  createdAt: string;
  updatedAt: string;
}

export interface ParticipationInput {
  participationStatus: ParticipationStatus;
}

export interface ParticipationRow {
  enrollmentId: string;
  sessionId: string;
  participationStatus: ParticipationStatus;
  assignedAt: string;
  updatedAt: string;
}

export interface EventExpense {
  id: string;
  eventId: string;
  poId: number;
  poCode: string;
  totalPrice: string;
  currency: string;
  rawState: string;
  economicState: EconomicState;
  budget: EventExpenseBudget;
  enrollmentIds: string[];
  createdAt: string;
  updatedAt: string;
}

export interface EventExpenseInput {
  poReference: string;
  enrollmentIds: string[];
}

export interface EventExpenseReplaceInput {
  poReference: string;
}

export interface EventExpenseEnrollmentsInput {
  enrollmentIds: string[];
}

export interface EventDetail {
  id: string;
  courseId: string;
  courseTitle: string;
  vendorId?: string;
  vendorName?: string;
  agreedPrice?: number;
  agreedConditions?: string;
  origin: string;
  sourceRuleId?: string;
  sourceRequestId?: string;
  ruleDeadline?: string;
  factorialClassId?: string;
  cancelledAt?: string;
  cancellationReason?: string;
  notes?: string;
  flags: EventFlags;
  sessions: SessionDetail[];
  enrollments: EnrollmentDetail[];
  participations: ParticipationRow[];
  expenses: EventExpense[];
  createdAt: string;
  updatedAt: string;
}

export interface FeedAddedRow {
  enrollmentId: string;
  employeeId: string;
  employeeName: string;
}

export interface FeedEventResponse {
  ok: boolean;
  eventId: string;
  added: FeedAddedRow[];
}

// ── Gesti massivi del workspace (#153) ──

export type BulkAssignMode = 'all_to_all' | 'distribute' | 'fill_session';

export interface BulkAssignmentsInput {
  mode: BulkAssignMode;
  sessionIds?: string[];
  sessionId?: string;
}

export interface BulkAssignmentsResponse {
  ok: boolean;
  assigned: number;
  perSession: Record<string, number>;
}

export interface BulkParticipationInput {
  participationStatus: ParticipationStatus;
  enrollmentIds?: string[];
}

export interface BulkParticipationResponse {
  ok: boolean;
  updated: number;
}

export interface BulkEnrollInput {
  employeeIds: string[];
  objective?: string;
  notes?: string;
}

export interface BulkEnrollCreatedRow {
  enrollmentId: string;
  employeeId: string;
}

export type BulkEnrollSkipReason = 'already_enrolled' | 'inactive';

export interface BulkEnrollSkippedRow {
  employeeId: string;
  reason: BulkEnrollSkipReason;
}

export interface BulkEnrollResponse {
  ok: boolean;
  created: BulkEnrollCreatedRow[];
  skipped: BulkEnrollSkippedRow[];
}

// ── Letture di dominio: catalogo, persone, team, aree, gruppi (#157) ──
// Campi consumati dalle superfici richieste/regole di questa slice, più i
// campi catalogo (skillAreaId/vendorId/providerKind su CourseListRow,
// customGroupId su SkillAreaListRow) attesi dal collegamento in catalogo
// della slice 6.7: le liste gestionali del backend (#152) portano altri
// campi non ripresi qui.

export interface CourseListRow {
  id: string;
  title: string;
  skillAreaId?: string;
  vendorId?: string;
  providerKind: string;
  leadsToCertId?: string;
  active: boolean;
}

export interface CourseListResponse {
  courses: CourseListRow[];
}

// CourseInput copre solo la creazione contestuale del corso fuori catalogo
// dal pannello di accoglimento (#157): deliveryMode e gli altri campi
// opzionali del corso restano ai valori di default del backend.
export interface CourseInput {
  title: string;
  vendorId?: string;
  skillAreaId?: string;
  providerKind?: 'internal' | 'external';
}

export interface PersonTeamRef {
  id: string;
  name: string;
}

export interface PersonListRow {
  id: string;
  firstName: string;
  lastName: string;
  status: string;
  teams: PersonTeamRef[];
}

export interface PersonListResponse {
  people: PersonListRow[];
}

export interface TeamLeadRef {
  employeeId: string;
  name: string;
}

export interface TeamListRow {
  id: string;
  name: string;
  leads: TeamLeadRef[];
}

export interface TeamListResponse {
  teams: TeamListRow[];
}

export interface SkillAreaListRow {
  id: string;
  name: string;
  customGroupId?: string;
}

export interface SkillAreaListResponse {
  skillAreas: SkillAreaListRow[];
}

export interface GroupListRow {
  id: string;
  name: string;
}

export interface GroupListResponse {
  groups: GroupListRow[];
}

// ── Richieste formative (#140, #157) ──
// Faccia originale (create-only) e faccia accolta separate; parere TL e
// decisione People sono fatti immutabili. Vedi backend/internal/training
// types_requests.go per il contratto completo.

export interface RequestInput {
  employeeId: string;
  courseId?: string;
  freeTextTitle?: string;
  skillAreaId?: string;
  motivation: string;
  selectedTeamId: string;
  desiredStart?: string;
  desiredEnd?: string;
}

export type TLOpinionValue = 'favorable' | 'unfavorable';

export interface TLOpinionInput {
  leadEmployeeId: string;
  opinion: TLOpinionValue;
  reason: string;
}

export interface RequestAcceptedInput {
  courseId: string;
  eventId?: string;
  vendorId?: string;
  periodStart?: string;
  periodEnd?: string;
  notes?: string;
  existingEnrollmentId?: string;
}

export type RequestDecisionValue = 'accepted' | 'rejected';

export interface RequestDecisionInput {
  decision: RequestDecisionValue;
  reason: string;
  accepted?: RequestAcceptedInput;
}

export interface RequestListRow {
  id: string;
  employeeName: string;
  courseTitle?: string;
  freeTextTitle?: string;
  selectedTeamName: string;
  tlOpinion?: TLOpinionValue;
  peopleDecision?: RequestDecisionValue;
  outcome?: string;
  createdAt: string;
}

export interface RequestListResponse {
  requests: RequestListRow[];
}

export interface RequestOriginalData {
  employeeName: string;
  courseTitle?: string;
  freeTextTitle?: string;
  skillAreaName?: string;
  motivation: string;
  selectedTeamId: string;
  selectedTeamName: string;
  desiredStart?: string;
  desiredEnd?: string;
}

export interface RequestTLOpinionFacts {
  opinion: TLOpinionValue;
  byName?: string;
  at: string;
  reason?: string;
}

export interface RequestDecisionFacts {
  decision: RequestDecisionValue;
  byName?: string;
  at: string;
  reason: string;
}

export interface RequestAcceptedData {
  courseTitle: string;
  eventId?: string;
  vendorName?: string;
  periodStart?: string;
  periodEnd?: string;
  notes?: string;
}

export interface RequestCoverageEnrollment {
  enrollmentId: string;
  completedOn: string;
}

export interface RequestCoverageAward {
  certificationName: string;
  awardedOn: string;
  expiresOn?: string;
}

export interface RequestCoverage {
  courseId?: string;
  courseTitle?: string;
  completedEnrollments: RequestCoverageEnrollment[];
  validAwards: RequestCoverageAward[];
}

export interface RequestDetail {
  id: string;
  requested: RequestOriginalData;
  tlOpinion?: RequestTLOpinionFacts;
  decision?: RequestDecisionFacts;
  outcome?: string;
  closedAt?: string;
  accepted?: RequestAcceptedData;
  resultingEnrollmentId?: string;
  existingCoverage: RequestCoverage;
  createdAt: string;
}

// ── Regole formative (#140, #157) ──

export type PopulationKind = 'all' | 'team' | 'skill_area' | 'custom_group' | 'people';

export interface RulePopulationInput {
  kind: PopulationKind;
  id?: string;
  personIds?: string[];
}

export interface RuleInput {
  name: string;
  courseId: string;
  population?: RulePopulationInput;
  seatCount?: number;
  isMandatory: boolean;
  deadline: string;
  recurrenceMonths?: number;
  recurrenceAnchor?: 'calendar' | 'completion' | '';
  notes?: string;
}

export interface RuleListRow {
  id: string;
  name: string;
  courseTitle: string;
  need: QueueNeed;
  populationKind?: PopulationKind;
  seatCount?: number;
  populationSize?: number;
  coveredCount: number;
  isMandatory: boolean;
  deadline: string;
  recurrenceMonths?: number;
  recurrenceAnchor?: string;
  nextRoundDeadline?: string;
  isActive: boolean;
}

export interface RuleListResponse {
  rules: RuleListRow[];
}

export interface RulePopulationMember {
  employeeId: string;
  name: string;
  covered: boolean;
}

export interface RulePopulationDetail {
  kind: PopulationKind;
  targetId?: string;
  personIds?: string[];
  size: number;
  coveredCount: number;
  members: RulePopulationMember[];
}

export interface RuleSeatsDetail {
  requested: number;
  covered: number;
}

export interface RuleRoundRow {
  eventId: string;
  ruleDeadline?: string;
  cancelled: boolean;
  enrollmentsCount: number;
}

export interface RuleDetail {
  id: string;
  name: string;
  courseId: string;
  courseTitle: string;
  need: QueueNeed;
  isMandatory: boolean;
  deadline: string;
  recurrenceMonths?: number;
  recurrenceAnchor?: string;
  isActive: boolean;
  notes?: string;
  population?: RulePopulationDetail;
  seats?: RuleSeatsDetail;
  nextRoundDeadline?: string;
  rounds: RuleRoundRow[];
}

export interface RuleEventResponse {
  ok: boolean;
  id: string;
  enrollmentsCreated: number;
}
