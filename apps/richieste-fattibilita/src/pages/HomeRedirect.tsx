import { hasAnyRole } from '@mrsmith/auth-client';
import { Navigate } from 'react-router-dom';
import { useOptionalAuth } from '../hooks/useOptionalAuth';
import { MANAGER_ROLES } from '../lib/format';

export function HomeRedirect() {
  const { user } = useOptionalAuth();
  const canManage = hasAnyRole(user?.roles, MANAGER_ROLES);
  return <Navigate to={canManage ? '/richieste/gestione' : '/richieste'} replace />;
}
