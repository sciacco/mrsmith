import { Navigate, type RouteObject } from 'react-router-dom';
import { TestPage } from './pages/TestPage';
import { TargetPage } from './pages/TargetPage';
import { ConfigPage } from './pages/ConfigPage';
import { CompanyDossierPage } from './pages/CompanyDossierPage';
import { WebSearchPage } from './pages/WebSearchPage';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/target" replace /> },
  { path: 'target', element: <TargetPage /> },
  { path: 'azienda', element: <CompanyDossierPage /> },
  { path: 'ricerca-web', element: <WebSearchPage /> },
  { path: 'config', element: <ConfigPage /> },
  { path: 'test', element: <TestPage /> },
];
