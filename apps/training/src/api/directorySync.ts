import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useApiClient } from './client';

export interface DirectorySyncSourceStats {
  people: number;
  teams: number;
  skippedNoLoginEmail: number;
  skippedDuplicate: number;
  skippedMembership: number;
}

export interface DirectorySyncStats {
  counts: Record<string, number>;
  samples: Record<string, string[]>;
  total: number;
  source: {
    fetchedAt?: string;
    stats: DirectorySyncSourceStats;
  };
}

export interface DirectorySyncRun {
  id: string;
  startedAt: string;
  finishedAt?: string;
  status: 'running' | 'ok' | 'failed';
  dryRun: boolean;
  actor: string;
  stats?: DirectorySyncStats;
  error?: string;
}

export function useDirectorySyncRuns(enabled: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'directory-sync', 'runs'],
    enabled,
    queryFn: async () => {
      const resp = await api.get<{ runs: DirectorySyncRun[] }>('/training/v1/directory/sync/runs');
      return resp.runs;
    },
  });
}

export function useRunDirectorySync() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (dryRun: boolean) =>
      api.post<DirectorySyncRun>('/training/v1/directory/sync', { dryRun }),
    onSuccess: (_run, dryRun) => {
      queryClient.invalidateQueries({ queryKey: ['training', 'directory-sync', 'runs'] });
      if (dryRun) return;
      queryClient.invalidateQueries({ queryKey: ['training', 'people-directory'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'person-profile'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'lookups'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'workspace'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'overview'] });
    },
  });
}
