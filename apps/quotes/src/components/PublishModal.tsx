import { useCallback, useState, type ReactNode } from 'react';
import { Modal, Button, Icon } from '@mrsmith/ui';
import { usePublishQuote } from '../api/queries';
import type { HSStatus, PublishPrecheck } from '../api/types';
import styles from './PublishModal.module.css';

const stepNames = [
  'Salvataggio dati',
  'Validazione prodotti',
  'Offerta HubSpot',
  'Sincronizzazione prodotti',
  'Aggiornamento stato',
];

interface PublishModalProps {
  open: boolean;
  quoteId: number;
  isRepublish: boolean;
  hsStatus: HSStatus | null;
  precheck: PublishPrecheck | null;
  onClose: () => void;
}

type ModalState = 'confirm' | 'progress' | 'success' | 'error';

export function PublishModal({
  open,
  quoteId,
  isRepublish,
  hsStatus,
  precheck,
  onClose,
}: PublishModalProps) {
  const [modalState, setModalState] = useState<ModalState>('confirm');
  const [completedSteps, setCompletedSteps] = useState<number[]>([]);
  const [errorStep, setErrorStep] = useState<{ step: number; message: string } | null>(null);
  const publishQuote = usePublishQuote();
  const blockers: string[] = [];

  if (hsStatus?.sign_status === 'ESIGN_COMPLETED') {
    blockers.push('La proposta risulta gia firmata su HubSpot e non puo essere ripubblicata.');
  }
  if (precheck?.has_missing_required_products) {
    blockers.push('Sono presenti gruppi prodotto obbligatori non configurati.');
  }

  const handlePublish = useCallback(async () => {
    if (blockers.length > 0) return;
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
  }, [blockers.length, quoteId, publishQuote]);

  const handleClose = useCallback(() => {
    setModalState('confirm');
    setCompletedSteps([]);
    setErrorStep(null);
    onClose();
  }, [onClose]);

  const title =
    modalState === 'confirm'
      ? isRepublish
        ? 'Ripubblica su HubSpot'
        : 'Pubblica su HubSpot'
      : modalState === 'progress'
        ? 'Pubblicazione in corso...'
        : modalState === 'error'
          ? 'Errore durante la pubblicazione'
          : 'Pubblicazione completata';

  const dismissible = modalState !== 'progress';

  return (
    <Modal open={open} onClose={handleClose} title={title} size="md" dismissible={dismissible}>
      {modalState === 'confirm' && (
        <>
          <p className={styles.introText}>
            La proposta verrà sincronizzata con HubSpot. Questo processo include validazione, creazione offerta e sincronizzazione prodotti.
          </p>
          {blockers.length > 0 && (
            <div className={styles.blockerBox}>{blockers.join(' ')}</div>
          )}
          <div className={styles.actions}>
            <Button variant="ghost" onClick={handleClose}>Annulla</Button>
            <Button
              variant="primary"
              disabled={blockers.length > 0}
              onClick={() => void handlePublish()}
            >
              Pubblica
            </Button>
          </div>
        </>
      )}

      {modalState === 'success' && (
        <>
          <div className={styles.successIcon}>
            <Icon name="check" size={32} strokeWidth={2.5} />
          </div>
          <div className={styles.successText}>Pubblicazione completata</div>
          <div className={styles.actions}>
            <Button variant="primary" onClick={handleClose}>Chiudi</Button>
          </div>
        </>
      )}

      {(modalState === 'progress' || modalState === 'error') && (
        <>
          <div className={styles.stepList}>
            {stepNames.map((name, i) => {
              const stepNum = i + 1;
              const isCompleted = completedSteps.includes(stepNum);
              const isError = errorStep?.step === stepNum;
              const isCurrent =
                !isCompleted &&
                !isError &&
                stepNum === Math.max(...completedSteps, 0) + 1 &&
                modalState === 'progress';

              let iconClass = styles.pending ?? '';
              let icon: ReactNode = String(stepNum);
              if (isCompleted) {
                iconClass = styles.completed ?? '';
                icon = <Icon name="check" size={14} strokeWidth={2.5} />;
              } else if (isError) {
                iconClass = styles.error ?? '';
                icon = <Icon name="x" size={14} strokeWidth={2.5} />;
              } else if (isCurrent) {
                iconClass = styles.inProgress ?? '';
                icon = <Icon name="loader" size={14} strokeWidth={2.5} />;
              }

              return (
                <div key={i}>
                  <div className={styles.step}>
                    <span className={`${styles.stepIcon} ${iconClass}`}>{icon}</span>
                    <span className={styles.stepLabel}>{name}</span>
                  </div>
                  {isError && errorStep && (
                    <div className={styles.stepErrorInline}>{errorStep.message}</div>
                  )}
                </div>
              );
            })}
          </div>
          {modalState === 'error' && (
            <div className={styles.actions}>
              <Button variant="ghost" onClick={handleClose}>Chiudi</Button>
              <Button variant="primary" onClick={() => void handlePublish()}>Riprova</Button>
            </div>
          )}
        </>
      )}
    </Modal>
  );
}
