import { useToast } from '@mrsmith/ui';
import type { Kit, KitProduct } from '../../types';
import { useApiClient } from '../../api/client';
import { KitMetadata } from './KitMetadata';
import { KitProductTable } from './KitProductTable';
import styles from './KitCard.module.css';

interface KitCardProps {
  kit: Kit;
  products: KitProduct[];
  helpUrl: string | null;
}

export function KitCard({ kit, products, helpUrl }: KitCardProps) {
  const api = useApiClient();
  const { toast } = useToast();

  async function handleDownloadPDF() {
    try {
      const blob = await api.postBlob(`/listini/v1/kits/${kit.id}/pdf`, {});
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `kit-${kit.id}.pdf`;
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      toast('Generazione PDF non disponibile', 'error');
    }
  }

  return (
    <div className={styles.card}>
      <div className={styles.headerRow}>
        <span className={styles.badge} style={{ background: kit.category_color || '#94a3b8' }}>
          {kit.category_name}
        </span>
        <div className={styles.headerActions}>
          {helpUrl && (
            <a
              className={styles.secondaryBtn}
              href={helpUrl}
              target="_blank"
              rel="noopener noreferrer"
            >
              Supporto
            </a>
          )}
          <button className={styles.primaryBtn} onClick={handleDownloadPDF} type="button">
            Genera PDF
          </button>
        </div>
      </div>
      <h2 className={styles.title}>{kit.internal_name}</h2>

      <KitMetadata kit={kit} />

      {kit.notes && (
        <div className={styles.notes}>
          <h3 className={styles.sectionTitle}>Note</h3>
          <p>{kit.notes}</p>
        </div>
      )}

      <KitProductTable products={products} />

      <div className={styles.footer}>
        Tutti i prezzi presenti sono IVA esclusa
      </div>
    </div>
  );
}
