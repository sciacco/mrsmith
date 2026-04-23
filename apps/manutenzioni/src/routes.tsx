import { hasAnyRole } from '@mrsmith/auth-client';
import { useToast } from '@mrsmith/ui';
import { useEffect, type ReactElement } from 'react';
import { Navigate, type RouteObject } from 'react-router-dom';
import { useOptionalAuth } from './hooks/useOptionalAuth';
import { MANUTENZIONI_MANAGER_ROLES } from './lib/roles';
import { MaintenanceCreatePage } from './pages/MaintenanceCreatePage';
import { MaintenanceDetailPage } from './pages/MaintenanceDetailPage';
import { MaintenanceListPage } from './pages/MaintenanceListPage';
import { ConfigurationIndexPage } from './pages/ConfigurationIndexPage';
import { ConfigurationLLMModelsPage } from './pages/ConfigurationLLMModelsPage';
import { ConfigurationResourcePage } from './pages/ConfigurationResourcePage';

function RequireManager({ children }: { children: ReactElement }) {
  const { user, loading } = useOptionalAuth();
  const toast = useToast();
  const canManage = hasAnyRole(user?.roles, MANUTENZIONI_MANAGER_ROLES);

  useEffect(() => {
    if (!loading && !canManage) {
      toast.toast('Permessi insufficienti per la configurazione.', 'error');
    }
  }, [loading, canManage, toast]);

  if (loading) return null;
  if (!canManage) return <Navigate to="/manutenzioni" replace />;
  return children;
}

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/manutenzioni" replace /> },
  { path: 'manutenzioni', element: <MaintenanceListPage /> },
  { path: 'manutenzioni/new', element: <MaintenanceCreatePage /> },
  {
    path: 'manutenzioni/configurazione',
    element: (
      <RequireManager>
        <ConfigurationIndexPage />
      </RequireManager>
    ),
  },
  {
    path: 'manutenzioni/configurazione/modelli-llm',
    element: (
      <RequireManager>
        <ConfigurationLLMModelsPage />
      </RequireManager>
    ),
  },
  {
    path: 'manutenzioni/configurazione/:resource',
    element: (
      <RequireManager>
        <ConfigurationResourcePage />
      </RequireManager>
    ),
  },
  { path: 'manutenzioni/:id', element: <MaintenanceDetailPage /> },
  { path: '*', element: <Navigate to="/manutenzioni" replace /> },
];
