import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { ApiError } from '@mrsmith/api-client';
import { useApiClient } from './client';
import type { AenadDocumentsPage, ArchiveDocumentFilters, DocumentTypeOption, AenadDocumentRow, AenadDocument, CustomerOption } from './types';

export const aenadQueryKeys = {
  all: ['aenad'] as const,
  documentTypes: () => [...aenadQueryKeys.all, 'document-types'] as const,
  customers: () => [...aenadQueryKeys.all, 'customers'] as const,
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
