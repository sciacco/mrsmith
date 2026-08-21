import { useQuery } from '@tanstack/react-query';
import { useApiClient } from './client';

export interface FactorialStatus {
  configured: boolean;
  ok: boolean;
  error?: string;
}

export interface FactorialPersonRef {
  id: string;
  name: string;
}

export interface FactorialEmployee {
  id: string;
  fullName: string;
  email?: string;
  loginEmail?: string;
  manager?: FactorialPersonRef;
  active: boolean;
  terminatedOn?: string;
}

export interface FactorialTeam {
  id: string;
  name: string;
  description?: string;
  members: FactorialPersonRef[];
  leads: FactorialPersonRef[];
}

export interface FactorialTraining {
  id: string;
  name: string;
  code?: string;
  year?: number;
  status?: string;
  catalog: boolean;
  external: boolean;
  provider?: string;
  cost?: string;
  categories?: string[];
}

export interface FactorialMembership {
  employeeId: string;
  employeeName?: string;
  status?: string;
  dueDate?: string;
  completedAt?: string;
}

export function useFactorialStatus(enabled: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'factorial', 'status'],
    enabled,
    queryFn: () => api.get<FactorialStatus>('/training/v1/factorial/status'),
  });
}

export function useFactorialEmployees(enabled: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'factorial', 'employees'],
    enabled,
    queryFn: async () => {
      const resp = await api.get<{ employees: FactorialEmployee[] }>('/training/v1/factorial/employees');
      return resp.employees;
    },
  });
}

export function useFactorialTeams(enabled: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'factorial', 'teams'],
    enabled,
    queryFn: async () => {
      const resp = await api.get<{ teams: FactorialTeam[] }>('/training/v1/factorial/teams');
      return resp.teams;
    },
  });
}

export function useFactorialTrainings(enabled: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'factorial', 'trainings'],
    enabled,
    queryFn: async () => {
      const resp = await api.get<{ trainings: FactorialTraining[] }>('/training/v1/factorial/trainings');
      return resp.trainings;
    },
  });
}

export function useFactorialTrainingMemberships(trainingId: string | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'factorial', 'memberships', trainingId],
    enabled: trainingId !== null,
    queryFn: async () => {
      const resp = await api.get<{ memberships: FactorialMembership[] }>(
        `/training/v1/factorial/trainings/${trainingId}/memberships`,
      );
      return resp.memberships;
    },
  });
}
