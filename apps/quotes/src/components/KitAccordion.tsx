import { useState, useCallback } from 'react';
import type { QuoteRow } from '../api/types';
import { useRowProducts } from '../api/queries';
import { ProductGroupRadio } from './ProductGroupRadio';
import styles from './KitAccordion.module.css';

interface KitAccordionProps {
  row: QuoteRow;
  quoteId: number;
  onDelete: (rowId: number) => void;
  draggable?: boolean;
  onDragStart?: (e: React.DragEvent, rowId: number) => void;
  onDragOver?: (e: React.DragEvent) => void;
  onDrop?: (e: React.DragEvent, rowId: number) => void;
}

export function KitAccordion({ row, quoteId, onDelete, draggable, onDragStart, onDragOver, onDrop }: KitAccordionProps) {
  const [open, setOpen] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const { data: products } = useRowProducts(quoteId, open ? row.id : 0);

  const handleRemove = useCallback(() => {
    if (confirming) {
      onDelete(row.id);
      setConfirming(false);
    } else {
      setConfirming(true);
      setTimeout(() => setConfirming(false), 3000);
    }
  }, [confirming, onDelete, row.id]);

  return (
    <div
      className={styles.accordion}
      draggable={draggable}
      onDragStart={e => onDragStart?.(e, row.id)}
      onDragOver={e => { e.preventDefault(); onDragOver?.(e); }}
      onDrop={e => onDrop?.(e, row.id)}
    >
      <div className={styles.header} onClick={() => setOpen(!open)}>
        {draggable && <span className={styles.dragHandle}>&#x2630;</span>}
        <span className={`${styles.chevron} ${open ? styles.chevronOpen : ''}`}>&#x25B6;</span>
        <span className={styles.kitName}>{row.internal_name}</span>
        <div className={styles.totals}>
          <span>
            <span className={styles.totalLabel}>NRC</span>
            <span className={styles.totalValue}>{row.nrc_row.toFixed(2)}</span>
          </span>
          <span>
            <span className={styles.totalLabel}>MRC</span>
            <span className={styles.totalValue}>{row.mrc_row.toFixed(2)}</span>
          </span>
        </div>
        <button
          className={confirming ? styles.confirmBtn : styles.removeBtn}
          onClick={e => { e.stopPropagation(); handleRemove(); }}
        >
          {confirming ? 'Conferma?' : '\u2715'}
        </button>
      </div>
      {open && (
        <div className={styles.body}>
          {products ? (
            <ProductGroupRadio products={products} quoteId={quoteId} rowId={row.id} />
          ) : (
            <p style={{ color: '#94a3b8', fontSize: '0.8125rem' }}>Caricamento prodotti...</p>
          )}
        </div>
      )}
    </div>
  );
}
