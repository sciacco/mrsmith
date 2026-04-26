import { hasRole } from '@mrsmith/auth-client';
import { useOptionalAuth } from './useOptionalAuth';

export function useHasRole(role: string): boolean {
  const { user } = useOptionalAuth();
  return hasRole(user?.roles, role);
}
