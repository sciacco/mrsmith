import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useApiClient } from './client';
import type {
  BlockRequest,
  BlockDomain,
  ReleaseRequest,
  ReleaseDomain,
  Origin,
  DomainStatus,
  HistoryEntry,
} from './types';

export const complianceKeys = {
  blocks: ['compliance', 'blocks'] as const,
  block: (id: number) => ['compliance', 'blocks', id] as const,
  blockDomains: (id: number) => ['compliance', 'blocks', id, 'domains'] as const,
  releases: ['compliance', 'releases'] as const,
  release: (id: number) => ['compliance', 'releases', id] as const,
  releaseDomains: (id: number) => ['compliance', 'releases', id, 'domains'] as const,
  domains: (status: string) => ['compliance', 'domains', status] as const,
  history: ['compliance', 'domains', 'history'] as const,
  origins: ['compliance', 'origins'] as const,
};

// ── Blocks ──

export function useBlocks() {
  const api = useApiClient();
  return useQuery({
    queryKey: complianceKeys.blocks,
    queryFn: () => api.get<BlockRequest[]>('/compliance/blocks'),
  });
}

export function useBlock(id: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: complianceKeys.block(id!),
    queryFn: () => api.get<BlockRequest>(`/compliance/blocks/${id}`),
    enabled: id != null,
  });
}

export function useBlockDomains(id: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: complianceKeys.blockDomains(id!),
    queryFn: () => api.get<BlockDomain[]>(`/compliance/blocks/${id}/domains`),
    enabled: id != null,
  });
}

export function useCreateBlock() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { request_date: string; reference: string; method_id: string; domains: string[] }) =>
      api.post<{ id: number; domains_count: number }>('/compliance/blocks', body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: complianceKeys.blocks });
      qc.invalidateQueries({ queryKey: ['compliance', 'domains'] });
      qc.invalidateQueries({ queryKey: complianceKeys.history });
    },
  });
}

export function useUpdateBlock() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, ...body }: { id: number; request_date: string; reference: string; method_id: string }) =>
      api.put<{ id: number }>(`/compliance/blocks/${id}`, body),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: complianceKeys.blocks });
      qc.invalidateQueries({ queryKey: complianceKeys.block(vars.id) });
    },
  });
}

export function useAddBlockDomains() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ blockId, domains }: { blockId: number; domains: string[] }) =>
      api.post<{ added_count: number }>(`/compliance/blocks/${blockId}/domains`, { domains }),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: complianceKeys.blockDomains(vars.blockId) });
      qc.invalidateQueries({ queryKey: ['compliance', 'domains'] });
      qc.invalidateQueries({ queryKey: complianceKeys.history });
    },
  });
}

export function useUpdateBlockDomain() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ blockId, domainId, domain }: { blockId: number; domainId: number; domain: string }) =>
      api.put<{ id: number; domain: string }>(`/compliance/blocks/${blockId}/domains/${domainId}`, { domain }),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: complianceKeys.blockDomains(vars.blockId) });
      qc.invalidateQueries({ queryKey: ['compliance', 'domains'] });
      qc.invalidateQueries({ queryKey: complianceKeys.history });
    },
  });
}

// ── Releases ──

export function useReleases() {
  const api = useApiClient();
  return useQuery({
    queryKey: complianceKeys.releases,
    queryFn: () => api.get<ReleaseRequest[]>('/compliance/releases'),
  });
}

export function useRelease(id: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: complianceKeys.release(id!),
    queryFn: () => api.get<ReleaseRequest>(`/compliance/releases/${id}`),
    enabled: id != null,
  });
}

export function useReleaseDomains(id: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: complianceKeys.releaseDomains(id!),
    queryFn: () => api.get<ReleaseDomain[]>(`/compliance/releases/${id}/domains`),
    enabled: id != null,
  });
}

export function useCreateRelease() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { request_date: string; reference: string; domains: string[] }) =>
      api.post<{ id: number; domains_count: number }>('/compliance/releases', body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: complianceKeys.releases });
      qc.invalidateQueries({ queryKey: ['compliance', 'domains'] });
      qc.invalidateQueries({ queryKey: complianceKeys.history });
    },
  });
}

export function useUpdateRelease() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, ...body }: { id: number; request_date: string; reference: string }) =>
      api.put<{ id: number }>(`/compliance/releases/${id}`, body),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: complianceKeys.releases });
      qc.invalidateQueries({ queryKey: complianceKeys.release(vars.id) });
    },
  });
}

export function useAddReleaseDomains() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ releaseId, domains }: { releaseId: number; domains: string[] }) =>
      api.post<{ added_count: number }>(`/compliance/releases/${releaseId}/domains`, { domains }),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: complianceKeys.releaseDomains(vars.releaseId) });
      qc.invalidateQueries({ queryKey: ['compliance', 'domains'] });
      qc.invalidateQueries({ queryKey: complianceKeys.history });
    },
  });
}

export function useUpdateReleaseDomain() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ releaseId, domainId, domain }: { releaseId: number; domainId: number; domain: string }) =>
      api.put<{ id: number; domain: string }>(`/compliance/releases/${releaseId}/domains/${domainId}`, { domain }),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: complianceKeys.releaseDomains(vars.releaseId) });
      qc.invalidateQueries({ queryKey: ['compliance', 'domains'] });
      qc.invalidateQueries({ queryKey: complianceKeys.history });
    },
  });
}

// ── Domain Status & History ──

export function useDomainStatus() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['compliance', 'domains', 'all'],
    queryFn: () => api.get<DomainStatus[]>('/compliance/domains'),
  });
}

export function useHistory() {
  const api = useApiClient();
  return useQuery({
    queryKey: complianceKeys.history,
    queryFn: () => api.get<HistoryEntry[]>('/compliance/domains/history'),
  });
}

// ── Origins ──

export function useOrigins(includeInactive?: boolean) {
  const api = useApiClient();
  const path = includeInactive ? '/compliance/origins?include_inactive=true' : '/compliance/origins';
  return useQuery({
    queryKey: [...complianceKeys.origins, includeInactive ? 'all' : 'active'],
    queryFn: () => api.get<Origin[]>(path),
  });
}

export function useCreateOrigin() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { method_id: string; description: string }) =>
      api.post<{ method_id: string }>('/compliance/origins', body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: complianceKeys.origins });
    },
  });
}

export function useUpdateOrigin() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ methodId, description }: { methodId: string; description: string }) =>
      api.put<{ method_id: string }>(`/compliance/origins/${methodId}`, { description }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: complianceKeys.origins });
    },
  });
}

export function useDeleteOrigin() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (methodId: string) =>
      api.delete<{ method_id: string }>(`/compliance/origins/${methodId}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: complianceKeys.origins });
    },
  });
}

export function useEnableOrigin() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (methodId: string) =>
      api.put<{ method_id: string }>(`/compliance/origins/${methodId}`, { is_active: true }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: complianceKeys.origins });
    },
  });
}
