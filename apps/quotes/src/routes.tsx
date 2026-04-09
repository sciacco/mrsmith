import { Navigate, type RouteObject } from 'react-router-dom';
import { QuoteListPage } from './pages/QuoteListPage';
import { QuoteDetailPage } from './pages/QuoteDetailPage';
import { QuoteCreatePage } from './pages/QuoteCreatePage';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/quotes" replace /> },
  { path: 'quotes', element: <QuoteListPage /> },
  { path: 'quotes/new', element: <QuoteCreatePage /> },
  { path: 'quotes/:id', element: <QuoteDetailPage /> },
  { path: '*', element: <Navigate to="/quotes" replace /> },
];
