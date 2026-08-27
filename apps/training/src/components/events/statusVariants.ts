// Mappa stato -> variante StatusBadge per le superfici evento (#156). Pura,
// nessuna logica di dominio: solo la resa visiva di stati gia calcolati dal
// backend.

import type { StatusBadgeVariant } from '@mrsmith/ui';
import type { DeliveryStatus, EconomicState } from '../../api/types';

export function deliveryStatusVariant(status: DeliveryStatus): StatusBadgeVariant {
  switch (status) {
    case 'completed':
      return 'success';
    case 'partially_completed':
      return 'warning';
    case 'not_attended':
    case 'cancelled':
      return 'danger';
    case 'in_progress':
      return 'accent';
    default:
      return 'neutral';
  }
}

export function economicStateVariant(state: EconomicState): StatusBadgeVariant {
  if (state === 'approved') return 'success';
  if (state === 'rejected') return 'danger';
  return 'warning';
}
