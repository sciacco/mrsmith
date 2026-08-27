import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { ApiClient } from '@mrsmith/api-client';
import { useApiClient } from './client';
import type {
  ActionResponse,
  BulkAssignmentsInput,
  BulkAssignmentsResponse,
  BulkEnrollInput,
  BulkEnrollResponse,
  BulkParticipationInput,
  BulkParticipationResponse,
  EnrollmentFactsInput,
  EventDetail,
  EventExpense,
  EventExpenseEnrollmentsInput,
  EventExpenseInput,
  EventExpenseReplaceInput,
  EventInput,
  EventListResponse,
  ExpiringCoverageResponse,
  FeedEventResponse,
  LookupResponse,
  MeResponse,
  ParticipationInput,
  ReasonInput,
  RequestsAwaitingDecisionResponse,
  RequestsWithoutTLOpinionResponse,
  RoundsWithoutEventResponse,
  SeatRuleCoverageResponse,
  SessionInput,
  StaleEnrollmentsResponse,
  UnapprovedEventExpensesResponse,
  UnfedPopulationResponse,
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
