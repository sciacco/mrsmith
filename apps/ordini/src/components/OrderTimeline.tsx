import { Icon } from '@mrsmith/ui';
import type { OrderState } from '../api/types';
import styles from './OrderTimeline.module.css';

interface OrderTimelineProps {
  state: OrderState | null | undefined;
  hasArxDoc: boolean;
}

export function OrderTimeline({ state, hasArxDoc }: OrderTimelineProps) {
  const normalizedState = (state ?? '').toUpperCase();
  const isDanger = normalizedState === 'ANNULLATO' || normalizedState === 'PERSO';

  let activeIndex = 0;
  if (normalizedState === 'ATTIVO') {
    activeIndex = 3;
  } else if (normalizedState === 'INVIATO') {
    activeIndex = 2;
  } else if (isDanger) {
    activeIndex = hasArxDoc ? 2 : 0;
  } else {
    // BOZZA or default
    activeIndex = hasArxDoc ? 1 : 0;
  }

  const steps = [
    {
      label: 'Creazione',
      desc: isDanger && activeIndex === 0 ? 'Ordine annullato' : 'Compilazione dell\'ordine',
    },
    {
      label: 'Firma',
      desc: hasArxDoc ? 'Documento firmato' : "In attesa dell'ordine firmato",
    },
    {
      label: 'Invio ERP',
      desc:
        normalizedState === 'ATTIVO' || normalizedState === 'INVIATO'
          ? 'Trasmesso in ERP'
          : isDanger && activeIndex === 2
          ? 'Invio annullato'
          : "Da inviare all'ERP",
    },
    {
      label: 'Attivazione',
      desc: normalizedState === 'ATTIVO' ? 'Servizi attivi' : 'In attesa di attivazione',
    },
  ];

  // Calculate progress bar width (percentage between steps 0, 1, 2, 3)
  const progressPercent = isDanger ? (activeIndex / 3) * 100 : (Math.min(activeIndex, 3) / 3) * 100;

  return (
    <div className={styles.timelineContainer} aria-label="Avanzamento stato ordine">
      {/* Background Track */}
      <div className={styles.progressTrack}>
        <div
          className={`${styles.progressBar} ${isDanger ? styles.progressBarDanger : ''}`}
          style={{ width: `${progressPercent}%` }}
        />
      </div>

      {/* Steps */}
      {steps.map((step, index) => {
        let stepClass = styles.stepPending;
        let isStepCompleted = false;
        let isStepDanger = false;

        if (isDanger && index === activeIndex) {
          stepClass = styles.stepDanger;
          isStepDanger = true;
        } else if (index < activeIndex || (normalizedState === 'ATTIVO' && index === 3)) {
          stepClass = styles.stepCompleted;
          isStepCompleted = true;
        } else if (index === activeIndex) {
          stepClass = styles.stepActive;
        }

        return (
          <div key={index} className={`${styles.stepNode} ${stepClass}`}>
            <div className={styles.indicator}>
              {isStepCompleted ? (
                <Icon name="check" size={16} strokeWidth={3} />
              ) : isStepDanger ? (
                <Icon name="x" size={16} strokeWidth={3} />
              ) : (
                index + 1
              )}
            </div>
            <div className={styles.textGroup}>
              <span className={styles.label}>{step.label}</span>
              <span className={styles.description}>{step.desc}</span>
            </div>
          </div>
        );
      })}
    </div>
  );
}
