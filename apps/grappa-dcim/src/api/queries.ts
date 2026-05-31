import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ApiError } from '@mrsmith/api-client';
import { useApiClient } from './client';
import { useOptionalAuth } from '../hooks/useOptionalAuth';
import type {
  Building,
  BuildingInput,
  CameraInput,
  CameraItem,
  Cable,
  CableInput,
  Datacenter,
  DatacenterInput,
  DatacenterMap,
  DestructiveActionRequest,
  LayoutGridResponse,
  EquipmentInput,
  EquipmentItem,
  Fiber,
  FiberAssignmentInput,
  FiberRing,
  FiberRingArcPatch,
  FiberRingInput,
  FiberRingKML,
  FiberRingNodePatch,
  FiberRingRoute,
  FiberRingTopology,
  Islet,
  IsletCanvasInput,
  LookupItem,
  MutationResponse,
  Plenum,
  PlenumInput,
  PlenumMatrix,
  PortItem,
  Position,
  RackDetail,
  RackInput,
  RackListItem,
  RackMediaInput,
  RackMoveInput,
  RackPowerReadingsResponse,
  RackPowerSummaryPoint,
  RackSocket,
  RackSocketInput,
  RackUnit,
  GrappaDCIMLookups,
  GrappaDCIMMeta,
  NICItem,
  ServerChildren,
  ServerCredentials,
  ServerCredentialsInput,
  ServerInput,
  ServerItem,
  StorageInput,
  StorageItem,
  Xcon,
  XconHop,
  XconInput,
} from './types';

export const grappaDCIMQueryKeys = {
  all: ['grappa-dcim'] as const,
  meta: () => [...grappaDCIMQueryKeys.all, 'meta'] as const,
  lookups: () => [...grappaDCIMQueryKeys.all, 'lookups'] as const,
  buildings: (filters: Record<string, unknown>) => [...grappaDCIMQueryKeys.all, 'buildings', filters] as const,
  datacenters: (filters: Record<string, unknown>) => [...grappaDCIMQueryKeys.all, 'datacenters', filters] as const,
  datacenterMap: (id: number | null) => [...grappaDCIMQueryKeys.all, 'datacenter-map', id] as const,
  datacenterLayoutGrid: (id: number | null) => [...grappaDCIMQueryKeys.all, 'datacenter-layout-grid', id] as const,
  islets: (datacenterId: number | null) => [...grappaDCIMQueryKeys.all, 'islets', datacenterId] as const,
  positions: (isletId: number | null) => [...grappaDCIMQueryKeys.all, 'positions', isletId] as const,
  racks: (filters: Record<string, unknown>) => [...grappaDCIMQueryKeys.all, 'racks', filters] as const,
  rack: (id: number | null) => [...grappaDCIMQueryKeys.all, 'rack', id] as const,
  rackUnits: (id: number | null) => [...grappaDCIMQueryKeys.all, 'rack-units', id] as const,
  rackSockets: (id: number | null) => [...grappaDCIMQueryKeys.all, 'rack-sockets', id] as const,
  rackPowerReadings: (id: number | null, page: number) => [...grappaDCIMQueryKeys.all, 'rack-power-readings', id, page] as const,
  rackPowerSummary: (id: number | null) => [...grappaDCIMQueryKeys.all, 'rack-power-summary', id] as const,
  equipment: (filters: Record<string, unknown>) => [...grappaDCIMQueryKeys.all, 'equipment', filters] as const,
  equipmentDetail: (id: number | null) => [...grappaDCIMQueryKeys.all, 'equipment-detail', id] as const,
  equipmentNics: (id: number | null) => [...grappaDCIMQueryKeys.all, 'equipment-nics', id] as const,
  equipmentTypes: () => [...grappaDCIMQueryKeys.all, 'equipment-types'] as const,
  servers: (filters: Record<string, unknown>) => [...grappaDCIMQueryKeys.all, 'servers', filters] as const,
  serverDetail: (id: number | null) => [...grappaDCIMQueryKeys.all, 'server-detail', id] as const,
  serverChildren: (id: number | null) => [...grappaDCIMQueryKeys.all, 'server-children', id] as const,
  serverCredentials: (id: number | null) => [...grappaDCIMQueryKeys.all, 'server-credentials', id] as const,
  storage: (filters: Record<string, unknown>) => [...grappaDCIMQueryKeys.all, 'storage', filters] as const,
  storageDetail: (id: number | null) => [...grappaDCIMQueryKeys.all, 'storage-detail', id] as const,
  cameras: (filters: Record<string, unknown>) => [...grappaDCIMQueryKeys.all, 'cameras', filters] as const,
  cameraDetail: (id: number | null) => [...grappaDCIMQueryKeys.all, 'camera-detail', id] as const,
  plenums: (filters: Record<string, unknown>) => [...grappaDCIMQueryKeys.all, 'plenums', filters] as const,
  plenumDetail: (id: number | null) => [...grappaDCIMQueryKeys.all, 'plenum-detail', id] as const,
  plenumMatrix: (id: number | null) => [...grappaDCIMQueryKeys.all, 'plenum-matrix', id] as const,
  cables: (filters: Record<string, unknown>) => [...grappaDCIMQueryKeys.all, 'cables', filters] as const,
  cableDetail: (id: number | null) => [...grappaDCIMQueryKeys.all, 'cable-detail', id] as const,
  cableFibers: (id: number | null) => [...grappaDCIMQueryKeys.all, 'cable-fibers', id] as const,
  ports: (filters: Record<string, unknown>) => [...grappaDCIMQueryKeys.all, 'ports', filters] as const,
  xcon: (filters: Record<string, unknown>) => [...grappaDCIMQueryKeys.all, 'xcon', filters] as const,
  xconDetail: (id: number | null) => [...grappaDCIMQueryKeys.all, 'xcon-detail', id] as const,
  xconProducts: () => [...grappaDCIMQueryKeys.all, 'xcon-products'] as const,
  fiberRings: (filters: Record<string, unknown>) => [...grappaDCIMQueryKeys.all, 'fiber-rings', filters] as const,
  fiberRingDetail: (id: number | null) => [...grappaDCIMQueryKeys.all, 'fiber-ring-detail', id] as const,
  fiberRingTopology: (id: number | null) => [...grappaDCIMQueryKeys.all, 'fiber-ring-topology', id] as const,
  fiberRingKml: (id: number | null) => [...grappaDCIMQueryKeys.all, 'fiber-ring-kml', id] as const,
};

function shouldRetry(failureCount: number, error: unknown) {
  if (error instanceof ApiError && [401, 403, 503].includes(error.status)) {
    return false;
  }
  return failureCount < 2;
}

function useDeleteWithBody() {
  const { getAccessToken, forceRefreshToken } = useOptionalAuth();

  return async <T>(path: string, body: unknown): Promise<T> => {
    const token = (await getAccessToken()) ?? (await forceRefreshToken?.());
    const res = await fetch(`/api${path}`, {
      method: 'DELETE',
      headers: {
        'Content-Type': 'application/json',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      let payload: unknown;
      try {
        payload = await res.clone().json();
      } catch {
        payload = undefined;
      }
      throw new ApiError(res.status, res.statusText, path, payload);
    }
    return res.json() as Promise<T>;
  };
}

export function useGrappaDCIMMeta() {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.meta(),
    queryFn: () => api.get<GrappaDCIMMeta>('/grappa-dcim/v1/meta'),
    retry: shouldRetry,
  });
}

export function useGrappaDCIMLookups() {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.lookups(),
    queryFn: () => api.get<GrappaDCIMLookups>('/grappa-dcim/v1/lookups'),
    retry: shouldRetry,
  });
}

export function isGrappaDCIMNotConfigured(error: unknown) {
  return error instanceof ApiError && error.status === 503;
}

function params(values: Record<string, string | number | boolean | null | undefined>) {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(values)) {
    if (value !== undefined && value !== null && value !== '') search.set(key, String(value));
  }
  const query = search.toString();
  return query ? `?${query}` : '';
}

function buildingPatchBody(body: BuildingInput & { id?: number }) {
  return {
    name: body.name,
    address: body.address,
    status: body.status,
    portalEnabled: body.portalEnabled,
    rackCapacity: body.rackCapacity,
  };
}

function datacenterPatchBody(body: DatacenterInput & { id?: number }) {
  return {
    name: body.name,
    address: body.address,
    note: body.note,
    rackCapacity: body.rackCapacity,
    status: body.status,
    customerId: body.customerId,
    portalEnabled: body.portalEnabled,
    orderCode: body.orderCode,
    buildingId: body.buildingId,
    isMmr: body.isMmr,
    setOrder: body.setOrder,
    mmrType: body.mmrType,
    serialNumber: body.serialNumber,
    floor: body.floor,
  };
}

function isletPatchBody(body: Partial<Islet> & { id?: number; datacenterId?: number }) {
  return {
    name: body.name,
    rackNum: body.rackNum,
    type: body.type,
    floor: body.floor,
    serial: body.serial,
    order: body.order,
    customerId: body.customerId,
  };
}

function positionPatchBody(body: Partial<Position> & { id: number }) {
  return {
    status: body.status,
    type: body.type,
    num: body.num,
  };
}

function rackPatchBody(body: RackInput & { id?: number }) {
  return {
    name: body.name,
    unitCount: body.unitCount,
    customerId: body.customerId,
    status: body.status,
    magnetotermico: body.magnetotermico,
    ampere: body.ampere,
    shared: body.shared,
    reserved: body.reserved,
    note: body.note,
    orderCode: body.orderCode,
    soldPower: body.soldPower,
    serialNumber: body.serialNumber,
    committedPower: body.committedPower,
    variableBilling: body.variableBilling,
  };
}

function rackSocketPatchBody(body: RackSocketInput & { id?: number }) {
  return {
    magnetotermico: body.magnetotermico,
    snmpMonitoringDevice: body.snmpMonitoringDevice,
    detectorIp: body.detectorIp,
    oid: body.oid,
    oid2: body.oid2,
    oid3: body.oid3,
    oid4: body.oid4,
    position: body.position,
    position2: body.position2,
    position3: body.position3,
    position4: body.position4,
    status: body.status,
  };
}

function equipmentPatchBody(body: EquipmentInput & { id?: number }) {
  return {
    name: body.name,
    rackId: body.rackId,
    unitPosition: body.unitPosition,
    unit: body.unit,
    managementIp: body.managementIp,
    note: body.note,
    type: body.type,
    serial: body.serial,
    os: body.os,
    model: body.model,
    customerId: body.customerId,
    status: body.status,
    bandwidth: body.bandwidth,
    portCount: body.portCount,
    portName: body.portName,
    portType: body.portType,
    portLayer: body.portLayer,
    activatedAt: body.activatedAt,
    monitoringActive: body.monitoringActive,
    firewallType: body.firewallType,
    serialNumber: body.serialNumber,
    orderCode: body.orderCode,
  };
}

function serverPatchBody(body: ServerInput & { id?: number }) {
  return {
    kind: body.kind,
    name: body.name,
    customerId: body.customerId,
    status: body.status,
    operatingSystem: body.operatingSystem,
    hostname: body.hostname,
    rackId: body.rackId,
    model: body.model,
    serial: body.serial,
    cpu: body.cpu,
    coreCount: body.coreCount,
    ram: body.ram,
    disks: body.disks,
    iloAddress: body.iloAddress,
    customerUsername: body.customerUsername,
    cdlanUsername: body.cdlanUsername,
    activatedAt: body.activatedAt,
    note: body.note,
    managementIp: body.managementIp,
    equipmentId: body.equipmentId,
    orderCode: body.orderCode,
    serialNumber: body.serialNumber,
    portCount: body.portCount,
  };
}

function storagePatchBody(body: StorageInput & { id?: number }) {
  return {
    protocol: body.protocol,
    size: body.size,
    customerId: body.customerId,
    equipmentId: body.equipmentId,
    note: body.note,
    sizeType: body.sizeType,
    status: body.status,
    orderCode: body.orderCode,
    serialNumber: body.serialNumber,
  };
}

function cameraPatchBody(body: CameraInput & { id?: number }) {
  return {
    code: body.code,
    model: body.model,
    brand: body.brand,
    position: body.position,
    ipaddr: body.ipaddr,
    status: body.status,
    serial: body.serial,
  };
}

function plenumPatchBody(body: PlenumInput & { id?: number }) {
  return {
    name: body.name,
    isle: body.isle,
    type: body.type,
    status: body.status,
  };
}

function cablePatchBody(body: CableInput & { id?: number }) {
  return {
    name: body.name,
    description: body.description,
    status: body.status,
  };
}

function fiberRingPatchBody(body: FiberRingInput & { id?: number }) {
  return {
    name: body.name,
    customerId: body.customerId,
    nodeCount: body.nodeCount,
    note: body.note,
    serialNumber: body.serialNumber,
    orderCode: body.orderCode,
    status: body.status,
  };
}

export function useBuildings(filters: { q?: string; status?: string }) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.buildings(filters),
    queryFn: () => api.get<Building[]>(`/grappa-dcim/v1/facilities/buildings${params(filters)}`),
    retry: shouldRetry,
  });
}

export function useDatacenters(filters: { q?: string; kind?: 'room' | 'mmr' | 'all'; status?: string; buildingId?: number | null }) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.datacenters(filters),
    queryFn: () => api.get<Datacenter[]>(`/grappa-dcim/v1/facilities/datacenters${params(filters)}`),
    retry: shouldRetry,
  });
}

export function useDatacenterMap(datacenterId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.datacenterMap(datacenterId),
    queryFn: () => api.get<DatacenterMap>(`/grappa-dcim/v1/facilities/datacenters/${datacenterId}/map`),
    enabled: datacenterId !== null,
    retry: shouldRetry,
  });
}

export function useDatacenterLayoutGrid(datacenterId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.datacenterLayoutGrid(datacenterId),
    queryFn: () => api.get<LayoutGridResponse>(`/grappa-dcim/v1/facilities/datacenters/${datacenterId}/layout-grid`),
    enabled: datacenterId !== null,
    retry: shouldRetry,
  });
}

export function useIslets(datacenterId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.islets(datacenterId),
    queryFn: () => api.get<Islet[]>(`/grappa-dcim/v1/layout/islets?datacenterId=${datacenterId}`),
    enabled: datacenterId !== null,
    retry: shouldRetry,
  });
}

export function usePositions(isletId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.positions(isletId),
    queryFn: () => api.get<Position[]>(`/grappa-dcim/v1/layout/islets/${isletId}/positions`),
    enabled: isletId !== null,
    retry: shouldRetry,
  });
}

export function useRacks(filters: { q?: string; status?: string; buildingId?: number | null; datacenterId?: number | null }) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.racks(filters),
    queryFn: () => api.get<RackListItem[]>(`/grappa-dcim/v1/racks${params(filters)}`),
    retry: shouldRetry,
  });
}

export function useRackDetail(rackId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.rack(rackId),
    queryFn: () => api.get<RackDetail>(`/grappa-dcim/v1/racks/${rackId}`),
    enabled: rackId !== null,
    retry: shouldRetry,
  });
}

export function useRackUnits(rackId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.rackUnits(rackId),
    queryFn: () => api.get<RackUnit[]>(`/grappa-dcim/v1/racks/${rackId}/units`),
    enabled: rackId !== null,
    retry: shouldRetry,
  });
}

export function useRackSockets(rackId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.rackSockets(rackId),
    queryFn: () => api.get<RackSocket[]>(`/grappa-dcim/v1/racks/${rackId}/sockets`),
    enabled: rackId !== null,
    retry: shouldRetry,
  });
}

export function useRackPowerReadings(rackId: number | null, page: number) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.rackPowerReadings(rackId, page),
    queryFn: () => api.get<RackPowerReadingsResponse>(`/grappa-dcim/v1/racks/${rackId}/power-readings${params({ page, size: 25 })}`),
    enabled: rackId !== null,
    retry: shouldRetry,
  });
}

export function useRackPowerSummary(rackId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.rackPowerSummary(rackId),
    queryFn: () => api.get<RackPowerSummaryPoint[]>(`/grappa-dcim/v1/racks/${rackId}/power-summary`),
    enabled: rackId !== null,
    retry: shouldRetry,
  });
}

export function useEquipment(filters: { q?: string; status?: string; rackId?: number | null }) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.equipment(filters),
    queryFn: () => api.get<EquipmentItem[]>(`/grappa-dcim/v1/equipment${params(filters)}`),
    retry: shouldRetry,
  });
}

export function useEquipmentDetail(equipmentId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.equipmentDetail(equipmentId),
    queryFn: () => api.get<EquipmentItem>(`/grappa-dcim/v1/equipment/${equipmentId}`),
    enabled: equipmentId !== null,
    retry: shouldRetry,
  });
}

export function useEquipmentNics(equipmentId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.equipmentNics(equipmentId),
    queryFn: () => api.get<NICItem[]>(`/grappa-dcim/v1/equipment/${equipmentId}/nics`),
    enabled: equipmentId !== null,
    retry: shouldRetry,
  });
}

export function useEquipmentTypes() {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.equipmentTypes(),
    queryFn: () => api.get<LookupItem[]>('/grappa-dcim/v1/equipment/type-options'),
    retry: shouldRetry,
  });
}

export function useServers(filters: { q?: string; status?: string; kind?: string }) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.servers(filters),
    queryFn: () => api.get<ServerItem[]>(`/grappa-dcim/v1/servers${params(filters)}`),
    retry: shouldRetry,
  });
}

export function useServerDetail(serverId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.serverDetail(serverId),
    queryFn: () => api.get<ServerItem>(`/grappa-dcim/v1/servers/${serverId}`),
    enabled: serverId !== null,
    retry: shouldRetry,
  });
}

export function useServerChildren(serverId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.serverChildren(serverId),
    queryFn: () => api.get<ServerChildren>(`/grappa-dcim/v1/servers/${serverId}/children`),
    enabled: serverId !== null,
    retry: shouldRetry,
  });
}

export function useServerCredentials(serverId: number | null, enabled: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.serverCredentials(serverId),
    queryFn: () => api.get<ServerCredentials>(`/grappa-dcim/v1/servers/${serverId}/credentials`),
    enabled: enabled && serverId !== null,
    retry: shouldRetry,
  });
}

export function useStorage(filters: { q?: string; status?: string }) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.storage(filters),
    queryFn: () => api.get<StorageItem[]>(`/grappa-dcim/v1/storage${params(filters)}`),
    retry: shouldRetry,
  });
}

export function useStorageDetail(storageId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.storageDetail(storageId),
    queryFn: () => api.get<StorageItem>(`/grappa-dcim/v1/storage/${storageId}`),
    enabled: storageId !== null,
    retry: shouldRetry,
  });
}

export function useCameras(filters: { q?: string }) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.cameras(filters),
    queryFn: () => api.get<CameraItem[]>(`/grappa-dcim/v1/cameras${params(filters)}`),
    retry: shouldRetry,
  });
}

export function useCameraDetail(cameraId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.cameraDetail(cameraId),
    queryFn: () => api.get<CameraItem>(`/grappa-dcim/v1/cameras/${cameraId}`),
    enabled: cameraId !== null,
    retry: shouldRetry,
  });
}

export function usePlenums(filters: { q?: string; datacenterId?: number | null }) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.plenums(filters),
    queryFn: () => api.get<Plenum[]>(`/grappa-dcim/v1/plenums${params(filters)}`),
    retry: shouldRetry,
  });
}

export function usePlenumDetail(plenumId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.plenumDetail(plenumId),
    queryFn: () => api.get<Plenum>(`/grappa-dcim/v1/plenums/${plenumId}`),
    enabled: plenumId !== null,
    retry: shouldRetry,
  });
}

export function usePlenumMatrix(plenumId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.plenumMatrix(plenumId),
    queryFn: () => api.get<PlenumMatrix>(`/grappa-dcim/v1/plenums/${plenumId}/matrix`),
    enabled: plenumId !== null,
    retry: shouldRetry,
  });
}

export function useCables(filters: { q?: string }) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.cables(filters),
    queryFn: () => api.get<Cable[]>(`/grappa-dcim/v1/cables${params(filters)}`),
    retry: shouldRetry,
  });
}

export function useCableDetail(cableId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.cableDetail(cableId),
    queryFn: () => api.get<Cable>(`/grappa-dcim/v1/cables/${cableId}`),
    enabled: cableId !== null,
    retry: shouldRetry,
  });
}

export function useCableFibers(cableId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.cableFibers(cableId),
    queryFn: () => api.get<Fiber[]>(`/grappa-dcim/v1/cables/${cableId}/fibers`),
    enabled: cableId !== null,
    retry: shouldRetry,
  });
}

export function usePorts(filters: { plenumId?: number | null; status?: string; availableForFiberId?: number | null }) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.ports(filters),
    queryFn: () => api.get<PortItem[]>(`/grappa-dcim/v1/ports${params(filters)}`),
    retry: shouldRetry,
  });
}

export function useXcon(filters: { tab: 'active' | 'ceased'; q?: string }) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.xcon(filters),
    queryFn: () => api.get<Xcon[]>(`/grappa-dcim/v1/xcon${params(filters)}`),
    retry: shouldRetry,
  });
}

export function useXconDetail(xconId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.xconDetail(xconId),
    queryFn: () => api.get<Xcon>(`/grappa-dcim/v1/xcon/${xconId}`),
    enabled: xconId !== null,
    retry: shouldRetry,
  });
}

export function useXconProducts() {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.xconProducts(),
    queryFn: () => api.get<LookupItem[]>('/grappa-dcim/v1/xcon/product-options'),
    retry: shouldRetry,
  });
}

export function useFiberRings(filters: { q?: string; status?: string; customerId?: number | null }) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.fiberRings(filters),
    queryFn: () => api.get<FiberRing[]>(`/grappa-dcim/v1/fiber-rings${params(filters)}`),
    retry: shouldRetry,
  });
}

export function useFiberRingDetail(ringId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.fiberRingDetail(ringId),
    queryFn: () => api.get<FiberRing>(`/grappa-dcim/v1/fiber-rings/${ringId}`),
    enabled: ringId !== null,
    retry: shouldRetry,
  });
}

export function useFiberRingTopology(ringId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.fiberRingTopology(ringId),
    queryFn: () => api.get<FiberRingTopology>(`/grappa-dcim/v1/fiber-rings/${ringId}/topology`),
    enabled: ringId !== null,
    retry: shouldRetry,
  });
}

export function useFiberRingKML(ringId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: grappaDCIMQueryKeys.fiberRingKml(ringId),
    queryFn: () => api.get<FiberRingKML>(`/grappa-dcim/v1/fiber-rings/${ringId}/kml`),
    enabled: ringId !== null,
    retry: shouldRetry,
  });
}

export function useFacilitiesMutations() {
  const api = useApiClient();
  const deleteWithBody = useDeleteWithBody();
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: grappaDCIMQueryKeys.all });

  return {
    saveBuilding: useMutation({
      mutationFn: (body: BuildingInput & { id?: number }) =>
        body.id
          ? api.patch<MutationResponse>(`/grappa-dcim/v1/facilities/buildings/${body.id}`, buildingPatchBody(body))
          : api.post<MutationResponse>('/grappa-dcim/v1/facilities/buildings', body),
      onSuccess: invalidate,
    }),
    ceaseBuilding: useMutation({
      mutationFn: ({ id, body }: { id: number; body: DestructiveActionRequest }) =>
        api.post<MutationResponse>(`/grappa-dcim/v1/facilities/buildings/${id}/cease`, body),
      onSuccess: invalidate,
    }),
    deleteBuilding: useMutation({
      mutationFn: ({ id, body }: { id: number; body: DestructiveActionRequest }) =>
        deleteWithBody<MutationResponse>(`/grappa-dcim/v1/facilities/buildings/${id}`, body),
      onSuccess: invalidate,
    }),
    saveDatacenter: useMutation({
      mutationFn: (body: DatacenterInput & { id?: number }) =>
        body.id
          ? api.patch<MutationResponse>(`/grappa-dcim/v1/facilities/datacenters/${body.id}`, datacenterPatchBody(body))
          : api.post<MutationResponse>('/grappa-dcim/v1/facilities/datacenters', body),
      onSuccess: invalidate,
    }),
    ceaseDatacenter: useMutation({
      mutationFn: ({ id, body }: { id: number; body: DestructiveActionRequest }) =>
        api.post<MutationResponse>(`/grappa-dcim/v1/facilities/datacenters/${id}/cease`, body),
      onSuccess: invalidate,
    }),
    deleteDatacenter: useMutation({
      mutationFn: ({ id, body }: { id: number; body: DestructiveActionRequest }) =>
        deleteWithBody<MutationResponse>(`/grappa-dcim/v1/facilities/datacenters/${id}`, body),
      onSuccess: invalidate,
    }),
  };
}

export function useLayoutMutations() {
  const api = useApiClient();
  const deleteWithBody = useDeleteWithBody();
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: grappaDCIMQueryKeys.all });

  return {
    saveIslet: useMutation({
      mutationFn: (body: Partial<Islet> & { id?: number; datacenterId?: number }) =>
        body.id
          ? api.patch<MutationResponse>(`/grappa-dcim/v1/layout/islets/${body.id}`, isletPatchBody(body))
          : api.post<MutationResponse>('/grappa-dcim/v1/layout/islets', body),
      onSuccess: invalidate,
    }),
    createPositions: useMutation({
      mutationFn: ({ isletId, count, type }: { isletId: number; count: number; type: string }) =>
        api.post<MutationResponse>(`/grappa-dcim/v1/layout/islets/${isletId}/positions/batch`, { count, type }),
      onSuccess: invalidate,
    }),
    saveIsletCanvas: useMutation({
      mutationFn: ({ id, body }: { id: number; body: IsletCanvasInput }) =>
        api.patch<MutationResponse>(`/grappa-dcim/v1/layout/islets/${id}/canvas`, body),
      onSuccess: invalidate,
    }),
    deleteIslet: useMutation({
      mutationFn: ({ id, body }: { id: number; body: DestructiveActionRequest }) =>
        deleteWithBody<MutationResponse>(`/grappa-dcim/v1/layout/islets/${id}`, body),
      onSuccess: invalidate,
    }),
    savePosition: useMutation({
      mutationFn: (body: Partial<Position> & { id: number }) =>
        api.patch<MutationResponse>(`/grappa-dcim/v1/layout/positions/${body.id}`, positionPatchBody(body)),
      onSuccess: invalidate,
    }),
    deletePosition: useMutation({
      mutationFn: ({ id, body }: { id: number; body: DestructiveActionRequest }) =>
        deleteWithBody<MutationResponse>(`/grappa-dcim/v1/layout/positions/${id}`, body),
      onSuccess: invalidate,
    }),
  };
}

export function useRackMutations() {
  const api = useApiClient();
  const deleteWithBody = useDeleteWithBody();
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: grappaDCIMQueryKeys.all });

  return {
    saveRack: useMutation({
      mutationFn: (body: RackInput & { id?: number }) =>
        body.id
          ? api.patch<MutationResponse>(`/grappa-dcim/v1/racks/${body.id}`, rackPatchBody(body))
          : api.post<MutationResponse>('/grappa-dcim/v1/racks', body),
      onSuccess: invalidate,
    }),
    moveRack: useMutation({
      mutationFn: ({ id, body }: { id: number; body: RackMoveInput }) =>
        api.post<MutationResponse>(`/grappa-dcim/v1/racks/${id}/move`, body),
      onSuccess: invalidate,
    }),
    ceaseRack: useMutation({
      mutationFn: ({ id, body }: { id: number; body: DestructiveActionRequest }) =>
        api.post<MutationResponse>(`/grappa-dcim/v1/racks/${id}/cease`, body),
      onSuccess: invalidate,
    }),
    deleteRack: useMutation({
      mutationFn: ({ id, body }: { id: number; body: DestructiveActionRequest }) =>
        deleteWithBody<MutationResponse>(`/grappa-dcim/v1/racks/${id}`, body),
      onSuccess: invalidate,
    }),
    saveRackSocket: useMutation({
      mutationFn: ({ rackId, body }: { rackId: number; body: RackSocketInput & { id?: number } }) =>
        body.id
          ? api.patch<MutationResponse>(`/grappa-dcim/v1/rack-sockets/${body.id}`, rackSocketPatchBody(body))
          : api.post<MutationResponse>(`/grappa-dcim/v1/racks/${rackId}/sockets`, body),
      onSuccess: invalidate,
    }),
    deleteRackSocket: useMutation({
      mutationFn: ({ id, body }: { id: number; body: DestructiveActionRequest }) =>
        deleteWithBody<MutationResponse>(`/grappa-dcim/v1/rack-sockets/${id}`, body),
      onSuccess: invalidate,
    }),
    replaceRackMedia: useMutation({
      mutationFn: ({ rackId, body }: { rackId: number; body: RackMediaInput }) =>
        api.put<MutationResponse>(`/grappa-dcim/v1/racks/${rackId}/media`, body),
      onSuccess: invalidate,
    }),
  };
}

export function useEquipmentMutations() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: grappaDCIMQueryKeys.all });

  return {
    saveEquipment: useMutation({
      mutationFn: (body: EquipmentInput & { id?: number }) =>
        body.id
          ? api.patch<MutationResponse>(`/grappa-dcim/v1/equipment/${body.id}`, equipmentPatchBody(body))
          : api.post<MutationResponse>('/grappa-dcim/v1/equipment', body),
      onSuccess: invalidate,
    }),
    ceaseEquipment: useMutation({
      mutationFn: ({ id, body }: { id: number; body: DestructiveActionRequest }) =>
        api.post<MutationResponse>(`/grappa-dcim/v1/equipment/${id}/cease`, body),
      onSuccess: invalidate,
    }),
  };
}

export function useServerMutations() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: grappaDCIMQueryKeys.all });

  return {
    saveServer: useMutation({
      mutationFn: (body: ServerInput & { id?: number }) =>
        body.id
          ? api.patch<MutationResponse>(`/grappa-dcim/v1/servers/${body.id}`, serverPatchBody(body))
          : api.post<MutationResponse>('/grappa-dcim/v1/servers', body),
      onSuccess: invalidate,
    }),
    saveCredentials: useMutation({
      mutationFn: ({ id, body }: { id: number; body: ServerCredentialsInput }) =>
        api.patch<MutationResponse>(`/grappa-dcim/v1/servers/${id}/credentials`, body),
      onSuccess: invalidate,
    }),
  };
}

export function useStorageMutations() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: grappaDCIMQueryKeys.all });

  return {
    saveStorage: useMutation({
      mutationFn: (body: StorageInput & { id?: number }) =>
        body.id
          ? api.patch<MutationResponse>(`/grappa-dcim/v1/storage/${body.id}`, storagePatchBody(body))
          : api.post<MutationResponse>('/grappa-dcim/v1/storage', body),
      onSuccess: invalidate,
    }),
    archiveStorage: useMutation({
      mutationFn: ({ id, body }: { id: number; body: DestructiveActionRequest }) =>
        api.post<MutationResponse>(`/grappa-dcim/v1/storage/${id}/archive`, body),
      onSuccess: invalidate,
    }),
  };
}

export function useCameraMutations() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: grappaDCIMQueryKeys.all });

  return {
    saveCamera: useMutation({
      mutationFn: (body: CameraInput & { id?: number }) =>
        body.id
          ? api.patch<MutationResponse>(`/grappa-dcim/v1/cameras/${body.id}`, cameraPatchBody(body))
          : api.post<MutationResponse>('/grappa-dcim/v1/cameras', body),
      onSuccess: invalidate,
    }),
  };
}

export function useCablingMutations() {
  const api = useApiClient();
  const { getAccessToken, forceRefreshToken } = useOptionalAuth();
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: grappaDCIMQueryKeys.all });
  const deleteWithBody = async <T>(path: string, body: unknown): Promise<T> => {
    const token = (await getAccessToken()) ?? (await forceRefreshToken?.());
    const res = await fetch(`/api${path}`, {
      method: 'DELETE',
      headers: {
        'Content-Type': 'application/json',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      let payload: unknown;
      try {
        payload = await res.clone().json();
      } catch {
        payload = undefined;
      }
      throw new ApiError(res.status, res.statusText, path, payload);
    }
    return res.json() as Promise<T>;
  };

  return {
    savePlenum: useMutation({
      mutationFn: (body: PlenumInput & { id?: number }) =>
        body.id
          ? api.patch<MutationResponse>(`/grappa-dcim/v1/plenums/${body.id}`, plenumPatchBody(body))
          : api.post<MutationResponse>('/grappa-dcim/v1/plenums', body),
      onSuccess: invalidate,
    }),
    initializeMatrix: useMutation({
      mutationFn: (id: number) => api.post<MutationResponse>(`/grappa-dcim/v1/plenums/${id}/initialize-matrix`, {}),
      onSuccess: invalidate,
    }),
    deletePlenum: useMutation({
      mutationFn: ({ id, body }: { id: number; body: DestructiveActionRequest }) =>
        deleteWithBody<MutationResponse>(`/grappa-dcim/v1/plenums/${id}`, body),
      onSuccess: invalidate,
    }),
    saveCable: useMutation({
      mutationFn: (body: CableInput & { id?: number }) =>
        body.id
          ? api.patch<MutationResponse>(`/grappa-dcim/v1/cables/${body.id}`, cablePatchBody(body))
          : api.post<MutationResponse>('/grappa-dcim/v1/cables', body),
      onSuccess: invalidate,
    }),
    deleteCable: useMutation({
      mutationFn: ({ id, body }: { id: number; body: DestructiveActionRequest }) =>
        deleteWithBody<MutationResponse>(`/grappa-dcim/v1/cables/${id}`, body),
      onSuccess: invalidate,
    }),
    assignFiber: useMutation({
      mutationFn: ({ id, body }: { id: number; body: FiberAssignmentInput }) =>
        api.patch<MutationResponse>(`/grappa-dcim/v1/fibers/${id}/assignment`, body),
      onSuccess: invalidate,
    }),
  };
}

export function useXconMutations() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: grappaDCIMQueryKeys.all });

  return {
    saveXcon: useMutation({
      mutationFn: (body: XconInput & { id?: number }) =>
        body.id
          ? api.patch<MutationResponse>(`/grappa-dcim/v1/xcon/${body.id}`, body)
          : api.post<MutationResponse>('/grappa-dcim/v1/xcon', body),
      onSuccess: invalidate,
    }),
    replaceHops: useMutation({
      mutationFn: ({ id, items }: { id: number; items: XconHop[] }) =>
        api.put<MutationResponse>(`/grappa-dcim/v1/xcon/${id}/hops`, { items }),
      onSuccess: invalidate,
    }),
  };
}

export function useFiberRingMutations() {
  const api = useApiClient();
  const { getAccessToken, forceRefreshToken } = useOptionalAuth();
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: grappaDCIMQueryKeys.all });
  const authFetch = async (path: string, init: RequestInit) => {
    const token = (await getAccessToken()) ?? (await forceRefreshToken?.());
    const headers = new Headers(init.headers);
    if (token) headers.set('Authorization', `Bearer ${token}`);
    const res = await fetch(`/api${path}`, { ...init, headers });
    if (!res.ok) {
      let payload: unknown;
      try {
        payload = await res.clone().json();
      } catch {
        payload = undefined;
      }
      throw new ApiError(res.status, res.statusText, path, payload);
    }
    return res;
  };
  const deleteWithBody = async <T>(path: string, body: unknown): Promise<T> => {
    const res = await authFetch(path, {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    return res.json() as Promise<T>;
  };

  return {
    saveRing: useMutation({
      mutationFn: (body: FiberRingInput & { id?: number }) =>
        body.id
          ? api.patch<MutationResponse>(`/grappa-dcim/v1/fiber-rings/${body.id}`, fiberRingPatchBody(body))
          : api.post<MutationResponse>('/grappa-dcim/v1/fiber-rings', body),
      onSuccess: invalidate,
    }),
    increaseNodes: useMutation({
      mutationFn: ({ id, nodeCount }: { id: number; nodeCount: number }) =>
        api.post<MutationResponse>(`/grappa-dcim/v1/fiber-rings/${id}/increase-nodes`, { nodeCount }),
      onSuccess: invalidate,
    }),
    ceaseRing: useMutation({
      mutationFn: ({ id, body }: { id: number; body: DestructiveActionRequest }) =>
        api.post<MutationResponse>(`/grappa-dcim/v1/fiber-rings/${id}/cease`, body),
      onSuccess: invalidate,
    }),
    deleteRing: useMutation({
      mutationFn: ({ id, body }: { id: number; body: DestructiveActionRequest }) =>
        deleteWithBody<MutationResponse>(`/grappa-dcim/v1/fiber-rings/${id}`, body),
      onSuccess: invalidate,
    }),
    updateNode: useMutation({
      mutationFn: ({ ringId, nodeId, body }: { ringId: number; nodeId: number; body: FiberRingNodePatch }) =>
        api.patch<MutationResponse>(`/grappa-dcim/v1/fiber-rings/${ringId}/nodes/${nodeId}`, body),
      onSuccess: invalidate,
    }),
    updateArc: useMutation({
      mutationFn: ({ ringId, arcId, body }: { ringId: number; arcId: number; body: FiberRingArcPatch }) =>
        api.patch<MutationResponse>(`/grappa-dcim/v1/fiber-rings/${ringId}/arcs/${arcId}`, body),
      onSuccess: invalidate,
    }),
    replaceRoutes: useMutation({
      mutationFn: ({ ringId, arcId, routes }: { ringId: number; arcId: number; routes: FiberRingRoute[] }) =>
        api.put<MutationResponse>(`/grappa-dcim/v1/fiber-rings/${ringId}/routes`, { arcId, routes }),
      onSuccess: invalidate,
    }),
    uploadKML: useMutation({
      mutationFn: async ({ ringId, file, name, detail }: { ringId: number; file: File; name?: string; detail?: string }) => {
        const form = new FormData();
        form.set('file', file);
        if (name) form.set('name', name);
        if (detail) form.set('detail', detail);
        const res = await authFetch(`/grappa-dcim/v1/fiber-rings/${ringId}/kml`, { method: 'POST', body: form });
        return res.json() as Promise<MutationResponse>;
      },
      onSuccess: invalidate,
    }),
    downloadArtifact: useMutation({
      mutationFn: async (artifactId: number) => {
        const res = await authFetch(`/grappa-dcim/v1/artifacts/${artifactId}/download`, { method: 'GET' });
        return res.blob();
      },
    }),
  };
}
