import { Navigate, type RouteObject } from 'react-router-dom';
import { RdaListPage } from './pages/RdaListPage';
import { InboxPage } from './pages/InboxPage';
import { NewRdaWizardPage } from './pages/NewRdaWizardPage';
import { PoDetailPage } from './pages/PoDetailPage';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/rda" replace /> },
  { path: '/rda', element: <RdaListPage /> },
  { path: '/rda/new', element: <NewRdaWizardPage /> },
  { path: '/rda/new/:poId', element: <NewRdaWizardPage /> },
  { path: '/rda/inbox/:kind', element: <InboxPage /> },
  { path: '/rda/po/:poId', element: <PoDetailPage /> },
  { path: '*', element: <Navigate to="/rda" replace /> },
];
