// Letture del sync formativo Factorial persistito (migrazione 133, #154):
// stesso pattern di directorySync.ts. Nessuna mutazione qui — l'avvio resta
// cablato solo su job runner e CLI (docs/knowledge/training.md).

import { useQuery } from '@tanstack/react-query';
import { useApiClient } from './client';

export interface FactorialSyncRunRecord {
  id: string;
  startedAt: string;
  finishedAt?: string;
  durationMs: number;
  actor: string;
  dryRun: boolean;
  outcome: string;
  error?: string;
  sessionsWithoutClass: number;
  counters: Record<string, number>;
  findingCount: number;
}

export interface FactorialSyncFindingRecord {
  id: string;
  phase: string;
  severity: string;
  kind: string;
  ref: string;
  localEntity?: string;
  localId?: string;
  localEventId?: string;
  employeeId?: string;
  employeeName?: string;
  detail?: Record<string, unknown>;
}

export interface FactorialSyncRunDetail extends FactorialSyncRunRecord {
  findings: FactorialSyncFindingRecord[];
}

export function useFactorialSyncRuns(enabled: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'factorial-sync', 'runs'],
    enabled,
    queryFn: async () => {
      const resp = await api.get<{ runs: FactorialSyncRunRecord[] }>('/training/v1/factorial/sync/runs');
      return resp.runs;
    },
  });
}

export function useFactorialSyncRun(id: string | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'factorial-sync', 'runs', id],
    enabled: id !== null,
    queryFn: () => api.get<FactorialSyncRunDetail>(`/training/v1/factorial/sync/runs/${id}`),
  });
}
