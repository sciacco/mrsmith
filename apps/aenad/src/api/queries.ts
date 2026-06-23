import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { ApiError } from '@mrsmith/api-client';
import { useApiClient } from './client';
import type {
  AenadDocumentsPage,
  ArchiveDocumentFilters,
  DocumentTypeOption,
  AenadDocumentRow,
  AenadDocument,
  CustomerOption,
  PaymentMethodOption,
  QuoteListResponse,
  QuoteResponse,
  SaveQuotePayload,
  CustomerSelection,
  ProspectPayload,
  ProspectResponse,
  ArticleLineInitializer,
  PaymentMethodSelection,
  QuoteDefaultsResponse,
  StagesResponse,
  PdfExport,
} from './types';


export const aenadQueryKeys = {
  all: ['aenad'] as const,
  documentTypes: () => [...aenadQueryKeys.all, 'document-types'] as const,
  customers: () => [...aenadQueryKeys.all, 'customers'] as const,
  paymentMethods: () => [...aenadQueryKeys.all, 'payment-methods'] as const,
  documents: (filters: ArchiveDocumentFilters) => [...aenadQueryKeys.all, 'documents', filters] as const,
  documentDetails: (idDoc: number) => [...aenadQueryKeys.all, 'documents', idDoc] as const,
  documentRows: (idDoc: number) => [...aenadQueryKeys.all, 'documents', idDoc, 'rows'] as const,
};

function shouldRetry(failureCount: number, error: unknown) {
  if (error instanceof ApiError && [400, 401, 403, 503].includes(error.status)) {
    return false;
  }
  return failureCount < 2;
}

function documentParams(filters: ArchiveDocumentFilters) {
  const params = new URLSearchParams();
  params.set('tipoDoc', filters.tipoDoc);
  params.set('dateFrom', filters.dateFrom);
  params.set('dateTo', filters.dateTo);
  if (filters.idAnagr !== undefined) {
    params.set('idAnagr', String(filters.idAnagr));
  }
  params.set('page', String(filters.page));
  params.set('pageSize', String(filters.pageSize));
  return params.toString();
}

export function useDocumentTypes() {
  const api = useApiClient();
  return useQuery({
    queryKey: aenadQueryKeys.documentTypes(),
    queryFn: () => api.get<DocumentTypeOption[]>('/aenad/v1/document-types'),
    retry: shouldRetry,
  });
}

export function useCustomers() {
  const api = useApiClient();
  return useQuery({
    queryKey: aenadQueryKeys.customers(),
    queryFn: () => api.get<CustomerOption[]>('/aenad/v1/customers'),
    retry: shouldRetry,
  });
}

export function usePaymentMethods() {
  const api = useApiClient();
  return useQuery({
    queryKey: aenadQueryKeys.paymentMethods(),
    queryFn: () => api.get<PaymentMethodOption[]>('/aenad/v1/payment-methods'),
    retry: shouldRetry,
  });
}

export function useArchiveDocuments(filters: ArchiveDocumentFilters, enabled: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: aenadQueryKeys.documents(filters),
    queryFn: () => api.get<AenadDocumentsPage>(`/aenad/v1/documents?${documentParams(filters)}`),
    enabled,
    placeholderData: (previous) => previous,
    retry: shouldRetry,
  });
}

export function useDocumentRows(idDoc: number, enabled: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: aenadQueryKeys.documentRows(idDoc),
    queryFn: () => api.get<AenadDocumentRow[]>(`/aenad/v1/documents/${idDoc}/rows`),
    enabled,
    retry: shouldRetry,
  });
}

export function useDocumentDetails(idDoc: number, enabled: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: aenadQueryKeys.documentDetails(idDoc),
    queryFn: () => api.get<AenadDocument>(`/aenad/v1/documents/${idDoc}`),
    enabled,
    retry: shouldRetry,
  });
}

export interface UpdateDocumentPayload {
  Anagr_Nome: string | null;
  Anagr_Indirizzo: string | null;
  Anagr_Cap: string | null;
  Anagr_Citta: string | null;
  Anagr_Prov: string | null;
  Anagr_Nazione: string | null;
  Anagr_CodiceFiscale: string | null;
  Anagr_PartitaIva: string | null;
  Anagr_DestNome: string | null;
  Anagr_DestIndirizzo: string | null;
  Anagr_DestCap: string | null;
  Anagr_DestCitta: string | null;
  Anagr_DestProv: string | null;
  Anagr_DestNazione: string | null;
  Pagamento: string | null;
  Pagam_CoordBancarie: string | null;
  NoteInterne: string | null;
  DescDoc: string | null;
  DataDoc: string | null;
  NumDoc: string | null;
  Rows: {
    IDDocRiga: number;
    CodArticolo: string | null;
    Desc: string | null;
    Qta: string | null;
    Udm: string | null;
    PrezzoNetto: string | null;
    Sconti: string | null;
  }[];
}

export function useDocumentPdfDownload() {
  const api = useApiClient();
  return (idDoc: number, condizioni: boolean) =>
    api.getBlob(`/aenad/v1/documents/${idDoc}/pdf?condizioni=${condizioni ? '1' : '0'}`);
}

export function useUpdateDocument() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  
  return useMutation({
    mutationFn: ({ idDoc, payload }: { idDoc: number; payload: UpdateDocumentPayload }) =>
      api.put<AenadDocument>(`/aenad/v1/documents/${idDoc}`, payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: aenadQueryKeys.all });
    },
  });
}

export const aenadQuoteKeys = {
  all: ['aenad-quotes'] as const,
  list: (page: number, pageSize: number) => [...aenadQuoteKeys.all, 'list', { page, pageSize }] as const,
  details: (id: number) => [...aenadQuoteKeys.all, 'details', id] as const,
  customers: (q: string) => [...aenadQuoteKeys.all, 'customers', q] as const,
  articles: (q: string) => [...aenadQuoteKeys.all, 'articles', q] as const,
  paymentMethods: () => [...aenadQuoteKeys.all, 'payment-methods'] as const,
  defaults: () => [...aenadQuoteKeys.all, 'defaults'] as const,
  pdfExports: (id: number) => [...aenadQuoteKeys.all, 'pdf-exports', id] as const,
  stages: () => [...aenadQuoteKeys.all, 'stages'] as const,
};

export function useQuotesList(page: number, pageSize: number) {
  const api = useApiClient();
  return useQuery({
    queryKey: aenadQuoteKeys.list(page, pageSize),
    queryFn: () => api.get<QuoteListResponse>(`/aenad/v1/quotes?page=${page}&page_size=${pageSize}`),
    placeholderData: (previous) => previous,
    retry: shouldRetry,
  });
}

export function useQuoteDetails(id: number, enabled: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: aenadQuoteKeys.details(id),
    queryFn: () => api.get<QuoteResponse>(`/aenad/v1/quotes/${id}`),
    enabled,
    retry: shouldRetry,
  });
}

export function useCreateQuote() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: SaveQuotePayload) => api.post<QuoteResponse>('/aenad/v1/quotes', payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: aenadQuoteKeys.all });
    },
  });
}

export function useUpdateQuote() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: SaveQuotePayload }) =>
      api.put<QuoteResponse>(`/aenad/v1/quotes/${id}`, payload),
    onSuccess: (data) => {
      void queryClient.invalidateQueries({ queryKey: aenadQuoteKeys.all });
      void queryClient.invalidateQueries({ queryKey: aenadQuoteKeys.details(data.id) });
    },
  });
}

export function useQuoteReady() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => api.post<QuoteResponse>(`/aenad/v1/quotes/${id}/ready`, {}),
    onSuccess: (data) => {
      void queryClient.invalidateQueries({ queryKey: aenadQuoteKeys.all });
      void queryClient.invalidateQueries({ queryKey: aenadQuoteKeys.details(data.id) });
    },
  });
}

export function useHubSpotRetry() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => api.post<QuoteResponse>(`/aenad/v1/quotes/${id}/hubspot/retry`, {}),
    onSuccess: (data) => {
      void queryClient.invalidateQueries({ queryKey: aenadQuoteKeys.all });
      void queryClient.invalidateQueries({ queryKey: aenadQuoteKeys.details(data.id) });
    },
  });
}

export function useQuoteCustomers(search: string, enabled: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: aenadQuoteKeys.customers(search),
    queryFn: () => api.get<CustomerSelection[]>(`/aenad/v1/quotes/customers?q=${encodeURIComponent(search)}&limit=100&include_without_numero_azienda=true`),
    enabled,
    retry: shouldRetry,
  });
}

export function useCreateProspect() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: ProspectPayload) => api.post<ProspectResponse>('/aenad/v1/quotes/prospects', payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: aenadQuoteKeys.customers('') });
    },
  });
}

export function useQuoteArticles(search: string, enabled: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: aenadQuoteKeys.articles(search),
    queryFn: () => api.get<ArticleLineInitializer[]>(`/aenad/v1/quotes/articles?q=${encodeURIComponent(search)}&limit=25`),
    enabled,
    retry: shouldRetry,
  });
}

export function useQuotePaymentMethods() {
  const api = useApiClient();
  return useQuery({
    queryKey: aenadQuoteKeys.paymentMethods(),
    queryFn: () => api.get<PaymentMethodSelection[]>('/aenad/v1/quotes/payment-methods'),
    retry: shouldRetry,
  });
}

export function useCustomerPayment(alyanteCustomerId: string | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: [...aenadQuoteKeys.all, 'customer-payment', alyanteCustomerId] as const,
    queryFn: () => api.get<{ payment_code: string }>(`/aenad/v1/customer-payment/${alyanteCustomerId}`),
    enabled: alyanteCustomerId !== null && alyanteCustomerId !== '',
  });
}

export function useQuoteDefaults() {
  const api = useApiClient();
  return useQuery({
    queryKey: aenadQuoteKeys.defaults(),
    queryFn: () => api.get<QuoteDefaultsResponse>('/aenad/v1/quotes/defaults'),
    retry: shouldRetry,
  });
}

export function usePdfExports(quoteId: number, enabled: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: aenadQuoteKeys.pdfExports(quoteId),
    queryFn: () => api.get<PdfExport[]>(`/aenad/v1/quotes/${quoteId}/pdf-exports`),
    enabled,
    retry: shouldRetry,
  });
}

export function useCreatePdfExport() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (quoteId: number) => api.post<PdfExport>(`/aenad/v1/quotes/${quoteId}/pdf-exports`, {}),
    onSuccess: (data) => {
      void queryClient.invalidateQueries({ queryKey: aenadQuoteKeys.pdfExports(data.quote_id) });
    },
  });
}

export function usePdfExportDownload() {
  const api = useApiClient();
  return (quoteId: number, exportId: number) =>
    api.getBlob(`/aenad/v1/quotes/${quoteId}/pdf-exports/${exportId}/download`);
}

export function useAttachPdfExport() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ quoteId, exportId }: { quoteId: number; exportId: number }) =>
      api.post<PdfExport>(`/aenad/v1/quotes/${quoteId}/pdf-exports/${exportId}/attach`, {}),
    onSuccess: (data) => {
      void queryClient.invalidateQueries({ queryKey: aenadQuoteKeys.pdfExports(data.quote_id) });
    },
  });
}

export function useQuoteStages() {
  const api = useApiClient();
  return useQuery({
    queryKey: aenadQuoteKeys.stages(),
    queryFn: () => api.get<StagesResponse>('/aenad/v1/quotes/stages'),
    retry: shouldRetry,
  });
}

export function useTransitionStage() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      quoteId,
      expectedDealstageId,
      targetDealstageId,
    }: {
      quoteId: number;
      expectedDealstageId: string;
      targetDealstageId: string;
    }) =>
      api.post<QuoteResponse>(`/aenad/v1/quotes/${quoteId}/hubspot/stage`, {
        expected_dealstage_id: expectedDealstageId,
        target_dealstage_id: targetDealstageId,
      }),
    onSuccess: (data) => {
      void queryClient.invalidateQueries({ queryKey: aenadQuoteKeys.all });
      void queryClient.invalidateQueries({ queryKey: aenadQuoteKeys.details(data.id) });
    },
  });
}

