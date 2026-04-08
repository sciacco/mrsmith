import { useState, useCallback } from 'react';
import { SearchInput, useTableFilter, useToast } from '@mrsmith/ui';
import { useRackCustomers, useCustomerRacks, useBatchUpdateRackDiscounts } from '../hooks/useApi';
import { CustomerDropdown, toGrappaOptions } from '../components/shared/CustomerDropdown';
import type { RackDiscountUpdateItem } from '../types';
import styles from './FormPage.module.css';

export function ScontiEnergiaPage() {
  const [customerId, setCustomerId] = useState<string | null>(null);
  const { data: customers, isLoading: customersLoading } = useRackCustomers();
  const { data: racks } = useCustomerRacks(customerId ? Number(customerId) : null);
  const mutation = useBatchUpdateRackDiscounts();
  const { toast } = useToast();

  const [searchQuery, setSearchQuery] = useState('');

  // Track dirty values by id_rack
  const [dirty, setDirty] = useState<Record<number, number>>({});

  const { filtered: filteredRacks } = useTableFilter({
    data: racks,
    searchQuery,
    searchFields: ['name', 'building', 'room'],
  });

  const handleCustomerChange = useCallback((value: string | null) => {
    setCustomerId(value);
    setDirty({});
    setSearchQuery('');
  }, []);

  const handleChange = useCallback((idRack: number, value: string) => {
    setDirty((prev) => ({ ...prev, [idRack]: Number(value) }));
  }, []);

  const hasDirty = Object.keys(dirty).length > 0;

  async function handleSave() {
    if (!racks) return;
    const items: RackDiscountUpdateItem[] = [];
    for (const [idRackStr, sconto] of Object.entries(dirty)) {
      const idRack = Number(idRackStr);
      const rack = racks.find((r) => r.id_rack === idRack);
      if (rack && rack.sconto !== sconto) {
        items.push({ id_rack: idRack, sconto });
      }
    }
    if (items.length === 0) {
      setDirty({});
      return;
    }
    try {
      await mutation.mutateAsync(items);
      setDirty({});
      toast('Sconti aggiornati');
    } catch {
      toast('Errore nel salvataggio', 'error');
    }
  }

  return (
    <>
      <div className={styles.pageHeader}>
        <h1 className={styles.pageTitle}>Sconti Energia</h1>
        <p className={styles.pageSubtitle}>Configura gli sconti sulla variabile energia per rack</p>
      </div>
      <div className={styles.page}>
      <div className={styles.card}>
        <div className={styles.dropdownRow}>
          <CustomerDropdown
            options={toGrappaOptions(customers)}
            selected={customerId}
            onChange={handleCustomerChange}
            placeholder={customersLoading ? 'Caricamento...' : 'Seleziona cliente...'}
          />
        </div>

        {!customerId && (
          <div className={styles.emptyPrompt}>
            <div className={styles.emptyPromptIcon}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
                <circle cx="12" cy="7" r="4" />
              </svg>
            </div>
            <p className={styles.emptyPromptTitle}>Seleziona un cliente per continuare</p>
            <p className={styles.emptyPromptHint}>Gli sconti energia per rack verranno mostrati dopo la selezione</p>
          </div>
        )}

        {customerId && racks && racks.length > 0 && (
          <>
            <div className={styles.tableActions}>
              <h1 className={styles.tableTitle}>Sconti variabile energia</h1>
              <SearchInput value={searchQuery} onChange={setSearchQuery} placeholder="Cerca rack..." />
              <button
                className={styles.primaryBtn}
                onClick={handleSave}
                disabled={!hasDirty || mutation.isPending}
                type="button"
              >
                {mutation.isPending ? 'Salvataggio...' : 'Salva modifiche'}
              </button>
            </div>
            <div className={styles.tableScrollWrapper}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>Rack</th>
                    <th>Edificio</th>
                    <th>Sala</th>
                    <th>Piano</th>
                    <th className={`${styles.numCol} ${styles.editCol}`}>Sconto %</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredRacks.map((rack) => {
                    const currentValue = dirty[rack.id_rack] ?? rack.sconto;
                    const isDirty = rack.id_rack in dirty && dirty[rack.id_rack] !== rack.sconto;
                    return (
                      <tr key={rack.id_rack}>
                        <td>{rack.name}</td>
                        <td>{rack.building}</td>
                        <td>{rack.room}</td>
                        <td>{rack.floor ?? '—'}</td>
                        <td className={`${styles.numCol} ${styles.editCol}`}>
                          <input
                            type="number"
                            className={`${styles.inlineInput} ${isDirty ? styles.dirtyInput : ''}`}
                            value={currentValue}
                            onChange={(e) => handleChange(rack.id_rack, e.target.value)}
                            min={0}
                            max={20}
                            step={0.01}
                          />
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </>
        )}

        {customerId && racks && racks.length === 0 && (
          <p className={styles.emptyState}>Nessun rack attivo per questo cliente</p>
        )}
      </div>
      </div>
    </>
  );
}
