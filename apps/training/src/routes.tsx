import { Navigate, type RouteObject } from 'react-router-dom';
import { TrainingWorkspacePage, type TrainingView } from './pages/TrainingWorkspacePage';
import { StubPage } from './pages/StubPage';
import { PipelinePage } from './pages/PipelinePage';
import { PeoplePage } from './pages/PeoplePage';
import { PersonPage } from './pages/PersonPage';
import { OverviewPage } from './pages/OverviewPage';
import { PlanningPage } from './pages/PlanningPage';

const legacyViewRoutes: Array<{ path: string; view: TrainingView }> = [
  { path: 'piano', view: 'piano' },
  { path: 'catalogo', view: 'catalogo' },
  { path: 'certificazioni', view: 'certificazioni' },
];

export function routes(isPeopleAdmin: boolean): RouteObject[] {
  return [
    { index: true, element: <OverviewPage isPeopleAdmin={isPeopleAdmin} /> },
    ...legacyViewRoutes.map(({ path, view }) => ({
      path,
      element: <TrainingWorkspacePage view={view} isPeopleAdmin={isPeopleAdmin} />,
    })),
    { path: 'report', element: <Navigate to="/" replace /> },
    { path: 'richieste', element: <Navigate to="/pipeline" replace /> },
    { path: 'pipeline', element: <PipelinePage isPeopleAdmin={isPeopleAdmin} /> },
    { path: 'persone', element: <PeoplePage isPeopleAdmin={isPeopleAdmin} /> },
    { path: 'persone/:id', element: <PersonPage isPeopleAdmin={isPeopleAdmin} /> },
    { path: 'compliance', element: <StubPage title="Compliance" description="Vista compliance: prossimamente." /> },
    { path: 'pianificazione', element: <PlanningPage isPeopleAdmin={isPeopleAdmin} /> },
    { path: '*', element: <Navigate to="/" replace /> },
  ];
}
