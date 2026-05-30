export interface GrappaDCIMMeta {
  canRead: boolean;
  canOperate: boolean;
  canViewCredentials: boolean;
  appVersion?: string;
}

export interface LookupItem {
  id: string | number;
  label: string;
}

export interface GrappaDCIMLookups {
  infrastructure: LookupItem[];
  assets: LookupItem[];
  connectivity: LookupItem[];
  topology: LookupItem[];
}

export interface DestructiveActionRequest {
  confirmPrimary: boolean;
  confirmSecondary: boolean;
  confirmationPhrase?: string;
  reason?: string;
}

export interface MutationResponse {
  id?: number;
  message: string;
}

export interface DependencySummary {
  allowed: boolean;
  counts: Record<string, number>;
  message?: string;
  details?: Array<{ label: string; count: number }>;
}

export interface Building {
  id: number;
  name: string;
  address: string;
  status: string;
  portalEnabled: boolean;
  rackCapacity: number;
  createdAt?: string;
  updatedAt?: string;
  ceasedAt?: string;
  datacenterCount: number;
  rackCount: number;
}

export interface BuildingInput {
  name: string;
  address: string;
  status: string;
  portalEnabled: boolean;
  rackCapacity: number;
}

export interface Datacenter {
  id: number;
  name: string;
  address: string;
  note?: string;
  rackCapacity: number;
  status?: string;
  customerId?: number;
  portalEnabled: boolean;
  activatedAt?: string;
  ceasedAt?: string;
  orderCode?: string;
  buildingId?: number;
  buildingName?: string;
  isMmr: boolean;
  setOrder?: number;
  mmrType?: string;
  serialNumber?: string;
  floor?: string;
  isletCount: number;
  rackCount: number;
}

export interface DatacenterInput {
  name: string;
  address: string;
  note?: string;
  rackCapacity: number;
  status?: string;
  customerId?: number;
  portalEnabled: boolean;
  orderCode?: string;
  buildingId?: number;
  isMmr: boolean;
  setOrder?: number;
  mmrType?: string;
  serialNumber?: string;
  floor?: string;
}

export interface Islet {
  id: number;
  datacenterId: number;
  name: string;
  rackNum: number;
  type: string;
  floor: number;
  serial?: string;
  order?: string;
  customerId?: number;
  positionCount: number;
  occupiedCount: number;
}

export interface PositionRack {
  id: number;
  name: string;
  type: string; // 'Full' | 'Half'
  pos: string; // 'F' | 'A' | 'B'
  shared: boolean; // condiviso: cabinet shared by multiple customers — never free
}

export interface Position {
  id: number;
  status: string;
  type: string; // 'Full' | 'Half'
  num: number;
  isletId: number;
  racks: PositionRack[];
}

export interface DatacenterMap {
  datacenter: Datacenter;
  islets: Islet[];
  positions: Position[];
  racks: RackListItem[];
  incomplete: boolean;
}

export interface LayoutGridResponse {
  datacenter: Datacenter;
  blocks: LayoutGridBlock[];
  positions: Position[];
  racks: RackListItem[];
  incomplete: boolean;
  warnings: string[];
}

export interface LayoutGridBlock {
  id: number;
  datacenterId: number;
  isletId?: number;
  isletName: string;
  title: string;
  layoutWidth?: string;
  displayOrder: number;
  schemaVersion: 'layout-grid-v1';
  grid: LayoutGridCell[][];
}

export interface LayoutGridCell {
  type: 'position' | 'empty' | 'label' | 'plenum';
  pos?: number;
  text?: string;
  plenumType?: string;
  positionId?: number;
  positionStatus?: string;
  positionType?: string;
  racks?: PositionRack[];
  plenumId?: number;
  plenumName?: string;
  plenumStatus?: string;
}

export interface RackListItem {
  id: number;
  name: string;
  unitCount: number;
  customerId?: number;
  datacenterId: number;
  datacenterName?: string;
  buildingName?: string;
  status?: string;
  magnetotermico?: string;
  ampere?: number;
  floor?: number;
  island?: number;
  type?: string;
  position?: string;
  rackNumber?: number;
  positionId?: number;
  isletId?: number;
  shared?: string;
  reserved?: string;
  note?: string;
  activatedAt?: string;
  ceasedAt?: string;
  orderCode?: string;
  soldPower?: number;
  serialNumber?: string;
  committedPower?: number;
  variableBilling?: number;
  socketCount: number;
}

export interface RackUnit {
  id: number;
  num?: number;
  rackId?: number;
  deviceId?: number;
}

export interface RackSocket {
  id: number;
  rackId?: number;
  magnetotermico: string;
  snmpMonitoringDevice: string;
  detectorIp: string;
  oid: string;
  oid2: string;
  oid3: string;
  oid4: string;
  position: string;
  position2: string;
  position3: string;
  position4: string;
  status: string;
  latestAmpere?: number;
  latestReadingAt?: string;
}

export interface RackSocketInput {
  magnetotermico?: string;
  snmpMonitoringDevice?: string;
  detectorIp?: string;
  oid?: string;
  oid2?: string;
  oid3?: string;
  oid4?: string;
  position?: string;
  position2?: string;
  position3?: string;
  position4?: string;
  status?: string;
}

export interface RackMedia {
  id: number;
  path?: string;
  unitId?: number;
  side?: string;
  updatedAt?: string;
}

export interface RackMediaWrite {
  unitId: number;
  side: string;
  path: string;
}

export interface RackMediaInput {
  items: RackMediaWrite[];
}

export interface RackDetail extends RackListItem {
  units: RackUnit[];
  sockets: RackSocket[];
  media: RackMedia[];
}

export interface RackInput {
  name: string;
  unitCount: number;
  customerId?: number;
  datacenterId: number;
  status?: string;
  magnetotermico?: string;
  ampere?: number;
  floor?: number;
  island?: number;
  type: string;
  position: string;
  rackNumber?: number;
  positionId?: number;
  isletId?: number;
  shared?: string;
  reserved?: string;
  note?: string;
  orderCode?: string;
  soldPower?: number;
  serialNumber?: string;
  committedPower?: number;
  variableBilling?: number;
  socketCount?: number;
}

export interface RackMoveInput {
  datacenterId: number;
  positionId: number;
  isletId?: number;
  type: string;
  position: string;
}

export interface RackPowerReading {
  id: number;
  oid: string;
  date: string;
  ampere: number;
  rackSocketId: number;
}

export interface RackPowerReadingsResponse {
  items: RackPowerReading[];
  total: number;
  page: number;
  size: number;
}

export interface RackPowerSummaryPoint {
  day?: string;
  kilowatt?: number;
}

export interface EquipmentItem {
  id: number;
  name: string;
  rackId?: number;
  rackName?: string;
  datacenterName?: string;
  unitPosition?: number;
  unit?: number;
  managementIp?: string;
  note?: string;
  type: string;
  serial?: string;
  os?: string;
  model?: string;
  customerId?: number;
  status?: string;
  bandwidth?: number;
  portCount?: number;
  portName?: string;
  portType?: string;
  portLayer?: string;
  activatedAt?: string;
  ceasedAt?: string;
  monitoringActive?: string;
  firewallType?: string;
  serialNumber?: string;
  orderCode?: string;
  nicCount: number;
}

export interface EquipmentInput {
  name: string;
  rackId?: number;
  unitPosition?: number;
  unit?: number;
  managementIp?: string;
  note?: string;
  type: string;
  serial?: string;
  os?: string;
  model?: string;
  customerId?: number;
  status?: string;
  bandwidth?: number;
  portCount?: number;
  portName?: string;
  portType?: string;
  portLayer?: string;
  activatedAt?: string;
  monitoringActive?: string;
  firewallType?: string;
  serialNumber?: string;
  orderCode?: string;
}

export interface NICItem {
  id: number;
  equipmentId?: number;
  identifier: string;
  name: string;
  customerId?: number;
  note?: string;
  type?: string;
  layer?: string;
  linkedServerId?: number;
  status?: string;
}

export interface ServerItem {
  id: number;
  kind: string;
  name?: string;
  customerId?: number;
  status?: string;
  operatingSystem?: string;
  hostname?: string;
  rackId?: number;
  rackName?: string;
  model?: string;
  serial?: string;
  cpu?: string;
  coreCount?: number;
  ram?: number;
  disks?: string;
  iloAddress?: string;
  customerUsername?: string;
  cdlanUsername?: string;
  activatedAt?: string;
  ceasedAt?: string;
  note?: string;
  managementIp?: string;
  equipmentId?: number;
  equipmentName?: string;
  orderCode?: string;
  serialNumber?: string;
  portCount?: number;
}

export interface ServerInput {
  kind: string;
  name?: string;
  customerId?: number;
  status?: string;
  operatingSystem?: string;
  hostname?: string;
  rackId?: number;
  model?: string;
  serial?: string;
  cpu?: string;
  coreCount?: number;
  ram?: number;
  disks?: string;
  iloAddress?: string;
  customerUsername?: string;
  cdlanUsername?: string;
  activatedAt?: string;
  note?: string;
  managementIp?: string;
  equipmentId?: number;
  orderCode?: string;
  serialNumber?: string;
  portCount?: number;
}

export interface ServerChildren {
  cards: Array<{ id: number; physicalName?: string; osName?: string; ip?: string; subnetmaskId?: number; note?: string }>;
  applications: Array<{ id: number; name?: string; managedByCdlan?: string }>;
  services: Array<{ id: number; name?: string }>;
  ports: Array<{ id: number; interfaceName?: string; destinationInterface?: string; portType?: string }>;
}

export interface ServerCredentials {
  serverId: number;
  iloAddress?: string;
  iloUsername?: string;
  customerRootAccess?: string;
  customerUsername?: string;
  cdlanUsername?: string;
  iloPasswordStored: boolean;
  rootAdministratorStored: boolean;
  customerPasswordStored: boolean;
  cdlanPasswordStored: boolean;
  passwordValueAccessEnabled: boolean;
  passwordWriteAccessEnabled: boolean;
}

export interface ServerCredentialsInput {
  iloAddress?: string;
  iloUsername?: string;
  customerRootAccess?: string;
  customerUsername?: string;
  cdlanUsername?: string;
}

export interface StorageItem {
  id: number;
  protocol?: string;
  size?: number;
  customerId: number;
  equipmentId: number;
  equipment?: string;
  note?: string;
  sizeType?: string;
  status: string;
  createdAt?: string;
  closedAt?: string;
  orderCode?: string;
  serialNumber?: string;
  readOnly: boolean;
}

export interface StorageInput {
  protocol?: string;
  size?: number;
  customerId: number;
  equipmentId: number;
  note?: string;
  sizeType?: string;
  status?: string;
  orderCode?: string;
  serialNumber?: string;
}

export interface CameraItem {
  id: number;
  code: string;
  model: string;
  brand: string;
  position: string;
  ipaddr?: string;
  status?: string;
  serial?: string;
}

export interface CameraInput {
  code: string;
  model: string;
  brand: string;
  position: string;
  ipaddr?: string;
  status?: string;
  serial?: string;
}

export interface Plenum {
  id: number;
  name?: string;
  isle?: string;
  type?: string;
  datacenterId: number;
  datacenterName?: string;
  status: string;
  slotCount: number;
  linkedPortCount: number;
}

export interface PlenumInput {
  name?: string;
  isle?: string;
  type?: string;
  datacenterId: number;
  status: string;
}

export interface PlenumMatrix {
  plenum: Plenum;
  slots: PlenumMatrixSlot[];
  incomplete: boolean;
  expectedSlots: number;
  expectedCells: number;
  freeCells: number;
  assignedCells: number;
  missingCells: number;
  mapOnlyRecords: number;
}

export interface PlenumMatrixSlot {
  id?: number;
  cable: number;
  number: number;
  type?: string;
  status?: string;
  missing: boolean;
  cells: PlenumMatrixCell[];
}

export interface PlenumMatrixCell {
  cable: number;
  slotNumber: number;
  fiber: number;
  status: string;
  portId?: number;
  portLabel?: string;
  fiberId?: number;
}

export interface Cable {
  id: number;
  name: string;
  description: string;
  fibersNum: number;
  status: string;
  assignedFibers: number;
}

export interface CableInput {
  name: string;
  description: string;
  fibersNum: number;
  status: string;
}

export interface Fiber {
  id: number;
  number: number;
  status: string;
  cableId: number;
  leftPortId?: number;
  rightPortId?: number;
  leftLabel?: string;
  rightLabel?: string;
}

export interface FiberAssignmentInput {
  leftPortId?: number;
  rightPortId?: number;
}

export interface PortItem {
  id: number;
  slotId: number;
  number?: number;
  status: string;
  plSlotId?: number;
  plPortNumber?: number;
  rackId?: number;
  rackName?: string;
  plenumId?: number;
  deviceId?: number;
  name?: string;
  cableFiberId?: number;
  label: string;
}

export interface Xcon {
  id: number;
  ticket: string;
  pa?: string;
  customerId: number;
  status: string;
  orderCode?: string;
  serialNumber?: string;
  type: string;
  activatedAt?: string;
  ceasedAt?: string;
  aEndUnit: string;
  aEndSlot?: string;
  aEndFibers: string;
  aEndEquipment: string;
  zEndUnit: string;
  zEndSlot?: string;
  zEndFibers: string;
  zEndEquipment: string;
  note?: string;
  extendedTicket?: string;
  customerNote?: string;
  source?: string;
  createdAt?: string;
  aEndRackId?: number;
  zEndRackId?: number;
  loaName?: string;
  loaId?: number;
  mmrPort?: string;
  hops?: XconHop[];
}

export interface XconInput {
  ticket: string;
  pa?: string;
  customerId: number;
  status: string;
  orderCode?: string;
  serialNumber?: string;
  type: string;
  activatedAt?: string;
  ceasedAt?: string;
  aEndUnit: string;
  aEndSlot?: string;
  aEndFibers: string;
  aEndEquipment: string;
  zEndUnit: string;
  zEndSlot?: string;
  zEndFibers: string;
  zEndEquipment: string;
  note?: string;
  extendedTicket?: string;
  customerNote?: string;
  source?: string;
  aEndRackId?: number;
  zEndRackId?: number;
  loaName?: string;
  loaId?: number;
  mmrPort?: string;
}

export interface XconHop {
  id?: number;
  xconId?: number;
  room: string;
  rack: string;
  unit: string;
  slot?: string;
  fibers: string;
  order: number;
  rackId: number;
}

export interface FiberRing {
  id: number;
  name: string;
  customerId?: number;
  nodeCount: number;
  note?: string;
  serialNumber?: string;
  orderCode?: string;
  status: string;
  kmlFilePresent: boolean;
  nodeTotal: number;
  arcTotal: number;
  routeTotal: number;
  kmlArtifactTotal: number;
  deleteCheck?: DependencySummary;
  topologyConsistent: boolean;
}

export interface FiberRingInput {
  name: string;
  customerId?: number;
  nodeCount: number;
  note?: string;
  serialNumber?: string;
  orderCode?: string;
  status?: string;
}

export interface FiberRingNode {
  id: number;
  identifier: string;
  address: string;
  lineSheetId?: number;
  customerId?: number;
  ringId: number;
  longitude?: number;
  latitude?: number;
  position?: number;
  switchModel?: string;
  switchSerialNumber?: string;
  switchMacAddress?: string;
  ipAddress?: string;
  upsIpAddress?: string;
  eapsMasterNode?: string;
  eastNodeId?: number;
  eastPort?: string;
  primaryEastPort?: string;
  secondaryEastPort?: string;
  eastTransceiverType?: string;
  westNodeId?: number;
  westPort?: string;
  primaryWestPort?: string;
  secondaryWestPort?: string;
  westTransceiverType?: string;
  note?: string;
}

export type FiberRingNodePatch = Partial<Omit<FiberRingNode, 'id' | 'ringId'>>;

export interface FiberRingArc {
  id: number;
  ringId: number;
  fromNodeId: number;
  toNodeId: number;
  fromIdentifier?: string;
  toIdentifier?: string;
  distance?: number;
  attenuation?: number;
  reference?: string;
  metrowebReference?: string;
  releasedAt?: string;
  routes?: FiberRingRoute[];
}

export interface FiberRingArcPatch {
  distance?: number;
  attenuation?: number;
  reference?: string;
  metrowebReference?: string;
  releasedAt?: string;
}

export interface FiberRingRoute {
  id?: number;
  arcId?: number;
  identifier?: string;
  sourceCabinet?: string;
  sourceLevel?: string;
  sourceCable?: string;
  sourceFibers?: string;
  sourceOpticalSegment?: string;
  destinationCabinet?: string;
  destinationLevel?: string;
  destinationCable?: string;
  destinationFibers?: string;
  destinationOpticalSegment?: string;
  routeLengthMeters?: number;
  dropLengthMeters?: number;
}

export interface FiberRingTopology {
  ring: FiberRing;
  nodes: FiberRingNode[];
  arcs: FiberRingArc[];
}

export interface Artifact {
  id: number;
  kind: string;
  name: string;
  fileName: string;
  ringName?: string;
  detail?: string;
  available: boolean;
  downloadUrl?: string;
}

export interface FiberRingKML {
  ringId: number;
  artifacts: Artifact[];
}
