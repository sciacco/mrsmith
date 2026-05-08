export const DEVADMIN_ROLE = 'app_devadmin';

export const APP_ACCESS_ROLES = {
  budget: ['app_budget_access'],
  fornitori: ['app_fornitori_access'],
  rda: ['app_rda_access'],
  compliance: ['app_compliance_access'],
  coperture: ['app_coperture_access'],
  'cp-backoffice': ['app_cpbackoffice_access'],
  'energia-dc': ['app_energiadc_access'],
  'kit-products': ['app_kitproducts_access'],
  'listini-e-sconti': ['app_listini_access'],
  manutenzioni: ['app_manutenzioni_access'],
  'panoramica-cliente': ['app_panoramica_access'],
  quotes: ['app_quotes_access'],
  'simulatori-vendita': ['app_simulatorivendita_access'],
  'richieste-fattibilita': ['app_rdf_access', 'app_rdf_manager'],
  'rdf-backend': ['app_rdf_backend_access'],
  reports: ['app_reports_access'],
  'afc-tools': ['app_afctools_access'],
} as const satisfies Record<string, readonly string[]>;

export type AppAccessState =
  | 'loading'
  | 'reauthenticating'
  | 'unauthenticated'
  | 'forbidden'
  | 'allowed';

interface AppAccessAuthLike {
  status: 'loading' | 'authenticated' | 'reauthenticating' | 'unauthenticated';
  authenticated: boolean;
  loading: boolean;
  user: { roles: readonly string[] } | null;
}

function containsRole(userRoles: readonly string[], role: string): boolean {
  return userRoles.some(userRole => userRole === role);
}

export function hasAnyRole(userRoles: readonly string[] | undefined, requiredRoles: readonly string[]): boolean {
  if (!requiredRoles.length) return true;
  if (!userRoles || !userRoles.length) return false;
  if (containsRole(userRoles, DEVADMIN_ROLE)) return true;
  return requiredRoles.some(role => role !== '' && containsRole(userRoles, role));
}

export function hasRole(userRoles: readonly string[] | undefined, requiredRole: string): boolean {
  if (!requiredRole) return false;
  return hasAnyRole(userRoles, [requiredRole]);
}

export function getAppAccessState(
  auth: AppAccessAuthLike,
  requiredRoles: readonly string[],
): AppAccessState {
  if (auth.loading || auth.status === 'loading') return 'loading';
  if (auth.status === 'reauthenticating') return 'reauthenticating';
  if (!auth.authenticated) return 'unauthenticated';
  if (!hasAnyRole(auth.user?.roles, requiredRoles)) return 'forbidden';
  return 'allowed';
}
