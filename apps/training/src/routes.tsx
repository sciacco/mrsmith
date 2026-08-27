import { Navigate, type RouteObject } from 'react-router-dom';
import { WorkQueuePage } from './pages/WorkQueuePage';
import { FactorialPage } from './pages/FactorialPage';
import { EventsPage } from './pages/EventsPage/EventsPage';
import { EventDetailPage } from './pages/EventDetailPage/EventDetailPage';

export const routes: RouteObject[] = [
  { index: true, element: <WorkQueuePage /> },
  { path: 'eventi', element: <EventsPage /> },
  { path: 'eventi/:id', element: <EventDetailPage /> },
  { path: 'factorial', element: <FactorialPage /> },
  { path: '*', element: <Navigate to="/" replace /> },
];
