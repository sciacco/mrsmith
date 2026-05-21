import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useApiClient } from './client';
import { mockWorkspace } from './mockData';
import type { ActionResponse, JobRunResponse, LookupResponse, WorkspaceResponse } from './types';

const useMocks =
  import.meta.env.DEV && import.meta.env.VITE_TRAINING_USE_MOCKS === 'true';

export type TrainingMasterDataKind =
  | 'vendors'
  | 'teams'
  | 'skill-areas'
  | 'certifications'
  | 'plans'
  | 'mandatory-rules';

export interface TrainingCoursePayload {
  title: string;
  vendorId?: string;
  skillAreaId?: string;
  leadsToCertId?: string;
  deliveryMode: string;
  providerKind: string;
  defaultHours?: number;
  defaultCost?: number;
  courseUrl?: string;
  description?: string;
  mandatory: boolean;
  recurrenceMonths?: number;
  complianceFramework?: string;
  active?: boolean;
}

export function useTrainingWorkspace(isPeopleAdmin: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'workspace', isPeopleAdmin, useMocks],
    queryFn: async (): Promise<WorkspaceResponse> => {
      if (useMocks) return mockWorkspace(isPeopleAdmin);
      const path = isPeopleAdmin ? '/training/v1/people/workspace' : '/training/v1/workspace';
      return api.get<WorkspaceResponse>(path);
    },
  });
}

export function useTrainingLookups(enabled: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'lookups', useMocks],
    enabled,
    queryFn: async (): Promise<LookupResponse> => {
      if (useMocks) {
        const workspace = mockWorkspace(true);
        return {
          employees: [
            { id: 'employee-dev', label: 'Doe John - john.doe@acme.com', active: true },
            { id: 'employee-laura', label: 'Bianchi Laura - laura.bianchi@cdlan.it', active: true },
          ],
          teams: [
            { id: 'team-cloud', label: 'CLOUD - Cloud Operations', active: true },
            { id: 'team-app', label: 'APPLICATIONS - Applications', active: true },
          ],
          vendors: [
            { id: 'vendor-linux', label: 'Linux Foundation', active: true },
            { id: 'vendor-hashicorp', label: 'HashiCorp', active: true },
            { id: 'vendor-internal', label: 'Internal Academy', active: true },
          ],
          skillAreas: [
            { id: 'skill-kubernetes', label: 'K8S - Kubernetes', active: true },
            { id: 'skill-iac', label: 'IAC - Infrastructure as Code', active: true },
            { id: 'skill-security', label: 'SEC - Security', active: true },
          ],
          courses: workspace.catalog.map((course) => ({ id: course.id, label: course.title, active: course.active })),
          certifications: [
            { id: 'cert-itil4', label: 'ITIL4 - ITIL 4 Foundation', active: true },
            { id: 'cert-cka', label: 'CKA - Certified Kubernetes Administrator', active: true },
            { id: 'cert-terraform', label: 'TF-A - Terraform Associate', active: true },
          ],
          plans: [{ id: 'plan-2026', label: '2026 - open', active: true }],
        };
      }
      return api.get<LookupResponse>('/training/v1/lookups');
    },
  });
}

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

export function useEnrollmentTransition(isPeopleAdmin: boolean) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, transition, reason }: { id: string; transition: string; reason?: string }) => {
      if (useMocks) return Promise.resolve({ ok: true, id, status: transition } satisfies ActionResponse);
      const path = isPeopleAdmin
        ? `/training/v1/people/enrollments/${id}/transition`
        : `/training/v1/enrollments/${id}/transition`;
      return api.post<ActionResponse>(path, { transition, reason });
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['training', 'workspace', isPeopleAdmin] }),
  });
}

export function useTrainingRequestAction(isPeopleAdmin: boolean) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      transition,
      reason,
      trainingPlanId,
    }: {
      id: string;
      transition: string;
      reason?: string;
      trainingPlanId?: string;
    }) => {
      if (useMocks) return Promise.resolve({ ok: true, id, status: transition } satisfies ActionResponse);
      return api.post<ActionResponse>(`/training/v1/requests/${id}/transition`, {
        transition,
        reason,
        trainingPlanId,
      });
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['training', 'workspace', isPeopleAdmin] }),
  });
}

export function useCreateTrainingRequest(isPeopleAdmin: boolean) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: { courseId?: string; freeTextTitle?: string; skillAreaId?: string; motivation: string; desiredYear?: number }) => {
      if (useMocks) return Promise.resolve({ ok: true, id: 'mock-request', status: 'submitted' } satisfies ActionResponse);
      return api.post<ActionResponse>('/training/v1/requests', body);
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['training', 'workspace', isPeopleAdmin] }),
  });
}

export function useCreateCourse(isPeopleAdmin: boolean) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: TrainingCoursePayload) => {
      if (useMocks) return Promise.resolve({ ok: true, id: 'mock-course' } satisfies ActionResponse);
      return api.post<ActionResponse>('/training/v1/people/courses', body);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['training', 'workspace', isPeopleAdmin] });
      queryClient.invalidateQueries({ queryKey: ['training', 'lookups'] });
    },
  });
}

export function useUpdateCourse(isPeopleAdmin: boolean) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: TrainingCoursePayload }) => {
      if (useMocks) return Promise.resolve({ ok: true, id } satisfies ActionResponse);
      return api.put<ActionResponse>(`/training/v1/people/courses/${id}`, body);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['training', 'workspace', isPeopleAdmin] });
      queryClient.invalidateQueries({ queryKey: ['training', 'lookups'] });
    },
  });
}

export function useCreateTrainingMasterData(isPeopleAdmin: boolean) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ kind, body }: { kind: TrainingMasterDataKind; body: Record<string, unknown> }) => {
      if (useMocks) return Promise.resolve({ ok: true, id: `mock-${kind}` } satisfies ActionResponse);
      return api.post<ActionResponse>(`/training/v1/people/${kind}`, body);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['training', 'workspace', isPeopleAdmin] });
      queryClient.invalidateQueries({ queryKey: ['training', 'lookups'] });
    },
  });
}

export function useUpdateTrainingMasterData(isPeopleAdmin: boolean) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ kind, id, body }: { kind: TrainingMasterDataKind; id: string; body: Record<string, unknown> }) => {
      if (useMocks) return Promise.resolve({ ok: true, id } satisfies ActionResponse);
      return api.put<ActionResponse>(`/training/v1/people/${kind}/${id}`, body);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['training', 'workspace', isPeopleAdmin] });
      queryClient.invalidateQueries({ queryKey: ['training', 'lookups'] });
    },
  });
}

export function useCreateAward(isPeopleAdmin: boolean) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: {
      employeeId?: string;
      certificationId: string;
      outcome: string;
      awardedOn: string;
      expiresOn?: string;
      validationSource?: string;
      notes?: string;
    }) => {
      if (useMocks) return Promise.resolve({ ok: true, id: 'mock-award', status: body.outcome } satisfies ActionResponse);
      return api.post<ActionResponse>('/training/v1/awards', body);
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['training', 'workspace', isPeopleAdmin] }),
  });
}

export function useUpdateAward(isPeopleAdmin: boolean) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      outcome,
      awardedOn,
      expiresOn,
      validationSource,
      notes,
    }: {
      id: string;
      outcome: string;
      awardedOn: string;
      expiresOn?: string;
      validationSource?: string;
      notes?: string;
    }) => {
      if (useMocks) return Promise.resolve({ ok: true, id, status: outcome } satisfies ActionResponse);
      return api.put<ActionResponse>(`/training/v1/people/awards/${id}`, {
        outcome,
        awardedOn,
        expiresOn,
        validationSource,
        notes,
      });
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['training', 'workspace', isPeopleAdmin] }),
  });
}

export function useCreateEnrollment(isPeopleAdmin: boolean) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: { employeeId: string; courseId: string; trainingPlanId: string }) => {
      if (useMocks) return Promise.resolve({ ok: true, id: 'mock-enrollment', status: 'proposed' } satisfies ActionResponse);
      return api.post<ActionResponse>('/training/v1/people/enrollments', body);
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['training', 'workspace', isPeopleAdmin] }),
  });
}

export function useUpdateEnrollment(isPeopleAdmin: boolean) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      body,
    }: {
      id: string;
      body: {
        priority?: number;
        levelAsIs?: number;
        levelToBe?: number;
        plannedStart?: string;
        plannedEnd?: string;
        hoursPlanned?: number;
        costPlanned?: number;
        motivation?: string;
        objective?: string;
        notes?: string;
      };
    }) => {
      if (useMocks) return Promise.resolve({ ok: true, id } satisfies ActionResponse);
      return api.put<ActionResponse>(`/training/v1/people/enrollments/${id}`, body);
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['training', 'workspace', isPeopleAdmin] }),
  });
}

export function useUploadEnrollmentDocument(isPeopleAdmin: boolean) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ enrollmentId, file }: { enrollmentId: string; file: File }) => {
      const body = new FormData();
      body.append('file', file);
      if (useMocks) return Promise.resolve({ id: 'mock-enrollment-document', filename: file.name });
      return api.postFormData(`/training/v1/enrollments/${enrollmentId}/documents`, body);
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['training', 'workspace', isPeopleAdmin] }),
  });
}

export function useUploadAwardDocument(isPeopleAdmin: boolean) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ awardId, file }: { awardId: string; file: File }) => {
      const body = new FormData();
      body.append('file', file);
      if (useMocks) return Promise.resolve({ id: 'mock-document', filename: file.name });
      return api.postFormData(`/training/v1/awards/${awardId}/documents`, body);
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['training', 'workspace', isPeopleAdmin] }),
  });
}

export function useValidateDocument(isPeopleAdmin: boolean) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (documentId: string) => {
      if (useMocks) return Promise.resolve({ ok: true, id: documentId } satisfies ActionResponse);
      return api.post<ActionResponse>(`/training/v1/people/documents/${documentId}/validate`);
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['training', 'workspace', isPeopleAdmin] }),
  });
}

export function useDownloadDocument() {
  const api = useApiClient();
  return useMutation({
    mutationFn: async ({ documentId, filename }: { documentId: string; filename: string }) => {
      if (useMocks) return undefined;
      const blob = await api.getBlob(`/training/v1/documents/${documentId}/download`);
      downloadBlob(blob, filename || 'attestato.pdf');
    },
  });
}

export function useRunTrainingJobs(isPeopleAdmin: boolean) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (): Promise<JobRunResponse> => {
      if (useMocks) {
        return Promise.resolve({
          ok: true,
          expiredEnrollments: 1,
          complianceNotifications: 2,
          certificationNotifications: 3,
        });
      }
      return api.post<JobRunResponse>('/training/v1/people/jobs/run');
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['training', 'workspace', isPeopleAdmin] }),
  });
}

export function useTrainingExport() {
  const api = useApiClient();
  return useMutation({
    mutationFn: async ({ kind, search }: { kind: string; search?: URLSearchParams }) => {
      const suffix = search?.toString();
      const path = `/training/v1/exports/${kind}.xlsx${suffix ? `?${suffix}` : ''}`;
      if (useMocks) return undefined;
      const blob = await api.getBlob(path);
      downloadBlob(blob, `formazione-${kind}.xlsx`);
    },
  });
}
