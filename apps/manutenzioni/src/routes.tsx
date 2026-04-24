import { hasAnyRole } from '@mrsmith/auth-client';
import { useToast } from '@mrsmith/ui';
import { useEffect, type ReactElement } from 'react';
import { Navigate, type RouteObject } from 'react-router-dom';
import { useOptionalAuth } from './hooks/useOptionalAuth';
import { MANUTENZIONI_APPROVER_ROLES, MANUTENZIONI_MANAGER_ROLES } from './lib/roles';
import { MaintenanceCreatePage } from './pages/MaintenanceCreatePage';
import { MaintenanceDetailPage } from './pages/MaintenanceDetailPage';
import { MaintenanceListPage } from './pages/MaintenanceListPage';
import { ConfigurationDependenciesPage } from './pages/ConfigurationDependenciesPage';
import { ConfigurationIndexPage } from './pages/ConfigurationIndexPage';
import { ConfigurationLLMModelsPage } from './pages/ConfigurationLLMModelsPage';
import { ConfigurationResourcePage } from './pages/ConfigurationResourcePage';

function RequireConfiguration({ children }: { children: ReactElement }) {
  const { user, loading } = useOptionalAuth();
  const toast = useToast();
  const canOpen = hasAnyRole(user?.roles, [
    ...MANUTENZIONI_MANAGER_ROLES,
    ...MANUTENZIONI_APPROVER_ROLES,
  ]);

  useEffect(() => {
    if (!loading && !canOpen) {
      toast.toast('Permessi insufficienti per la configurazione.', 'error');
    }
  }, [loading, canOpen, toast]);

  if (loading) return null;
  if (!canOpen) return <Navigate to="/manutenzioni" replace />;
  return children;
}

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/manutenzioni" replace /> },
  { path: 'manutenzioni', element: <MaintenanceListPage /> },
  { path: 'manutenzioni/new', element: <MaintenanceCreatePage /> },
  {
    path: 'manutenzioni/configurazione',
    element: (
      <RequireConfiguration>
        <ConfigurationIndexPage />
      </RequireConfiguration>
    ),
  },
  {
    path: 'manutenzioni/configurazione/modelli-llm',
    element: (
      <RequireConfiguration>
        <ConfigurationLLMModelsPage />
      </RequireConfiguration>
    ),
  },
  {
    path: 'manutenzioni/configurazione/dipendenze',
    element: (
      <RequireConfiguration>
        <ConfigurationDependenciesPage />
      </RequireConfiguration>
    ),
  },
  {
    path: 'manutenzioni/configurazione/:resource',
    element: (
      <RequireConfiguration>
        <ConfigurationResourcePage />
      </RequireConfiguration>
    ),
  },
  { path: 'manutenzioni/:id', element: <MaintenanceDetailPage /> },
  { path: '*', element: <Navigate to="/manutenzioni" replace /> },
];
