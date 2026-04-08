import { useMemo, useState } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { Skeleton, useToast } from '@mrsmith/ui';
import {
  useBatchUpdateCustomerGroups,
  useCreateCustomerGroup,
  useCustomerGroups,
} from '../../api/queries';
import styles from './SettingsPage.module.css';

type Drafts = Record<number, { name: string; is_partner: boolean }>;

export function CustomerGroupsPage() {
  const { toast } = useToast();
  const [drafts, setDrafts] = useState<Drafts>({});
  const [newRow, setNewRow] = useState({ name: '', is_partner: false });

  const { data, isLoading, error } = useCustomerGroups();
  const createCustomerGroup = useCreateCustomerGroup();
  const batchUpdate = useBatchUpdateCustomerGroups();

  const groups = data ?? [];
  const changedItems = useMemo(
    () =>
      groups
        .filter((group) => {
          const draft = drafts[group.id];
          return draft != null && !group.read_only && (
            draft.name.trim() !== group.name ||
            draft.is_partner !== group.is_partner
          );
        })
        .map((group) => ({
          id: group.id,
          name: drafts[group.id]!.name.trim(),
          is_partner: drafts[group.id]!.is_partner,
        })),
    [drafts, groups],
  );

  async function handleCreate() {
    if (!newRow.name.trim()) {
      toast('Il nome gruppo e obbligatorio', 'error');
      return;
    }

    try {
      await createCustomerGroup.mutateAsync({
        name: newRow.name.trim(),
        is_partner: newRow.is_partner,
      });
      setNewRow({ name: '', is_partner: false });
      toast('Gruppo creato', 'success');
    } catch (err) {
      toast(getErrorMessage(err, 'Impossibile creare il gruppo'), 'error');
    }
  }

  async function handleBatchSave() {
    if (changedItems.length === 0) {
      return;
    }

    try {
      await batchUpdate.mutateAsync({ items: changedItems });
      setDrafts({});
      toast('Gruppi aggiornati', 'success');
    } catch (err) {
      const message = getErrorMessage(err, 'Impossibile aggiornare i gruppi');
      toast(message === 'read_only_group' ? 'Uno dei gruppi selezionati e protetto in lettura' : message, 'error');
    }
  }

  if (isLoading) {
    return (
      <section className={styles.page}>
        <Skeleton rows={6} />
      </section>
    );
  }

  return (
    <section className={styles.page}>
      <header className={styles.header}>
        <div>
          <p className={styles.eyebrow}>Settings</p>
          <h1>Gruppi cliente</h1>
          <p className={styles.lead}>
            Gestisci i profili commerciali condivisi, mantenendo bloccati quelli protetti dal flag di sola lettura.
          </p>
        </div>
        <div className={styles.highlight}>
          <span>Pending batch</span>
          <strong>{changedItems.length}</strong>
          <p>modifiche pronte per un singolo commit transazionale.</p>
        </div>
      </header>

      <section className={styles.card}>
        <div className={styles.sectionHeader}>
          <div>
            <h2>Nuovo gruppo</h2>
            <p>Aggiungi un profilo sconto senza uscire dalla pagina di configurazione.</p>
          </div>
        </div>

        <div className={styles.inlineForm}>
          <label className={styles.field}>
            <span>Nome</span>
            <input
              value={newRow.name}
              onChange={(event) => setNewRow((current) => ({ ...current, name: event.target.value }))}
              placeholder="Commerciale Wholesale"
            />
          </label>
          <label className={`${styles.field} ${styles.checkboxField}`}>
            <span>Partner</span>
            <input
              type="checkbox"
              checked={newRow.is_partner}
              onChange={(event) => setNewRow((current) => ({ ...current, is_partner: event.target.checked }))}
            />
          </label>
          <button
            type="button"
            className={styles.primaryButton}
            onClick={() => void handleCreate()}
            disabled={createCustomerGroup.isPending}
          >
            Aggiungi
          </button>
        </div>
      </section>

      <section className={styles.card}>
        <div className={styles.sectionHeader}>
          <div>
            <h2>Profili commerciali</h2>
            <p>Le modifiche ai gruppi editabili vengono salvate in batch con rollback se un elemento fallisce.</p>
          </div>
          <button
            type="button"
            className={styles.primaryButton}
            disabled={changedItems.length === 0 || batchUpdate.isPending}
            onClick={() => void handleBatchSave()}
          >
            Salva modifiche
          </button>
        </div>

        {error ? (
          <div className={styles.emptyState}>
            <p className={styles.emptyTitle}>Impossibile caricare i gruppi</p>
            <p className={styles.emptyText}>{getErrorMessage(error, 'Riprova tra poco.')}</p>
          </div>
        ) : groups.length === 0 ? (
          <div className={styles.emptyState}>
            <p className={styles.emptyTitle}>Nessun gruppo disponibile</p>
            <p className={styles.emptyText}>Aggiungi il primo profilo commerciale per iniziare.</p>
          </div>
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>Nome</th>
                  <th>Base discount</th>
                  <th>Partner</th>
                  <th>Stato</th>
                </tr>
              </thead>
              <tbody>
                {groups.map((group) => {
                  const draft = drafts[group.id] ?? {
                    name: group.name,
                    is_partner: group.is_partner,
                  };

                  return (
                    <tr key={group.id}>
                      <td>
                        <input
                          value={draft.name}
                          disabled={group.read_only}
                          onChange={(event) => {
                            const value = event.target.value;
                            setDrafts((current) => ({
                              ...current,
                              [group.id]: {
                                name: value,
                                is_partner: current[group.id]?.is_partner ?? group.is_partner,
                              },
                            }));
                          }}
                        />
                      </td>
                      <td>
                        <span className={styles.numericValue}>
                          {group.base_discount == null ? 'n/d' : `${(group.base_discount * 100).toFixed(1)}%`}
                        </span>
                      </td>
                      <td>
                        <label className={styles.checkboxInline}>
                          <input
                            type="checkbox"
                            checked={draft.is_partner}
                            disabled={group.read_only}
                            onChange={(event) => {
                              const value = event.target.checked;
                              setDrafts((current) => ({
                                ...current,
                                [group.id]: {
                                  name: current[group.id]?.name ?? group.name,
                                  is_partner: value,
                                },
                              }));
                            }}
                          />
                          <span>{draft.is_partner ? 'Si' : 'No'}</span>
                        </label>
                      </td>
                      <td>
                        <div className={styles.badgeRow}>
                          {group.is_default ? <span className={styles.badge}>Default</span> : null}
                          {group.read_only ? <span className={styles.badgeMuted}>Read only</span> : null}
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </section>
  );
}

function getErrorMessage(error: unknown, fallback: string) {
  if (error instanceof ApiError && typeof error.body === 'object' && error.body && 'error' in error.body) {
    const message = error.body.error;
    if (typeof message === 'string') {
      return message;
    }
  }
  if (error instanceof Error) {
    return error.message;
  }
  return fallback;
}
