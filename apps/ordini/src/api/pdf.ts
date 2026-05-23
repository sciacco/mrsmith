import type { OrderDetail } from './types';
import { orderCode } from '../lib/formatters';
import { safeFilenamePart } from '../lib/downloads';

function code(order: OrderDetail): string {
  return safeFilenamePart(orderCode(order.cdlan_ndoc, order.cdlan_anno).replace('—', `ordine_${order.id}`));
}

export function kickoffFilename(order: OrderDetail): string {
  return `kick off_${code(order)}.pdf`;
}

export function activationFormFilename(order: OrderDetail): string {
  return `${order.profile_lang === 'en' ? 'Activation Form' : 'Modulo di Attivazione'}_${code(order)}.pdf`;
}

export function orderPdfFilename(order: OrderDetail): string {
  return `${code(order)}.pdf`;
}

export function signedPdfFilename(order: OrderDetail): string {
  return `${code(order)}_firmato.pdf`;
}
