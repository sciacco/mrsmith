import type { ReactNode } from 'react';
import { Navigate, type RouteObject } from 'react-router-dom';
import { ProductsPage } from './views/products/ProductsPage';
import { CategoriesPage } from './views/settings/CategoriesPage';
import { CustomerGroupsPage } from './views/settings/CustomerGroupsPage';
import { ProductGroupsPage } from './views/settings/ProductGroupsPage';
import { KitDetailPage } from './views/kit/KitDetailPage';
import { KitListPage } from './views/kit/KitListPage';
import { KitDiscountsPage } from './views/mistra/KitDiscountsPage';
import { PriceSimulatorPage } from './views/mistra/PriceSimulatorPage';
import { getRuntimeConfig } from './runtimeConfig';
import { WorkspacePlaceholder } from './views/WorkspacePlaceholder';

function ArakFeature({
  children,
  title,
  description,
}: {
  children: ReactNode;
  title: string;
  description: string;
}) {
  return getRuntimeConfig().arakEnabled ? (
    <>{children}</>
  ) : (
    <WorkspacePlaceholder eyebrow="Config" title={title} description={description} />
  );
}

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
    element: (
      <ArakFeature
        title="Sconti kit non disponibili"
        description="La configurazione Arak non e presente in questo ambiente. Le viste proxy Mistra restano disabilitate."
      >
        <KitDiscountsPage />
      </ArakFeature>
    ),
  },
  {
    path: 'simulator',
    element: (
      <ArakFeature
        title="Simulatore non disponibile"
        description="La configurazione Arak non e presente in questo ambiente. Le viste proxy Mistra restano disabilitate."
      >
        <PriceSimulatorPage />
      </ArakFeature>
    ),
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
  {
    path: 'settings/product-groups',
    element: <ProductGroupsPage />,
  },
  { path: '*', element: <Navigate to="/kit" replace /> },
];
