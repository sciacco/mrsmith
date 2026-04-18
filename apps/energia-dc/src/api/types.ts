export interface LookupItem {
  id: number;
  name: string;
}

export interface RackDetail {
  id: number;
  name: string;
  customerId: number;
  customerName: string;
  buildingName: string;
  roomName: string;
  floor?: number;
  island?: number;
  rackType?: string;
  position?: string;
  orderCode?: string;
  serialNumber?: string;
  committedPower?: number;
  variableBilling: boolean;
  billingStartDate?: string;
}

export interface RackSocketStatus {
  socketId: number;
  label: string;
  ampere: number;
  maxAmpere: number;
  usagePercent: number;
  powerMeter: string;
  breaker: string;
  positions: string[];
}

export interface PowerReadingRow {
  id: number;
  socketId: number;
  socketLabel: string;
  oid: string;
  date: string;
  ampere: number;
}

export interface PowerReadingsPage {
  items: PowerReadingRow[];
  total: number;
  page: number;
  size: number;
}

export interface RackStatPoint {
  bucket: string;
  ampere: number;
  kilowatt: number;
}

export type KWPeriod = 'day' | 'month';

export interface KWPoint {
  bucket: string;
  label: string;
  rangeLabel: string;
  kilowatt: number;
}

export interface BillingCharge {
  id: number;
  startPeriod?: string;
  endPeriod?: string;
  ampere: number;
  eccedenti: number;
  amount?: number;
  pun: number;
  coefficiente: number;
  fissoCu: number;
  importoEccedenti: number;
}

export interface NoVariableRack {
  id: number;
  name: string;
  buildingName: string;
  roomName: string;
  floor?: number;
  island?: number;
  rackType?: string;
  position?: string;
  orderCode?: string;
  serialNumber?: string;
  variableBilling: boolean;
}

export interface LowConsumptionRow {
  customerId: number;
  customerName: string;
  buildingName: string;
  roomName: string;
  rackName: string;
  socketId: number;
  socketLabel: string;
  ampere: number;
  powerMeter: string;
  breaker: string;
  positions: string[];
}

export interface PowerReadingsParams {
  rackId: number;
  from: string;
  to: string;
  page: number;
  size: number;
}

export interface LowConsumptionParams {
  min: number;
  customerId: number | null;
}

export interface CustomerKWParams {
  customerId: number;
  period: KWPeriod;
  cosfi: number;
}
