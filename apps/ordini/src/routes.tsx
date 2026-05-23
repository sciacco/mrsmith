import { Navigate, type RouteObject } from 'react-router-dom';
import { OrderDetailPage } from './pages/OrderDetailPage';
import { OrderListPage } from './pages/OrderListPage';

export const routes: RouteObject[] = [
  { path: '/', element: <Navigate to="/ordini" replace /> },
  { path: '/ordini', element: <OrderListPage /> },
  { path: '/ordini/:id', element: <OrderDetailPage /> },
  { path: '*', element: <Navigate to="/ordini" replace /> },
];
