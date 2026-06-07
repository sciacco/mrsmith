import { Navigate, type RouteObject } from 'react-router-dom';
import { ArchivioPage } from './pages/ArchivioPage';
import { PreventiviPage } from './pages/PreventiviPage';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/preventivi" replace /> },
  { path: 'preventivi', element: <PreventiviPage /> },
  { path: 'archivio', element: <ArchivioPage /> },
  { path: '*', element: <Navigate to="/preventivi" replace /> },
];
