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
  id?: string;
  accessId?: string;
  trainingId?: string;
  employeeId: string;
  employeeName?: string;
  status?: string;
  dueDate?: string;
  completedAt?: string;
}

export interface FactorialTrainingClass {
  id: string;
  trainingId?: string;
  name?: string;
  description?: string;
  startDate?: string;
  endDate?: string;
  cost?: string;
  indirectCost?: string;
  salaryCost?: string;
  subsidizedCost?: string;
  grossCost?: string;
  netCost?: string;
  currency?: string;
  paymentStatus?: string;
  completedAttendancesCount?: number;
  totalAttendancesCount?: number;
}

export interface FactorialSession {
  id: string;
  trainingId?: string;
  trainingClassId?: string;
  name?: string;
  description?: string;
  startsAt?: string;
  endsAt?: string;
  dueDate?: string;
  duration?: string;
  modality?: string;
  schedule?: string;
  location?: string;
  status?: string;
  parentId?: string;
}

export interface FactorialTrainingStructure {
  classes: FactorialTrainingClass[];
  sessions: FactorialSession[];
}

export interface FactorialSessionAttendance {
  id: string;
  sessionAccessMembershipId?: string;
  accessId?: string;
  employeeId?: string;
  status?: string;
  completedDuration?: string;
}

export interface FactorialSessionParticipant {
  sessionAccessMembershipId: string;
  sessionId?: string;
  accessId?: string;
  employeeId?: string;
  firstName?: string;
  lastName?: string;
  jobTitle?: string;
  attendances: FactorialSessionAttendance[];
}

export interface FactorialSessionParticipants {
  participants: FactorialSessionParticipant[];
  unmatchedAttendances: FactorialSessionAttendance[];
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

export function useFactorialTrainingStructure(trainingId: string | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'factorial', 'structure', trainingId],
    enabled: trainingId !== null,
    queryFn: async () => {
      const resp = await api.get<FactorialTrainingStructure>(
        `/training/v1/factorial/trainings/${trainingId}/structure`,
      );
      return resp;
    },
  });
}

export function useFactorialSessionParticipants(sessionId: string | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'factorial', 'participants', sessionId],
    enabled: sessionId !== null,
    queryFn: async () => {
      const resp = await api.get<FactorialSessionParticipants>(
        `/training/v1/factorial/sessions/${sessionId}/participants`,
      );
      return resp;
    },
  });
}
