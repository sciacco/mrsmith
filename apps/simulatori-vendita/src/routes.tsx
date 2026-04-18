import { Navigate, type RouteObject } from 'react-router-dom';
import { CalcolatoreIaaSPage } from './pages/CalcolatoreIaaSPage';

export const routes: RouteObject[] = [
  { index: true, element: <CalcolatoreIaaSPage /> },
  { path: '*', element: <Navigate to="/" replace /> },
];
