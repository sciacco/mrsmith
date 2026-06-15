import { Navigate, type RouteObject } from 'react-router-dom';
import { ArchivioPage } from './pages/ArchivioPage';
import { PreventiviPage } from './pages/PreventiviPage';
import { ModificaDocumentoPage } from './pages/ModificaDocumentoPage';
import { ModificaPreventivoPage } from './pages/ModificaPreventivoPage';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/preventivi" replace /> },
  { path: 'preventivi', element: <PreventiviPage /> },
  { path: 'preventivi/nuovo', element: <ModificaPreventivoPage /> },
  { path: 'preventivi/:id', element: <ModificaPreventivoPage /> },
  { path: 'archivio', element: <ArchivioPage /> },
  { path: 'archivio/:id/modifica', element: <ModificaDocumentoPage /> },
  { path: '*', element: <Navigate to="/preventivi" replace /> },
];

