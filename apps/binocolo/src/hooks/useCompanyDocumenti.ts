import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ApiError } from '@mrsmith/api-client';
import { useApiClient } from '../api/client';
import type { MACardDriveFolder, MACompanyDocuments } from '../api/types';

// Query-key factory for the company Drive documents listing (mirrors
// useCompanyContacts). Used both by the panel and to invalidate after a card
// subfolder ensure, so the Scheda listing refreshes when a subfolder is born.
// The lens initiative is part of the key: it changes the response
// (cardFolderId). The prefix-invalidation after an ensure still covers every
// lens variant.
export const companyDocumentiKey = (companyKey: string, initiativeId?: string) =>
  ['ma-company-documenti', companyKey, initiativeId ?? ''] as const;

// Drive error codes that are terminal from the UI point of view: they can't be
// fixed by retrying (config missing, folder trashed/moved/deleted, access
// revoked). Shared with the panel so the two surfaces never drift on which
// codes settle.
export const TERMINAL_DRIVE_CODES = new Set([
  'googledrive_not_configured',
  'googledrive_trashed',
  'googledrive_outside_context_root',
  'googledrive_not_found',
]);

/** Extracts the backend error-code string from an ApiError, or '' otherwise. */
export function driveErrorCode(err: unknown): string {
  if (err instanceof ApiError) {
    const body = err.body as { error?: string } | undefined;
    return body?.error ?? '';
  }
  return '';
}

export function useCompanyDocumenti(companyKey: string, initiativeId?: string) {
  const api = useApiClient();
  return useQuery({
    queryKey: companyDocumentiKey(companyKey, initiativeId),
    queryFn: () =>
      api.get<MACompanyDocuments>(
        `/binocolo/v1/ma/companies/${encodeURIComponent(companyKey)}/documents${
          initiativeId ? `?initiativeId=${encodeURIComponent(initiativeId)}` : ''
        }`,
      ),
    enabled: Boolean(companyKey),
    // Terminal errors (missing config, trashed/moved folder) must surface
    // immediately instead of being retried a few times first.
    retry: (_failureCount, err) => !TERMINAL_DRIVE_CODES.has(driveErrorCode(err)),
  });
}

/**
 * Idempotent ensure of a card's Drive subfolder (POST). On success the company
 * documents listing is invalidated: a newly created subfolder must appear in the
 * Scheda without a manual refresh.
 */
export function useEnsureMACardDriveFolder() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation<MACardDriveFolder, Error, { initiativeId: string; companyKey: string }>({
    mutationFn: ({ initiativeId, companyKey }) =>
      api.post<MACardDriveFolder>(
        `/binocolo/v1/ma/initiatives/${encodeURIComponent(initiativeId)}/cards/${encodeURIComponent(companyKey)}/drive-folder`,
      ),
    onSuccess: (_data, { companyKey }) => {
      // Prefix match: covers the listing under every lens variant of the key.
      void queryClient.invalidateQueries({ queryKey: ['ma-company-documenti', companyKey] });
    },
  });
}
