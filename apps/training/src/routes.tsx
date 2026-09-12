import { Navigate, type RouteObject } from 'react-router-dom';
import { WorkQueuePage } from './pages/WorkQueuePage';
import { FactorialPage } from './pages/FactorialPage';
import { EventsPage } from './pages/EventsPage/EventsPage';
import { EventDetailPage } from './pages/EventDetailPage/EventDetailPage';
import { RequestsPage } from './pages/RequestsPage/RequestsPage';
import { RulesPage } from './pages/RulesPage/RulesPage';
import { PeoplePage } from './pages/PeoplePage/PeoplePage';
import { PersonPage } from './pages/PersonPage/PersonPage';
import { CoursesPage } from './pages/CatalogPage/CoursesPage';
import { AnagrafichePage } from './pages/CatalogPage/AnagrafichePage';
import { CertificationsPage } from './pages/CatalogPage/CertificationsPage';
import { PathsPage } from './pages/CatalogPage/PathsPage';
import { ReportPage } from './pages/ReportPage/ReportPage';
import { PlanningPage } from './pages/PlanningPage/PlanningPage';
import { NeedsPage } from './pages/NeedsPage/NeedsPage';
import { NeedDetailPage } from './pages/NeedDetailPage/NeedDetailPage';

export const routes: RouteObject[] = [
  { index: true, element: <WorkQueuePage /> },
  { path: 'pianificazione', element: <PlanningPage /> },
  { path: 'richieste', element: <RequestsPage /> },
  { path: 'esigenze', element: <NeedsPage /> },
  { path: 'esigenze/:id', element: <NeedDetailPage /> },
  { path: 'regole', element: <RulesPage /> },
  { path: 'eventi', element: <EventsPage /> },
  { path: 'eventi/:id', element: <EventDetailPage /> },
  { path: 'persone', element: <PeoplePage /> },
  { path: 'persone/:id', element: <PersonPage /> },
  // Il gruppo Catalogo non ha una pagina propria: apre la prima sottopagina.
  { path: 'catalogo', element: <Navigate to="/catalogo/corsi" replace /> },
  { path: 'catalogo/corsi', element: <CoursesPage /> },
  { path: 'catalogo/anagrafiche', element: <AnagrafichePage /> },
  { path: 'catalogo/certificazioni', element: <CertificationsPage /> },
  { path: 'catalogo/percorsi', element: <PathsPage /> },
  { path: 'report', element: <ReportPage /> },
  { path: 'factorial', element: <FactorialPage /> },
  { path: '*', element: <Navigate to="/" replace /> },
];
