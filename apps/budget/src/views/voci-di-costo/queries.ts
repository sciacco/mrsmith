import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useApiClient } from '../../api/client';
import type {
  PaginatedResponse,
  MessageResponse,
  IdResponse,
  Budget,
  BudgetDetails,
  BudgetNew,
  BudgetEdit,
  UserBudgetNew,
  UserBudgetEdit,
  CostCenterBudgetNew,
  CostCenterBudgetEdit,
  UserBudgetApprovalRule,
  UserBudgetApprovalRuleNew,
  UserBudgetApprovalRuleEdit,
  CcBudgetApprovalRule,
  CcBudgetApprovalRuleNew,
  CcBudgetApprovalRuleEdit,
} from '../../api/types';

export { useUsers, useCostCenters } from '../../api/shared-queries';

// ── Query keys ──

export interface BudgetListFilters {
  // Currently empty. Future: search_string?: string; year?: number;
}

export const budgetKeys = {
  list: (filters: BudgetListFilters = {}) => ['budget', 'budgets', filters] as const,
  details: (id: number) => ['budget', 'budget-details', id] as const,
};

export const ruleKeys = {
  userRules: (budgetId: number, userId: number) =>
    ['budget', 'user-rules', budgetId, userId] as const,
  ccRules: (budgetId: number, costCenter: string) =>
    ['budget', 'cc-rules', budgetId, costCenter] as const,
};

// ── Budget hooks ──

export function useBudgets(filters?: BudgetListFilters) {
  const api = useApiClient();
  return useQuery({
    queryKey: budgetKeys.list(filters),
    queryFn: async () => {
      const res = await api.get<PaginatedResponse<Budget>>(
        '/budget/v1/budget?page_number=1&disable_pagination=true',
      );
      return res.items;
    },
  });
}

export function useBudgetDetails(id: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: budgetKeys.details(id!),
    queryFn: async () => api.get<BudgetDetails>(`/budget/v1/budget/${id}`),
    enabled: id != null,
  });
}

export function useCreateBudget() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: BudgetNew) =>
      api.post<IdResponse>('/budget/v1/budget', body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: budgetKeys.list() });
    },
  });
}

export function useEditBudget() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: number; body: BudgetEdit }) =>
      api.put<MessageResponse>(`/budget/v1/budget/${id}`, body),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: budgetKeys.list() });
      queryClient.invalidateQueries({ queryKey: budgetKeys.details(variables.id) });
    },
  });
}

export function useDeleteBudget() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: number) =>
      api.delete<MessageResponse>(`/budget/v1/budget/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: budgetKeys.list() });
    },
  });
}

// ── Allocation hooks ──

export function useCreateUserBudget(budgetId: number) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: UserBudgetNew) =>
      api.post<MessageResponse>(`/budget/v1/budget/${budgetId}/user`, body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: budgetKeys.details(budgetId) });
    },
  });
}

export function useEditUserBudget(budgetId: number) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: UserBudgetEdit) =>
      api.put<MessageResponse>(`/budget/v1/budget/${budgetId}/user`, body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: budgetKeys.details(budgetId) });
    },
  });
}

export function useCreateCcBudget(budgetId: number) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: CostCenterBudgetNew) =>
      api.post<MessageResponse>(`/budget/v1/budget/${budgetId}/cost-center`, body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: budgetKeys.details(budgetId) });
    },
  });
}

export function useEditCcBudget(budgetId: number) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: CostCenterBudgetEdit) =>
      api.put<MessageResponse>(`/budget/v1/budget/${budgetId}/cost-center`, body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: budgetKeys.details(budgetId) });
    },
  });
}

// ── Approval rule hooks ──

export function useUserApprovalRules(budgetId: number, userId: number, enabled: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: ruleKeys.userRules(budgetId, userId),
    queryFn: async () => {
      const res = await api.get<PaginatedResponse<UserBudgetApprovalRule>>(
        `/budget/v1/approval-rules/user-budget?page_number=1&disable_pagination=true&budget_id=${budgetId}&user_id=${userId}`,
      );
      return res.items;
    },
    enabled,
  });
}

export function useCreateUserRule(budgetId: number, userId: number) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: UserBudgetApprovalRuleNew) =>
      api.post<IdResponse>('/budget/v1/approval-rules/user-budget', body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ruleKeys.userRules(budgetId, userId) });
    },
  });
}

export function useEditUserRule(budgetId: number, userId: number) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ ruleId, body }: { ruleId: number; body: UserBudgetApprovalRuleEdit }) =>
      api.put<MessageResponse>(`/budget/v1/approval-rules/user-budget/${ruleId}`, body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ruleKeys.userRules(budgetId, userId) });
    },
  });
}

export function useDeleteUserRule(budgetId: number, userId: number) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (ruleId: number) =>
      api.delete<MessageResponse>(`/budget/v1/approval-rules/user-budget/${ruleId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ruleKeys.userRules(budgetId, userId) });
    },
  });
}

export function useCcApprovalRules(budgetId: number, costCenter: string, enabled: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: ruleKeys.ccRules(budgetId, costCenter),
    queryFn: async () => {
      const res = await api.get<PaginatedResponse<CcBudgetApprovalRule>>(
        `/budget/v1/approval-rules/cost-center-budget?page_number=1&disable_pagination=true&budget_id=${budgetId}&cost_center=${encodeURIComponent(costCenter)}`,
      );
      return res.items;
    },
    enabled,
  });
}

export function useCreateCcRule(budgetId: number, costCenter: string) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: CcBudgetApprovalRuleNew) =>
      api.post<IdResponse>('/budget/v1/approval-rules/cost-center-budget', body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ruleKeys.ccRules(budgetId, costCenter) });
    },
  });
}

export function useEditCcRule(budgetId: number, costCenter: string) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ ruleId, body }: { ruleId: number; body: CcBudgetApprovalRuleEdit }) =>
      api.put<MessageResponse>(`/budget/v1/approval-rules/cost-center-budget/${ruleId}`, body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ruleKeys.ccRules(budgetId, costCenter) });
    },
  });
}

export function useDeleteCcRule(budgetId: number, costCenter: string) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (ruleId: number) =>
      api.delete<MessageResponse>(`/budget/v1/approval-rules/cost-center-budget/${ruleId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ruleKeys.ccRules(budgetId, costCenter) });
    },
  });
}
