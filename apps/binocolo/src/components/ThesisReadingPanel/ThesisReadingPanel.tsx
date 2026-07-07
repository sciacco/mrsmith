import { Button, Icon, Skeleton } from '@mrsmith/ui';
import type { MAThesisReadingRecord } from '../../api/types';
import styles from './ThesisReadingPanel.module.css';

const FIT_LEVEL_LABEL: Record<string, string> = {
  alto: 'Fit alto',
  medio: 'Fit medio',
  basso: 'Fit basso',
  non_valutabile: 'Fit non valutabile',
};

type ThesisReadingPanelProps = {
  record?: MAThesisReadingRecord | null;
  loading?: boolean;
  notGenerated?: boolean;
  queryError?: string | null;
  generationError?: string | null;
  generating?: boolean;
  deepReady: boolean;
  onGenerate?: () => void;
  generateLabel?: string;
  regenerateLabel?: string;
  showBlockedAction?: boolean;
  regenerateRequiresDeepReady?: boolean;
  compact?: boolean;
  title?: string;
  emptyTitle?: string;
  readyText?: string;
  blockedText?: string;
};

function formatDate(value?: string): string | null {
  if (!value) return null;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return null;
  return date.toLocaleDateString('it-IT');
}

function ReadingList({ title, items }: { title: string; items?: string[] }) {
  if (!items || items.length === 0) return null;
  return (
    <div className={styles.group}>
      <h4>{title}</h4>
      <ul>
        {items.map((item) => (
          <li key={item}>{item}</li>
        ))}
      </ul>
    </div>
  );
}

function ErrorMessage({ children }: { children: string }) {
  return (
    <div className={styles.error} role="alert">
      <Icon name="triangle-alert" size={16} />
      <span>{children}</span>
    </div>
  );
}

export function ThesisReadingPanel({
  record,
  loading = false,
  notGenerated = false,
  queryError,
  generationError,
  generating = false,
  deepReady,
  onGenerate,
  generateLabel = 'Genera lettura di tesi',
  regenerateLabel = 'Rigenera con la tesi corrente',
  showBlockedAction = true,
  regenerateRequiresDeepReady = true,
  compact = false,
  title = 'Lettura di tesi',
  emptyTitle = 'Lettura di tesi non ancora generata',
  readyText = 'Applica la tesi della ricerca al dossier: fit, flag ri-pesate, domande DD di tesi e ipotesi di sinergia.',
  blockedText = "Serve prima l'analisi approfondita: la lettura di tesi si appoggia ai fatti del dossier.",
}: ThesisReadingPanelProps) {
  const className = compact ? `${styles.panel} ${styles.compact}` : styles.panel;

  if (loading) {
    return (
      <section className={className} aria-label={title}>
        <Skeleton rows={compact ? 3 : 5} />
      </section>
    );
  }

  if (queryError) {
    return (
      <section className={className} aria-label={title}>
        <ErrorMessage>{queryError}</ErrorMessage>
      </section>
    );
  }

  if (notGenerated || !record) {
    return (
      <section className={className} aria-labelledby="thesis-reading-empty-title">
        <div className={styles.emptyState}>
          <span className={styles.emptyIcon} aria-hidden="true">
            <Icon name="target" size={compact ? 20 : 28} />
          </span>
          <div>
            <h3 id="thesis-reading-empty-title">{emptyTitle}</h3>
            <p>{deepReady ? readyText : blockedText}</p>
          </div>
        </div>
        {(deepReady || showBlockedAction) && onGenerate ? (
          <Button size={compact ? 'sm' : 'md'} onClick={onGenerate} loading={generating} disabled={!deepReady || generating}>
            {generateLabel}
          </Button>
        ) : null}
        {generationError ? <ErrorMessage>{generationError}</ErrorMessage> : null}
      </section>
    );
  }

  const reading = record.reading;
  const webEvidenceDate = formatDate(record.webEvidenceDate);
  const updatedAt = formatDate(record.updatedAt);

  return (
    <section className={className} aria-labelledby="thesis-reading-title">
      <div className={styles.header}>
        <div>
          <p className={styles.eyebrow}>{title}</p>
          <h3 id="thesis-reading-title">
            {reading?.fitLevel ? FIT_LEVEL_LABEL[reading.fitLevel] ?? reading.fitLevel : 'Lettura di tesi'}
          </h3>
        </div>
        {record.staleThesis ? <span className={styles.stalePill}>Tesi aggiornata dopo la generazione</span> : null}
      </div>

      {reading?.fit ? <p className={styles.fit}>{reading.fit}</p> : null}

      <ReadingList title="Bloccanti per questa tesi" items={reading?.blockingFlags} />
      <ReadingList title="Tollerabili per questa tesi" items={reading?.tolerableFlags} />
      <ReadingList title="Domande DD di tesi" items={reading?.thesisDdQuestions} />
      <ReadingList title="Ipotesi di sinergia da validare" items={reading?.synergyHypotheses} />

      {reading?.valuationStance ? (
        <div className={styles.group}>
          <h4>Postura sulla valutazione</h4>
          <p>{reading.valuationStance}</p>
        </div>
      ) : null}

      <ReadingList title="La tesi non si esprime su" items={reading?.notAddressed} />

      {!reading ? <p className={styles.muted}>Lettura registrata senza contenuto strutturato.</p> : null}

      <div className={styles.meta}>
        {record.thesisSnapshot ? <span>Tesi: “{record.thesisSnapshot}”</span> : null}
        {webEvidenceDate ? <span>Evidenza web: {webEvidenceDate}</span> : null}
        {updatedAt ? <span>Generata: {updatedAt}</span> : null}
        {record.generatedByEmail ? <span>Da: {record.generatedByEmail}</span> : null}
      </div>

      {(!regenerateRequiresDeepReady || deepReady || showBlockedAction) && onGenerate ? (
        <div className={styles.actions}>
          <Button
            variant="secondary"
            size={compact ? 'sm' : 'md'}
            onClick={onGenerate}
            loading={generating}
            disabled={(regenerateRequiresDeepReady && !deepReady) || generating}
          >
            {regenerateLabel}
          </Button>
        </div>
      ) : null}
      {generationError ? <ErrorMessage>{generationError}</ErrorMessage> : null}
    </section>
  );
}
