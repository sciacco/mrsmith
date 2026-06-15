import { Navigate, type RouteObject } from 'react-router-dom';
import { TestPage } from './pages/TestPage';
import { TargetPage } from './pages/TargetPage';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/target" replace /> },
  { path: 'target', element: <TargetPage /> },
  { path: 'test', element: <TestPage /> },
];
