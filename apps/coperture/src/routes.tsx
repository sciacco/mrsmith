import { Navigate, type RouteObject } from 'react-router-dom';
import { CoverageLookupPage } from './pages/CoverageLookupPage';
import { CoverageLookupPageV2 } from './pages/CoverageLookupPageV2';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/coperture" replace /> },
  { path: 'coperture', element: <CoverageLookupPageV2 /> },
  { path: 'coperture/old', element: <CoverageLookupPage /> },
  { path: '*', element: <Navigate to="/coperture" replace /> },
];
