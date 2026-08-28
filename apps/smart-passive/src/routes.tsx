import { Navigate, type RouteObject } from 'react-router-dom';
import { DashboardPage } from './pages/DashboardPage';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/dashboard" replace /> },
  { path: 'dashboard', element: <DashboardPage /> },
  { path: '*', element: <Navigate to="/dashboard" replace /> },
];
