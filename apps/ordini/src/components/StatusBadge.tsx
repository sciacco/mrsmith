import { StatusBadge as SharedStatusBadge, type StatusBadgeVariant } from '@mrsmith/ui';
import type { OrderState } from '../api/types';
import { formatStato } from '../lib/formatters';

function variantForState(state: OrderState | null | undefined): StatusBadgeVariant {
  switch ((state ?? '').toUpperCase()) {
    case 'ATTIVO':
      return 'success';
    case 'INVIATO':
      return 'accent';
    case 'BOZZA':
      return 'warning';
    case 'ANNULLATO':
    case 'PERSO':
      return 'danger';
    default:
      return 'neutral';
  }
}

export function StatusBadge({ state, className }: { state: OrderState | null | undefined; className?: string }) {
  return <SharedStatusBadge value={formatStato(state)} variant={variantForState(state)} className={className} />;
}
