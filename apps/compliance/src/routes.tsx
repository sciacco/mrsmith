import { Navigate, type RouteObject } from 'react-router-dom';
import { BlocksPage } from './views/blocks/BlocksPage';
import { ReleasesPage } from './views/releases/ReleasesPage';
import { DomainsPage } from './views/domains/DomainsPage';
import { HistoryPage } from './views/history/HistoryPage';
import { OriginsPage } from './views/origins/OriginsPage';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/blocks" replace /> },
  { path: 'blocks', element: <BlocksPage /> },
  { path: 'releases', element: <ReleasesPage /> },
  { path: 'domains', element: <DomainsPage /> },
  { path: 'history', element: <HistoryPage /> },
  { path: 'origins', element: <OriginsPage /> },
];
