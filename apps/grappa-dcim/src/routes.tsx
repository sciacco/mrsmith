import { Navigate, type RouteObject } from 'react-router-dom';
import { WorkspaceStub } from './components/WorkspaceStub';
import { CamerasPage } from './features/cameras/CameraPages';
import { CableFiberPage, PlenumPage } from './features/cabling/CablingPages';
import { EquipmentPage } from './features/equipment/EquipmentPages';
import { BuildingsPage, DatacentersPage, LayoutPage } from './features/facilities/FacilitiesPages';
import { RacksPage } from './features/racks/RackPages';
import { FiberRingsPage } from './features/rings/RingPages';
import { ServersPage } from './features/servers/ServerPages';
import { StoragePage } from './features/storage/StoragePages';
import { XconPage } from './features/xcon/XconPages';

export const routes: RouteObject[] = [
  {
    index: true,
    element: (
      <WorkspaceStub
        eyebrow="Grappa DCIM"
        title="Seleziona un'area"
        message="Scegli una voce dal menu per aprire il workspace DCIM."
      />
    ),
  },
  {
    path: 'edifici',
    element: <BuildingsPage />,
  },
  {
    path: 'sale-mmr',
    element: <DatacentersPage />,
  },
  {
    path: 'sale-mmr/:datacenterId',
    element: <DatacentersPage />,
  },
  {
    path: 'rack',
    element: <RacksPage />,
  },
  {
    path: 'rack/:rackId',
    element: <RacksPage />,
  },
  {
    path: 'rack/:rackId/potenza',
    element: <RacksPage />,
  },
  {
    path: 'isole-posizioni',
    element: <LayoutPage />,
  },
  {
    path: 'apparati',
    element: <EquipmentPage />,
  },
  {
    path: 'apparati/:apparatoId',
    element: <EquipmentPage />,
  },
  {
    path: 'server',
    element: <ServersPage />,
  },
  {
    path: 'server/:serverId',
    element: <ServersPage />,
  },
  {
    path: 'storage',
    element: <StoragePage />,
  },
  {
    path: 'storage/:storageId',
    element: <StoragePage />,
  },
  {
    path: 'telecamere',
    element: <CamerasPage />,
  },
  {
    path: 'plenum',
    element: <PlenumPage />,
  },
  {
    path: 'plenum/:plenumId',
    element: <PlenumPage />,
  },
  {
    path: 'cavi-fibre',
    element: <CableFiberPage />,
  },
  {
    path: 'cavi-fibre/:cableId',
    element: <CableFiberPage />,
  },
  {
    path: 'cross-connect',
    element: <XconPage />,
  },
  {
    path: 'cross-connect/:xconId',
    element: <XconPage />,
  },
  {
    path: 'anelli-fibra',
    element: <FiberRingsPage />,
  },
  {
    path: 'anelli-fibra/:ringId',
    element: <FiberRingsPage />,
  },
  { path: '*', element: <Navigate to="/" replace /> },
];
