import { Navigate, type RouteObject } from 'react-router-dom';
import { OrdiniRicorrentiPage } from './pages/OrdiniRicorrentiPage';
import { OrdiniDettaglioPage } from './pages/OrdiniDettaglioPage';
import { FatturePage } from './pages/FatturePage';
import { AccessiPage } from './pages/AccessiPage';
import { IaaSPayPerUsePage } from './pages/IaaSPayPerUsePage';
import { TimooTenantsPage } from './pages/TimooTenantsPage';
import { LicenzeWindowsPage } from './pages/LicenzeWindowsPage';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/ordini-dettaglio" replace /> },
  { path: 'ordini-ricorrenti', element: <OrdiniRicorrentiPage /> },
  { path: 'ordini-dettaglio', element: <OrdiniDettaglioPage /> },
  { path: 'fatture', element: <FatturePage /> },
  { path: 'accessi', element: <AccessiPage /> },
  { path: 'iaas-ppu', element: <IaaSPayPerUsePage /> },
  { path: 'timoo', element: <TimooTenantsPage /> },
  { path: 'licenze-windows', element: <LicenzeWindowsPage /> },
  { path: '*', element: <Navigate to="/ordini-dettaglio" replace /> },
];
