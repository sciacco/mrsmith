import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useApiClient } from './client';
import type {
  ActivationResponse,
  CustomerRef,
  OrderDetail,
  OrderRow,
  OrderSummary,
  RevertConversionResponse,
  SendToERPResponse,
  TechnicalRow,
  UpdateHeaderPayload,
  UpdateReferentsPayload,
} from './types';

export function useOrders() {
  const api = useApiClient();
  return useQuery<OrderSummary[]>({
    queryKey: ['ordini', 'orders'],
    queryFn: () => api.get<OrderSummary[]>('/ordini/v1/orders'),
  });
}

export function useOrder(id: number | null) {
  const api = useApiClient();
  return useQuery<OrderDetail>({
    queryKey: ['ordini', 'orders', id],
    queryFn: () => api.get<OrderDetail>(`/ordini/v1/orders/${id}`),
    enabled: id != null && Number.isFinite(id) && id > 0,
  });
}

export function useOrderRows(id: number | null) {
  const api = useApiClient();
  return useQuery<OrderRow[]>({
    queryKey: ['ordini', 'orders', id, 'rows'],
    queryFn: () => api.get<OrderRow[]>(`/ordini/v1/orders/${id}/rows`),
    enabled: id != null && Number.isFinite(id) && id > 0,
  });
}

export function useTechnicalRows(id: number | null) {
  const api = useApiClient();
  return useQuery<TechnicalRow[]>({
    queryKey: ['ordini', 'orders', id, 'technical-rows'],
    queryFn: () => api.get<TechnicalRow[]>(`/ordini/v1/orders/${id}/technical-rows`),
    enabled: id != null && Number.isFinite(id) && id > 0,
  });
}

export function useCustomers(enabled: boolean) {
  const api = useApiClient();
  return useQuery<CustomerRef[]>({
    queryKey: ['ordini', 'customers'],
    queryFn: () => api.get<CustomerRef[]>('/ordini/v1/ref/customers'),
    enabled,
    staleTime: 10 * 60 * 1000,
  });
}

export function usePatchOrderHeader(orderId: number) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: UpdateHeaderPayload) => api.patch<OrderDetail>(`/ordini/v1/orders/${orderId}`, payload),
    onSuccess: (order) => {
      queryClient.setQueryData(['ordini', 'orders', orderId], order);
      void queryClient.invalidateQueries({ queryKey: ['ordini', 'orders'] });
    },
  });
}

export function usePatchReferents(orderId: number) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: UpdateReferentsPayload) => api.patch<OrderDetail>(`/ordini/v1/orders/${orderId}/referents`, payload),
    onSuccess: (order) => {
      queryClient.setQueryData(['ordini', 'orders', orderId], order);
    },
  });
}

export function usePatchSerialNumber(orderId: number) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ rowId, serialNumber }: { rowId: number; serialNumber: string }) =>
      api.patch<OrderRow>(`/ordini/v1/orders/${orderId}/rows/${rowId}/serial-number`, {
        serial_number: serialNumber,
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['ordini', 'orders', orderId, 'rows'] });
    },
  });
}

export function usePatchTechnicalNotes(orderId: number) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ rowId, technicalNotes }: { rowId: number; technicalNotes: string }) =>
      api.patch<TechnicalRow>(`/ordini/v1/orders/${orderId}/rows/${rowId}/technical-notes`, {
        technical_notes: technicalNotes,
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['ordini', 'orders', orderId, 'technical-rows'] });
    },
  });
}

export function useActivateRow(orderId: number) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ rowId, activationDate }: { rowId: number; activationDate: string }) =>
      api.patch<ActivationResponse>(`/ordini/v1/orders/${orderId}/rows/${rowId}/activate`, {
        activation_date: activationDate,
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['ordini', 'orders', orderId] });
      void queryClient.invalidateQueries({ queryKey: ['ordini', 'orders', orderId, 'rows'] });
      void queryClient.invalidateQueries({ queryKey: ['ordini', 'orders'] });
    },
  });
}

export function useSendToERP(orderId: number) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (file: File) => {
      const form = new FormData();
      form.append('file', file);
      return api.postFormData<SendToERPResponse>(`/ordini/v1/orders/${orderId}/send-to-erp`, form);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['ordini', 'orders', orderId] });
      void queryClient.invalidateQueries({ queryKey: ['ordini', 'orders'] });
    },
  });
}

export function useRevertConversion(orderId: number) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<RevertConversionResponse>(`/ordini/v1/orders/${orderId}/revert-conversion`, {}),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['ordini', 'orders'] });
      void queryClient.invalidateQueries({ queryKey: ['ordini', 'orders', orderId] });
      void queryClient.invalidateQueries({ queryKey: ['ordini', 'orders', orderId, 'rows'] });
      void queryClient.invalidateQueries({ queryKey: ['ordini', 'orders', orderId, 'technical-rows'] });
    },
  });
}

export function useOrdiniDownloads() {
  const api = useApiClient();
  return {
    kickoff: (orderId: number) => api.getBlob(`/ordini/v1/orders/${orderId}/kickoff.pdf`),
    activationForm: (orderId: number) => api.getBlob(`/ordini/v1/orders/${orderId}/activation-form.pdf`),
    orderPdf: (orderId: number) => api.getBlob(`/ordini/v1/orders/${orderId}/pdf`),
    signedPdf: (orderId: number) => api.getBlob(`/ordini/v1/orders/${orderId}/signed-pdf`),
  };
}
