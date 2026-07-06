import { Navigate, type RouteObject } from 'react-router-dom';
import { ArchivioPaPage } from './pages/ArchivioPaPage';
import { RdaPlaceholderPage } from './pages/RdaPlaceholderPage';
import { RiepilogoPaPage } from './pages/RiepilogoPaPage';
import { RiepilogoRdaPage } from './pages/RiepilogoRdaPage';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/riepilogo-rda" replace /> },
  { path: 'archivio-pa', element: <ArchivioPaPage /> },
  { path: 'archivio-pa/:issueKey', element: <ArchivioPaPage /> },
  { path: 'riepilogo-pa', element: <RiepilogoPaPage /> },
  { path: 'riepilogo-rda', element: <RiepilogoRdaPage /> },
  { path: 'rda', element: <RdaPlaceholderPage /> },
  { path: '*', element: <Navigate to="/riepilogo-rda" replace /> },
];
