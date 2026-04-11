import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { Modal, Button, Icon } from '@mrsmith/ui';
import { usePublishQuote, useQuote, useQuoteRows } from '../api/queries';
import type { HSStatus, PublishPrecheck } from '../api/types';
import { StatusBadge } from './StatusBadge';
import { SuccessCheckmark } from './SuccessCheckmark';
import styles from './PublishModal.module.css';

const stepNames = [
  'Salvataggio dati',
  'Validazione prodotti',
  'Offerta HubSpot',
  'Sincronizzazione prodotti',
  'Aggiornamento stato',
];

const SIMULATED_STEP_INTERVAL_MS = 400;

interface PublishModalProps {
  open: boolean;
  quoteId: number;
  isRepublish: boolean;
  hsStatus: HSStatus | null;
  precheck: PublishPrecheck | null;
  onClose: () => void;
}

type ModalState = 'confirm' | 'progress' | 'success' | 'error';

function formatCurrency(value: number): string {
  return value.toLocaleString('it-IT', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
}

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
  const [simulatedCurrentStep, setSimulatedCurrentStep] = useState(1);
  const [errorStep, setErrorStep] = useState<{ step: number; message: string } | null>(null);
  const [showSuccess, setShowSuccess] = useState(false);
  const simulationTimerRef = useRef<number | null>(null);
  const successTimerRef = useRef<number | null>(null);

  const publishQuote = usePublishQuote();
  const { data: quote } = useQuote(quoteId);
  const { data: rows } = useQuoteRows(quoteId);

  const totals = useMemo(() => {
    if (!rows) return { nrc: 0, mrc: 0 };
    return rows.reduce(
      (acc, r) => ({ nrc: acc.nrc + r.nrc_row, mrc: acc.mrc + r.mrc_row }),
      { nrc: 0, mrc: 0 },
    );
  }, [rows]);

  const hasLegalNotes = (quote?.notes ?? '').trim().length > 0;
  const previewStatus = hasLegalNotes ? 'PENDING_APPROVAL' : 'APPROVAL_NOT_NEEDED';

  const blockers: string[] = [];
  if (hsStatus?.sign_status === 'ESIGN_COMPLETED') {
    blockers.push('La proposta è già firmata su HubSpot e non può essere ripubblicata.');
  }
  if (precheck?.has_missing_required_products) {
    blockers.push('Sono presenti gruppi prodotto obbligatori non configurati.');
  }

  const clearTimers = useCallback(() => {
    if (simulationTimerRef.current !== null) {
      window.clearInterval(simulationTimerRef.current);
      simulationTimerRef.current = null;
    }
    if (successTimerRef.current !== null) {
      window.clearTimeout(successTimerRef.current);
      successTimerRef.current = null;
    }
  }, []);

  useEffect(() => () => clearTimers(), [clearTimers]);

  // Reset internal state whenever the modal is closed from outside
  useEffect(() => {
    if (!open) {
      setModalState('confirm');
      setCompletedSteps([]);
      setSimulatedCurrentStep(1);
      setErrorStep(null);
      setShowSuccess(false);
      clearTimers();
    }
  }, [open, clearTimers]);

  const startSimulation = useCallback(() => {
    clearTimers();
    setSimulatedCurrentStep(1);
    simulationTimerRef.current = window.setInterval(() => {
      setSimulatedCurrentStep(prev => {
        // Stop at stepNames.length - 1 so the last step waits for the real response.
        if (prev >= stepNames.length - 1) return prev;
        setCompletedSteps(done => (done.includes(prev) ? done : [...done, prev]));
        return prev + 1;
      });
    }, SIMULATED_STEP_INTERVAL_MS);
  }, [clearTimers]);

  const handlePublish = useCallback(async () => {
    if (blockers.length > 0) return;
    setModalState('progress');
    setCompletedSteps([]);
    setSimulatedCurrentStep(1);
    setErrorStep(null);
    setShowSuccess(false);
    startSimulation();

    try {
      const result = await publishQuote.mutateAsync(quoteId);
      clearTimers();
      if (result.success) {
        setCompletedSteps(result.steps.map(s => s.step));
        setModalState('success');
        // Step list fades, then the big checkmark appears
        successTimerRef.current = window.setTimeout(() => {
          setShowSuccess(true);
        }, 500);
      } else {
        const failed = result.steps.find(s => s.status === 'error');
        const completed = result.steps.filter(s => s.status === 'completed').map(s => s.step);
        setCompletedSteps(completed);
        if (failed) {
          setErrorStep({ step: failed.step, message: failed.error ?? 'Errore sconosciuto' });
        }
        setModalState('error');
      }
    } catch (e) {
      clearTimers();
      setModalState('error');
      setErrorStep({ step: 0, message: e instanceof Error ? e.message : 'Errore di rete' });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [blockers.length, quoteId, publishQuote, startSimulation, clearTimers]);

  const handleClose = useCallback(() => {
    clearTimers();
    setModalState('confirm');
    setCompletedSteps([]);
    setSimulatedCurrentStep(1);
    setErrorStep(null);
    setShowSuccess(false);
    onClose();
  }, [onClose, clearTimers]);

  const title =
    modalState === 'confirm'
      ? isRepublish
        ? 'Ripubblica su HubSpot'
        : 'Pubblica su HubSpot'
      : modalState === 'progress'
        ? 'Pubblicazione in corso'
        : modalState === 'error'
          ? 'Errore durante la pubblicazione'
          : 'Pubblicazione completata';

  const dismissible = modalState !== 'progress';

  return (
    <Modal open={open} onClose={handleClose} title={title} size="md" dismissible={dismissible}>
      {modalState === 'confirm' && (
        <>
          <p className={styles.introText}>
            La proposta verrà sincronizzata con HubSpot. Verifica il riepilogo prima di procedere.
          </p>

          <div className={styles.summaryCard}>
            <div className={styles.summaryRow}>
              <span className={styles.summaryLabel}>Numero</span>
              <span className={styles.summaryValueMono}>{quote?.quote_number ?? '—'}</span>
            </div>
            <div className={styles.summaryRow}>
              <span className={styles.summaryLabel}>Cliente</span>
              <span className={styles.summaryValue}>{quote?.customer_name ?? '—'}</span>
            </div>
            {quote?.deal_name && (
              <div className={styles.summaryRow}>
                <span className={styles.summaryLabel}>Deal</span>
                <span className={styles.summaryValue}>{quote.deal_name}</span>
              </div>
            )}
            <div className={styles.totalsRow}>
              <div className={styles.totalsCell}>
                <span className={styles.totalsLabel}>NRC</span>
                <span className={styles.totalsValue}>{formatCurrency(totals.nrc)}</span>
              </div>
              <div className={styles.totalsDivider} aria-hidden="true" />
              <div className={styles.totalsCell}>
                <span className={styles.totalsLabel}>MRC</span>
                <span className={styles.totalsValue}>{formatCurrency(totals.mrc)}</span>
              </div>
            </div>
          </div>

          <div className={styles.statusPreview}>
            <span className={styles.statusPreviewLabel}>Stato dopo pubblicazione</span>
            <StatusBadge status={previewStatus} />
          </div>

          {hasLegalNotes && (
            <div className={styles.legalBanner}>
              <Icon name="triangle-alert" size={16} />
              <span>
                La proposta contiene pattuizioni speciali e richiederà l&apos;approvazione di un responsabile commerciale.
              </span>
            </div>
          )}

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
              {isRepublish ? 'Ripubblica' : 'Pubblica'}
            </Button>
          </div>
        </>
      )}

      {modalState === 'success' && (
        <div className={styles.successWrap}>
          {showSuccess && (
            <>
              <SuccessCheckmark size={64} />
              <div className={styles.successTitle}>Pubblicazione completata</div>
              <div className={styles.successMeta}>
                <span>Proposta</span>
                <span className={styles.successQuoteNumber}>{quote?.quote_number ?? '—'}</span>
                <span>·</span>
                <StatusBadge status={previewStatus} />
              </div>
              <div className={styles.actions}>
                {hsStatus?.quote_url ? (
                  <Button
                    variant="primary"
                    leftIcon={<Icon name="external-link" size={16} />}
                    onClick={() => {
                      window.open(hsStatus.quote_url!, '_blank', 'noopener,noreferrer');
                      handleClose();
                    }}
                  >
                    Apri su HubSpot
                  </Button>
                ) : (
                  <Button variant="primary" onClick={handleClose}>
                    Chiudi
                  </Button>
                )}
                {hsStatus?.quote_url && (
                  <Button variant="ghost" onClick={handleClose}>
                    Chiudi
                  </Button>
                )}
              </div>
            </>
          )}
          {!showSuccess && (
            <div className={styles.stepList}>
              {stepNames.map((name, i) => (
                <div key={i} className={styles.step}>
                  <span className={`${styles.stepIcon} ${styles.completed}`}>
                    <Icon name="check" size={14} strokeWidth={2.5} />
                  </span>
                  <span className={styles.stepLabel}>{name}</span>
                </div>
              ))}
            </div>
          )}
        </div>
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
                stepNum === simulatedCurrentStep &&
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
