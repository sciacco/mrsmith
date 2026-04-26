export const MANUTENZIONI_MANAGER_ROLES = ['app_manutenzioni_manager'];
export const MANUTENZIONI_OPERATOR_ROLES = ['app_manutenzioni_operator'];
export const MANUTENZIONI_APPROVER_ROLES = ['app_manutenzioni_approver'];

export const MANUTENZIONI_OPERATIONAL_ROLES = [
  ...MANUTENZIONI_MANAGER_ROLES,
  ...MANUTENZIONI_OPERATOR_ROLES,
];

export const MANUTENZIONI_APPROVAL_ROLES = [
  ...MANUTENZIONI_MANAGER_ROLES,
  ...MANUTENZIONI_APPROVER_ROLES,
];
