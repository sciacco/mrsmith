import { Button, Icon, SingleSelect, Tooltip } from '@mrsmith/ui';
import { useLayoutEffect, useRef } from 'react';
import { Link } from 'react-router-dom';
import { RatingStars } from '../RatingStars';
import styles from './LensBar.module.css';

export interface LensOption {
  value: string;
  label: string;
}

export interface LensKeyNumber {
  key: string;
  label: string;
  value: string;
  // Id dell'heading di destinazione (senza `#`): il tile è un button che
  // chiama l'helper di scroll con offset a runtime, non un salto nativo.
  targetId: string;
}

export interface LensStaffetta {
  // Posizione 1-based e totale, per la copy «azienda N di M».
  position: number;
  total: number;
  // undefined = estremo della lista → freccia disabilitata (nessun wrap).
  onPrev?: () => void;
  onNext?: () => void;
}

interface LensBarProps {
  companyName: string;
  originLabel: string;
  originTo?: string;
  lensOptions: LensOption[];
  selectedLens: string;
  onLensChange: (value: string) => void;
  notInLensNote?: string;
  verdict?: { label: string; bucket: string };
  rating?: { value: number; onRate: (rating: number) => void };
  keyNumbers?: LensKeyNumber[];
  onKeyNumberClick?: (targetId: string) => void;
  staffetta?: LensStaffetta;
}

export function LensBar({
  companyName,
  originLabel,
  originTo,
  lensOptions,
  selectedLens,
  onLensChange,
  notInLensNote,
  verdict,
  rating,
  keyNumbers,
  onKeyNumberClick,
  staffetta,
}: LensBarProps) {
  const barRef = useRef<HTMLDivElement>(null);

  // La barra cambia altezza (striscia numeri, wrapping su viewport stretti):
  // pubblica il suo bordo inferiore "da incollata" come CSS variable, così
  // spina e scroll-margin dei blocchi si tengono sempre sotto di lei.
  useLayoutEffect(() => {
    const el = barRef.current;
    if (!el) return;
    const update = () => {
      const top = Number.parseFloat(getComputedStyle(el).top) || 0;
      document.documentElement.style.setProperty(
        '--scheda-lensbar-bottom',
        `${Math.round(top + el.getBoundingClientRect().height)}px`,
      );
    };
    update();
    const observer = new ResizeObserver(update);
    observer.observe(el);
    return () => {
      observer.disconnect();
      document.documentElement.style.removeProperty('--scheda-lensbar-bottom');
    };
  }, []);

  return (
    <div ref={barRef} className={styles.bar} data-scheda-sticky="bar">
      <div className={styles.primary}>
        <nav className={styles.breadcrumb} aria-label="Contesto scheda">
          {originTo ? (
            <Link to={originTo} className={styles.origin}>
              {originLabel}
            </Link>
          ) : (
            <span className={styles.origin}>{originLabel}</span>
          )}
          <span className={styles.separator} aria-hidden="true">
            ›
          </span>
          <span className={styles.company}>{companyName}</span>
        </nav>

        <div className={styles.lensPicker}>
          <SingleSelect
            options={lensOptions}
            selected={selectedLens}
            onChange={(value) => value && onLensChange(String(value))}
            searchable={lensOptions.length > 6}
          />
        </div>

        {notInLensNote ? <span className={styles.note}>{notInLensNote}</span> : null}

        {verdict ? (
          <span className={styles.verdict}>
            <span className={styles.verdictLabel}>{verdict.label}</span>
            <span className={styles.verdictBucket}>{verdict.bucket}</span>
          </span>
        ) : null}

        {rating ? (
          <span className={styles.rating}>
            <RatingStars rating={rating.value} onRate={rating.onRate} />
          </span>
        ) : null}

        {staffetta ? (
          <div className={styles.staffetta}>
            <Tooltip content="Precedente · K">
              <Button
                variant="ghost"
                size="sm"
                aria-label="Azienda precedente"
                disabled={!staffetta.onPrev}
                onClick={staffetta.onPrev}
                leftIcon={<Icon name="chevron-left" size={16} />}
              />
            </Tooltip>
            <span className={styles.staffettaLabel}>
              azienda {staffetta.position} di {staffetta.total}
            </span>
            <Tooltip content="Successiva · J">
              <Button
                variant="ghost"
                size="sm"
                aria-label="Azienda successiva"
                disabled={!staffetta.onNext}
                onClick={staffetta.onNext}
                leftIcon={<Icon name="chevron-right" size={16} />}
              />
            </Tooltip>
          </div>
        ) : null}
      </div>

      {keyNumbers && keyNumbers.length > 0 ? (
        <div className={styles.keyStrip}>
          {keyNumbers.map((tile) => (
            <button
              key={tile.key}
              type="button"
              className={styles.keyTile}
              onClick={onKeyNumberClick ? () => onKeyNumberClick(tile.targetId) : undefined}
            >
              <span className={styles.keyLabel}>{tile.label}</span>
              <span className={styles.keyValue}>{tile.value}</span>
            </button>
          ))}
        </div>
      ) : null}
    </div>
  );
}
