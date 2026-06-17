import { Navigate, type RouteObject } from 'react-router-dom';
import { TestPage } from './pages/TestPage';
import { TargetPage } from './pages/TargetPage';
import { ConfigPage } from './pages/ConfigPage';
import { CompanyDossierPage } from './pages/CompanyDossierPage';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/target" replace /> },
  { path: 'target', element: <TargetPage /> },
  { path: 'azienda', element: <CompanyDossierPage /> },
  { path: 'config', element: <ConfigPage /> },
  { path: 'test', element: <TestPage /> },
];
