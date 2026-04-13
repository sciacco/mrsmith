import { Navigate, type RouteObject } from 'react-router-dom';
import HomePage from './pages/HomePage';
import OrdiniPage from './pages/OrdiniPage';
import AccessiAttiviPage from './pages/AccessiAttiviPage';
import AttivazioniInCorsoPage from './pages/AttivazioniInCorsoPage';
import RinnoviInArrivoPage from './pages/RinnoviInArrivoPage';
import AnomalieMorPage from './pages/AnomalieMorPage';
import AccountingTimooPage from './pages/AccountingTimooPage';
import AovPage from './pages/AovPage';

export const routes: RouteObject[] = [
  { index: true, element: <HomePage /> },
  { path: 'ordini', element: <OrdiniPage /> },
  { path: 'aov', element: <AovPage /> },
  { path: 'accessi-attivi', element: <AccessiAttiviPage /> },
  { path: 'attivazioni-in-corso', element: <AttivazioniInCorsoPage /> },
  { path: 'rinnovi-in-arrivo', element: <RinnoviInArrivoPage /> },
  { path: 'anomalie-mor', element: <AnomalieMorPage /> },
  { path: 'accounting-timoo', element: <AccountingTimooPage /> },
  { path: '*', element: <Navigate to="/" replace /> },
];
