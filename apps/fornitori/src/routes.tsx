import { Navigate, type RouteObject } from 'react-router-dom';
import {
  ArticleCategoriesPage,
  DashboardPage,
  FornitoriRoute,
  PaymentMethodsPage,
  ProviderDetailPage,
  QualificationSettingsPage,
} from './views';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/dashboard" replace /> },
  { path: '/dashboard', element: <DashboardPage /> },
  { path: '/fornitori', element: <FornitoriRoute /> },
  { path: '/fornitori/:providerId', element: <ProviderDetailPage /> },
  { path: '/impostazioni-qualifica', element: <QualificationSettingsPage /> },
  { path: '/modalita-pagamenti-rda', element: <PaymentMethodsPage /> },
  { path: '/articoli-categorie', element: <ArticleCategoriesPage /> },
  { path: '*', element: <Navigate to="/dashboard" replace /> },
];
