import { Navigate, type RouteObject } from 'react-router-dom';
import TransazioniWhmcsPage from './pages/TransazioniWhmcsPage';
import FatturePrometeusPage from './pages/FatturePrometeusPage';
import NuoviArticoliPage from './pages/NuoviArticoliPage';
import ReportXConnectRhPage from './pages/ReportXConnectRhPage';
import TicketRemoteHandsPage from './pages/TicketRemoteHandsPage';
import ConsumiEnergiaColoPage from './pages/ConsumiEnergiaColoPage';
import OrdiniSalesPage from './pages/OrdiniSalesPage';
import OrdiniSalesDetailPage from './pages/OrdiniSalesDetailPage';
import ReportDdtCespitiPage from './pages/ReportDdtCespitiPage';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/ordini-sales" replace /> },
  { path: 'transazioni-whmcs', element: <TransazioniWhmcsPage /> },
  { path: 'fatture-prometeus', element: <FatturePrometeusPage /> },
  { path: 'nuovi-articoli', element: <NuoviArticoliPage /> },
  { path: 'report-xconnect-rh', element: <ReportXConnectRhPage /> },
  { path: 'ticket-remote-hands', element: <TicketRemoteHandsPage /> },
  { path: 'consumi-energia-colo', element: <ConsumiEnergiaColoPage /> },
  { path: 'ordini-sales', element: <OrdiniSalesPage /> },
  { path: 'ordini-sales/:id', element: <OrdiniSalesDetailPage /> },
  { path: 'report-ddt-cespiti', element: <ReportDdtCespitiPage /> },
  { path: '*', element: <Navigate to="/ordini-sales" replace /> },
];
