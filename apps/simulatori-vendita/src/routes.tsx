import { Navigate, type RouteObject } from 'react-router-dom';
import { CalcolatoreIaaSPage } from './pages/CalcolatoreIaaSPage';
import { CalcolatoreIaaSLabPage } from './pages/CalcolatoreIaaSLabPage';
import { Layout } from './pages/Layout';

export const routes: RouteObject[] = [
  {
    element: <Layout />,
    children: [
      { index: true, element: <CalcolatoreIaaSPage /> },
      { path: 'lab', element: <CalcolatoreIaaSLabPage /> },
    ],
  },
  { path: '*', element: <Navigate to="/" replace /> },
];
