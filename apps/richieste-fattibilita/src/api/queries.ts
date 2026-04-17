import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useRDFApiClient } from './client';
import type {
  AnalysisJSON,
  AnalysisTextResponse,
  CreateFattibilitaBody,
  CreateRichiestaBody,
  Deal,
  Fattibilita,
  LookupItem,
  PagedResponse,
  RichiestaBase,
  RichiestaFull,
  RichiestaSummary,
  UpdateFattibilitaBody,
  UpdateRichiestaStatoBody,
} from './types';

export interface SummaryParams {
  stato: string[];
  q?: string;
  data_da?: string;
  data_a?: string;
  page?: number;
  page_size?: number;
}

export interface DealsParams {
  q?: string;
  cliente?: string;
  page?: number;
  page_size?: number;
}

const queryKeys = {
  summary: (params: SummaryParams) => ['rdf', 'summary', params] as const,
  full: (id: number) => ['rdf', 'full', id] as const,
  pdf: (id: number) => ['rdf', 'pdf', id] as const,
  analysis: (id: number) => ['rdf', 'analysis', id] as const,
  analysisJson: (id: number) => ['rdf', 'analysis-json', id] as const,
  deals: (params: DealsParams) => ['rdf', 'deals', params] as const,
  fornitori: ['rdf', 'fornitori'] as const,
  tecnologie: ['rdf', 'tecnologie'] as const,
};

export function useRichiesteSummary(params: SummaryParams) {
  const api = useRDFApiClient();
  const search = new URLSearchParams();
  if (params.stato.length) search.set('stato', params.stato.join(','));
  if (params.q) search.set('q', params.q);
  if (params.data_da) search.set('data_da', params.data_da);
  if (params.data_a) search.set('data_a', params.data_a);
  if (params.page) search.set('page', String(params.page));
  if (params.page_size) search.set('page_size', String(params.page_size));

  return useQuery({
    queryKey: queryKeys.summary(params),
    queryFn: () => api.get<PagedResponse<RichiestaSummary>>(`/rdf/v1/richieste/summary?${search.toString()}`),
  });
}

export function useDeals(params: DealsParams) {
  const api = useRDFApiClient();
  const search = new URLSearchParams();
  if (params.q) search.set('q', params.q);
  if (params.cliente) search.set('cliente', params.cliente);
  if (params.page) search.set('page', String(params.page));
  if (params.page_size) search.set('page_size', String(params.page_size));

  return useQuery({
    queryKey: queryKeys.deals(params),
    queryFn: () => api.get<PagedResponse<Deal>>(`/rdf/v1/deals?${search.toString()}`),
  });
}

export function useFornitori() {
  const api = useRDFApiClient();
  return useQuery({
    queryKey: queryKeys.fornitori,
    queryFn: () => api.get<LookupItem[]>('/rdf/v1/fornitori'),
  });
}

export function useTecnologie() {
  const api = useRDFApiClient();
  return useQuery({
    queryKey: queryKeys.tecnologie,
    queryFn: () => api.get<LookupItem[]>('/rdf/v1/tecnologie'),
  });
}

export function useRichiestaFull(id: number | null) {
  const api = useRDFApiClient();
  return useQuery({
    queryKey: id ? queryKeys.full(id) : ['rdf', 'full', 'empty'],
    enabled: id !== null,
    queryFn: () => api.get<RichiestaFull>(`/rdf/v1/richieste/${id}/full`),
  });
}

export function useAnalysis(id: number | null, enabled = true) {
  const api = useRDFApiClient();
  return useQuery({
    queryKey: id ? queryKeys.analysis(id) : ['rdf', 'analysis', 'empty'],
    enabled: id !== null && enabled,
    queryFn: async () => (await api.post<AnalysisTextResponse>(`/rdf/v1/richieste/${id}/analisi`)).analysis,
  });
}

export function useAnalysisJSON(id: number | null, enabled = true) {
  const api = useRDFApiClient();
  return useQuery({
    queryKey: id ? queryKeys.analysisJson(id) : ['rdf', 'analysis-json', 'empty'],
    enabled: id !== null && enabled,
    queryFn: () => api.post<AnalysisJSON>(`/rdf/v1/richieste/${id}/analisi-json`),
  });
}

export function useRichiestaPdf(id: number | null, enabled = true) {
  const api = useRDFApiClient();
  return useQuery({
    queryKey: id ? queryKeys.pdf(id) : ['rdf', 'pdf', 'empty'],
    enabled: id !== null && enabled,
    queryFn: () => api.getBlob(`/rdf/v1/richieste/${id}/pdf`),
  });
}

export function useCreateRichiesta() {
  const api = useRDFApiClient();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (body: CreateRichiestaBody) => api.post<RichiestaFull>('/rdf/v1/richieste', body),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['rdf', 'summary'] });
    },
  });
}

export function useCreateFattibilita() {
  const api = useRDFApiClient();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ richiestaId, body }: { richiestaId: number; body: CreateFattibilitaBody }) =>
      api.post<Fattibilita[]>(`/rdf/v1/richieste/${richiestaId}/fattibilita`, body),
    onSuccess: async (_data, variables) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.full(variables.richiestaId) });
      await queryClient.invalidateQueries({ queryKey: ['rdf', 'summary'] });
    },
  });
}

export function useUpdateRichiestaStato() {
  const api = useRDFApiClient();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ richiestaId, body }: { richiestaId: number; body: UpdateRichiestaStatoBody }) =>
      api.patch<RichiestaBase>(`/rdf/v1/richieste/${richiestaId}/stato`, body),
    onSuccess: async (_data, variables) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.full(variables.richiestaId) });
      await queryClient.invalidateQueries({ queryKey: ['rdf', 'summary'] });
    },
  });
}

export function useUpdateFattibilita() {
  const api = useRDFApiClient();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      fattibilitaId,
      body,
    }: {
      fattibilitaId: number;
      richiestaId: number;
      body: UpdateFattibilitaBody;
    }) => api.patch<Fattibilita>(`/rdf/v1/fattibilita/${fattibilitaId}`, body),
    onSuccess: async (_data, variables) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.full(variables.richiestaId) });
      await queryClient.invalidateQueries({ queryKey: ['rdf', 'summary'] });
      await queryClient.invalidateQueries({ queryKey: queryKeys.analysis(variables.richiestaId) });
      await queryClient.invalidateQueries({ queryKey: queryKeys.analysisJson(variables.richiestaId) });
      await queryClient.invalidateQueries({ queryKey: queryKeys.pdf(variables.richiestaId) });
    },
  });
}
