import { Navigate, type RouteObject } from 'react-router-dom';
import { WorkQueuePage } from './pages/WorkQueuePage';
import { FactorialPage } from './pages/FactorialPage';

export const routes: RouteObject[] = [
  { index: true, element: <WorkQueuePage /> },
  { path: 'factorial', element: <FactorialPage /> },
  { path: '*', element: <Navigate to="/" replace /> },
];
