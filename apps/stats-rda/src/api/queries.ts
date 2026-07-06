import { useQuery } from '@tanstack/react-query';
import type { ApiClient } from '@mrsmith/api-client';
import { useApiClient } from './client';
import type {
  AutocompleteResponse,
  FiltersResponse,
  IssueDetail,
  IssueListResponse,
  RiepilogoPeriodSelection,
  RiepilogoRdaResponse,
  RiepilogoResponse,
} from './types';

const PA_ROOT = '/stats-rda/v1/pa';
const RDA_ROOT = '/stats-rda/v1/rda';

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
    queryFn: () => api.get<FiltersResponse>(`${PA_ROOT}/filters`),
    staleTime: 10 * 60 * 1000,
  });
}

export function useFornitoriAutocomplete(q: string, enabled = true) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['stats-rda', 'fornitori', q],
    queryFn: () => api.get<AutocompleteResponse>(`${PA_ROOT}/fornitori${buildSearch({ q, limit: 20 })}`),
    enabled: enabled && q.trim().length >= 2,
    staleTime: 2 * 60 * 1000,
  });
}

export function useRichiedentiAutocomplete(q: string, enabled = true) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['stats-rda', 'richiedenti', q],
    queryFn: () => api.get<AutocompleteResponse>(`${PA_ROOT}/richiedenti${buildSearch({ q, limit: 20 })}`),
    enabled: enabled && q.trim().length >= 2,
    staleTime: 2 * 60 * 1000,
  });
}

export function useIssueList(params: IssueListParams, enabled = true) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['stats-rda', 'issues', params],
    queryFn: () => api.get<IssueListResponse>(`${PA_ROOT}/issues${buildSearch(params as Record<string, string | number | undefined>)}`),
    enabled,
    placeholderData: (prev) => prev,
  });
}

export function useIssueDetail(issueKey: string | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['stats-rda', 'issue', issueKey],
    queryFn: () => api.get<IssueDetail>(`${PA_ROOT}/issues/${encodeURIComponent(issueKey!)}`),
    enabled: Boolean(issueKey),
  });
}

export interface RiepilogoPaParams {
  period: RiepilogoPeriodSelection;
  from?: string;
  to?: string;
}

export interface RiepilogoRdaParams {
  period: RiepilogoPeriodSelection;
  from?: string;
  to?: string;
}

function buildRiepilogoSearchParams(params: RiepilogoPaParams | RiepilogoRdaParams) {
  return params.period === 'custom'
    ? { period: params.period, from: params.from, to: params.to }
    : { period: params.period };
}

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}

export function useRiepilogoPa(params: RiepilogoPaParams) {
  const api = useApiClient();
  const searchParams = buildRiepilogoSearchParams(params);

  return useQuery({
    queryKey: ['stats-rda', 'riepilogo-pa', searchParams],
    queryFn: () => api.get<RiepilogoResponse>(`${PA_ROOT}/riepilogo${buildSearch(searchParams)}`),
    placeholderData: (prev) => prev,
  });
}

export function useRiepilogoRda(params: RiepilogoRdaParams) {
  const api = useApiClient();
  const searchParams = buildRiepilogoSearchParams(params);

  return useQuery({
    queryKey: ['stats-rda', 'riepilogo-rda', searchParams],
    queryFn: () => api.get<RiepilogoRdaResponse>(`${RDA_ROOT}/riepilogo${buildSearch(searchParams)}`),
    placeholderData: (prev) => prev,
  });
}

export async function downloadRiepilogoPaExcel(
  api: ApiClient,
  params: RiepilogoPaParams,
  filename = `riepilogo-pa-jira_${params.period}.xlsx`,
) {
  const searchParams = buildRiepilogoSearchParams(params);
  const blob = await api.getBlob(`${PA_ROOT}/riepilogo/export${buildSearch(searchParams)}`);
  downloadBlob(blob, filename);
}

export async function downloadRiepilogoRdaExcel(
  api: ApiClient,
  params: RiepilogoRdaParams,
  filename = `riepilogo-rda_${params.period}.xlsx`,
) {
  const searchParams = buildRiepilogoSearchParams(params);
  const blob = await api.getBlob(`${RDA_ROOT}/riepilogo/export${buildSearch(searchParams)}`);
  downloadBlob(blob, filename);
}
