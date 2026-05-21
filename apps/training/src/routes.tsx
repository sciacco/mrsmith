import { Navigate, type RouteObject } from 'react-router-dom';
import { TrainingWorkspacePage, type TrainingView } from './pages/TrainingWorkspacePage';

const viewRoutes: Array<{ path: string; view: TrainingView }> = [
  { path: 'piano', view: 'piano' },
  { path: 'richieste', view: 'richieste' },
  { path: 'catalogo', view: 'catalogo' },
  { path: 'certificazioni', view: 'certificazioni' },
  { path: 'report', view: 'report' },
];

export function routes(isPeopleAdmin: boolean): RouteObject[] {
  return [
    { index: true, element: <Navigate to="/piano" replace /> },
    ...viewRoutes.map(({ path, view }) => ({
      path,
      element: <TrainingWorkspacePage view={view} isPeopleAdmin={isPeopleAdmin} />,
    })),
    { path: '*', element: <Navigate to="/piano" replace /> },
  ];
}
