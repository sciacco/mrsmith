import { useMemo, useState } from 'react';
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { Button, Icon } from '@mrsmith/ui';
import type { DocumentType, ProductGroup, QuoteRow } from '../api/types';
import { useRowProducts } from '../api/queries';
import styles from './KitCard.module.css';

interface KitCardProps {
  row: QuoteRow;
  quoteId: number;
  documentType: DocumentType;
  onEdit: (row: QuoteRow) => void;
  onDelete: (row: QuoteRow) => void;
  isDeleting?: boolean;
  sortable?: boolean;
}

function formatCurrency(value: number): string {
  return value.toFixed(2);
}

interface GroupSummary {
  group: ProductGroup;
  missing: boolean;
}

function summarize(groups: ProductGroup[] | undefined): GroupSummary[] {
  if (!groups) return [];
  return [...groups]
    .sort((a, b) => a.position - b.position)
    .map(g => ({ group: g, missing: g.required && g.included_product === null }));
}

export function KitCard({
  row,
  quoteId,
  documentType: _documentType,
  onEdit,
  onDelete,
  isDeleting = false,
  sortable = false,
}: KitCardProps) {
  const [expanded, setExpanded] = useState(true);
  const { data: groups, isLoading } = useRowProducts(quoteId, row.id);

  const summaries = useMemo(() => summarize(groups), [groups]);
  const missingCount = summaries.filter(s => s.missing).length;
  const hasIssues = missingCount > 0;

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
    <article
      ref={setNodeRef}
      style={style}
      className={`${styles.card} ${isDragging ? styles.dragging : ''} ${hasIssues ? styles.cardIssue : ''}`}
      {...attributes}
    >
      <header className={styles.header}>
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
          className={styles.toggle}
          onClick={() => setExpanded(v => !v)}
          aria-expanded={expanded}
          aria-label={expanded ? 'Comprimi kit' : 'Espandi kit'}
        >
          <span className={`${styles.chevron} ${expanded ? styles.chevronOpen : ''}`} aria-hidden>
            <Icon name="chevron-right" size={16} />
          </span>
        </button>
        <div className={styles.title}>
          <span className={styles.kitName}>{row.internal_name}</span>
          {hasIssues && (
            <span className={styles.badgeIssue}>
              <Icon name="triangle-alert" size={12} />
              {missingCount} grupp{missingCount === 1 ? 'o' : 'i'} da configurare
            </span>
          )}
        </div>
        <div className={styles.totals}>
          <div className={styles.totalItem}>
            <span className={styles.totalLabel}>NRC</span>
            <span className={styles.totalValue}>{formatCurrency(row.nrc_row)}</span>
          </div>
          <div className={styles.totalItem}>
            <span className={styles.totalLabel}>MRC</span>
            <span className={styles.totalValue}>{formatCurrency(row.mrc_row)}</span>
          </div>
        </div>
        <div className={styles.actions}>
          <Button
            variant="secondary"
            size="sm"
            leftIcon={<Icon name="pencil" size={14} />}
            onClick={() => onEdit(row)}
          >
            Modifica
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => onDelete(row)}
            loading={isDeleting}
            leftIcon={<Icon name="trash" size={16} />}
            aria-label="Elimina kit"
          >
            {''}
          </Button>
        </div>
      </header>

      {expanded && (
        <div className={styles.body}>
          {isLoading && !groups ? (
            <p className={styles.hint}>Caricamento prodotti…</p>
          ) : summaries.length === 0 ? (
            <p className={styles.hint}>Nessun gruppo prodotto in questo kit.</p>
          ) : (() => {
            const visible = summaries.filter(
              ({ group, missing }) => missing || group.included_product !== null,
            );
            if (visible.length === 0) {
              return (
                <p className={styles.hint}>
                  Nessun prodotto incluso.
                </p>
              );
            }
            return (
            <>
            <ul className={styles.productList}>
              {visible.map(({ group, missing }) => {
                if (missing) {
                  return (
                    <li key={group.group_name} className={styles.missingRow}>
                      <Icon name="triangle-alert" size={14} />
                      <span>
                        Selezione richiesta — <strong>{group.group_name}</strong>
                      </span>
                    </li>
                  );
                }
                const p = group.included_product!;
                return (
                  <li key={group.group_name} className={styles.productRow}>
                    <div className={styles.productMain}>
                      <div className={styles.productNameRow}>
                        <span className={styles.productName}>{p.product_name}</span>
                        <span className={styles.productCode}>{p.product_code}</span>
                      </div>
                      {p.extended_description && (
                        <div
                          className={styles.productDesc}
                          dangerouslySetInnerHTML={{ __html: p.extended_description }}
                        />
                      )}
                    </div>
                    <div className={styles.productQty}>× {p.quantity}</div>
                    <div className={styles.productPrices}>
                      <span>
                        <span className={styles.priceLabel}>NRC</span>
                        <span className={styles.priceValue}>{formatCurrency(p.nrc)}</span>
                      </span>
                      <span>
                        <span className={styles.priceLabel}>MRC</span>
                        <span className={styles.priceValue}>{formatCurrency(p.mrc)}</span>
                      </span>
                    </div>
                  </li>
                );
              })}
            </ul>
            </>
            );
          })()}
        </div>
      )}
    </article>
  );
}
