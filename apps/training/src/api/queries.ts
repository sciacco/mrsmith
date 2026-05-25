import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useApiClient } from './client';
import { mockCustomGroups, mockMandatoryRules, mockPeopleDirectory, mockWorkspace } from './mockData';
import type {
  ActionResponse,
  BulkAssignResponse,
  BulkPlanFromSuggestionInput,
  BulkReviewEmployeeRequestsInput,
  BulkReviewEmployeeRequestsResponse,
  BulkTargetState,
  BulkTransitionResponse,
  CatalogListResponse,
  ComplianceOverviewResponse,
  CreatePlanInput,
  CustomGroup,
  CustomGroupInput,
  CustomGroupsResponse,
  JobRunResponse,
  LookupResponse,
  MandatoryRuleInput,
  MandatoryRuleMutationResponse,
  MandatoryRulesResponse,
  PersonCreateInput,
  PersonUpdateInput,
  OverviewResponse,
  PersonProfile,
  PersonSummary,
  PlanAuditResponse,
  PlanningResponse,
  PlanTransition,
  TrainingPlansResponse,
  TrainingPlanRow,
  TransitionPlanResponse,
  UpdatePlanInput,
  UpdatePlanResponse,
  WorkspaceResponse,
} from './types';

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
  complianceRelated: boolean;
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
            { id: 'team-cloud', label: 'Cloud Operations', active: true },
            { id: 'team-app', label: 'Applications', active: true },
          ],
          vendors: [
            { id: 'vendor-linux', label: 'Linux Foundation', active: true },
            { id: 'vendor-hashicorp', label: 'HashiCorp', active: true },
            { id: 'vendor-internal', label: 'Internal Academy', active: true },
          ],
          skillAreas: [
            { id: 'skill-kubernetes', label: 'Kubernetes', active: true },
            { id: 'skill-iac', label: 'Infrastructure as Code', active: true },
            { id: 'skill-security', label: 'Security', active: true },
          ],
          courses: workspace.catalog.map((course) => ({
            id: course.id,
            label: course.title,
            active: course.active,
            complianceRelated: course.complianceRelated,
            complianceFramework: course.complianceFramework,
          })),
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

export interface PeopleDirectoryFilters {
  year?: string;
  team?: string;
  group?: string;
  filter?: string;
  q?: string;
}

export function usePeopleDirectory(filters: PeopleDirectoryFilters, enabled: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'people-directory', filters],
    enabled,
    queryFn: async (): Promise<PersonSummary[]> => {
      const params = new URLSearchParams();
      if (filters.year) params.set('year', filters.year);
      if (filters.team) params.set('team', filters.team);
      if (filters.group) params.set('group', filters.group);
      if (filters.filter) params.set('filter', filters.filter);
      if (filters.q) params.set('q', filters.q);
      const suffix = params.toString();
      const path = `/training/v1/people/directory${suffix ? `?${suffix}` : ''}`;
      if (useMocks) return mockPeopleDirectory(filters);
      return api.get<PersonSummary[]>(path);
    },
  });
}

export function useOverviewKpis(filters: { year?: string; team?: string }, enabled: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'overview', filters],
    enabled,
    queryFn: async (): Promise<OverviewResponse> => {
      const params = new URLSearchParams();
      if (filters.year) params.set('year', filters.year);
      if (filters.team) params.set('team', filters.team);
      const suffix = params.toString();
      const path = `/training/v1/people/overview${suffix ? `?${suffix}` : ''}`;
      return api.get<OverviewResponse>(path);
    },
  });
}

export function usePersonProfile(id: string | undefined, year: string, enabled: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'person-profile', id, year],
    enabled: enabled && !!id,
    queryFn: async (): Promise<PersonProfile> => {
      const params = new URLSearchParams();
      if (year) params.set('year', year);
      const suffix = params.toString();
      const path = `/training/v1/people/${id}/profile${suffix ? `?${suffix}` : ''}`;
      return api.get<PersonProfile>(path);
    },
  });
}

export function useUpdatePerson() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: PersonUpdateInput }): Promise<ActionResponse> => {
      if (useMocks) return Promise.resolve({ ok: true, id, status: body.status } satisfies ActionResponse);
      return api.patch<ActionResponse>(`/training/v1/people/${id}`, body);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['training', 'person-profile'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'people-directory'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'workspace'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'overview'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'planning'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'compliance'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'lookups'] });
    },
  });
}

export function useCreatePerson() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: PersonCreateInput): Promise<ActionResponse> => {
      if (useMocks) {
        const slug = `${body.lastName}-${body.firstName}`
          .toLowerCase()
          .replace(/[^a-z0-9]+/g, '-')
          .replace(/^-|-$/g, '');
        return Promise.resolve({ ok: true, id: `employee-${slug || 'new'}`, status: body.status } satisfies ActionResponse);
      }
      return api.post<ActionResponse>('/training/v1/people', body);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['training', 'person-profile'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'people-directory'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'workspace'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'overview'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'planning'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'compliance'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'lookups'] });
    },
  });
}

export function useBulkAssignEnrollment(isPeopleAdmin: boolean) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (body: {
      employeeIds: string[];
      courseId: string;
      planParams: {
        year: number;
        plannedStart?: string;
        plannedEnd?: string;
        hoursPlanned?: number;
        costPlanned?: number;
      };
      mandatoryRuleId?: string;
      sourceCustomGroupId?: string;
    }): Promise<BulkAssignResponse> => {
      if (useMocks) return { created: body.employeeIds.length, failed: 0 };
      return api.post<BulkAssignResponse>('/training/v1/people/enrollments/bulk-assign', {
        employee_ids: body.employeeIds,
        course_id: body.courseId,
        plan_params: {
          year: body.planParams.year,
          planned_start: body.planParams.plannedStart,
          planned_end: body.planParams.plannedEnd,
          hours_planned: body.planParams.hoursPlanned,
          cost_planned: body.planParams.costPlanned,
        },
        mandatory_rule_id: body.mandatoryRuleId,
        source_custom_group_id: body.sourceCustomGroupId,
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['training', 'workspace', isPeopleAdmin] });
      queryClient.invalidateQueries({ queryKey: ['training', 'people-directory'] });
    },
  });
}

export function useBulkEnrollmentTransition(isPeopleAdmin: boolean) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      enrollmentIds,
      targetState,
      motivation,
    }: {
      enrollmentIds: string[];
      targetState: BulkTargetState;
      motivation?: string;
    }): Promise<BulkTransitionResponse> => {
      if (useMocks) {
        return { succeeded: enrollmentIds.length, failed: 0, failures: [] };
      }
      return api.post<BulkTransitionResponse>('/training/v1/people/enrollments/bulk-transition', {
        enrollment_ids: enrollmentIds,
        target_state: targetState,
        motivation,
      });
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

export function usePlanningSuggestions(year: string, team: string, enabled: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'planning', year, team],
    enabled,
    queryFn: async (): Promise<PlanningResponse> => {
      const params = new URLSearchParams();
      if (year) params.set('year', year);
      if (team) params.set('team', team);
      const suffix = params.toString();
      return api.get<PlanningResponse>(`/training/v1/people/planning/suggestions${suffix ? `?${suffix}` : ''}`);
    },
  });
}

export function useCreatePlan() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: CreatePlanInput): Promise<TrainingPlanRow> =>
      api.post<TrainingPlanRow>('/training/v1/people/plans', body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['training', 'planning'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'plans'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'overview'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'lookups'] });
    },
  });
}

export function useTrainingPlans(enabled: boolean, status?: string) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'plans', status ?? 'all'],
    enabled,
    queryFn: async (): Promise<TrainingPlansResponse> => {
      const params = new URLSearchParams();
      if (status) params.set('status', status);
      const suffix = params.toString();
      return api.get<TrainingPlansResponse>(`/training/v1/people/plans${suffix ? `?${suffix}` : ''}`);
    },
  });
}

export function useUpdatePlan() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ planId, body }: { planId: string; body: UpdatePlanInput }): Promise<UpdatePlanResponse> =>
      api.patch<UpdatePlanResponse>(`/training/v1/people/plans/${planId}`, body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['training', 'planning'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'plans'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'overview'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'plan-audit'] });
    },
  });
}

export function useDeletePlan() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (planId: string): Promise<{ ok: boolean }> =>
      api.delete<{ ok: boolean }>(`/training/v1/people/plans/${planId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['training', 'planning'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'plans'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'overview'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'lookups'] });
    },
  });
}

export function useTransitionPlan() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ planId, target }: { planId: string; target: PlanTransition }): Promise<TransitionPlanResponse> =>
      api.post<TransitionPlanResponse>(`/training/v1/people/plans/${planId}/transition`, { target }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['training', 'planning'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'overview'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'plans'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'plan-audit'] });
    },
  });
}

export function useBulkPlanFromSuggestion() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: BulkPlanFromSuggestionInput): Promise<BulkAssignResponse> =>
      api.post<BulkAssignResponse>('/training/v1/people/enrollments/bulk-plan-from-suggestion', body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['training', 'planning'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'workspace'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'overview'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'people-directory'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'plans'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'plan-audit'] });
    },
  });
}

export function useBulkReviewEmployeeRequests() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: BulkReviewEmployeeRequestsInput): Promise<BulkReviewEmployeeRequestsResponse> => {
      const params = new URLSearchParams();
      if (body.year) params.set('year', String(body.year));
      const suffix = params.toString();
      return api.post<BulkReviewEmployeeRequestsResponse>(
        `/training/v1/people/enrollments/bulk-review${suffix ? `?${suffix}` : ''}`,
        body,
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['training', 'planning'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'workspace'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'plan-audit'] });
    },
  });
}

export function usePlanAudit(planId: string | undefined, before: string | undefined, enabled: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'plan-audit', planId, before ?? 'first'],
    enabled: enabled && !!planId,
    queryFn: async (): Promise<PlanAuditResponse> => {
      const params = new URLSearchParams();
      params.set('limit', '50');
      if (before) params.set('before', before);
      return api.get<PlanAuditResponse>(`/training/v1/people/plans/${planId}/audit?${params.toString()}`);
    },
  });
}

export interface CatalogQueryFilters {
  skillArea?: string;
  fornitore?: string;
  stato?: 'attivo' | 'disattivato' | '';
  q?: string;
}

export function useCatalogCourses(filters: CatalogQueryFilters, enabled: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'catalog-courses', filters],
    enabled,
    queryFn: async (): Promise<CatalogListResponse> => {
      const params = new URLSearchParams();
      if (filters.skillArea) params.set('skill_area', filters.skillArea);
      if (filters.fornitore) params.set('fornitore', filters.fornitore);
      if (filters.stato) params.set('stato', filters.stato);
      if (filters.q) params.set('q', filters.q);
      const suffix = params.toString();
      return api.get<CatalogListResponse>(`/training/v1/courses${suffix ? `?${suffix}` : ''}`);
    },
  });
}

export function useArchiveCourse() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      api.post<ActionResponse>(`/training/v1/people/courses/${id}/archive`, {}),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['training', 'catalog-courses'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'lookups'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'workspace'] });
    },
  });
}

export function useComplianceOverview(year: string, team: string, deadlineDays: number, enabled: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'compliance', year, team, deadlineDays],
    enabled,
    queryFn: async (): Promise<ComplianceOverviewResponse> => {
      const params = new URLSearchParams();
      if (year) params.set('year', year);
      if (team) params.set('team', team);
      if (deadlineDays) params.set('deadline_days', String(deadlineDays));
      return api.get<ComplianceOverviewResponse>(`/training/v1/people/compliance?${params.toString()}`);
    },
  });
}

export function useMandatoryRules(
  filters: { status?: string; populationKind?: string; q?: string },
  enabled: boolean,
) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'mandatory-rules-v2', filters],
    enabled,
    queryFn: async (): Promise<MandatoryRulesResponse> => {
      const params = new URLSearchParams();
      if (filters.status) params.set('status', filters.status);
      if (filters.populationKind) params.set('population_kind', filters.populationKind);
      if (filters.q) params.set('q', filters.q);
      const suffix = params.toString();
      if (useMocks) return { rules: mockMandatoryRules() };
      return api.get<MandatoryRulesResponse>(`/training/v1/compliance/rules${suffix ? `?${suffix}` : ''}`);
    },
  });
}

export function useCreateMandatoryRule() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: MandatoryRuleInput): Promise<MandatoryRuleMutationResponse> =>
      api.post<MandatoryRuleMutationResponse>('/training/v1/compliance/rules', body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['training', 'mandatory-rules-v2'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'compliance'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'planning'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'people-directory'] });
    },
  });
}

export function useUpdateMandatoryRule() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: MandatoryRuleInput }): Promise<MandatoryRuleMutationResponse> =>
      api.patch<MandatoryRuleMutationResponse>(`/training/v1/compliance/rules/${id}`, body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['training', 'mandatory-rules-v2'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'compliance'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'planning'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'people-directory'] });
    },
  });
}

export function useDeleteMandatoryRule() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string): Promise<{ ok: boolean }> =>
      api.delete<{ ok: boolean }>(`/training/v1/compliance/rules/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['training', 'mandatory-rules-v2'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'compliance'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'planning'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'people-directory'] });
    },
  });
}

export function useCustomGroups(filters: { status?: string; q?: string }, enabled: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['training', 'custom-groups', filters],
    enabled,
    queryFn: async (): Promise<CustomGroupsResponse> => {
      const params = new URLSearchParams();
      if (filters.status) params.set('status', filters.status);
      if (filters.q) params.set('q', filters.q);
      const suffix = params.toString();
      if (useMocks) return { groups: mockCustomGroups() };
      return api.get<CustomGroupsResponse>(`/training/v1/people/groups${suffix ? `?${suffix}` : ''}`);
    },
  });
}

export function useCreateCustomGroup() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: CustomGroupInput): Promise<CustomGroup> =>
      api.post<CustomGroup>('/training/v1/people/groups', body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['training', 'custom-groups'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'mandatory-rules-v2'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'people-directory'] });
    },
  });
}

export function useUpdateCustomGroup() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: CustomGroupInput }): Promise<CustomGroup> =>
      api.patch<CustomGroup>(`/training/v1/people/groups/${id}`, body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['training', 'custom-groups'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'mandatory-rules-v2'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'people-directory'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'planning'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'compliance'] });
    },
  });
}

export function useDeleteCustomGroup() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string): Promise<{ ok: boolean }> =>
      api.delete<{ ok: boolean }>(`/training/v1/people/groups/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['training', 'custom-groups'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'people-directory'] });
    },
  });
}

export function useDismissSuggestion() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ suggestionId, planId }: { suggestionId: string; planId: string }) =>
      api.post<{ ok: boolean }>(`/training/v1/people/planning/suggestions/${suggestionId}/dismiss`, { plan_id: planId }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['training', 'planning'] });
      queryClient.invalidateQueries({ queryKey: ['training', 'plan-audit'] });
    },
  });
}
