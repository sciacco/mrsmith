import { useState } from 'react';
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { Button, Icon } from '@mrsmith/ui';
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
  sortable?: boolean;
}

export function KitAccordion({
  row,
  quoteId,
  documentType,
  onDelete,
  isDeleting = false,
  sortable = false,
}: KitAccordionProps) {
  const [open, setOpen] = useState(false);
  const { data: groups } = useRowProducts(quoteId, open ? row.id : 0);

  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: row.id, disabled: !sortable });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.4 : 1,
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={`${styles.accordion} ${isDragging ? styles.dragging : ''}`}
      {...attributes}
    >
      <div className={styles.header}>
        {sortable && (
          <span
            className={styles.dragHandle}
            aria-label="Trascina per riordinare"
            {...listeners}
          >
            <Icon name="grip-vertical" size={16} />
          </span>
        )}
        <button
          type="button"
          className={styles.expandBtn}
          onClick={() => setOpen(!open)}
          aria-expanded={open}
        >
          <span className={`${styles.chevron} ${open ? styles.chevronOpen : ''}`} aria-hidden="true">
            <Icon name="chevron-right" size={16} />
          </span>
          <span className={styles.kitName}>{row.internal_name}</span>
          <div className={styles.totals}>
            <span>
              <span className={styles.totalLabel}>NRC</span>
              <span key={`nrc-${row.nrc_row}`} className={styles.totalValue}>
                {row.nrc_row.toFixed(2)}
              </span>
            </span>
            <span>
              <span className={styles.totalLabel}>MRC</span>
              <span key={`mrc-${row.mrc_row}`} className={styles.totalValue}>
                {row.mrc_row.toFixed(2)}
              </span>
            </span>
          </div>
        </button>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => void onDelete(row.id)}
          loading={isDeleting}
          leftIcon={<Icon name="trash" size={16} />}
          aria-label="Elimina kit"
        >
          {''}
        </Button>
      </div>
      {open && (
        <div className={styles.body}>
          {groups ? (
            <ProductGroupRadio
              groups={groups}
              quoteId={quoteId}
              rowId={row.id}
              documentType={documentType}
            />
          ) : (
            <p className={styles.loadingHint}>Caricamento prodotti...</p>
          )}
        </div>
      )}
    </div>
  );
}
