import { Navigate, type RouteObject } from 'react-router-dom';
import {
  DashboardPage,
  FornitoriRoute,
  ProviderDetailPage,
} from './views';
import {
  ArticleCategoriesPage,
  DocumentTypesPage,
  PaymentMethodsPage,
  QualificationSettingsPage,
  SettingsLayout,
} from './views/settings';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/dashboard" replace /> },
  { path: '/dashboard', element: <DashboardPage /> },
  { path: '/fornitori', element: <FornitoriRoute /> },
  { path: '/fornitori/:providerId', element: <ProviderDetailPage /> },
  {
    path: '/impostazioni',
    element: <SettingsLayout />,
    children: [
      { index: true, element: <Navigate to="/impostazioni/qualifica" replace /> },
      { path: 'qualifica', element: <QualificationSettingsPage /> },
      { path: 'tipi-documento', element: <DocumentTypesPage /> },
      { path: 'pagamenti-rda', element: <PaymentMethodsPage /> },
      { path: 'articoli-categorie', element: <ArticleCategoriesPage /> },
    ],
  },
  { path: '/impostazioni-qualifica', element: <Navigate to="/impostazioni/qualifica" replace /> },
  { path: '/modalita-pagamenti-rda', element: <Navigate to="/impostazioni/pagamenti-rda" replace /> },
  { path: '/articoli-categorie', element: <Navigate to="/impostazioni/articoli-categorie" replace /> },
  { path: '*', element: <Navigate to="/dashboard" replace /> },
];
