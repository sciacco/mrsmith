import { Navigate, type RouteObject } from 'react-router-dom';
import { KitPage } from './pages/KitPage';
import { IaaSPrezziPage } from './pages/IaaSPrezziPage';
import { TimooPrezziPage } from './pages/TimooPrezziPage';
import { GruppiScontoPage } from './pages/GruppiScontoPage';
import { ScontiEnergiaPage } from './pages/ScontiEnergiaPage';
import { IaaSCreditiPage } from './pages/IaaSCreditiPage';
import { GestioneCreditiPage } from './pages/GestioneCreditiPage';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/kit" replace /> },
  { path: 'kit', element: <KitPage /> },
  { path: 'iaas-prezzi', element: <IaaSPrezziPage /> },
  { path: 'timoo-prezzi', element: <TimooPrezziPage /> },
  { path: 'gruppi-sconto', element: <GruppiScontoPage /> },
  { path: 'sconti-energia', element: <ScontiEnergiaPage /> },
  { path: 'iaas-crediti', element: <IaaSCreditiPage /> },
  { path: 'gestione-crediti', element: <GestioneCreditiPage /> },
  { path: '*', element: <Navigate to="/kit" replace /> },
];
