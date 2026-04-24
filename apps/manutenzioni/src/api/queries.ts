import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useManutenzioniApiClient } from './client';
import type {
  AssistancePreviewRequest,
  AssistancePreviewResponse,
  ClassificationInput,
  CustomerSearchItem,
  ImpactedCustomerBody,
  LLMModel,
  MaintenanceAssistanceDraft,
  MaintenanceAssistanceDraftBody,
  MaintenanceDetail,
  MaintenanceFilters,
  MaintenanceFormBody,
  MaintenanceListItem,
  MaintenancePatchBody,
  NoticeBody,
  PagedResponse,
  ReferenceData,
  ReferenceItem,
  ServiceDependency,
  ServiceDependencyBody,
  TargetBody,
  WindowBody,
} from './types';

const queryKeys = {
  list: (params: MaintenanceFilters) => ['manutenzioni', 'list', params] as const,
  detail: (id: number) => ['manutenzioni', 'detail', id] as const,
  reference: (id?: number) => ['manutenzioni', 'reference', id ?? 'new'] as const,
  customers: (q: string) => ['manutenzioni', 'customers', q] as const,
  config: (resource: string, active: string, q: string) =>
    ['manutenzioni', 'config', resource, active, q] as const,
  configSummary: () => ['manutenzioni', 'config-summary'] as const,
  configUsage: (resource: string, id: number) =>
    ['manutenzioni', 'config-usage', resource, id] as const,
  llmModels: () => ['manutenzioni', 'llm-models'] as const,
  serviceDependencies: (active: string, q: string) =>
    ['manutenzioni', 'service-dependencies', active, q] as const,
};

export type ConfigResourceCounts = { active: number; inactive: number };
export type ConfigSummary = Record<string, ConfigResourceCounts>;
export type ConfigUsage = { active_maintenances: number };

function searchParams(params: MaintenanceFilters): string {
  const search = new URLSearchParams();
  if (params.q) search.set('q', params.q);
  if (params.status?.length) search.set('status', params.status.join(','));
  if (params.scheduled_from) search.set('scheduled_from', params.scheduled_from);
  if (params.scheduled_to) search.set('scheduled_to', params.scheduled_to);
  if (params.technical_domain_id) search.set('technical_domain_id', params.technical_domain_id);
  if (params.maintenance_kind_id) search.set('maintenance_kind_id', params.maintenance_kind_id);
  if (params.customer_scope_id) search.set('customer_scope_id', params.customer_scope_id);
  if (params.site_id) search.set('site_id', params.site_id);
  if (params.page) search.set('page', String(params.page));
  if (params.page_size) search.set('page_size', String(params.page_size));
  return search.toString();
}

export function useMaintenances(params: MaintenanceFilters) {
  const api = useManutenzioniApiClient();
  return useQuery({
    queryKey: queryKeys.list(params),
    queryFn: () =>
      api.get<PagedResponse<MaintenanceListItem>>(
        `/manutenzioni/v1/maintenances?${searchParams(params)}`,
      ),
  });
}

export function useMaintenance(id: number | null) {
  const api = useManutenzioniApiClient();
  return useQuery({
    queryKey: id ? queryKeys.detail(id) : ['manutenzioni', 'detail', 'empty'],
    enabled: id !== null,
    queryFn: () => api.get<MaintenanceDetail>(`/manutenzioni/v1/maintenances/${id}`),
  });
}

export function useReferenceData(maintenanceId?: number) {
  const api = useManutenzioniApiClient();
  const suffix = maintenanceId ? `?maintenance_id=${maintenanceId}` : '';
  return useQuery({
    queryKey: queryKeys.reference(maintenanceId),
    queryFn: () => api.get<ReferenceData>(`/manutenzioni/v1/reference-data${suffix}`),
  });
}

export function useCustomerSearch(q: string, enabled: boolean) {
  const api = useManutenzioniApiClient();
  return useQuery({
    queryKey: queryKeys.customers(q),
    enabled: enabled && q.trim().length >= 2,
    queryFn: () =>
      api.get<CustomerSearchItem[]>(
        `/manutenzioni/v1/customers?q=${encodeURIComponent(q)}&page_size=20`,
      ),
  });
}

export function useCreateMaintenance() {
  const api = useManutenzioniApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: MaintenanceFormBody) =>
      api.post<MaintenanceDetail>('/manutenzioni/v1/maintenances', body),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['manutenzioni', 'list'] });
    },
  });
}

export function useMaintenanceAssistanceDraft(id: number) {
  const api = useManutenzioniApiClient();
  return useMutation({
    mutationFn: (body: MaintenanceAssistanceDraftBody) =>
      api.post<MaintenanceAssistanceDraft>(
        `/manutenzioni/v1/maintenances/${id}/assistance/draft`,
        body,
      ),
  });
}

export function useMaintenanceAssistancePreview() {
  const api = useManutenzioniApiClient();
  return useMutation({
    mutationFn: (body: AssistancePreviewRequest) =>
      api.post<AssistancePreviewResponse>(
        '/manutenzioni/v1/maintenances/assistance/preview',
        body,
      ),
  });
}

export function useUpdateMaintenance() {
  const api = useManutenzioniApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: number; body: MaintenancePatchBody }) =>
      api.patch<MaintenanceDetail>(`/manutenzioni/v1/maintenances/${id}`, body),
    onSuccess: async (_data, variables) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.detail(variables.id) });
      await queryClient.invalidateQueries({ queryKey: ['manutenzioni', 'list'] });
    },
  });
}

export function useStatusAction() {
  const api = useManutenzioniApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      action,
      reason_it,
    }: {
      id: number;
      action: string;
      reason_it?: string;
    }) =>
      api.post<MaintenanceDetail>(`/manutenzioni/v1/maintenances/${id}/status`, {
        action,
        reason_it,
      }),
    onSuccess: async (_data, variables) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.detail(variables.id) });
      await queryClient.invalidateQueries({ queryKey: ['manutenzioni', 'list'] });
    },
  });
}

export function useWindowMutations(id: number) {
  const api = useManutenzioniApiClient();
  const queryClient = useQueryClient();
  const invalidate = async () => {
    await queryClient.invalidateQueries({ queryKey: queryKeys.detail(id) });
    await queryClient.invalidateQueries({ queryKey: ['manutenzioni', 'list'] });
  };
  return {
    create: useMutation({
      mutationFn: (body: WindowBody) =>
        api.post<MaintenanceDetail>(`/manutenzioni/v1/maintenances/${id}/windows`, body),
      onSuccess: invalidate,
    }),
    update: useMutation({
      mutationFn: ({ windowId, body }: { windowId: number; body: WindowBody }) =>
        api.patch<MaintenanceDetail>(
          `/manutenzioni/v1/maintenances/${id}/windows/${windowId}`,
          body,
        ),
      onSuccess: invalidate,
    }),
    reschedule: useMutation({
      mutationFn: (body: WindowBody) =>
        api.post<MaintenanceDetail>(
          `/manutenzioni/v1/maintenances/${id}/windows/reschedule`,
          body,
        ),
      onSuccess: invalidate,
    }),
    cancel: useMutation({
      mutationFn: ({ windowId, reason_it }: { windowId: number; reason_it: string }) =>
        api.post<MaintenanceDetail>(
          `/manutenzioni/v1/maintenances/${id}/windows/${windowId}/cancel`,
          { reason_it },
        ),
      onSuccess: invalidate,
    }),
  };
}

export function useReplaceClassifications(id: number, resource: string) {
  const api = useManutenzioniApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (items: ClassificationInput[]) =>
      api.put<MaintenanceDetail>(`/manutenzioni/v1/maintenances/${id}/${resource}`, { items }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.detail(id) });
      await queryClient.invalidateQueries({ queryKey: ['manutenzioni', 'list'] });
    },
  });
}

export function useTargetMutations(id: number) {
  const api = useManutenzioniApiClient();
  const queryClient = useQueryClient();
  const invalidate = async () => {
    await queryClient.invalidateQueries({ queryKey: queryKeys.detail(id) });
    await queryClient.invalidateQueries({ queryKey: ['manutenzioni', 'list'] });
  };
  return {
    create: useMutation({
      mutationFn: (body: TargetBody) =>
        api.post<MaintenanceDetail>(`/manutenzioni/v1/maintenances/${id}/targets`, body),
      onSuccess: invalidate,
    }),
    update: useMutation({
      mutationFn: ({ targetId, body }: { targetId: number; body: TargetBody }) =>
        api.patch<MaintenanceDetail>(
          `/manutenzioni/v1/maintenances/${id}/targets/${targetId}`,
          body,
        ),
      onSuccess: invalidate,
    }),
    remove: useMutation({
      mutationFn: (targetId: number) =>
        api.delete<MaintenanceDetail>(`/manutenzioni/v1/maintenances/${id}/targets/${targetId}`),
      onSuccess: invalidate,
    }),
  };
}

export function useCustomerImpactMutations(id: number) {
  const api = useManutenzioniApiClient();
  const queryClient = useQueryClient();
  const invalidate = async () => {
    await queryClient.invalidateQueries({ queryKey: queryKeys.detail(id) });
    await queryClient.invalidateQueries({ queryKey: ['manutenzioni', 'list'] });
  };
  return {
    create: useMutation({
      mutationFn: (body: ImpactedCustomerBody) =>
        api.post<MaintenanceDetail>(
          `/manutenzioni/v1/maintenances/${id}/impacted-customers`,
          body,
        ),
      onSuccess: invalidate,
    }),
    update: useMutation({
      mutationFn: ({
        customerImpactId,
        body,
      }: {
        customerImpactId: number;
        body: ImpactedCustomerBody;
      }) =>
        api.patch<MaintenanceDetail>(
          `/manutenzioni/v1/maintenances/${id}/impacted-customers/${customerImpactId}`,
          body,
        ),
      onSuccess: invalidate,
    }),
    remove: useMutation({
      mutationFn: (customerImpactId: number) =>
        api.delete<MaintenanceDetail>(
          `/manutenzioni/v1/maintenances/${id}/impacted-customers/${customerImpactId}`,
        ),
      onSuccess: invalidate,
    }),
  };
}

export function useNoticeMutations(id: number) {
  const api = useManutenzioniApiClient();
  const queryClient = useQueryClient();
  const invalidate = async () => {
    await queryClient.invalidateQueries({ queryKey: queryKeys.detail(id) });
    await queryClient.invalidateQueries({ queryKey: ['manutenzioni', 'list'] });
  };
  return {
    create: useMutation({
      mutationFn: (body: NoticeBody) =>
        api.post<MaintenanceDetail>(`/manutenzioni/v1/maintenances/${id}/notices`, body),
      onSuccess: invalidate,
    }),
    update: useMutation({
      mutationFn: ({ noticeId, body }: { noticeId: number; body: NoticeBody }) =>
        api.patch<MaintenanceDetail>(
          `/manutenzioni/v1/maintenances/${id}/notices/${noticeId}`,
          body,
        ),
      onSuccess: invalidate,
    }),
    status: useMutation({
      mutationFn: ({
        noticeId,
        send_status,
        sent_at,
      }: {
        noticeId: number;
        send_status: string;
        sent_at?: string | null;
      }) =>
        api.post<MaintenanceDetail>(
          `/manutenzioni/v1/maintenances/${id}/notices/${noticeId}/status`,
          { send_status, sent_at },
        ),
      onSuccess: invalidate,
    }),
  };
}

export function useConfigList(resource: string, active: string, q: string) {
  const api = useManutenzioniApiClient();
  return useQuery({
    queryKey: queryKeys.config(resource, active, q),
    queryFn: () =>
      api.get<ReferenceItem[]>(
        `/manutenzioni/v1/config/${resource}?active=${encodeURIComponent(active)}&q=${encodeURIComponent(q)}`,
      ),
  });
}

export function useConfigSummary() {
  const api = useManutenzioniApiClient();
  return useQuery({
    queryKey: queryKeys.configSummary(),
    queryFn: () => api.get<ConfigSummary>('/manutenzioni/v1/config/summary'),
  });
}

export function useConfigCounts(resource: string) {
  const summary = useConfigSummary();
  const data = summary.data?.[resource];
  return {
    data: data ?? null,
    isLoading: summary.isLoading,
    error: summary.error,
    refetch: summary.refetch,
  };
}

export function useConfigUsage(resource: string, id: number | null) {
  const api = useManutenzioniApiClient();
  return useQuery({
    queryKey: id !== null ? queryKeys.configUsage(resource, id) : ['manutenzioni', 'config-usage', 'empty'],
    enabled: id !== null,
    queryFn: () =>
      api.get<ConfigUsage>(`/manutenzioni/v1/config/${resource}/${id}/usage`),
  });
}

export function useConfigMutations(resource: string) {
  const api = useManutenzioniApiClient();
  const queryClient = useQueryClient();
  const invalidate = async () => {
    await queryClient.invalidateQueries({ queryKey: ['manutenzioni', 'config', resource] });
    await queryClient.invalidateQueries({ queryKey: ['manutenzioni', 'config-summary'] });
    await queryClient.invalidateQueries({ queryKey: ['manutenzioni', 'config-usage', resource] });
    await queryClient.invalidateQueries({ queryKey: ['manutenzioni', 'reference'] });
  };
  return {
    create: useMutation({
      mutationFn: (body: Partial<ReferenceItem>) =>
        api.post<ReferenceItem>(`/manutenzioni/v1/config/${resource}`, body),
      onSuccess: invalidate,
    }),
    update: useMutation({
      mutationFn: ({ id, body }: { id: number; body: Partial<ReferenceItem> }) =>
        api.patch<ReferenceItem>(`/manutenzioni/v1/config/${resource}/${id}`, body),
      onSuccess: invalidate,
    }),
    deactivate: useMutation({
      mutationFn: (id: number) =>
        api.post<ReferenceItem>(`/manutenzioni/v1/config/${resource}/${id}/deactivate`),
      onSuccess: invalidate,
    }),
    reactivate: useMutation({
      mutationFn: (id: number) =>
        api.post<ReferenceItem>(`/manutenzioni/v1/config/${resource}/${id}/reactivate`),
      onSuccess: invalidate,
    }),
    reorder: useMutation({
      mutationFn: (items: Array<{ id: number; sort_order: number }>) =>
        api.post<{ ok: boolean }>(`/manutenzioni/v1/config/${resource}/reorder`, { items }),
      onMutate: async (items) => {
        await queryClient.cancelQueries({ queryKey: ['manutenzioni', 'config', resource] });
        const orderById = new Map(items.map((it, index) => [it.id, index]));
        const snapshots = queryClient
          .getQueriesData<ReferenceItem[]>({ queryKey: ['manutenzioni', 'config', resource] })
          .map(([key, data]) => {
            if (!data) return [key, data] as const;
            const next = [...data].sort((a, b) => {
              const ai = orderById.get(a.id);
              const bi = orderById.get(b.id);
              if (ai === undefined && bi === undefined) return 0;
              if (ai === undefined) return 1;
              if (bi === undefined) return -1;
              return ai - bi;
            });
            queryClient.setQueryData(key, next);
            return [key, data] as const;
          });
        return { snapshots };
      },
      onError: (_err, _vars, context) => {
        context?.snapshots.forEach(([key, data]) => queryClient.setQueryData(key, data));
      },
      onSettled: invalidate,
    }),
  };
}

export function useLLMModels() {
  const api = useManutenzioniApiClient();
  return useQuery({
    queryKey: queryKeys.llmModels(),
    queryFn: () => api.get<LLMModel[]>('/manutenzioni/v1/llm-models'),
  });
}

export function useLLMModelMutations() {
  const api = useManutenzioniApiClient();
  const queryClient = useQueryClient();
  const invalidate = async () => {
    await queryClient.invalidateQueries({ queryKey: queryKeys.llmModels() });
  };
  return {
    create: useMutation({
      mutationFn: (body: LLMModel) =>
        api.post<LLMModel>('/manutenzioni/v1/llm-models', body),
      onSuccess: invalidate,
    }),
    update: useMutation({
      mutationFn: ({ scope, model }: LLMModel) =>
        api.patch<LLMModel>(
          `/manutenzioni/v1/llm-models/${encodeURIComponent(scope)}`,
          { scope, model },
        ),
      onSuccess: invalidate,
    }),
  };
}

export function useServiceDependencies(active = 'active', q = '') {
  const api = useManutenzioniApiClient();
  return useQuery({
    queryKey: queryKeys.serviceDependencies(active, q),
    queryFn: () =>
      api.get<ServiceDependency[]>(
        `/manutenzioni/v1/service-dependencies?active=${encodeURIComponent(active)}&q=${encodeURIComponent(q)}`,
      ),
  });
}

export function useServiceDependencyMutations() {
  const api = useManutenzioniApiClient();
  const queryClient = useQueryClient();
  const invalidate = async () => {
    await queryClient.invalidateQueries({ queryKey: ['manutenzioni', 'service-dependencies'] });
  };
  return {
    create: useMutation({
      mutationFn: (body: ServiceDependencyBody) =>
        api.post<ServiceDependency>('/manutenzioni/v1/service-dependencies', body),
      onSuccess: invalidate,
    }),
    update: useMutation({
      mutationFn: ({ id, body }: { id: number; body: ServiceDependencyBody }) =>
        api.patch<ServiceDependency>(`/manutenzioni/v1/service-dependencies/${id}`, body),
      onSuccess: invalidate,
    }),
    deactivate: useMutation({
      mutationFn: (id: number) =>
        api.post<ServiceDependency>(`/manutenzioni/v1/service-dependencies/${id}/deactivate`),
      onSuccess: invalidate,
    }),
    reactivate: useMutation({
      mutationFn: (id: number) =>
        api.post<ServiceDependency>(`/manutenzioni/v1/service-dependencies/${id}/reactivate`),
      onSuccess: invalidate,
    }),
  };
}
