import { useCallback, useEffect, useRef, useState } from 'react';
import { Icon } from '@mrsmith/ui';
import type { DocumentType, QuoteRow } from '../api/types';
import { useRowProducts } from '../api/queries';
import { ProductGroupRadio } from './ProductGroupRadio';
import styles from './KitAccordion.module.css';

interface KitAccordionProps {
  row: QuoteRow;
  quoteId: number;
  documentType: DocumentType;
  onDelete: (rowId: number) => void | Promise<void>;
  isDeleting?: boolean;
  draggable?: boolean;
  onDragStart?: (e: React.DragEvent, rowId: number) => void;
  onDragOver?: (e: React.DragEvent) => void;
  onDrop?: (e: React.DragEvent, rowId: number) => void;
}

export function KitAccordion({
  row,
  quoteId,
  documentType,
  onDelete,
  isDeleting = false,
  draggable,
  onDragStart,
  onDragOver,
  onDrop,
}: KitAccordionProps) {
  const [open, setOpen] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const confirmTimeoutRef = useRef<number | null>(null);
  const { data: groups } = useRowProducts(quoteId, open ? row.id : 0);

  useEffect(() => () => {
    if (confirmTimeoutRef.current !== null) {
      window.clearTimeout(confirmTimeoutRef.current);
    }
  }, []);

  const handleRemove = useCallback(() => {
    if (isDeleting) return;
    if (confirming) {
      if (confirmTimeoutRef.current !== null) {
        window.clearTimeout(confirmTimeoutRef.current);
        confirmTimeoutRef.current = null;
      }
      void onDelete(row.id);
      setConfirming(false);
    } else {
      setConfirming(true);
      confirmTimeoutRef.current = window.setTimeout(() => {
        setConfirming(false);
        confirmTimeoutRef.current = null;
      }, 3000);
    }
  }, [confirming, isDeleting, onDelete, row.id]);

  return (
    <div
      className={styles.accordion}
      draggable={draggable}
      onDragStart={e => onDragStart?.(e, row.id)}
      onDragOver={e => { e.preventDefault(); onDragOver?.(e); }}
      onDrop={e => onDrop?.(e, row.id)}
    >
      <div className={styles.header} onClick={() => setOpen(!open)}>
        {draggable && (
          <span className={styles.dragHandle} aria-label="Trascina per riordinare">
            <Icon name="grip-vertical" size={16} />
          </span>
        )}
        <span className={`${styles.chevron} ${open ? styles.chevronOpen : ''}`}>
          <Icon name="chevron-right" size={16} />
        </span>
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
          disabled={isDeleting}
          onClick={e => { e.stopPropagation(); handleRemove(); }}
          aria-label="Rimuovi kit"
        >
          {isDeleting ? '...' : confirming ? 'Conferma?' : <Icon name="x" size={16} />}
        </button>
      </div>
      {open && (
        <div className={styles.body}>
          {groups ? (
            <ProductGroupRadio groups={groups} quoteId={quoteId} rowId={row.id} documentType={documentType} />
          ) : (
            <p style={{ color: '#94a3b8', fontSize: '0.8125rem' }}>Caricamento prodotti...</p>
          )}
        </div>
      )}
    </div>
  );
}
