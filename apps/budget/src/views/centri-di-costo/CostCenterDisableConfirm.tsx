import { Modal, useToast } from '@mrsmith/ui';
import { useCostCenterDetails, useDisableCostCenter } from './queries';
import { ApiError } from '@mrsmith/api-client';
import styles from './CentriDiCostoPage.module.css';

interface CostCenterDisableConfirmProps {
  open: boolean;
  onClose: () => void;
  costCenterName: string;
}

export function CostCenterDisableConfirm({
  open,
  onClose,
  costCenterName,
}: CostCenterDisableConfirmProps) {
  const { toast } = useToast();
  const { data: details, isFetching } = useCostCenterDetails(costCenterName);
  const disableCC = useDisableCostCenter();

  function handleConfirm() {
    disableCC.mutate(costCenterName, {
      onSuccess: (res) => {
        toast(res.message);
        onClose();
      },
      onError: (error) => {
        if (error instanceof ApiError) {
          toast((error.body as { message?: string })?.message ?? error.statusText, 'error');
        } else {
          toast('Errore di connessione', 'error');
        }
      },
    });
  }

  const users = details?.users ?? [];

  return (
    <Modal open={open} onClose={onClose} title="Disabilita centro di costo">
      <div className={styles.confirmMessage}>
        Disabilitare <strong>{costCenterName}</strong>?
        {users.length > 0 && (
          <>
            <br />
            Questo centro di costo ha {users.length} {users.length === 1 ? 'utente assegnato' : 'utenti assegnati'}:
            <div className={styles.impactList}>
              {users.map((u) => (
                <div key={u.id} className={styles.impactUser}>
                  <span className={styles.impactAvatar}>
                    {u.first_name[0]}{u.last_name[0]}
                  </span>
                  {u.first_name} {u.last_name}
                </div>
              ))}
            </div>
          </>
        )}
      </div>
      <div className={styles.actions}>
        <button type="button" className={styles.btnSecondary} onClick={onClose}>
          Annulla
        </button>
        <button
          type="button"
          className={styles.btnDanger}
          onClick={handleConfirm}
          disabled={disableCC.isPending || isFetching}
        >
          {isFetching ? 'Caricamento...' : disableCC.isPending ? 'Disabilitazione...' : 'Disabilita'}
        </button>
      </div>
    </Modal>
  );
}
