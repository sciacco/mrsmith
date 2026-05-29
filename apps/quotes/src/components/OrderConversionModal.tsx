import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { Button, Icon, Modal } from '@mrsmith/ui';
import { useConvertQuoteToOrder, useQuote, useQuoteRows } from '../api/queries';
import type { OrderConversionResult, OrderConversionStatus, OrderConversionStep } from '../api/types';
import styles from './PublishModal.module.css';

const stepLabels: Record<string, string> = {
  order: 'Ordine',
  bridge: 'Collegamento proposta',
  pdf: 'PDF ordine',
  hubspot_file: 'Archivio HubSpot',
  hubspot_note: 'Nota sul deal',
};

const expectedSteps = ['order', 'bridge', 'pdf', 'hubspot_file', 'hubspot_note'];

type ModalState = 'confirm' | 'progress' | 'success' | 'error';

interface OrderConversionModalProps {
  open: boolean;
  quoteId: number;
  status: OrderConversionStatus | null;
  onClose: () => void;
}

function formatCurrency(value: number): string {
  return value.toLocaleString('it-IT', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
}

function stepLabel(step: OrderConversionStep | string): string {
  const name = typeof step === 'string' ? step : step.name;
  return stepLabels[name] ?? name;
}

function apiErrorMessage(error: unknown): string {
  if (error instanceof ApiError && error.body && typeof error.body === 'object' && 'error' in error.body) {
    const code = String(error.body.error);
    switch (code) {
      case 'order_already_exists_without_bridge':
        return 'Esiste già un ordine con lo stesso codice. Verifica il deal prima di procedere.';
      case 'quote_status_not_approved':
        return 'Non è possibile effettuare la conversione di una proposta in stato diverso da APPROVED.';
      case 'deal_number_invalid':
        return 'Il codice deal non è nel formato numero/anno.';
      case 'hubspot_deal_not_found':
        return 'Non trovo il deal HubSpot collegato alla proposta.';
      case 'quote_has_no_included_products':
        return 'La proposta non contiene prodotti inclusi.';
      case 'vodka_database_not_configured':
        return 'Il database ordini non è disponibile.';
      case 'arak_gateway_not_configured':
        return 'Il servizio PDF ordini non è disponibile.';
      case 'hubspot_not_configured':
        return 'HubSpot non è disponibile.';
      default:
        return code;
    }
  }
  return error instanceof Error ? error.message : 'Errore di rete';
}

export function OrderConversionModal({ open, quoteId, status, onClose }: OrderConversionModalProps) {
  const [modalState, setModalState] = useState<ModalState>('confirm');
  const [result, setResult] = useState<OrderConversionResult | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const convertQuote = useConvertQuoteToOrder();
  const { data: quote } = useQuote(quoteId);
  const { data: rows } = useQuoteRows(quoteId);

  const totals = useMemo(() => {
    if (!rows) return { nrc: 0, mrc: 0 };
    return rows.reduce(
      (acc, row) => ({ nrc: acc.nrc + row.nrc_row, mrc: acc.mrc + row.mrc_row }),
      { nrc: 0, mrc: 0 },
    );
  }, [rows]);

  useEffect(() => {
    if (!open) {
      setModalState('confirm');
      setResult(null);
      setErrorMessage(null);
    }
  }, [open]);

  const handleConvert = useCallback(async () => {
    setModalState('progress');
    setResult(null);
    setErrorMessage(null);

    try {
      const response = await convertQuote.mutateAsync(quoteId);
      setResult(response);
      if (response.success) {
        setModalState('success');
      } else {
        const failed = response.steps.find(step => step.status === 'error');
        setErrorMessage(failed?.error ?? 'Conversione non completata.');
        setModalState('error');
      }
    } catch (error) {
      setErrorMessage(apiErrorMessage(error));
      setModalState('error');
    }
  }, [convertQuote, quoteId]);

  const handleClose = useCallback(() => {
    setModalState('confirm');
    setResult(null);
    setErrorMessage(null);
    onClose();
  }, [onClose]);

  const steps = result?.steps ?? [];
  const visibleStepNames = steps.length > 0 ? steps.map(step => step.name) : expectedSteps;
  const title =
    modalState === 'confirm'
      ? status?.converted
        ? 'Completa invio ordine'
        : 'Converti in ordine'
      : modalState === 'progress'
        ? 'Conversione in corso'
        : modalState === 'success'
          ? 'Ordine convertito'
          : 'Conversione non completata';

  const orderNumber = result?.order_number ?? status?.order_number ?? null;
  const orderCode = result?.order_code ?? status?.order_code ?? null;
  const orderId = result?.order_id ?? status?.order_id ?? null;
  const displayedOrder = orderNumber ?? (orderCode ? orderCode.split('/')[0] : (orderId ?? '—'));
  const hubspotURL = result?.hubspot_deal_url ?? status?.hubspot_deal_url ?? null;

  return (
    <Modal open={open} onClose={handleClose} title={title} size="md" dismissible={modalState !== 'progress'}>
      {modalState === 'confirm' && (
        <>
          <p className={styles.introText}>
            Verrà creato l&apos;ordine e il PDF sarà allegato al deal HubSpot.
          </p>

          <div className={styles.summaryCard}>
            <div className={styles.summaryRow}>
              <span className={styles.summaryLabel}>Proposta</span>
              <span className={styles.summaryValueMono}>{quote?.quote_number ?? '—'}</span>
            </div>
            <div className={styles.summaryRow}>
              <span className={styles.summaryLabel}>Cliente</span>
              <span className={styles.summaryValue}>{quote?.customer_name ?? '—'}</span>
            </div>
            {quote?.deal_number && (
              <div className={styles.summaryRow}>
                <span className={styles.summaryLabel}>Deal</span>
                <span className={styles.summaryValueMono}>{quote.deal_number}</span>
              </div>
            )}
            {status?.converted && (
              <div className={styles.summaryRow}>
                <span className={styles.summaryLabel}>Ordine</span>
                <span className={styles.summaryValueMono}>
                  {status.order_id ? (
                    <a
                      href={`/ordini/${status.order_id}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className={styles.orderLink}
                    >
                      {displayedOrder}
                    </a>
                  ) : (
                    displayedOrder
                  )}
                </span>
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

          {status?.converted && (
            <div className={styles.infoBanner}>
              <Icon name="info" size={16} />
              <span>L&apos;ordine è già stato creato. Verranno completati PDF e nota HubSpot.</span>
            </div>
          )}

          <div className={styles.actions}>
            <Button variant="ghost" onClick={handleClose}>Annulla</Button>
            <Button
              variant="primary"
              loading={convertQuote.isPending}
              leftIcon={<Icon name="shopping-cart" size={16} />}
              onClick={() => void handleConvert()}
            >
              {status?.converted ? 'Completa invio' : 'Converti'}
            </Button>
          </div>
        </>
      )}

      {modalState === 'progress' && (
        <div className={styles.stepList}>
          {expectedSteps.map((name, index) => (
            <div className={styles.step} key={name}>
              <span className={`${styles.stepIcon} ${index === 0 ? styles.inProgress : styles.pending}`}>
                {index === 0 ? <Icon name="loader" size={14} strokeWidth={2.5} /> : index + 1}
              </span>
              <span className={styles.stepLabel}>{stepLabel(name)}</span>
            </div>
          ))}
        </div>
      )}

      {modalState === 'success' && (
        <div className={styles.successWrap}>
          <Icon name="check-circle" size={64} strokeWidth={1.5} />
          <div className={styles.successTitle}>Ordine pronto</div>
          <div className={styles.successMeta}>
            <span>Ordine</span>
            <span className={styles.successQuoteNumber}>
              {orderId ? (
                <a
                  href={`/ordini/${orderId}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className={styles.orderLink}
                >
                  {displayedOrder}
                </a>
              ) : (
                displayedOrder
              )}
            </span>
          </div>
          <div className={styles.stepList}>
            {visibleStepNames.map((name, index) => {
              const step = steps.find(s => s.name === name);
              const completed = step?.status === 'completed' || step?.status === 'skipped';
              return (
                <div className={styles.step} key={`${name}-${index}`}>
                  <span className={`${styles.stepIcon} ${completed ? styles.completed : styles.pending}`}>
                    {completed ? <Icon name="check" size={14} strokeWidth={2.5} /> : index + 1}
                  </span>
                  <span className={styles.stepLabel}>{stepLabel(step ?? name)}</span>
                </div>
              );
            })}
          </div>
          <div className={styles.actions}>
            {hubspotURL ? (
              <Button
                variant="primary"
                leftIcon={<Icon name="external-link" size={16} />}
                onClick={() => {
                  window.open(hubspotURL, '_blank', 'noopener,noreferrer');
                  handleClose();
                }}
              >
                Apri su HubSpot
              </Button>
            ) : (
              <Button variant="primary" onClick={handleClose}>Chiudi</Button>
            )}
            {hubspotURL && <Button variant="ghost" onClick={handleClose}>Chiudi</Button>}
          </div>
        </div>
      )}

      {modalState === 'error' && (
        <>
          <div className={styles.blockerBox}>{errorMessage ?? 'Conversione non completata.'}</div>
          {steps.length > 0 && (
            <div className={styles.stepList}>
              {steps.map((step, index) => {
                const iconClass = step.status === 'error' ? styles.error : styles.completed;
                const icon: ReactNode = step.status === 'error'
                  ? <Icon name="x" size={14} strokeWidth={2.5} />
                  : <Icon name="check" size={14} strokeWidth={2.5} />;
                return (
                  <div key={`${step.name}-${index}`}>
                    <div className={styles.step}>
                      <span className={`${styles.stepIcon} ${iconClass}`}>{icon}</span>
                      <span className={styles.stepLabel}>{stepLabel(step)}</span>
                    </div>
                    {step.status === 'error' && step.error && (
                      <div className={styles.stepErrorInline}>{step.error}</div>
                    )}
                  </div>
                );
              })}
            </div>
          )}
          <div className={styles.actions}>
            <Button variant="ghost" onClick={handleClose}>Chiudi</Button>
            <Button variant="primary" onClick={() => void handleConvert()}>Riprova</Button>
          </div>
        </>
      )}
    </Modal>
  );
}
