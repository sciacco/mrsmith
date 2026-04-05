/**
 * API types for Budget Management.
 * Source of truth: docs/mistra-dist.yaml (Mistra NG Internal API v2.7.14)
 *
 * Each type references its OpenAPI schema name in a JSDoc comment.
 * If mistra-dist.yaml is updated, sync these types manually.
 */

/** Standard paginated response envelope (all list endpoints) */
export interface PaginatedResponse<T> {
  total_number: number;
  current_page: number;
  total_pages: number;
  items: T[];
}

/** Standard mutation response (POST/PUT/DELETE) — schema: message */
export interface MessageResponse {
  message: string;
}

/** schema: arak-int-user-state */
export interface ArakIntUserState {
  name: string;
  enabled: boolean;
}

/** schema: arak-int-role */
export interface ArakIntRole {
  name: string;
  created: string;
  updated: string;
}

/** schema: arak-int-user */
export interface ArakIntUser {
  id: number;
  first_name: string;
  last_name: string;
  email: string;
  created: string;
  updated: string;
  enabled: boolean;
  state: ArakIntUserState;
  role: ArakIntRole;
}

/** schema: group */
export interface Group {
  name: string;
  user_count: number;
}

/** schema: group-details */
export interface GroupDetails {
  name: string;
  users: ArakIntUser[];
}

/** schema: group-new */
export interface GroupNew {
  name: string;
  user_ids: number[];
}

/** schema: group-edit */
export interface GroupEdit {
  new_name?: string;
  user_ids?: number[];
}

/** schema: cost-center */
export interface CostCenter {
  name: string;
  manager_email: string;
  user_count: number;
  group_count: number;
  group_user_count: number;
  enabled: boolean;
}

/** schema: cost-center-details */
export interface CostCenterDetails {
  name: string;
  manager: ArakIntUser;
  users: ArakIntUser[] | null;
  groups: GroupDetails[];
  enabled: boolean;
}

/** schema: cost-center-new */
export interface CostCenterNew {
  name: string;
  manager_id: number;
  user_ids: number[];
  group_names: string[];
  enabled: boolean;
}

/** schema: cost-center-edit */
export interface CostCenterEdit {
  new_name?: string;
  manager_id?: number;
  user_ids?: number[];
  group_names?: string[];
  enabled?: boolean;
}

// ── Budget types ──

/** schema: id-object — returned by NewBudget, NewUserBudgetApprovalRule, NewCCBudgetApprovalRule */
export interface IdResponse {
  id: number;
}

/** schema: budget */
export interface Budget {
  id: number;
  name: string;
  year: number;
  limit: string;   // decimal as string — DO NOT parse to number in state
  current: string;  // decimal as string — DO NOT parse to number in state
}

/** schema: budget-details */
export interface BudgetDetails extends Budget {
  cost_center_budgets: CostCenterBudgetAllocation[];
  user_budgets: UserBudgetAllocation[];
}

/** schema: budget-new */
export interface BudgetNew {
  name: string;
  year: number;
}

/** schema: budget-edit */
export interface BudgetEdit {
  name?: string;
  year?: number;
}

// ── Allocation types ──

/** schema: user-budget */
export interface UserBudgetAllocation {
  limit: string;
  current: string;
  user_id: number;
  user_email: string;
  budget_id: number;
  enabled: boolean;
}

/** schema: user-budget-upsert */
export interface UserBudgetNew {
  limit: string;
  user_id: number;
}

/** schema: user-budget-edit */
export interface UserBudgetEdit {
  user_id: number;
  limit?: string;
  enabled?: boolean;
}

/** schema: cost_center-budget */
export interface CostCenterBudgetAllocation {
  limit: string;
  current: string;
  cost_center: string;
  budget_id: number;
  enabled: boolean;
}

/** schema: cost_center-budget-upsert */
export interface CostCenterBudgetNew {
  limit: string;
  cost_center: string;
}

/** schema: cost_center-budget-edit */
export interface CostCenterBudgetEdit {
  cost_center: string;
  limit?: string;
  enabled?: boolean;
}

// ── Approval rule types ──

/** schema: user-budget-approval-rule */
export interface UserBudgetApprovalRule {
  id: number;
  threshold: string;
  approver_id: number;
  approver_email: string;
  budget_id: number;
  user_id: number;
  level: number;
  send_email: boolean;
}

/** schema: user-budget-approval-rule-new */
export interface UserBudgetApprovalRuleNew {
  threshold: string;
  approver_id: number;
  budget_id: number;
  user_id: number;
  level: number;
  send_email: boolean;
}

/** schema: user-budget-approval-rule-edit — MUTABLE FIELDS ONLY */
export interface UserBudgetApprovalRuleEdit {
  threshold?: string;
  approver_id?: number;
  level?: number;
  send_email?: boolean;
}

/** schema: cc-budget-approval-rule */
export interface CcBudgetApprovalRule {
  id: number;
  threshold: string;
  approver_id: number;
  approver_email: string;
  budget_id: number;
  cost_center: string;
  level: number;
  send_email: boolean;
}

/** schema: cc-budget-approval-rule-new */
export interface CcBudgetApprovalRuleNew {
  threshold: string;
  approver_id: number;
  budget_id: number;
  cost_center: string;
  level: number;
  send_email: boolean;
}

/** schema: cc-budget-approval-rule-edit — MUTABLE FIELDS ONLY */
export interface CcBudgetApprovalRuleEdit {
  threshold?: string;
  approver_id?: number;
  level?: number;
  send_email?: boolean;
}
