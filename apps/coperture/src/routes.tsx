import { Navigate, type RouteObject } from 'react-router-dom';
import { CoverageLookupPage } from './pages/CoverageLookupPage';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/coperture" replace /> },
  { path: 'coperture', element: <CoverageLookupPage /> },
  { path: '*', element: <Navigate to="/coperture" replace /> },
];
