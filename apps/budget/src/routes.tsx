import { Navigate, type RouteObject } from 'react-router-dom';
import { GruppiPage } from './views/gruppi/GruppiPage';
import { HomePage } from './views/home/HomePage';
import { VociDiCostoPage } from './views/voci-di-costo/VociDiCostoPage';
import { CentriDiCostoPage } from './views/centri-di-costo/CentriDiCostoPage';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/groups" replace /> },
  { path: 'home', element: <HomePage /> },
  { path: 'groups', element: <GruppiPage /> },
  { path: 'cost-centers', element: <CentriDiCostoPage /> },
  { path: 'cost-items', element: <VociDiCostoPage /> },
];
