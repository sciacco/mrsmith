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
