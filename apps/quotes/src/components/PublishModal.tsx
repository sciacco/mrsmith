import { useCallback, useState } from 'react';
import { usePublishQuote } from '../api/queries';
import styles from './PublishModal.module.css';

const stepNames = ['Salvataggio dati', 'Validazione prodotti', 'Offerta HubSpot', 'Sincronizzazione prodotti', 'Aggiornamento stato'];

interface PublishModalProps {
  quoteId: number;
  isRepublish: boolean;
  onClose: () => void;
}

type ModalState = 'confirm' | 'progress' | 'success' | 'error';

export function PublishModal({ quoteId, isRepublish, onClose }: PublishModalProps) {
  const [modalState, setModalState] = useState<ModalState>('confirm');
  const [completedSteps, setCompletedSteps] = useState<number[]>([]);
  const [errorStep, setErrorStep] = useState<{ step: number; message: string } | null>(null);
  const publishQuote = usePublishQuote();

  const handlePublish = useCallback(async () => {
    setModalState('progress');
    setCompletedSteps([]);
    setErrorStep(null);

    try {
      const result = await publishQuote.mutateAsync(quoteId);
      if (result.success) {
        setCompletedSteps(result.steps.map(s => s.step));
        setModalState('success');
      } else {
        const failed = result.steps.find(s => s.status === 'error');
        const completed = result.steps.filter(s => s.status === 'completed').map(s => s.step);
        setCompletedSteps(completed);
        if (failed) {
          setErrorStep({ step: failed.step, message: failed.error ?? 'Unknown error' });
        }
        setModalState('error');
      }
    } catch (e) {
      setModalState('error');
      setErrorStep({ step: 0, message: e instanceof Error ? e.message : 'Errore di rete' });
    }
  }, [quoteId, publishQuote]);

  if (modalState === 'confirm') {
    return (
      <div className={styles.overlay} onClick={onClose}>
        <div className={styles.modal} onClick={e => e.stopPropagation()}>
          <div className={styles.title}>
            {isRepublish ? 'Ripubblica su HubSpot' : 'Pubblica su HubSpot'}
          </div>
          <p style={{ fontSize: '0.875rem', color: '#64748b', marginBottom: '1.5rem' }}>
            La proposta verrà sincronizzata con HubSpot. Questo processo include validazione, creazione offerta e sincronizzazione prodotti.
          </p>
          <div className={styles.actions}>
            <button className={styles.btnSecondary} onClick={onClose}>Annulla</button>
            <button className={styles.btnPrimary} onClick={() => void handlePublish()}>Pubblica</button>
          </div>
        </div>
      </div>
    );
  }

  if (modalState === 'success') {
    return (
      <div className={styles.overlay}>
        <div className={styles.modal}>
          <div className={styles.successIcon}>{'\u2713'}</div>
          <div className={styles.successText}>Pubblicazione completata</div>
          <div className={styles.actions}>
            <button className={styles.btnSecondary} onClick={onClose}>Chiudi</button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className={styles.overlay}>
      <div className={styles.modal}>
        <div className={styles.title}>
          {modalState === 'error' ? 'Errore durante la pubblicazione' : 'Pubblicazione in corso...'}
        </div>
        <div className={styles.stepList}>
          {stepNames.map((name, i) => {
            const stepNum = i + 1;
            const isCompleted = completedSteps.includes(stepNum);
            const isError = errorStep?.step === stepNum;
            const isCurrent = !isCompleted && !isError && stepNum === Math.max(...completedSteps, 0) + 1 && modalState === 'progress';

            let iconClass = styles.pending ?? '';
            let icon = String(stepNum);
            if (isCompleted) { iconClass = styles.completed ?? ''; icon = '\u2713'; }
            else if (isError) { iconClass = styles.error ?? ''; icon = '\u2717'; }
            else if (isCurrent) { iconClass = styles.inProgress ?? ''; icon = '\u25CF'; }

            return (
              <div key={i}>
                <div className={styles.step}>
                  <span className={`${styles.stepIcon} ${iconClass}`}>{icon}</span>
                  <span className={styles.stepLabel}>{name}</span>
                </div>
                {isError && errorStep && (
                  <div className={styles.stepError} style={{ marginLeft: '2.5rem' }}>
                    {errorStep.message}
                  </div>
                )}
              </div>
            );
          })}
        </div>
        {modalState === 'error' && (
          <div className={styles.actions}>
            <button className={styles.btnSecondary} onClick={onClose}>Chiudi</button>
            <button className={styles.btnPrimary} onClick={() => void handlePublish()}>Riprova</button>
          </div>
        )}
      </div>
    </div>
  );
}
