import { useState } from 'react';
import { Skeleton } from '@mrsmith/ui';
import { useKits, useKitProducts, useKitHelpUrl } from '../hooks/useApi';
import { KitList } from '../components/Kit/KitList';
import { KitCard } from '../components/Kit/KitCard';
import styles from './KitPage.module.css';

export function KitPage() {
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const { data: kits, isLoading: kitsLoading } = useKits();
  const { data: products, isLoading: productsLoading } = useKitProducts(selectedId);
  const { data: helpData } = useKitHelpUrl(selectedId);

  const selectedKit = kits?.find((k) => k.id === selectedId) ?? null;

  return (
    <div className={styles.page}>
      <div className={styles.container}>
        {kitsLoading ? (
          <div className={styles.skeletonList}>
            {Array.from({ length: 8 }, (_, i) => (
              <Skeleton key={i} />
            ))}
          </div>
        ) : (
          <KitList
            kits={kits ?? []}
            selectedId={selectedId}
            onSelect={setSelectedId}
          />
        )}
        <div className={styles.detail}>
          {selectedKit ? (
            productsLoading ? (
              <div className={styles.skeletonCard}>
                <Skeleton />
                <Skeleton />
                <Skeleton />
              </div>
            ) : (
              <KitCard
                kit={selectedKit}
                products={products ?? []}
                helpUrl={helpData?.help_url ?? null}
              />
            )
          ) : (
            <div className={styles.placeholder}>
              <h1 className={styles.placeholderTitle}>Consultazione listino</h1>
              <p className={styles.placeholderHint}>Seleziona un kit dalla lista per visualizzarne i dettagli</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
