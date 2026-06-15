import { Navigate, type RouteObject } from 'react-router-dom';
import { TestPage } from './pages/TestPage';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/test" replace /> },
  { path: 'test', element: <TestPage /> },
];
