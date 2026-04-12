export const DEVADMIN_ROLE = 'devadmin';

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
