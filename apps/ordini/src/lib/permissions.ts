import { hasAnyRole } from '@mrsmith/auth-client';
import type { OrderDetail, OrderRow, OrderState } from '../api/types';

export const CUSTOMER_RELATIONS_ROLES = ['app_customer_relations'] as const;

export function hasCustomerRelations(userRoles: readonly string[] | undefined): boolean {
  return hasAnyRole(userRoles, CUSTOMER_RELATIONS_ROLES);
}

function state(order: OrderDetail | null | undefined): OrderState | null {
  return order?.cdlan_stato ?? null;
}

export function canEditBozzaHeader(order: OrderDetail | null | undefined, roles: readonly string[] | undefined): boolean {
  return state(order) === 'BOZZA' && hasCustomerRelations(roles);
}

export function canSendToErp(order: OrderDetail | null | undefined, roles: readonly string[] | undefined, fileSelected: boolean): boolean {
  return canEditBozzaHeader(order, roles) && Boolean(order?.cdlan_dataconferma) && Boolean(order?.cdlan_cliente) && fileSelected;
}

export function canEditReferents(order: OrderDetail | null | undefined, roles: readonly string[] | undefined): boolean {
  return (state(order) === 'BOZZA' || state(order) === 'INVIATO') && hasCustomerRelations(roles);
}

export function canEditSerialNumber(order: OrderDetail | null | undefined): boolean {
  return state(order) === 'BOZZA';
}

export function canEditTechnicalNotes(): boolean {
  return true;
}

export function canOpenActivationModal(order: OrderDetail | null | undefined, roles: readonly string[] | undefined, row: OrderRow): boolean {
  return state(order) === 'INVIATO' && hasCustomerRelations(roles) && row.data_annullamento == null;
}

export function canShowArxivarFilePicker(order: OrderDetail | null | undefined, roles: readonly string[] | undefined): boolean {
  const current = state(order);
  return hasCustomerRelations(roles) && current !== 'ANNULLATO' && current !== 'PERSO' && current !== 'ATTIVO';
}

export function canDownloadKickoffPdf(order: OrderDetail | null | undefined, roles: readonly string[] | undefined): boolean {
  return state(order) === 'INVIATO' && hasCustomerRelations(roles);
}

export function canDownloadActivationFormPdf(order: OrderDetail | null | undefined, roles: readonly string[] | undefined): boolean {
  const current = state(order);
  return (current === 'INVIATO' || current === 'ATTIVO') && hasCustomerRelations(roles);
}

export function canDownloadOrderPdf(order: OrderDetail | null | undefined): boolean {
  return !order?.arx_doc_number;
}

export function canDownloadSignedPdf(order: OrderDetail | null | undefined): boolean {
  return Boolean(order?.arx_doc_number);
}
