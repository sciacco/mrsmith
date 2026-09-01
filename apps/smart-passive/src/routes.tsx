import { Navigate, type RouteObject } from 'react-router-dom';
import { AlyanteInvoicesPage } from './pages/AlyanteInvoicesPage';
import { DashboardPage } from './pages/DashboardPage';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/dashboard" replace /> },
  { path: 'dashboard', element: <DashboardPage /> },
  { path: 'fatture-alyante', element: <AlyanteInvoicesPage /> },
  { path: '*', element: <Navigate to="/dashboard" replace /> },
];
