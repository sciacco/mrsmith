import { Navigate, type RouteObject } from 'react-router-dom';
import { MaintenanceCreatePage } from './pages/MaintenanceCreatePage';
import { MaintenanceDetailPage } from './pages/MaintenanceDetailPage';
import { MaintenanceListPage } from './pages/MaintenanceListPage';
import { ConfigurationIndexPage } from './pages/ConfigurationIndexPage';
import { ConfigurationResourcePage } from './pages/ConfigurationResourcePage';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/manutenzioni" replace /> },
  { path: 'manutenzioni', element: <MaintenanceListPage /> },
  { path: 'manutenzioni/new', element: <MaintenanceCreatePage /> },
  { path: 'manutenzioni/configurazione', element: <ConfigurationIndexPage /> },
  { path: 'manutenzioni/configurazione/:resource', element: <ConfigurationResourcePage /> },
  { path: 'manutenzioni/:id', element: <MaintenanceDetailPage /> },
  { path: '*', element: <Navigate to="/manutenzioni" replace /> },
];
