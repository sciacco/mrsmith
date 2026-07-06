import { useQuery } from '@tanstack/react-query';
import type { ApiClient } from '@mrsmith/api-client';
import { useApiClient } from './client';
import type {
  AutocompleteResponse,
  FiltersResponse,
  IssueDetail,
  IssueListResponse,
  PeriodPreset,
  RiepilogoResponse,
} from './types';

const ROOT = '/stats-rda/v1/pa';

export interface IssueListParams {
  q?: string;
  budget?: string;
  stato?: string;
  tipo?: string;
  fornitore?: string;
  richiedente?: string;
  valuta?: string;
  from?: string;
  to?: string;
  sort?: string;
  dir?: string;
  page?: number;
  limit?: number;
}

function buildSearch(params: Record<string, string | number | undefined>): string {
  const sp = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '' && value !== null) {
      sp.set(key, String(value));
    }
  }
  const s = sp.toString();
  return s ? `?${s}` : '';
}

export function useFilters() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['stats-rda', 'filters'],
    queryFn: () => api.get<FiltersResponse>(`${ROOT}/filters`),
    staleTime: 10 * 60 * 1000,
  });
}

export function useFornitoriAutocomplete(q: string, enabled = true) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['stats-rda', 'fornitori', q],
    queryFn: () => api.get<AutocompleteResponse>(`${ROOT}/fornitori${buildSearch({ q, limit: 20 })}`),
    enabled: enabled && q.trim().length >= 2,
    staleTime: 2 * 60 * 1000,
  });
}

export function useRichiedentiAutocomplete(q: string, enabled = true) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['stats-rda', 'richiedenti', q],
    queryFn: () => api.get<AutocompleteResponse>(`${ROOT}/richiedenti${buildSearch({ q, limit: 20 })}`),
    enabled: enabled && q.trim().length >= 2,
    staleTime: 2 * 60 * 1000,
  });
}

export function useIssueList(params: IssueListParams, enabled = true) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['stats-rda', 'issues', params],
    queryFn: () => api.get<IssueListResponse>(`${ROOT}/issues${buildSearch(params as Record<string, string | number | undefined>)}`),
    enabled,
    placeholderData: (prev) => prev,
  });
}

export function useIssueDetail(issueKey: string | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['stats-rda', 'issue', issueKey],
    queryFn: () => api.get<IssueDetail>(`${ROOT}/issues/${encodeURIComponent(issueKey!)}`),
    enabled: Boolean(issueKey),
  });
}

export function useRiepilogoPa(period: PeriodPreset) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['stats-rda', 'riepilogo-pa', period],
    queryFn: () => api.get<RiepilogoResponse>(`${ROOT}/riepilogo${buildSearch({ period })}`),
    placeholderData: (prev) => prev,
  });
}

export async function downloadRiepilogoPaExcel(
  api: ApiClient,
  period: PeriodPreset,
  filename = `riepilogo-pa-jira_${period}.xlsx`,
) {
  const blob = await api.getBlob(`${ROOT}/riepilogo/export${buildSearch({ period })}`);
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}
