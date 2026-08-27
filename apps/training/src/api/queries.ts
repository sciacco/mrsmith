import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useApiClient } from './client';
import type {
  EventListResponse,
  ExpiringCoverageResponse,
  LookupResponse,
  MeResponse,
  RequestsAwaitingDecisionResponse,
  RequestsWithoutTLOpinionResponse,
  RoundsWithoutEventResponse,
  SeatRuleCoverageResponse,
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
