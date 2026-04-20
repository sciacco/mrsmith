import { useEffect, useMemo, useState } from 'react';
import { Button, useToast } from '@mrsmith/ui';
import { useBiometricRequests } from '../../hooks/useBiometricRequests';
import { useSetBiometricCompleted } from '../../hooks/useSetBiometricCompleted';
import type { BiometricRequestRow } from '../../api/biometric';
import styles from './AccessiBiometricoPage.module.css';

// Column labels are preserved VERBATIM in v1 for operator-facing parity
// (FINAL.md §Slice S5c / PROMPT.md "User Copy Rules"). Do not capitalize.
// The label `data conferma` intentionally contains a space and maps to the
// `data_approvazione` backend field; `data della richiesta` maps to
// `data_richiesta`.
interface ColumnDef {
  label: string;
  render: (row: BiometricRequestRow) => string;
}

function formatTimestamp(value: string | null): string {
  if (!value) return '';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return '';
  const day = String(d.getDate()).padStart(2, '0');
  const month = String(d.getMonth() + 1).padStart(2, '0');
  const year = d.getFullYear();
  const hours = String(d.getHours()).padStart(2, '0');
  const minutes = String(d.getMinutes()).padStart(2, '0');
  return `${day}/${month}/${year} ${hours}:${minutes}`;
}

const TEXT_COLUMNS: ColumnDef[] = [
  { label: 'nome', render: (r) => r.nome },
  { label: 'cognome', render: (r) => r.cognome },
  { label: 'email', render: (r) => r.email },
  { label: 'azienda', render: (r) => r.azienda },
  { label: 'tipo_richiesta', render: (r) => r.tipo_richiesta },
];

export function AccessiBiometricoPage() {
  const { data, isLoading, isError } = useBiometricRequests();
  const mutation = useSetBiometricCompleted();
  const { toast } = useToast();

  // Local per-row edit state for the editable `stato_richiesta` checkbox.
  // Keyed by row id; presence indicates the row is dirty.
  const [pendingByRow, setPendingByRow] = useState<Record<number, boolean>>({});

  // When a server refetch lands (or rows disappear), drop stale dirty state
  // so Discard always reverts to the authoritative server value.
  useEffect(() => {
    if (!data) return;
    setPendingByRow((prev) => {
      const validIds = new Set(data.map((r) => r.id));
      let changed = false;
      const next: Record<number, boolean> = {};
      for (const [key, value] of Object.entries(prev)) {
        const idNum = Number(key);
        if (validIds.has(idNum)) {
          next[idNum] = value;
        } else {
          changed = true;
        }
      }
      return changed ? next : prev;
    });
  }, [data]);

  const rows = useMemo<BiometricRequestRow[]>(() => data ?? [], [data]);

  function onToggle(row: BiometricRequestRow, nextValue: boolean) {
    setPendingByRow((prev) => {
      // If the new value matches the server value, clear the dirty flag.
      if (nextValue === row.stato_richiesta) {
        if (!(row.id in prev)) return prev;
        const { [row.id]: _removed, ...rest } = prev;
        return rest;
      }
      return { ...prev, [row.id]: nextValue };
    });
  }

  function onDiscard(rowId: number) {
    setPendingByRow((prev) => {
      if (!(rowId in prev)) return prev;
      const { [rowId]: _removed, ...rest } = prev;
      return rest;
    });
  }

  function onSave(row: BiometricRequestRow) {
    const pending = pendingByRow[row.id];
    if (pending === undefined) return;
    mutation.mutate(
      { id: row.id, completed: pending },
      {
        onSuccess: () => {
          // Clear dirty state optimistically; list refetch will reconcile.
          setPendingByRow((prev) => {
            if (!(row.id in prev)) return prev;
            const { [row.id]: _removed, ...rest } = prev;
            return rest;
          });
          toast('Perfetto, stato biometrico cambiato', 'success');
        },
        onError: () => {
          toast("Qualcosa e' andato storto", 'error');
        },
      },
    );
  }

  return (
    <section className={styles.page}>
      <h1 className={styles.title}>Accessi Biometrico</h1>
      <p className={styles.subtitle}>
        Elenco delle richieste di accesso biometrico. Modifica lo stato e conferma riga per riga.
      </p>

      {isError && (
        <div className={styles.error} role="alert">
          Impossibile caricare l&apos;elenco delle richieste. Riprova piu tardi.
        </div>
      )}

      {isLoading && !isError && (
        <div className={styles.loading}>Caricamento in corso.</div>
      )}

      {!isLoading && !isError && rows.length === 0 && (
        <div className={styles.empty}>Nessuna richiesta da mostrare.</div>
      )}

      {!isLoading && !isError && rows.length > 0 && (
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                {TEXT_COLUMNS.map((col) => (
                  <th key={col.label}>{col.label}</th>
                ))}
                <th>stato_richiesta</th>
                <th>data conferma</th>
                <th>data della richiesta</th>
                <th aria-label="azioni" />
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => {
                const pending = pendingByRow[row.id];
                const isDirty = pending !== undefined;
                const checked = isDirty ? pending : row.stato_richiesta;
                const isSavingThisRow =
                  mutation.isPending && mutation.variables?.id === row.id;
                return (
                  <tr key={row.id}>
                    {TEXT_COLUMNS.map((col) => (
                      <td key={col.label}>{col.render(row)}</td>
                    ))}
                    <td className={styles.checkboxCell}>
                      <input
                        type="checkbox"
                        className={styles.checkbox}
                        checked={checked}
                        onChange={(e) => onToggle(row, e.target.checked)}
                        aria-label={`stato_richiesta ${row.nome} ${row.cognome}`}
                      />
                    </td>
                    <td>{formatTimestamp(row.data_approvazione)}</td>
                    <td>{formatTimestamp(row.data_richiesta)}</td>
                    <td>
                      {isDirty && (
                        <div className={styles.rowActions}>
                          <Button
                            size="sm"
                            variant="primary"
                            onClick={() => onSave(row)}
                            loading={isSavingThisRow}
                          >
                            Salva
                          </Button>
                          <Button
                            size="sm"
                            variant="secondary"
                            onClick={() => onDiscard(row.id)}
                            disabled={isSavingThisRow}
                          >
                            Annulla
                          </Button>
                        </div>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
