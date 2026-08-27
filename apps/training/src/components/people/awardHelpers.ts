// Helper condivisi tra Conseguimenti (people/AwardsSection) e Dettaglio
// certificazione (catalog/CertificationDetailDrawer): mappa stato->variante
// del conseguimento e salvataggio di un blob scaricato come file (#162).

import type { StatusBadgeVariant } from '@mrsmith/ui';
import type { ApiClient } from '@mrsmith/api-client';

export function awardStatusVariant(status: string): StatusBadgeVariant {
  if (status === 'expired') return 'danger';
  if (status === 'valid_no_expiry') return 'neutral';
  return 'success';
}

export async function downloadFile(api: ApiClient, url: string, filename: string): Promise<void> {
  const blob = await api.getBlob(url);
  const objectUrl = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = objectUrl;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(objectUrl);
}
