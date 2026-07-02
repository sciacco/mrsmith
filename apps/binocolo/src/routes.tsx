import { Navigate, type RouteObject } from 'react-router-dom';
import { TestPage } from './pages/TestPage';
import { TargetPage } from './pages/TargetPage';
import { ConfigPage } from './pages/ConfigPage';
import { CompanyDossierPage } from './pages/CompanyDossierPage';
import { WebSearchPage } from './pages/WebSearchPage';
import { RicerchePage } from './pages/ricerche/RicerchePage';
import { NuovaRicercaPage } from './pages/ricerche/NuovaRicercaPage';
import { RicercaDetailPage } from './pages/ricerche/RicercaDetailPage';
import { IniziativePage } from './pages/iniziative/IniziativePage';
import { IniziativaBoardPage } from './pages/iniziative/IniziativaBoardPage';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/ricerche" replace /> },
  { path: 'ricerche', element: <RicerchePage /> },
  { path: 'ricerche/nuova', element: <NuovaRicercaPage /> },
  { path: 'ricerche/:id', element: <RicercaDetailPage /> },
  { path: 'iniziative', element: <IniziativePage /> },
  { path: 'iniziative/:id', element: <IniziativaBoardPage /> },
  { path: 'target', element: <TargetPage /> },
  { path: 'azienda', element: <CompanyDossierPage /> },
  { path: 'ricerca-web', element: <WebSearchPage /> },
  { path: 'config', element: <ConfigPage /> },
  { path: 'test', element: <TestPage /> },
];
