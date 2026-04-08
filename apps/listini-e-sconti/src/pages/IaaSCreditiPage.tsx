import { useState, useCallback } from 'react';
import { SearchInput, useTableFilter, useToast } from '@mrsmith/ui';
import { useIaaSAccounts, useBatchUpdateIaaSCredits } from '../hooks/useApi';
import type { IaaSCreditUpdateItem } from '../types';
import styles from './FormPage.module.css';

export function IaaSCreditiPage() {
  const { data: accounts, isLoading } = useIaaSAccounts();
  const mutation = useBatchUpdateIaaSCredits();
  const { toast } = useToast();

  const [searchQuery, setSearchQuery] = useState('');

  // Track dirty values by domainuuid
  const [dirty, setDirty] = useState<Record<string, number>>({});

  const handleChange = useCallback((domainuuid: string, value: string) => {
    setDirty((prev) => ({ ...prev, [domainuuid]: Number(value) }));
  }, []);

  const { filtered: filteredAccounts } = useTableFilter({
    data: accounts,
    searchQuery,
    searchFields: ['intestazione', 'abbreviazione', 'serialnumber', 'codice_ordine', 'infrastructure_platform'],
  });

  const hasDirty = Object.keys(dirty).length > 0;

  async function handleSave() {
    if (!accounts) return;
    const items: IaaSCreditUpdateItem[] = [];
    for (const [domainuuid, credito] of Object.entries(dirty)) {
      const account = accounts.find((a) => a.domainuuid === domainuuid);
      if (account && account.credito !== credito) {
        items.push({
          domainuuid,
          id_cli_fatturazione: account.id_cli_fatturazione,
          credito,
        });
      }
    }
    if (items.length === 0) {
      setDirty({});
      return;
    }
    try {
      await mutation.mutateAsync(items);
      setDirty({});
      toast('Crediti aggiornati');
    } catch {
      toast('Errore nel salvataggio', 'error');
    }
  }

  if (isLoading) {
    return (
      <>
        <div className={styles.pageHeader}>
          <h1 className={styles.pageTitle}>Crediti IaaS</h1>
          <p className={styles.pageSubtitle}>Gestisci i crediti omaggio per gli account IaaS</p>
        </div>
        <div className={styles.page}><div className={styles.card}><p>Caricamento...</p></div></div>
      </>
    );
  }

  return (
    <>
      <div className={styles.pageHeader}>
        <h1 className={styles.pageTitle}>Crediti IaaS</h1>
        <p className={styles.pageSubtitle}>Gestisci i crediti omaggio per gli account IaaS</p>
      </div>
      <div className={styles.page}>
      <div className={styles.card}>
        <div className={styles.tableActions}>
          <h1 className={styles.tableTitle}>Crediti Omaggio IaaS</h1>
          <SearchInput value={searchQuery} onChange={setSearchQuery} placeholder="Cerca account..." />
          {(accounts?.length ?? 0) > 0 && (
            <button
              className={styles.primaryBtn}
              onClick={handleSave}
              disabled={!hasDirty || mutation.isPending}
              type="button"
            >
              {mutation.isPending ? 'Salvataggio...' : 'Salva modifiche'}
            </button>
          )}
        </div>
        <div className={styles.tableScrollWrapper}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>Cliente</th>
                <th className={`${styles.numCol} ${styles.editCol}`}>Credito</th>
                <th>Account</th>
                <th>Serialnumber</th>
                <th>Attivazione</th>
                <th>Codice ordine</th>
                <th>Piattaforma</th>
                <th>Domain</th>
              </tr>
            </thead>
            <tbody>
              {filteredAccounts.map((a) => {
                const isCloudstack = a.infrastructure_platform === 'cloudstack';
                const currentValue = dirty[a.domainuuid] ?? a.credito;
                const isDirty = a.domainuuid in dirty && dirty[a.domainuuid] !== a.credito;
                return (
                  <tr
                    key={a.domainuuid}
                    className={!isCloudstack ? styles.mutedRow : undefined}
                  >
                    <td>{a.intestazione}</td>
                    <td className={`${styles.numCol} ${styles.editCol}`}>
                      {isCloudstack ? (
                        <input
                          type="number"
                          className={`${styles.inlineInput} ${isDirty ? styles.dirtyInput : ''}`}
                          value={currentValue}
                          onChange={(e) => handleChange(a.domainuuid, e.target.value)}
                          step="0.01"
                        />
                      ) : (
                        a.credito
                      )}
                    </td>
                    <td>{a.abbreviazione}</td>
                    <td>{a.serialnumber}</td>
                    <td>{formatDate(a.data_attivazione)}</td>
                    <td>{a.codice_ordine}</td>
                    <td>{a.infrastructure_platform}</td>
                    <td className={styles.truncated}>{a.domainuuid}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
        {accounts?.length === 0 && (
          <p className={styles.emptyState}>Nessun account IaaS trovato</p>
        )}
      </div>
      </div>
    </>
  );
}

function formatDate(raw: string): string {
  if (!raw) return '';
  const d = new Date(raw);
  if (isNaN(d.getTime())) return raw;
  return new Intl.DateTimeFormat('it-IT', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
  }).format(d);
}
