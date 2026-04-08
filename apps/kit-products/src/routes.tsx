import { Navigate, type RouteObject } from 'react-router-dom';
import { ProductsPage } from './views/products/ProductsPage';
import { CategoriesPage } from './views/settings/CategoriesPage';
import { CustomerGroupsPage } from './views/settings/CustomerGroupsPage';
import { KitDetailPage } from './views/kit/KitDetailPage';
import { KitListPage } from './views/kit/KitListPage';
import { KitDiscountsPage } from './views/mistra/KitDiscountsPage';
import { PriceSimulatorPage } from './views/mistra/PriceSimulatorPage';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/kit" replace /> },
  {
    path: 'kit',
    element: <KitListPage />,
  },
  {
    path: 'kit/:id',
    element: <KitDetailPage />,
  },
  {
    path: 'products',
    element: <ProductsPage />,
  },
  {
    path: 'discounts',
    element: <KitDiscountsPage />,
  },
  {
    path: 'simulator',
    element: <PriceSimulatorPage />,
  },
  { path: 'settings', element: <Navigate to="/settings/categories" replace /> },
  {
    path: 'settings/categories',
    element: <CategoriesPage />,
  },
  {
    path: 'settings/customer-groups',
    element: <CustomerGroupsPage />,
  },
];
