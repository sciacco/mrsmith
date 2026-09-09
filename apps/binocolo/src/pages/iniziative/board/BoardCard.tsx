import { forwardRef, useLayoutEffect, useRef, useState, type CSSProperties, type HTMLAttributes } from 'react';
import { useDraggable } from '@dnd-kit/core';
import { Icon, Tooltip } from '@mrsmith/ui';
import type { MAInitiativeCardView, MACardProvenance } from '../../../api/types';
import { MATagChips } from '../../../components/tags/MATagChips';
import { stateVars, esitoLabel, isTerminalState } from '../../../lib/cardStates';
import { dateLabel } from '../../ricerche/helpers';
import { dossierState } from './useBoardData';
import styles from './board.module.css';

const REGISTRY_LABELS: Record<string, { label: string; kind: 'info' | 'warn' }> = {
  non_vende: { label: 'Non vende', kind: 'warn' },
  in_trattativa_altrui: { label: 'In trattativa con altri', kind: 'warn' },
  da_evitare: { label: 'Da evitare', kind: 'warn' },
  gia_cliente: { label: 'Già cliente', kind: 'info' },
  partner: { label: 'Partner', kind: 'info' },
};

function registryLabel(kind: string) {
  return REGISTRY_LABELS[kind] ?? { label: kind, kind: 'info' as const };
}

function latestProvenance(card: { provenances?: MACardProvenance[] }): MACardProvenance | undefined {
  return card.provenances?.[0];
}

function Stars({ rating }: { rating: number }) {
  const r = Math.max(0, Math.min(3, rating));
  return (
    <span className={styles.stars}>
      {'★'.repeat(r)}
      {r < 3 ? <span className={styles.off}>{'★'.repeat(3 - r)}</span> : null}
    </span>
  );
}

function TruncatedCardEvent({ text }: { text: string }) {
  const textRef = useRef<HTMLSpanElement>(null);
  const [truncated, setTruncated] = useState(false);

  useLayoutEffect(() => {
    const node = textRef.current;
    if (!node) return;

    const update = () => setTruncated(node.scrollWidth > node.clientWidth);
    update();

    if (typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(update);
    observer.observe(node);
    return () => observer.disconnect();
  }, [text]);

  return (
    <Tooltip content={text} placement="top" maxWidth={420} disabled={!truncated}>
      <span ref={textRef} className={styles.txt} tabIndex={truncated ? 0 : undefined}>
        {text}
      </span>
    </Tooltip>
  );
}

interface BoardCardProps extends HTMLAttributes<HTMLDivElement> {
  card: MAInitiativeCardView;
  compact?: boolean;
  dragging?: boolean;
  /** Chip iniziativa (vista aggregata /pipeline). */
  initiativeChip?: string;
  onOpenDossier?: (card: MAInitiativeCardView) => void;
}

/** Card presentazionale della board. I colori di stato arrivano via stateVars()
 *  (bordo sinistro, chip). forwardRef + spread props così i view la avvolgono in
 *  un nodo draggable di @dnd-kit senza duplicare il markup. */
export const BoardCard = forwardRef<HTMLDivElement, BoardCardProps>(function BoardCard(
  { card, compact, dragging, initiativeChip, onOpenDossier, className, style, ...rest },
  ref,
) {
  const prov = latestProvenance(card);
  const ds = dossierState(card.dossierStatus);
  const terminal = isTerminalState(card.state);
  const facts = card.registryFacts ?? [];
  const collisions = card.collisions ?? [];

  return (
    <div
      ref={ref}
      className={[styles.kcard, compact ? styles.compact : '', dragging ? styles.dragging : '', className ?? '']
        .filter(Boolean)
        .join(' ')}
      style={{ ...(stateVars(card.state) as CSSProperties), ...style }}
      data-flip-id={card.companyKey}
      data-state={card.state}
      tabIndex={0}
      {...rest}
    >
      <div className={styles.cardName}>
        <span className={styles.nm} title={card.companyName}>
          {card.companyName}
        </span>
        {!compact ? <Icon name="grip-vertical" size={14} className={styles.grab} /> : null}
      </div>

      <div className={styles.cardMeta}>
        {card.province ? <span className={styles.prov}>{card.province}</span> : null}
        {prov ? (
          <span title={`${prov.sessionTitle} · ${dateLabel(prov.ratedAt)}`}>
            <Stars rating={prov.rating} />
          </span>
        ) : null}
        {/* Tag aziendali: chip neutre, distinte dai colori di stato/esito; le
            stesse associazioni della scheda, anche su card compatte e in
            PipelinePage (le associazioni appartengono all'azienda). */}
        <MATagChips tags={card.tags ?? []} size="sm" />
        {card.origin === 'direct' ? <span className={`${styles.chip} ${styles.chipDiretta}`}>Diretta</span> : null}
        {facts.map((kind) => {
          const info = registryLabel(kind);
          return (
            <span
              key={kind}
              className={`${styles.chip} ${info.kind === 'warn' ? styles.chipRegWarn : styles.chipRegInfo}`}
              title={info.label}
            >
              <Icon name={info.kind === 'warn' ? 'triangle-alert' : 'info'} size={10} />
              {info.label}
            </span>
          );
        })}
        {collisions.map((collision) => (
          <span
            key={collision.initiativeId}
            className={`${styles.chip} ${styles.chipColl}`}
            title={`Anche in lavorazione: ${collision.initiativeTitle}`}
          >
            <Icon name="git-branch" size={10} />
            <span className={styles.also}>Anche in:</span> {collision.initiativeTitle}
          </span>
        ))}
        {initiativeChip ? (
          <span className={`${styles.chip} ${styles.chipRegInfo}`} title={initiativeChip}>
            {initiativeChip}
          </span>
        ) : null}
        {card.state === 'ricontattare' && card.recontactOn ? (
          <span className={styles.recdate}>
            <Icon name="calendar" size={10} />
            {new Intl.DateTimeFormat('it-IT').format(new Date(card.recontactOn))}
          </span>
        ) : null}
        {terminal && card.esito ? (
          <span className={`${styles.chip} ${card.state === 'won' ? styles.chipWon : styles.chipKo}`}>
            {esitoLabel(card.esito)}
          </span>
        ) : null}
      </div>

      {!compact ? (
        <div className={styles.cardFoot}>
          <button
            type="button"
            className={[styles.dossier, ds === 'ready' ? styles.dossierReady : '', ds === 'working' ? styles.dossierRun : '']
              .filter(Boolean)
              .join(' ')}
            onClick={(e) => {
              e.stopPropagation();
              onOpenDossier?.(card);
            }}
          >
            <Icon name={ds === 'working' ? 'loader' : 'file-text'} size={11} />
            {ds === 'ready' ? 'Dossier pronto' : ds === 'working' ? 'In analisi…' : 'Dossier'}
          </button>
          {card.lastEvent ? (
            <span className={styles.cardEvent} style={{ marginLeft: 'auto' }}>
              <TruncatedCardEvent text={card.lastEvent} />
            </span>
          ) : null}
        </div>
      ) : null}
    </div>
  );
});

/** Card avvolta in un nodo draggable di @dnd-kit. Il visual "sollevato" è reso dal
 *  DragOverlay in BoardPage; qui la sorgente resta ghost (dragging → opacity .4).
 *  {...attributes}{...listeners} rendono la card operabile anche da tastiera (AA). */
export function DraggableCard({
  card,
  onClick,
  onOpenDossier,
  initiativeChip,
}: {
  card: MAInitiativeCardView;
  onClick?: () => void;
  onOpenDossier?: (card: MAInitiativeCardView) => void;
  initiativeChip?: string;
}) {
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({ id: card.companyKey });
  return (
    <BoardCard
      ref={setNodeRef}
      card={card}
      dragging={isDragging}
      onClick={onClick}
      onOpenDossier={onOpenDossier}
      initiativeChip={initiativeChip}
      {...attributes}
      {...listeners}
    />
  );
}
