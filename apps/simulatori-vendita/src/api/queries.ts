import { useMutation } from '@tanstack/react-query';
import { useApiClient } from './client';
import type { QuotePayload } from './types';

export function useGenerateQuote() {
  const api = useApiClient();

  return useMutation({
    mutationFn: (body: QuotePayload) => api.postBlob('/simulatori-vendita/v1/iaas/quote', body),
  });
}
