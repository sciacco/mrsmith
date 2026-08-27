import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { ApiClient } from '@mrsmith/api-client';
import { useApiClient } from './client';
import type {
  ActionResponse,
  AssessmentInput,
  AssessmentUpdateInput,
  AwardInput,
  AwardUpdateInput,
  BulkAssignmentsInput,
  BulkAssignmentsResponse,
  BulkEnrollInput,
  BulkEnrollResponse,
  BulkParticipationInput,
  BulkParticipationResponse,
  CertificationCatalogResponse,
  CertificationDetail,
  CertificationInput,
  CourseDetail,
  CourseInput,
  CourseListResponse,
  DocumentMetadata,
  EnrollmentFactsInput,
  EventDetail,
  EventExpense,
  EventExpenseEnrollmentsInput,
  EventExpenseInput,
  EventExpenseReplaceInput,
  EventInput,
  EventListResponse,
  ExpiringCertificationsResponse,
  ExpiringCoverageResponse,
  FeedEventResponse,
  GroupInput,
  GroupListResponse,
  GroupMembersInput,
  LookupResponse,
  MeResponse,
  ParticipationInput,
  PathAssignmentInput,
  PathAssignmentUpdateInput,
  PathDetail,
  PathInput,
  PathListResponse,
  PathStepsInput,
  PersonCreateInput,
  PersonDetail,
  PersonListResponse,
  PersonUpdateInput,
  ReasonInput,
  RequestDecisionInput,
  RequestDetail,
  RequestInput,
  RequestListResponse,
  RequestsAwaitingDecisionResponse,
  RequestsWithoutTLOpinionResponse,
  RoundsWithoutEventResponse,
  RuleDetail,
  RuleEventResponse,
  RuleInput,
  RuleListResponse,
  SeatRuleCoverageResponse,
  SessionInput,
  SkillAreaInput,
  SkillAreaListResponse,
  StaleEnrollmentsResponse,
  TeamInput,
  TeamListResponse,
  TLOpinionInput,
  UnapprovedEventExpensesResponse,
  UnfedPopulationResponse,
  VendorInput,
  VendorListResponse,
} from './types';

const TRAINING_PREFIX = '/training/v1';

function withDaysParam(name: string, value: number): string {
  return `?${new URLSearchParams({ [name]: String(value) }).toString()}`;
}

export function useMe() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'me'],
    queryFn: () => api.get<MeResponse>(`${TRAINING_PREFIX}/me`),
  });
}

export function useTrainingLookups() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'lookups'],
    queryFn: () => api.get<LookupResponse>(`${TRAINING_PREFIX}/lookups`),
  });
}

// ── Letture di dominio: catalogo, persone, team, aree, gruppi (#157) ──

export function useTrainingCourses() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'courses'],
    queryFn: async () => (await api.get<CourseListResponse>(`${TRAINING_PREFIX}/courses`)).courses,
  });
}

export function useTrainingPeople() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'people'],
    queryFn: async () => (await api.get<PersonListResponse>(`${TRAINING_PREFIX}/people`)).people,
  });
}

export function useTrainingTeams() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'teams'],
    queryFn: async () => (await api.get<TeamListResponse>(`${TRAINING_PREFIX}/teams`)).teams,
  });
}

export function useTrainingSkillAreas() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'skill-areas'],
    queryFn: async () => (await api.get<SkillAreaListResponse>(`${TRAINING_PREFIX}/skill-areas`)).skillAreas,
  });
}

export function useTrainingGroups() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'groups'],
    queryFn: async () => (await api.get<GroupListResponse>(`${TRAINING_PREFIX}/groups`)).groups,
  });
}

export function useTrainingVendors() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'vendors'],
    queryFn: async () => (await api.get<VendorListResponse>(`${TRAINING_PREFIX}/vendors`)).vendors,
  });
}

export function useTrainingCertifications() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'certifications'],
    queryFn: async () =>
      (await api.get<CertificationCatalogResponse>(`${TRAINING_PREFIX}/certifications`)).certifications,
  });
}

export function useCourseDetail(id: string | undefined) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'courses', id],
    queryFn: () => api.get<CourseDetail>(`${TRAINING_PREFIX}/courses/${id}`),
    enabled: id !== undefined && id !== '',
  });
}

export function usePersonDetail(id: string | undefined) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'people', 'detail', id],
    queryFn: () => api.get<PersonDetail>(`${TRAINING_PREFIX}/people/${id}`),
    enabled: id !== undefined && id !== '',
  });
}

export function useCertificationDetail(id: string | undefined) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'certifications', 'detail', id],
    queryFn: () => api.get<CertificationDetail>(`${TRAINING_PREFIX}/certifications/${id}`),
    enabled: id !== undefined && id !== '',
  });
}

// ── Percorsi formativi: catalogo (#162) ──

export function useTrainingPaths() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'paths'],
    queryFn: async () => (await api.get<PathListResponse>(`${TRAINING_PREFIX}/paths`)).paths,
  });
}

export function usePathDetail(id: string | undefined) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'paths', 'detail', id],
    queryFn: () => api.get<PathDetail>(`${TRAINING_PREFIX}/paths/${id}`),
    enabled: id !== undefined && id !== '',
  });
}

export function useTrainingEvents() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'events'],
    queryFn: async () => (await api.get<EventListResponse>(`${TRAINING_PREFIX}/events`)).events,
  });
}

export function useEventDetail(id: string | undefined) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'events', id],
    queryFn: () => api.get<EventDetail>(`${TRAINING_PREFIX}/events/${id}`),
    enabled: id !== undefined && id !== '',
  });
}

export function useRequestsWithoutTLOpinion() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'queues', 'requests-without-tl-opinion'],
    queryFn: async () =>
      (
        await api.get<RequestsWithoutTLOpinionResponse>(
          `${TRAINING_PREFIX}/queues/requests-without-tl-opinion`,
        )
      ).requests,
  });
}

export function useRequestsAwaitingDecision() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'queues', 'requests-awaiting-decision'],
    queryFn: async () =>
      (
        await api.get<RequestsAwaitingDecisionResponse>(
          `${TRAINING_PREFIX}/queues/requests-awaiting-decision`,
        )
      ).requests,
  });
}

export function useSeatRuleCoverage() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'queues', 'seat-rule-coverage'],
    queryFn: async () =>
      (await api.get<SeatRuleCoverageResponse>(`${TRAINING_PREFIX}/queues/seat-rule-coverage`)).rules,
  });
}

export function useExpiringCoverage(withinDays: number) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'queues', 'expiring-coverage', withinDays],
    queryFn: () =>
      api.get<ExpiringCoverageResponse>(
        `${TRAINING_PREFIX}/queues/expiring-coverage${withDaysParam('withinDays', withinDays)}`,
      ),
  });
}

export function useUnfedPopulation() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'queues', 'unfed-population'],
    queryFn: async () =>
      (await api.get<UnfedPopulationResponse>(`${TRAINING_PREFIX}/queues/unfed-population`)).rules,
  });
}

export function useRoundsWithoutEvent(withinDays: number) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'queues', 'rounds-without-event', withinDays],
    queryFn: () =>
      api.get<RoundsWithoutEventResponse>(
        `${TRAINING_PREFIX}/queues/rounds-without-event${withDaysParam('withinDays', withinDays)}`,
      ),
  });
}

export function useUnapprovedEventExpenses() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'queues', 'unapproved-event-expenses'],
    queryFn: async () =>
      (
        await api.get<UnapprovedEventExpensesResponse>(
          `${TRAINING_PREFIX}/queues/unapproved-event-expenses`,
        )
      ).expenses,
  });
}

export function useStaleEnrollments(olderThanDays: number) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'queues', 'stale-enrollments', olderThanDays],
    queryFn: () =>
      api.get<StaleEnrollmentsResponse>(
        `${TRAINING_PREFIX}/queues/stale-enrollments${withDaysParam('olderThanDays', olderThanDays)}`,
      ),
  });
}

// Coda "certificazioni in scadenza" (#162, §Home 6): stesso pattern di
// soglia in giorni delle code esistenti.
export function useExpiringCertifications(withinDays: number) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'queues', 'expiring-certifications', withinDays],
    queryFn: () =>
      api.get<ExpiringCertificationsResponse>(
        `${TRAINING_PREFIX}/queues/expiring-certifications${withDaysParam('withinDays', withinDays)}`,
      ),
  });
}

// Punto unico di invalidazione della cache Training: le mutazioni introdotte
// dalle prossime slice lo richiamano invece di elencare le query key a mano.
export function useInvalidateTrainingQueries() {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: ['training'] });
}

// ── Mutazioni evento/sessione/iscrizione/partecipazione/spesa (#156) ──
// Fabbrica comune: ogni mutazione invalida l'intera cache Training al
// successo (lista eventi, dettaglio, code di lavoro derivate).

function useTrainingMutation<TVars, TResult>(fn: (api: ApiClient, vars: TVars) => Promise<TResult>) {
  const api = useApiClient();
  const invalidate = useInvalidateTrainingQueries();
  return useMutation({
    mutationFn: (vars: TVars) => fn(api, vars),
    onSuccess: () => invalidate(),
  });
}

export function useCreateEvent() {
  return useTrainingMutation<EventInput, ActionResponse>((api, input) =>
    api.post(`${TRAINING_PREFIX}/events`, input),
  );
}

export function useUpdateEvent() {
  return useTrainingMutation<{ id: string; input: EventInput }, ActionResponse>((api, { id, input }) =>
    api.put(`${TRAINING_PREFIX}/events/${id}`, input),
  );
}

export function useCancelEvent() {
  return useTrainingMutation<{ id: string; input: ReasonInput }, ActionResponse>((api, { id, input }) =>
    api.post(`${TRAINING_PREFIX}/events/${id}/cancel`, input),
  );
}

export function useFeedEvent() {
  return useTrainingMutation<string, FeedEventResponse>((api, eventId) =>
    api.post(`${TRAINING_PREFIX}/events/${eventId}/feed`),
  );
}

export function useCreateSession() {
  return useTrainingMutation<{ eventId: string; input: SessionInput }, ActionResponse>((api, { eventId, input }) =>
    api.post(`${TRAINING_PREFIX}/events/${eventId}/sessions`, input),
  );
}

export function useUpdateSession() {
  return useTrainingMutation<{ id: string; input: SessionInput }, ActionResponse>((api, { id, input }) =>
    api.put(`${TRAINING_PREFIX}/sessions/${id}`, input),
  );
}

export function useDeleteSession() {
  return useTrainingMutation<string, ActionResponse>((api, id) => api.delete(`${TRAINING_PREFIX}/sessions/${id}`));
}

export function useUpdateEnrollmentFacts() {
  return useTrainingMutation<{ id: string; input: EnrollmentFactsInput }, ActionResponse>((api, { id, input }) =>
    api.put(`${TRAINING_PREFIX}/enrollments/${id}`, input),
  );
}

export function useCancelEnrollment() {
  return useTrainingMutation<{ id: string; input: ReasonInput }, ActionResponse>((api, { id, input }) =>
    api.post(`${TRAINING_PREFIX}/enrollments/${id}/cancel`, input),
  );
}

export function useCompleteEnrollmentHistorical() {
  return useTrainingMutation<string, ActionResponse>((api, id) =>
    api.post(`${TRAINING_PREFIX}/enrollments/${id}/complete-historical`),
  );
}

export function useReopenEnrollment() {
  return useTrainingMutation<{ id: string; input: ReasonInput }, ActionResponse>((api, { id, input }) =>
    api.post(`${TRAINING_PREFIX}/enrollments/${id}/reopen`, input),
  );
}

export function useAssignParticipation() {
  return useTrainingMutation<{ enrollmentId: string; sessionId: string }, ActionResponse>(
    (api, { enrollmentId, sessionId }) =>
      api.post(`${TRAINING_PREFIX}/enrollments/${enrollmentId}/sessions/${sessionId}`),
  );
}

export function useRemoveParticipation() {
  return useTrainingMutation<{ enrollmentId: string; sessionId: string }, ActionResponse>(
    (api, { enrollmentId, sessionId }) =>
      api.delete(`${TRAINING_PREFIX}/enrollments/${enrollmentId}/sessions/${sessionId}`),
  );
}

export function useUpdateParticipation() {
  return useTrainingMutation<
    { enrollmentId: string; sessionId: string; input: ParticipationInput },
    ActionResponse
  >((api, { enrollmentId, sessionId, input }) =>
    api.patch(`${TRAINING_PREFIX}/enrollments/${enrollmentId}/sessions/${sessionId}`, input),
  );
}

export function useBulkAssignments() {
  return useTrainingMutation<{ eventId: string; input: BulkAssignmentsInput }, BulkAssignmentsResponse>(
    (api, { eventId, input }) => api.post(`${TRAINING_PREFIX}/events/${eventId}/bulk-assignments`, input),
  );
}

export function useBulkParticipation() {
  return useTrainingMutation<{ sessionId: string; input: BulkParticipationInput }, BulkParticipationResponse>(
    (api, { sessionId, input }) => api.post(`${TRAINING_PREFIX}/sessions/${sessionId}/bulk-participation`, input),
  );
}

export function useBulkEnrollments() {
  return useTrainingMutation<{ eventId: string; input: BulkEnrollInput }, BulkEnrollResponse>(
    (api, { eventId, input }) => api.post(`${TRAINING_PREFIX}/events/${eventId}/enrollments/bulk`, input),
  );
}

export function useCreateExpense() {
  return useTrainingMutation<{ eventId: string; input: EventExpenseInput }, EventExpense>(
    (api, { eventId, input }) => api.post(`${TRAINING_PREFIX}/events/${eventId}/expenses`, input),
  );
}

export function useReplaceExpensePO() {
  return useTrainingMutation<{ id: string; input: EventExpenseReplaceInput }, EventExpense>((api, { id, input }) =>
    api.put(`${TRAINING_PREFIX}/expenses/${id}`, input),
  );
}

export function useReplaceExpenseEnrollments() {
  return useTrainingMutation<{ id: string; input: EventExpenseEnrollmentsInput }, ActionResponse>(
    (api, { id, input }) => api.put(`${TRAINING_PREFIX}/expenses/${id}/enrollments`, input),
  );
}

export function useDeleteExpense() {
  return useTrainingMutation<string, ActionResponse>((api, id) => api.delete(`${TRAINING_PREFIX}/expenses/${id}`));
}

// ── Richieste formative (#157) ──

export function useTrainingRequests(state: 'open' | 'closed' | 'all') {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'requests', state],
    queryFn: async () =>
      (await api.get<RequestListResponse>(`${TRAINING_PREFIX}/requests?${new URLSearchParams({ state })}`)).requests,
  });
}

export function useRequestDetail(id: string | undefined) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'requests', 'detail', id],
    queryFn: () => api.get<RequestDetail>(`${TRAINING_PREFIX}/requests/${id}`),
    enabled: id !== undefined && id !== '',
  });
}

export function useCreateRequest() {
  return useTrainingMutation<RequestInput, ActionResponse>((api, input) =>
    api.post(`${TRAINING_PREFIX}/requests`, input),
  );
}

export function useRecordTLOpinion() {
  return useTrainingMutation<{ id: string; input: TLOpinionInput }, ActionResponse>((api, { id, input }) =>
    api.post(`${TRAINING_PREFIX}/requests/${id}/tl-opinion`, input),
  );
}

export function useRecordRequestDecision() {
  return useTrainingMutation<{ id: string; input: RequestDecisionInput }, ActionResponse>((api, { id, input }) =>
    api.post(`${TRAINING_PREFIX}/requests/${id}/decision`, input),
  );
}

export function useWithdrawRequest() {
  return useTrainingMutation<string, ActionResponse>((api, id) =>
    api.post(`${TRAINING_PREFIX}/requests/${id}/withdraw`),
  );
}

// ── Regole formative (#157) ──

export function useTrainingRules() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'rules'],
    queryFn: async () => (await api.get<RuleListResponse>(`${TRAINING_PREFIX}/rules`)).rules,
  });
}

export function useRuleDetail(id: string | undefined) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'rules', 'detail', id],
    queryFn: () => api.get<RuleDetail>(`${TRAINING_PREFIX}/rules/${id}`),
    enabled: id !== undefined && id !== '',
  });
}

export function useCreateRule() {
  return useTrainingMutation<RuleInput, ActionResponse>((api, input) => api.post(`${TRAINING_PREFIX}/rules`, input));
}

export function useUpdateRule() {
  return useTrainingMutation<{ id: string; input: RuleInput }, ActionResponse>((api, { id, input }) =>
    api.put(`${TRAINING_PREFIX}/rules/${id}`, input),
  );
}

export function useSetRuleActive() {
  return useTrainingMutation<{ id: string; active: boolean }, ActionResponse>((api, { id, active }) =>
    api.post(`${TRAINING_PREFIX}/rules/${id}/${active ? 'activate' : 'deactivate'}`),
  );
}

export function useCreateRuleEvent() {
  return useTrainingMutation<string, RuleEventResponse>((api, ruleId) =>
    api.post(`${TRAINING_PREFIX}/rules/${ruleId}/events`),
  );
}

// Creazione contestuale del corso fuori catalogo dal pannello di
// accoglimento richiesta (#157), riusata dal catalogo (#158) per la
// creazione integrale.
export function useCreateCourse() {
  return useTrainingMutation<CourseInput, ActionResponse>((api, input) =>
    api.post(`${TRAINING_PREFIX}/courses`, input),
  );
}

// ── Persone, gruppi locali e anagrafiche del catalogo (#158) ──

export function useCreatePerson() {
  return useTrainingMutation<PersonCreateInput, ActionResponse>((api, input) =>
    api.post(`${TRAINING_PREFIX}/people`, input),
  );
}

export function useUpdatePerson() {
  return useTrainingMutation<{ id: string; input: PersonUpdateInput }, ActionResponse>((api, { id, input }) =>
    api.patch(`${TRAINING_PREFIX}/people/${id}`, input),
  );
}

export function useUpsertGroup() {
  return useTrainingMutation<{ id?: string; input: GroupInput }, ActionResponse>((api, { id, input }) =>
    id ? api.put(`${TRAINING_PREFIX}/groups/${id}`, input) : api.post(`${TRAINING_PREFIX}/groups`, input),
  );
}

export function useReplaceGroupMembers() {
  return useTrainingMutation<{ id: string; input: GroupMembersInput }, ActionResponse>((api, { id, input }) =>
    api.put(`${TRAINING_PREFIX}/groups/${id}/members`, input),
  );
}

export function useDeleteGroup() {
  return useTrainingMutation<string, ActionResponse>((api, id) => api.delete(`${TRAINING_PREFIX}/groups/${id}`));
}

export function useUpsertVendor() {
  return useTrainingMutation<{ id?: string; input: VendorInput }, ActionResponse>((api, { id, input }) =>
    id ? api.put(`${TRAINING_PREFIX}/vendors/${id}`, input) : api.post(`${TRAINING_PREFIX}/vendors`, input),
  );
}

export function useUpsertTeam() {
  return useTrainingMutation<{ id: string; input: TeamInput }, ActionResponse>((api, { id, input }) =>
    api.put(`${TRAINING_PREFIX}/teams/${id}`, input),
  );
}

export function useUpsertSkillArea() {
  return useTrainingMutation<{ id?: string; input: SkillAreaInput }, ActionResponse>((api, { id, input }) =>
    id ? api.put(`${TRAINING_PREFIX}/skill-areas/${id}`, input) : api.post(`${TRAINING_PREFIX}/skill-areas`, input),
  );
}

export function useUpsertCertification() {
  return useTrainingMutation<{ id?: string; input: CertificationInput }, ActionResponse>((api, { id, input }) =>
    id
      ? api.put(`${TRAINING_PREFIX}/certifications/${id}`, input)
      : api.post(`${TRAINING_PREFIX}/certifications`, input),
  );
}

export function useUpdateCourse() {
  return useTrainingMutation<{ id: string; input: CourseInput }, ActionResponse>((api, { id, input }) =>
    api.put(`${TRAINING_PREFIX}/courses/${id}`, input),
  );
}

export function useArchiveCourse() {
  return useTrainingMutation<string, ActionResponse>((api, id) =>
    api.post(`${TRAINING_PREFIX}/courses/${id}/archive`),
  );
}

// ── Conseguimenti, documenti, valutazioni, percorsi (#162, slice 3 del task 7) ──

export function useCreateAward() {
  return useTrainingMutation<AwardInput, ActionResponse>((api, input) =>
    api.post(`${TRAINING_PREFIX}/awards`, input),
  );
}

export function useUpdateAward() {
  return useTrainingMutation<{ id: string; input: AwardUpdateInput }, ActionResponse>((api, { id, input }) =>
    api.put(`${TRAINING_PREFIX}/awards/${id}`, input),
  );
}

export function useUploadAwardDocument() {
  return useTrainingMutation<{ awardId: string; file: File }, DocumentMetadata>((api, { awardId, file }) => {
    const form = new FormData();
    form.append('file', file);
    return api.postFormData(`${TRAINING_PREFIX}/awards/${awardId}/documents`, form);
  });
}

export function useValidateDocument() {
  return useTrainingMutation<string, ActionResponse>((api, documentId) =>
    api.post(`${TRAINING_PREFIX}/documents/${documentId}/validate`),
  );
}

export function useCreateAssessment() {
  return useTrainingMutation<{ personId: string; input: AssessmentInput }, ActionResponse>(
    (api, { personId, input }) => api.post(`${TRAINING_PREFIX}/people/${personId}/assessments`, input),
  );
}

export function useUpdateAssessment() {
  return useTrainingMutation<{ id: string; input: AssessmentUpdateInput }, ActionResponse>((api, { id, input }) =>
    api.put(`${TRAINING_PREFIX}/assessments/${id}`, input),
  );
}

export function useDeleteAssessment() {
  return useTrainingMutation<string, ActionResponse>((api, id) =>
    api.delete(`${TRAINING_PREFIX}/assessments/${id}`),
  );
}

export function useUpsertPath() {
  return useTrainingMutation<{ id?: string; input: PathInput }, ActionResponse>((api, { id, input }) =>
    id ? api.put(`${TRAINING_PREFIX}/paths/${id}`, input) : api.post(`${TRAINING_PREFIX}/paths`, input),
  );
}

export function useReplacePathSteps() {
  return useTrainingMutation<{ pathId: string; input: PathStepsInput }, ActionResponse>((api, { pathId, input }) =>
    api.put(`${TRAINING_PREFIX}/paths/${pathId}/steps`, input),
  );
}

export function useAssignPersonPath() {
  return useTrainingMutation<{ personId: string; input: PathAssignmentInput }, ActionResponse>(
    (api, { personId, input }) => api.post(`${TRAINING_PREFIX}/people/${personId}/paths`, input),
  );
}

export function useUpdatePersonPath() {
  return useTrainingMutation<
    { personId: string; pathId: string; input: PathAssignmentUpdateInput },
    ActionResponse
  >((api, { personId, pathId, input }) =>
    api.put(`${TRAINING_PREFIX}/people/${personId}/paths/${pathId}`, input),
  );
}

export function useRemovePersonPath() {
  return useTrainingMutation<{ personId: string; pathId: string }, ActionResponse>((api, { personId, pathId }) =>
    api.delete(`${TRAINING_PREFIX}/people/${personId}/paths/${pathId}`),
  );
}
