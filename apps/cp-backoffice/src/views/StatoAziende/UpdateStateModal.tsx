import { useEffect, useState } from 'react';
import { Modal, SingleSelect, useToast } from '@mrsmith/ui';
import type { Customer } from '../../api/customers';
import type { CustomerState } from '../../api/customerStates';
import { formatErrorToast, useUpdateCustomerState } from '../../hooks/useUpdateCustomerState';
import styles from './StatoAziendePage.module.css';

interface UpdateStateModalProps {
  open: boolean;
  onClose: () => void;
  customer: Customer;
  states: CustomerState[] | undefined;
  statesLoading: boolean;
  statesError: unknown;
}

/**
 * UpdateStateModal is the only mutation surface for Stato Aziende.
 * - Single select backed by the prefetched customer-states list.
 * - Confirm label is EXACTLY `Conferma` (locked by FINAL.md §S5a).
 * - On success: close modal, refetch customers (invalidation wired in the
 *   hook).
 * - On error: toast format `{HTTP status} \u2014 {upstream message}` with
 *   `Qualcosa e' andato storto` as the fallback.
 */
export function UpdateStateModal({
  open,
  onClose,
  customer,
  states,
  statesLoading,
  statesError,
}: UpdateStateModalProps) {
  const { toast } = useToast();
  const updateState = useUpdateCustomerState();
  const [selectedStateId, setSelectedStateId] = useState<number | null>(null);

  // Reset the select every time the modal opens or the target customer
  // changes, so the operator never sees a stale choice.
  useEffect(() => {
    if (open) setSelectedStateId(null);
  }, [open, customer.id]);

  const stateOptions = (states ?? []).map((s) => ({ value: s.id, label: s.name }));
  const hasStates = stateOptions.length > 0;
  const canConfirm = selectedStateId != null && !updateState.isPending;

  function handleConfirm() {
    if (selectedStateId == null) return;
    updateState.mutate(
      { customerId: customer.id, stateId: selectedStateId },
      {
        onSuccess: () => {
          onClose();
        },
        onError: (error) => {
          toast(formatErrorToast(error), 'error');
        },
      },
    );
  }

  return (
    <Modal open={open} onClose={onClose} title="Aggiorna stato azienda">
      <div className={styles.modalBody}>
        <div>
          <span className={styles.fieldLabel}>Azienda</span>
          <div className={styles.customerBadge}>{customer.name}</div>
        </div>
        <div>
          <span className={styles.fieldLabel}>Nuovo stato</span>
          <div className={styles.selectWrapper}>
            <SingleSelect
              options={stateOptions}
              selected={selectedStateId}
              onChange={(v) => setSelectedStateId(v as number | null)}
              placeholder={
                statesLoading
                  ? 'Caricamento...'
                  : !hasStates
                    ? 'Nessuno stato disponibile'
                    : 'Seleziona uno stato'
              }
            />
          </div>
          {statesError != null && (
            <p className={styles.modalHelp}>Impossibile caricare gli stati disponibili.</p>
          )}
        </div>
      </div>

      <div className={styles.modalActions}>
        <button type="button" className={styles.btnSecondary} onClick={onClose}>
          Annulla
        </button>
        <button
          type="button"
          className={styles.primaryBtn}
          onClick={handleConfirm}
          disabled={!canConfirm}
        >
          {updateState.isPending ? 'Invio...' : 'Conferma'}
        </button>
      </div>
    </Modal>
  );
}
