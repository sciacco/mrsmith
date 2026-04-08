import React, { useMemo, useState } from 'react';
import type { KitProduct } from '../../types';
import styles from './KitCard.module.css';

interface KitProductTableProps {
  products: KitProduct[];
}

export function KitProductTable({ products }: KitProductTableProps) {
  const grouped = useMemo(() => {
    const map = new Map<string, KitProduct[]>();

    for (const p of products) {
      const key = p.group_name ?? 'Nessun gruppo';
      let group = map.get(key);
      if (!group) {
        group = [];
        map.set(key, group);
      }
      group.push(p);
    }

    return Array.from(map.entries()).map(([name, items]) => ({
      key: name,
      label: name,
      items,
    }));
  }, [products]);

  if (products.length === 0) {
    return <p className={styles.empty}>Nessun prodotto associato</p>;
  }

  return (
    <div className={styles.groupedSections}>
      {grouped.map((section) => (
        <ProductGroup
          key={section.key}
          label={section.label}
          items={section.items}
        />
      ))}
    </div>
  );
}

interface ProductGroupProps {
  label: string;
  items: KitProduct[];
}

function ProductGroup({ label, items }: ProductGroupProps) {
  const [open, setOpen] = useState(true);

  return (
    <div className={`${styles.groupSection} ${open ? styles.groupSectionOpen : ''}`}>
      <button
        type="button"
        className={styles.groupSectionHeader}
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
      >
        <span className={styles.groupHeaderLeft}>
          <span className={`${styles.groupChevron} ${open ? styles.groupChevronOpen : ''}`}>
            <ChevronIcon />
          </span>
          <span className={styles.groupLabel}>{label}</span>
        </span>
        <span className={styles.groupCount}>{items.length}</span>
      </button>

      <div className={styles.groupBody} aria-hidden={!open}>
        <div className={styles.productTable}>
          <table>
            <thead>
              <tr>
                <th>Prodotto</th>
                <th className={styles.numCol}>NRC</th>
                <th className={styles.numCol}>MRC</th>
                <th className={styles.numCol}>Min</th>
                <th className={styles.numCol}>Max</th>
                <th>Obbl.</th>
              </tr>
            </thead>
            <tbody>
              {items.map((p, i) => (
                <React.Fragment key={`${p.product_code}-${p.position}`}>
                  <tr
                    style={{ animationDelay: `${i * 30}ms` }}
                    className={styles.productRow}
                  >
                    <td>{p.internal_name}</td>
                    <td className={styles.numCol}>{formatCurrency(p.nrc)}</td>
                    <td className={styles.numCol}>{formatCurrency(p.mrc)}</td>
                    <td className={styles.numCol}>{p.minimum}</td>
                    <td className={styles.numCol}>{p.maximum ?? '—'}</td>
                    <td>{p.required ? 'Sì' : ''}</td>
                  </tr>
                  {p.notes && (
                    <tr className={styles.notesRow}>
                      <td colSpan={6}>{p.notes}</td>
                    </tr>
                  )}
                </React.Fragment>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}

function ChevronIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
      <path d="M3 5l4 4 4-4" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function formatCurrency(value: number): string {
  return new Intl.NumberFormat('it-IT', {
    style: 'currency',
    currency: 'EUR',
    minimumFractionDigits: 2,
  }).format(value);
}
