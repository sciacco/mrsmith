// Mappa stato -> variante StatusBadge per le superfici richieste (#157). Pura,
// nessuna logica di dominio: solo la resa visiva di stati gia calcolati dal
// backend.

import type { StatusBadgeVariant } from '@mrsmith/ui';
import type { TLOpinionValue } from '../../api/types';

export function tlOpinionVariant(value: TLOpinionValue): StatusBadgeVariant {
  return value === 'favorable' ? 'success' : 'danger';
}

export function outcomeVariant(value: string): StatusBadgeVariant {
  if (value === 'accepted') return 'success';
  if (value === 'rejected') return 'danger';
  return 'neutral';
}
