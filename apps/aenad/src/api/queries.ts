import { useQuery } from '@tanstack/react-query';
import { ApiError } from '@mrsmith/api-client';
import { useApiClient } from './client';
import type { AenadDocumentsPage, ArchiveDocumentFilters, DocumentTypeOption, AenadDocumentRow } from './types';

export const aenadQueryKeys = {
  all: ['aenad'] as const,
  documentTypes: () => [...aenadQueryKeys.all, 'document-types'] as const,
  documents: (filters: ArchiveDocumentFilters) => [...aenadQueryKeys.all, 'documents', filters] as const,
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
