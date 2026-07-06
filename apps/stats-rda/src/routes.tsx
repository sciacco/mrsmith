import { Navigate, type RouteObject } from 'react-router-dom';
import { ArchivioPaPage } from './pages/ArchivioPaPage';
import { RdaPlaceholderPage } from './pages/RdaPlaceholderPage';
import { RiepilogoPaPage } from './pages/RiepilogoPaPage';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/archivio-pa" replace /> },
  { path: 'archivio-pa', element: <ArchivioPaPage /> },
  { path: 'archivio-pa/:issueKey', element: <ArchivioPaPage /> },
  { path: 'riepilogo-pa', element: <RiepilogoPaPage /> },
  { path: 'rda', element: <RdaPlaceholderPage /> },
  { path: '*', element: <Navigate to="/archivio-pa" replace /> },
];
