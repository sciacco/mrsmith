import { useState } from 'react';
import { Modal, MultiSelect, SearchInput, useTableFilter, useToast } from '@mrsmith/ui';
import {
  useERPLinkedCustomers, useCustomerGroups, useCustomerGroupIds,
  useSyncCustomerGroups, useKitDiscountsByGroup,
} from '../hooks/useApi';
import { CustomerDropdown, toCustomerOptions } from '../components/shared/CustomerDropdown';
import styles from './FormPage.module.css';

export function GruppiScontoPage() {
  const [customerId, setCustomerId] = useState<string | null>(null);
  const [selectedGroupId, setSelectedGroupId] = useState<number | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');

  const { data: customers, isLoading: customersLoading } = useERPLinkedCustomers();
  const { data: allGroups } = useCustomerGroups();
  const { data: groupData } = useCustomerGroupIds(customerId ? Number(customerId) : null);
  const syncMutation = useSyncCustomerGroups();
  const { data: kitDiscounts } = useKitDiscountsByGroup(selectedGroupId);
  const { toast } = useToast();

  const customerGroupIds = groupData?.groupIds ?? [];
  const customerGroups = (allGroups ?? []).filter((g) => customerGroupIds.includes(g.id));

  const { filtered: filteredDiscounts } = useTableFilter({
    data: kitDiscounts,
    searchQuery,
    searchFields: ['kit_name'],
  });

  // MultiSelect state
  const [modalSelectedIds, setModalSelectedIds] = useState<number[]>([]);

  function openModal() {
    setModalSelectedIds(customerGroupIds);
    setModalOpen(true);
  }

  async function handleSaveGroups() {
    if (!customerId) return;
    try {
      await syncMutation.mutateAsync({
        customerId: Number(customerId),
        groupIds: modalSelectedIds,
      });
      setModalOpen(false);
      toast('Gruppi aggiornati');
    } catch {
      toast('Errore nel salvataggio', 'error');
    }
  }

  function handleCustomerChange(value: string | null) {
    setCustomerId(value);
    setSelectedGroupId(null);
    setSearchQuery('');
  }

  return (
    <>
      <div className={styles.pageHeader}>
        <h1 className={styles.pageTitle}>Gruppi Sconto</h1>
        <p className={styles.pageSubtitle}>Associa gruppi sconto ai clienti e visualizza gli sconti kit applicati</p>
      </div>
      <div className={styles.page}>
      <div className={styles.card}>
        <div className={styles.dropdownRow}>
          <CustomerDropdown
            options={toCustomerOptions(customers)}
            selected={customerId}
            onChange={handleCustomerChange}
            placeholder={customersLoading ? 'Caricamento...' : 'Seleziona cliente (ERP-linked)...'}
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
            <p className={styles.emptyPromptHint}>I gruppi sconto e le relative configurazioni verranno mostrati dopo la selezione</p>
          </div>
        )}

        {customerId && (
          <div className={styles.threeCol}>
            <div className={styles.colCard}>
              <h3 className={styles.colTitle}>Gruppi associati</h3>
              <div className={styles.groupList}>
                {customerGroups.length === 0 && (
                  <p style={{ fontSize: '0.8125rem', color: 'var(--color-text-muted)' }}>Nessun gruppo associato</p>
                )}
                {customerGroups.map((g) => (
                  <button
                    key={g.id}
                    type="button"
                    className={`${styles.groupItem} ${g.id === selectedGroupId ? styles.groupItemActive : ''}`}
                    onClick={() => setSelectedGroupId(g.id)}
                  >
                    {g.name}
                  </button>
                ))}
              </div>
              <div style={{ marginTop: 'var(--space-4)' }}>
                <button className={styles.secondaryBtn} onClick={openModal} type="button">
                  Associa gruppi
                </button>
              </div>
            </div>

            <div className={styles.colCard} style={{ gridColumn: 'span 2' }}>
              <div className={styles.tableActions}>
                <h3 className={styles.colTitle} style={{ marginBottom: 0 }}>Sconti kit per gruppo</h3>
                {selectedGroupId != null && (
                  <SearchInput value={searchQuery} onChange={setSearchQuery} placeholder="Cerca kit..." />
                )}
              </div>
              {selectedGroupId == null ? (
                <p style={{ fontSize: '0.8125rem', color: 'var(--color-text-muted)' }}>
                  Seleziona un gruppo per vedere gli sconti kit
                </p>
              ) : (
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th>Kit</th>
                      <th className={styles.numCol}>MRC %</th>
                      <th className={styles.numCol}>NRC %</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredDiscounts.map((d) => (
                      <tr key={d.kit_id}>
                        <td>{d.kit_name}</td>
                        <td className={styles.numCol}>{d.discount_mrc}%</td>
                        <td className={styles.numCol}>{d.discount_nrc}%</td>
                      </tr>
                    ))}
                    {kitDiscounts?.length === 0 && (
                      <tr>
                        <td colSpan={3} style={{ textAlign: 'center', color: 'var(--color-text-muted)' }}>
                          Nessuno sconto configurato
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              )}
            </div>
          </div>
        )}
      </div>
      </div>

      <Modal open={modalOpen} onClose={() => setModalOpen(false)} title="Associa gruppi">
        <MultiSelect
          options={(allGroups ?? []).map((g) => ({ value: g.id, label: g.name }))}
          selected={modalSelectedIds}
          onChange={setModalSelectedIds}
        />
        <div className={styles.actions} style={{ marginTop: 'var(--space-5)' }}>
          <button className={styles.secondaryBtn} onClick={() => setModalOpen(false)} type="button">
            Annulla
          </button>
          <button
            className={styles.primaryBtn}
            onClick={handleSaveGroups}
            disabled={syncMutation.isPending}
            type="button"
          >
            {syncMutation.isPending ? 'Salvataggio...' : 'Salva'}
          </button>
        </div>
      </Modal>
    </>
  );
}
