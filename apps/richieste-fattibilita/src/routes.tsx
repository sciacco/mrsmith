import { Navigate, type RouteObject } from 'react-router-dom';
import { HomeRedirect } from './pages/HomeRedirect';
import { NewRequestPage } from './pages/NewRequestPage';
import { RequestDetailPage } from './pages/RequestDetailPage';
import { RequestListPage } from './pages/RequestListPage';
import { RequestViewPage } from './pages/RequestViewPage';

export const routes: RouteObject[] = [
  { index: true, element: <HomeRedirect /> },
  { path: 'richieste', element: <RequestListPage /> },
  { path: 'richieste/new', element: <NewRequestPage /> },
  { path: 'richieste/:id', element: <RequestDetailPage /> },
  { path: 'richieste/:id/view', element: <RequestViewPage /> },
  { path: '*', element: <Navigate to="/" replace /> },
];
