import { Navigate, type RouteObject } from 'react-router-dom';
import { WorkQueuePage } from './pages/WorkQueuePage';
import { FactorialPage } from './pages/FactorialPage';
import { EventsPage } from './pages/EventsPage/EventsPage';
import { EventDetailPage } from './pages/EventDetailPage/EventDetailPage';
import { RequestsPage } from './pages/RequestsPage/RequestsPage';
import { RulesPage } from './pages/RulesPage/RulesPage';
import { PeoplePage } from './pages/PeoplePage/PeoplePage';
import { PersonPage } from './pages/PersonPage/PersonPage';
import { CatalogPage } from './pages/CatalogPage/CatalogPage';
import { ReportPage } from './pages/ReportPage/ReportPage';
import { PlanningPage } from './pages/PlanningPage/PlanningPage';

export const routes: RouteObject[] = [
  { index: true, element: <WorkQueuePage /> },
  { path: 'pianificazione', element: <PlanningPage /> },
  { path: 'richieste', element: <RequestsPage /> },
  { path: 'regole', element: <RulesPage /> },
  { path: 'eventi', element: <EventsPage /> },
  { path: 'eventi/:id', element: <EventDetailPage /> },
  { path: 'persone', element: <PeoplePage /> },
  { path: 'persone/:id', element: <PersonPage /> },
  { path: 'catalogo', element: <CatalogPage /> },
  { path: 'report', element: <ReportPage /> },
  { path: 'factorial', element: <FactorialPage /> },
  { path: '*', element: <Navigate to="/" replace /> },
];
