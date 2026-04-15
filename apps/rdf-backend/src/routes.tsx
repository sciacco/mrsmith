import { Navigate, type RouteObject } from 'react-router-dom';
import { FornitoriPage } from './pages/FornitoriPage';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/fornitori" replace /> },
  { path: 'fornitori', element: <FornitoriPage /> },
  { path: '*', element: <Navigate to="/fornitori" replace /> },
];
