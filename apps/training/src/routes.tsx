import { Navigate, type RouteObject } from 'react-router-dom';
import { PipelinePage } from './pages/PipelinePage';
import { PeoplePage } from './pages/PeoplePage';
import { PersonPage } from './pages/PersonPage';
import { OverviewPage } from './pages/OverviewPage';
import { PlanningPage } from './pages/PlanningPage';
import { CompliancePage } from './pages/CompliancePage';
import { CatalogPage } from './pages/CatalogPage';
import { RulesPage } from './pages/RulesPage';
import { GroupsPage } from './pages/GroupsPage';
import { SettingsPage } from './pages/SettingsPage';

export function routes(isPeopleAdmin: boolean): RouteObject[] {
  return [
    { index: true, element: <OverviewPage isPeopleAdmin={isPeopleAdmin} /> },
    { path: 'report', element: <Navigate to="/" replace /> },
    { path: 'richieste', element: <Navigate to="/pipeline" replace /> },
    { path: 'piano', element: <Navigate to="/pipeline" replace /> },
    { path: 'certificazioni', element: <Navigate to="/compliance" replace /> },
    { path: 'pipeline', element: <PipelinePage isPeopleAdmin={isPeopleAdmin} /> },
    { path: 'persone', element: <PeoplePage isPeopleAdmin={isPeopleAdmin} /> },
    { path: 'persone/gruppi', element: <GroupsPage isPeopleAdmin={isPeopleAdmin} /> },
    { path: 'persone/:id', element: <PersonPage isPeopleAdmin={isPeopleAdmin} /> },
    { path: 'compliance', element: <CompliancePage isPeopleAdmin={isPeopleAdmin} /> },
    { path: 'compliance/regole', element: <RulesPage isPeopleAdmin={isPeopleAdmin} /> },
    { path: 'pianificazione', element: <PlanningPage isPeopleAdmin={isPeopleAdmin} /> },
    { path: 'catalogo', element: <CatalogPage isPeopleAdmin={isPeopleAdmin} /> },
    { path: 'impostazioni', element: <SettingsPage isPeopleAdmin={isPeopleAdmin} /> },
    { path: '*', element: <Navigate to="/" replace /> },
  ];
}
