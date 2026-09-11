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

export type TLOpinion = "favorable" | "unfavorable";

// Richiesta aperta senza decisione People (#200): con o senza team scelto e
// con o senza parere TL, presente solo quando registrato.
export interface RequestAwaitingDecisionRow {
  requestId: string;
  employeeId: string;
  employeeName: string;
  selectedTeamId?: string;
  selectedTeamName?: string;
  courseId?: string;
  courseTitle?: string;
  tlOpinion?: TLOpinion;
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

export type QueueNeed = "attendance" | "certification";

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
  | "uncovered"
  | "never_completed"
  | "personal_deadline"
  | "award_expiring"
  | "award_expired"
  | "never_awarded";

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

export type EconomicState = "approved" | "pending" | "rejected";

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
  | "planned"
  | "in_progress"
  | "completed"
  | "partially_completed"
  | "not_attended"
  | "cancelled";

export type ParticipationStatus =
  | "assigned"
  | "in_progress"
  | "completed"
  | "not_attended";

export type LearningOutcome =
  | "passed"
  | "failed"
  | "not_taken"
  | "not_required";

export interface EventFlags {
  cancelled: boolean;
  withoutSessions: boolean;
  unassignedEnrollments: boolean;
  inProgress: boolean;
  needsReconciliation: boolean;
  concluded: boolean;
}

export interface TrainerRef {
  employeeId: string;
  name: string;
}

export interface EventListRow {
  id: string;
  courseId: string;
  courseTitle: string;
  title: string;
  reminderText?: string;
  reminderAt?: string;
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
  // Alla creazione il titolo è sempre ereditato dal corso; in modifica il
  // campo vuoto conserva il titolo corrente.
  title?: string;
  reminderText?: string;
  reminderAt?: string; // YYYY-MM-DD
  trainerIds?: string[];
}

export interface ReasonInput {
  reason: string;
}

export type ScheduleType = "scheduled" | "self_paced";

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
  topic?: string;
  modality?: string;
  durationHours?: number;
  location?: string;
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
  learningOutcome?: LearningOutcome | "";
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
  completedHours?: number;
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
  title: string;
  reminderText?: string;
  reminderAt?: string;
  trainers: TrainerRef[];
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

export type BulkAssignMode = "all_to_all" | "distribute" | "fill_session";

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

export type BulkEnrollSkipReason = "already_enrolled" | "inactive";

export interface BulkEnrollSkippedRow {
  employeeId: string;
  reason: BulkEnrollSkipReason;
}

export interface BulkEnrollResponse {
  ok: boolean;
  created: BulkEnrollCreatedRow[];
  skipped: BulkEnrollSkippedRow[];
}

// ── Letture di dominio: catalogo, persone, team, aree, gruppi (#157, #158) ──
// Le interfacce coprono l'intero payload del backend (#152): le superfici
// richieste/regole della #157 continuano a leggere solo il sottoinsieme che
// usavano; persone/catalogo/anagrafiche della #158 usano il resto.

export interface SkillAreaRef {
  id: string;
  name: string;
}

export interface CourseListRow {
  id: string;
  title: string;
  skillAreas: SkillAreaRef[];
  vendorId?: string;
  vendorName?: string;
  deliveryMode: string;
  providerKind: string;
  defaultHours?: number;
  defaultCost?: number;
  leadsToCertId?: string;
  leadsToCertName?: string;
  complianceRelated: boolean;
  complianceFramework?: string;
  active: boolean;
  factorialTrainingId?: string;
  tags: string[];
  reminderText?: string;
  reminderAt?: string;
  suspendedAt?: string;
  trainers: TrainerRef[];
  updatedAt: string;
}

export interface CourseListResponse {
  courses: CourseListRow[];
}

export interface CourseRuleRef {
  id: string;
  name: string;
  isActive: boolean;
}

export interface CourseEventRef {
  id: string;
  createdAt: string;
  cancelledAt?: string;
  enrollmentsCount: number;
  sessionsCount: number;
}

export interface CourseVisibility {
  kind: "all" | "team" | "skill_area" | "custom_group" | "people";
  id?: string;
  employeeIds?: string[];
}

export interface CourseDetail extends CourseListRow {
  description?: string;
  courseUrl?: string;
  notes?: string;
  suspensionReason?: string;
  visibility?: CourseVisibility;
  rules: CourseRuleRef[];
  events: CourseEventRef[];
}

// CourseInput copre sia la creazione contestuale del corso fuori catalogo
// dal pannello di accoglimento (#157) sia l'upsert completo dal catalogo
// (#158): sostituzione integrale, i campi omessi restano ai default del
// backend in creazione o si azzerano in modifica.
export interface CourseInput {
  title: string;
  vendorId?: string;
  skillAreaIds?: string[];
  leadsToCertId?: string;
  deliveryMode?: string;
  providerKind?: "internal" | "external";
  defaultHours?: number;
  defaultCost?: number;
  courseUrl?: string;
  description?: string;
  complianceRelated?: boolean;
  complianceFramework?: string;
  tags?: string[];
  active?: boolean;
  notes?: string;
  reminderText?: string;
  reminderAt?: string; // YYYY-MM-DD
  trainerIds?: string[];
  visibility?: CourseVisibility;
}

export interface PersonTeamRef {
  id: string;
  name: string;
  role?: string;
}

export interface PersonGroupRef {
  id: string;
  name: string;
}

export interface PersonListRow {
  id: string;
  firstName: string;
  lastName: string;
  email: string;
  status: string;
  directoryExempt: boolean;
  teams: PersonTeamRef[];
  groups: PersonGroupRef[];
}

export interface PersonListResponse {
  people: PersonListRow[];
}

export interface PersonEnrollmentRef {
  enrollmentId: string;
  eventId: string;
  courseTitle: string;
  deliveryStatus: DeliveryStatus;
  learningOutcome?: LearningOutcome;
  actualStart?: string;
  actualEnd?: string;
  cancelledAt?: string;
  createdAt: string;
}

export interface PersonRequestRef {
  id: string;
  courseTitle?: string;
  outcome: string | null;
  createdAt: string;
}

export interface PersonRuleCoverageRef {
  ruleId: string;
  ruleName: string;
  need: QueueNeed;
  covered: boolean;
  deadline: string;
}

export interface PersonDetail extends PersonListRow {
  enrollments: PersonEnrollmentRef[];
  requests: PersonRequestRef[];
  ruleCoverage: PersonRuleCoverageRef[];
  awards: PersonAwardRef[];
  assessments: PersonAssessmentRef[];
  paths: PersonPathRef[];
}

// PersonCreateInput/PersonUpdateInput: sostituzione integrale come gli altri
// input di dominio. directoryExempt esiste solo in modifica — è la valvola
// che rende modificabile a mano una persona agganciata alla directory.
export interface PersonCreateInput {
  firstName: string;
  lastName: string;
  email: string;
  status: string;
  teamId?: string;
  notes?: string;
}

export interface PersonUpdateInput extends PersonCreateInput {
  directoryExempt?: boolean;
}

export interface TeamLeadRef {
  employeeId: string;
  name: string;
}

export interface TeamListRow {
  id: string;
  code: string;
  name: string;
  active: boolean;
  managedBySync: boolean;
  leads: TeamLeadRef[];
  activeMembers: number;
}

export interface TeamListResponse {
  teams: TeamListRow[];
}

// TeamInput: rinomina dei soli team non gestiti dalla sync (#158, §Catalogo
// 5) — il backend rifiuta con team_managed_by_directory ogni altro caso.
export interface TeamInput {
  code: string;
  name: string;
  description?: string;
  active?: boolean;
}

export interface VendorListRow {
  id: string;
  name: string;
  website?: string;
  notes?: string;
  active: boolean;
}

export interface VendorListResponse {
  vendors: VendorListRow[];
}

export interface VendorInput {
  name: string;
  website?: string;
  notes?: string;
  active?: boolean;
}

export interface SkillAreaListRow {
  id: string;
  code: string;
  name: string;
  description?: string;
  active: boolean;
  customGroupId?: string;
  customGroupName?: string;
  parentId?: string;
}

export interface SkillAreaListResponse {
  skillAreas: SkillAreaListRow[];
}

export interface SkillAreaInput {
  code: string;
  name: string;
  parentId?: string;
  customGroupId?: string;
  description?: string;
  active?: boolean;
}

export interface CertificationCatalogRow {
  id: string;
  code: string;
  name: string;
  description?: string;
  active: boolean;
  issuerVendorId?: string;
  issuerVendorName?: string;
  skillAreaId?: string;
  skillAreaName?: string;
  typicalValidityMonths?: number;
  attestedLevel?: number;
}

export interface CertificationCatalogResponse {
  certifications: CertificationCatalogRow[];
}

export interface CertificationInput {
  code: string;
  name: string;
  issuerVendorId?: string;
  skillAreaId?: string;
  typicalValidityMonths?: number;
  attestedLevel?: number;
  description?: string;
  active?: boolean;
}

export interface GroupMemberRef {
  employeeId: string;
  name: string;
  email: string;
}

export interface GroupListRow {
  id: string;
  name: string;
  description?: string;
  members: GroupMemberRef[];
}

export interface GroupListResponse {
  groups: GroupListRow[];
}

export interface GroupInput {
  name: string;
  description?: string;
}

export interface GroupMembersInput {
  employeeIds: string[];
}

// ── Richieste formative (#140, #157, #171) ──
// Faccia originale (modificabile finche la richiesta non e chiusa; la
// persona e invariata) e faccia accolta separate; parere TL e decisione
// People sono riscrivibili (la decisione anche a richiesta chiusa da
// decisione). Vedi backend/internal/training types_requests.go per il
// contratto completo.

export interface RequestSkillAreaInput {
  id: string;
  levelCurrent?: number;
  levelTarget?: number;
}

export interface RequestInput {
  employeeId: string;
  courseId?: string;
  newCourseTitle?: string;
  skillAreas?: RequestSkillAreaInput[];
  motivation: string;
  selectedTeamId?: string;
  desiredStart?: string;
  desiredEnd?: string;
  priority?: number;
  notes?: string;
  reminderText?: string;
  reminderAt?: string; // YYYY-MM-DD
}

export interface RequestAnnotationsInput {
  notes?: string;
  reminderText?: string;
  reminderAt?: string; // YYYY-MM-DD
  priority?: number;
}

export type TLOpinionValue = "favorable" | "unfavorable";

export interface TLOpinionInput {
  leadEmployeeId: string;
  opinion: TLOpinionValue;
  reason?: string;
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

export type RequestDecisionValue = "accepted" | "rejected";

export interface RequestDecisionInput {
  decision: RequestDecisionValue;
  reason?: string;
  accepted?: RequestAcceptedInput;
}

// RequestOriginalDataInput sostituisce i dati originali di una richiesta
// aperta (#171): corso a catalogo o titolo nuovo (alternativi), aree con
// livelli, motivazione, team e date desiderate. La persona e invariata.
// Il team e obbligatorio solo quando la persona ha appartenenze attive;
// selectedTeamId omesso = nessun team (#200).
export interface RequestOriginalDataInput {
  courseId?: string;
  newCourseTitle?: string;
  skillAreas?: RequestSkillAreaInput[];
  motivation: string;
  selectedTeamId?: string;
  desiredStart?: string;
  desiredEnd?: string;
}

export interface RequestListRow {
  id: string;
  employeeName: string;
  courseTitle?: string;
  selectedTeamName?: string;
  priority?: number;
  reminderText?: string;
  reminderAt?: string;
  suspendedAt?: string;
  tlOpinion?: TLOpinionValue;
  peopleDecision?: RequestDecisionValue;
  outcome?: string;
  createdAt: string;
}

export interface RequestListResponse {
  requests: RequestListRow[];
}

export interface RequestAreaRef {
  id: string;
  name: string;
  levelCurrent?: number;
  levelTarget?: number;
}

export interface RequestOriginalData {
  employeeId: string;
  courseId?: string;
  employeeName: string;
  courseTitle?: string;
  skillAreas: RequestAreaRef[];
  motivation: string;
  // Team assente quando la persona non ha appartenenze attive (#200):
  // entrambi i campi sono omessi dal JSON, non null.
  selectedTeamId?: string;
  selectedTeamName?: string;
  desiredStart?: string;
  desiredEnd?: string;
}

export interface RequestTLOpinionFacts {
  opinion: TLOpinionValue;
  byEmployeeId?: string;
  byName?: string;
  at: string;
  reason?: string;
}

export interface RequestDecisionFacts {
  decision: RequestDecisionValue;
  byEmployeeId?: string;
  byName?: string;
  at: string;
  reason?: string;
}

export interface RequestAcceptedData {
  courseId: string;
  courseTitle: string;
  eventId?: string;
  vendorId?: string;
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
  priority?: number;
  notes?: string;
  reminderText?: string;
  reminderAt?: string;
  suspendedAt?: string;
  suspendedByName?: string;
  suspensionReason?: string;
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

export type PopulationKind =
  | "all"
  | "team"
  | "skill_area"
  | "custom_group"
  | "people";

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
  recurrenceAnchor?: "calendar" | "completion" | "";
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

// ── Conseguimenti e documenti (#162, slice 3 del task 7): specchio dei tipi
// Go di backend/internal/training/types.go, types_reads.go — campi verbatim.

export interface PersonAwardDocumentRef {
  id: string;
  filename: string;
  isValidated: boolean;
}

export interface PersonAwardRef {
  awardId: string;
  certificationId: string;
  certificationCode: string;
  certificationName: string;
  outcome: string;
  awardedOn: string;
  expiresOn?: string;
  currentStatus: string;
  validationSource: string;
  enrollmentId: string | null;
  document: PersonAwardDocumentRef | null;
}

// AwardInput copre sia la creazione da scheda persona (employeeId
// precompilato) sia, in prospettiva, altri punti di ingresso: employeeId
// resta nel tipo perche il backend lo accetta sempre in JSON.
export interface AwardInput {
  employeeId: string;
  certificationId: string;
  enrollmentId?: string;
  outcome: string;
  awardedOn: string;
  expiresOn?: string;
  validationSource?: string;
  externalCredentialId?: string;
  externalCredentialUrl?: string;
  notes?: string;
  reason?: string;
}

// AwardUpdateInput: PUT di correzione. notes e opzionale: se omesso (undefined)
// il backend (store_documents.go, NULLIF su $6/$7) lascia la nota esistente
// invariata — il valore esistente non e leggibile qui, quindi il client invia
// notes solo quando l'operatore scrive qualcosa di nuovo.
export interface AwardUpdateInput {
  outcome: string;
  awardedOn: string;
  expiresOn?: string;
  validationSource?: string;
  notes?: string;
}

export interface DocumentMetadata {
  id: string;
  enrollmentId?: string;
  certificationAwardId?: string;
  filename: string;
  sha256: string;
  mime: string;
  sizeBytes: number;
  uploadedAt: string;
  validated: boolean;
}

// ── Valutazioni di competenza (#162, specchio di types_assessments.go) ──

export interface AssessmentInput {
  skillAreaId: string;
  level: number;
  assessedOn?: string;
  source?: string;
  notes?: string;
}

export interface AssessmentUpdateInput {
  level: number;
  assessedOn: string;
  source?: string;
  notes?: string;
}

export interface PersonAssessmentRef {
  id: string;
  skillAreaId: string;
  skillAreaName: string;
  level: number;
  assessedOn: string;
  source: string;
  notes?: string;
}

// ── Percorsi formativi (#162, specchio di types_paths.go) ──

export interface PathInput {
  code: string;
  name: string;
  skillAreaId?: string;
  description?: string;
  active?: boolean;
}

export interface PathListRow {
  id: string;
  code: string;
  name: string;
  skillAreaId?: string;
  skillAreaName?: string;
  description?: string;
  active: boolean;
  stepsCount: number;
  assigneesCount: number;
}

export interface PathListResponse {
  paths: PathListRow[];
}

export interface PathStepRef {
  stepId: string;
  stepOrder: number;
  courseId?: string;
  courseTitle?: string;
  certificationId?: string;
  certificationName?: string;
  isRequired: boolean;
  notes?: string;
}

export interface PathStepInput {
  stepOrder: number;
  courseId?: string;
  certificationId?: string;
  isRequired?: boolean;
  notes?: string;
}

export interface PathStepsInput {
  steps: PathStepInput[];
}

export interface PathStepProgressRef {
  stepId: string;
  stepOrder: number;
  courseId?: string;
  courseTitle?: string;
  certificationId?: string;
  certificationName?: string;
  isRequired: boolean;
  covered: boolean;
  enrollmentId?: string;
  eventId?: string;
  awardId?: string;
}

// PathProgress e condiviso (embedding Go) da PathAssigneeRef e
// PersonPathRef: stesso progresso calcolato, due punti di vista.
export interface PathProgress {
  steps: PathStepProgressRef[];
  requiredTotal: number;
  requiredCovered: number;
  allRequiredCovered: boolean;
}

export interface PathAssigneeRef extends PathProgress {
  employeeId: string;
  employeeName: string;
  startedOn: string;
  targetCompletion?: string;
  completedOn?: string;
  notes?: string;
}

export interface PathDetail extends PathListRow {
  steps: PathStepRef[];
  assignees: PathAssigneeRef[];
}

export interface PersonPathRef extends PathProgress {
  pathId: string;
  pathName: string;
  startedOn: string;
  targetCompletion?: string;
  completedOn?: string;
  notes?: string;
}

export interface PathAssignmentInput {
  pathId: string;
  startedOn?: string;
  targetCompletion?: string;
  notes?: string;
}

// PathAssignmentUpdateInput: sostituzione completa (stesso idioma di
// AwardUpdateInput/EnrollmentFactsInput) — completedOn omesso azzera la
// conclusione registrata.
export interface PathAssignmentUpdateInput {
  startedOn: string;
  targetCompletion?: string;
  completedOn?: string;
  notes?: string;
}

// ── Certificazioni: dettaglio (#162, specchio di types_reads.go) ──

export interface CertificationHolderRef {
  awardId: string;
  employeeId: string;
  employeeName: string;
  outcome: string;
  awardedOn: string;
  expiresOn?: string;
  currentStatus: string;
  validationSource: string;
  documentId?: string;
  documentFilename?: string;
  documentValidated: boolean;
}

export interface CertificationCourseRef {
  id: string;
  title: string;
  active: boolean;
}

export interface CertificationDetail extends CertificationCatalogRow {
  holders: CertificationHolderRef[];
  courses: CertificationCourseRef[];
  rules: CourseRuleRef[];
}

// ── Coda "certificazioni in scadenza" (#162, specchio di types_queues.go) ──

export interface ExpiringCertificationQueueRow {
  awardId: string;
  employeeId: string;
  employeeName: string;
  employeeEmail: string;
  certificationCode: string;
  certificationName: string;
  expiresOn: string;
  daysToExpiry: number;
}

export interface ExpiringCertificationsResponse {
  withinDays: number;
  certifications: ExpiringCertificationQueueRow[];
}

// ── Report: consuntivo economico ed erogato (#163, specchio di types_reports.go) ──

export interface EconomicReportRow {
  expenseId: string;
  eventId: string;
  courseTitle: string;
  eventCancelled: boolean;
  createdAt: string;
  coveredEnrollments: number;
  poId: number;
  poCode?: string;
  amount?: string;
  currency?: string;
  economicState?: EconomicState;
  budgetName?: string;
  budgetYear?: number;
  poError?: string;
}

export interface EconomicReportResponse {
  rows: EconomicReportRow[];
}

export interface DeliveredReportRow {
  enrollmentId: string;
  employeeId: string;
  employeeName: string;
  teams: PersonTeamRef[];
  courseId: string;
  courseTitle: string;
  skillAreaNames: string[];
  eventId: string;
  deliveryStatus: DeliveryStatus;
  learningOutcome?: LearningOutcome | "";
  referenceDate: string;
  hours?: number;
}

export interface DeliveredReportResponse {
  from: string;
  to: string;
  rows: DeliveredReportRow[];
}

// ── Pannello storia: lettura di audit_log (#164, specchio di types_audit.go) ──

export interface AuditEntry {
  occurredAt: string;
  actorId?: string;
  actorName: string;
  entityType: string;
  entityId: string;
  action: string;
  changedFields: string[];
  valuesRecorded: boolean;
  before: Record<string, unknown> | unknown[] | null;
  after: Record<string, unknown> | unknown[] | null;
}

export interface AuditHistoryResponse {
  entries: AuditEntry[];
}

// ── Pianificazione operativa (#186) ──

export interface PlanningRef {
  id: string;
  name: string;
}

export type ReminderOwnerKind = "course" | "request" | "event";
export type PlanningView = "operative" | "reminders" | "suspended" | "history";
export type PlanningItemKind =
  | "requests"
  | "enrollments"
  | "events"
  | "reminders";
export type ReminderTiming = "overdue" | "today" | "future" | "undated";

export interface PlanningReminder {
  ownerKind: ReminderOwnerKind;
  ownerId: string;
  ownerLabel: string;
  courseId: string;
  text: string;
  date: string | null;
  operative: boolean;
  timing: ReminderTiming;
}

export interface PlanningRequestArea extends PlanningRef {
  levelCurrent: number | null;
  levelTarget: number | null;
}

export interface PlanningRequestItem {
  id: string;
  employee: PlanningRef;
  team: PlanningRef | null;
  course: PlanningRef;
  priority: number | null;
  createdAt: string;
  areas: PlanningRequestArea[];
  tlOpinion: string | null;
  peopleDecision: string | null;
  outcome: string | null;
  suspended: boolean;
  operative: boolean;
  acceptedCourse: PlanningRef | null;
  acceptedEventId: string | null;
  resultingEnrollmentId: string | null;
}

export interface PlanningEventItem {
  id: string;
  title: string;
  courseId: string;
  origin: string;
  cancelled: boolean;
  operative: boolean;
  sessionsCount: number;
  startsAt: string | null;
  endsAt: string | null;
  dueOn: string | null;
  enrollmentsCount: number;
  enrollmentsByStatus: Record<DeliveryStatus, number>;
  withoutSessions: boolean;
  unassignedEnrollments: boolean;
  needsReconciliation: boolean;
}

export interface PlanningEnrollmentItem {
  id: string;
  employee: PlanningRef;
  teams: PlanningRef[];
  event: PlanningRef;
  deliveryStatus: DeliveryStatus;
  requestIds: string[];
}

export interface PlanningEconomic {
  distinctPOCount: number;
  approved: number;
  pending: number;
  rejected: number;
  coveredEnrollments: number;
  approvedCoveredEnrollments: number;
}

export interface CourseSummary {
  id: string;
  title: string;
  tags: string[];
  areas: PlanningRef[];
  courseSuspended: boolean;
  suspensionReason: string | null;
  operative: boolean;
  suspendedWork: boolean;
  history: boolean;
  reasons: Array<"requests" | "events" | "reminders" | "expenses">;
  priority: number | null;
  peopleCount: number;
  enrolledPeopleCount: number;
  requestsCount: number;
  operativeRequestsCount: number;
  suspendedRequestsCount: number;
  eventsCount: number;
  operativeEventsCount: number;
  enrollmentsCount: number;
  enrollmentsByStatus: Record<DeliveryStatus, number>;
  economic: PlanningEconomic;
  reminderCount: number;
  operativeReminderCount: number;
  dueReminderCount: number;
  primaryReminder: PlanningReminder | null;
}

export interface PlanningListResponse {
  today: string;
  generatedAt: string;
  items: CourseSummary[];
  total: number;
  limit: number;
  offset: number;
  dueCoursesTotal: number;
}

export interface PlanningFiltersResponse {
  tags: string[];
  people: PlanningRef[];
  teams: PlanningRef[];
  areas: PlanningRef[];
}

export interface PlanningCourseDetail {
  today: string;
  course: CourseSummary;
  requestsPreview: PlanningRequestItem[];
  eventsPreview: PlanningEventItem[];
  eventOptions: PlanningRef[];
  teamOptions: PlanningRef[];
}

interface PlanningItemsBase<K extends PlanningItemKind, T> {
  today: string;
  kind: K;
  items: T[];
  total: number;
  unfilteredTotal: number;
  limit: number;
  offset: number;
}

export type PlanningItemsResponse =
  | PlanningItemsBase<"requests", PlanningRequestItem>
  | PlanningItemsBase<"enrollments", PlanningEnrollmentItem>
  | PlanningItemsBase<"events", PlanningEventItem>
  | PlanningItemsBase<"reminders", PlanningReminder>;

export interface PlanningListParams {
  view: PlanningView;
  q: string;
  tag: string;
  employeeId: string;
  teamId: string;
  skillAreaId: string;
  limit: 25 | 50;
  offset: number;
}

export interface PlanningItemsParams {
  kind: PlanningItemKind;
  q: string;
  teamId: string;
  eventId: string;
  status: string;
  limit: 25 | 50;
  offset: number;
}

export interface ReminderUpdateInput {
  text: string;
  date: string | null;
}
