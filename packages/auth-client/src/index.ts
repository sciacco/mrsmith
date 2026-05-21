export { AuthProvider, useAuth } from './AuthProvider';
export type { AuthContextValue, AuthStatus } from './AuthProvider';
export {
  APP_ACCESS_ROLES,
  CP_BACKOFFICE_APP_ACCESS_ROLES,
  CP_BACKOFFICE_BIOMETRIC_ACCESS_ROLES,
  CP_BACKOFFICE_FULL_ACCESS_ROLES,
  DEVADMIN_ROLE,
  TRAINING_ACCESS_ROLES,
  TRAINING_APP_ACCESS_ROLES,
  TRAINING_PEOPLE_ADMIN_ROLES,
  getAppAccessState,
  hasAnyRole,
  hasRole,
} from './roles';
export type { AppAccessState } from './roles';
