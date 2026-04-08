import { useState } from 'react';
import { Modal, SearchInput, useTableFilter, useToast } from '@mrsmith/ui';
import { useCustomers, useCreditBalance, useTransactions, useCreateTransaction } from '../hooks/useApi';
import { CustomerDropdown, toCustomerOptions } from '../components/shared/CustomerDropdown';
import styles from './FormPage.module.css';

export function GestioneCreditiPage() {
  const [customerId, setCustomerId] = useState<string | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');

  const { data: customers, isLoading: customersLoading } = useCustomers();
  const { data: balance } = useCreditBalance(customerId ? Number(customerId) : null);
  const { data: transactions } = useTransactions(customerId ? Number(customerId) : null);
  const createMutation = useCreateTransaction();
  const { toast } = useToast();

  const { filtered: filteredTransactions } = useTableFilter({
    data: transactions,
    searchQuery,
    searchFields: ['description', 'operated_by'],
  });

  // Modal form state
  const [amount, setAmount] = useState('');
  const [operationSign, setOperationSign] = useState<'+' | '-'>('+');
  const [description, setDescription] = useState('');

  function openModal() {
    setAmount('');
    setOperationSign('+');
    setDescription('');
    setModalOpen(true);
  }

  async function handleCreate() {
    if (!customerId) return;
    try {
      await createMutation.mutateAsync({
        customerId: Number(customerId),
        data: {
          amount: Number(amount),
          operation_sign: operationSign,
          description,
        },
      });
      setModalOpen(false);
      toast('Transazione registrata');
    } catch {
      toast('Errore nella registrazione', 'error');
    }
  }

  return (
    <>
      <div className={styles.pageHeader}>
        <h1 className={styles.pageTitle}>Gestione Crediti</h1>
        <p className={styles.pageSubtitle}>Visualizza il saldo e registra transazioni di credito per cliente</p>
      </div>
      <div className={styles.page}>
      <div className={styles.card}>
        <div className={styles.dropdownRow}>
          <CustomerDropdown
            options={toCustomerOptions(customers)}
            selected={customerId}
            onChange={setCustomerId}
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
            <p className={styles.emptyPromptHint}>Il saldo e le transazioni verranno mostrati dopo la selezione</p>
          </div>
        )}

        {customerId && (
          <>
            <div className={styles.balanceCard}>
              <span className={styles.balanceLabel}>Saldo attuale:</span>
              <span className={styles.balanceValue}>
                {formatCurrency(balance?.balance ?? 0)}
              </span>
            </div>

            <div className={styles.tableActions}>
              <SearchInput value={searchQuery} onChange={setSearchQuery} placeholder="Cerca transazioni..." />
            </div>

            <div style={{ overflowX: 'auto' }}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>Data</th>
                    <th className={styles.numCol}>Importo</th>
                    <th>+/-</th>
                    <th>Descrizione</th>
                    <th>Operatore</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredTransactions.map((t) => (
                    <tr key={t.id}>
                      <td>{formatDate(t.transaction_date)}</td>
                      <td className={styles.numCol}>{formatCurrency(t.amount)}</td>
                      <td>{t.operation_sign}</td>
                      <td>{t.description}</td>
                      <td>{t.operated_by}</td>
                    </tr>
                  ))}
                  {transactions?.length === 0 && (
                    <tr>
                      <td colSpan={5} style={{ textAlign: 'center', color: 'var(--color-text-muted)' }}>
                        Nessuna transazione registrata
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>

            <div className={styles.tableActions}>
              <button className={styles.primaryBtn} onClick={openModal} type="button">
                Nuova transazione
              </button>
            </div>
          </>
        )}
      </div>
      </div>

      <Modal open={modalOpen} onClose={() => setModalOpen(false)} title="Nuova transazione">
        <div className={styles.formGrid} style={{ gridTemplateColumns: '1fr' }}>
          <div className={styles.field}>
            <label className={styles.label}>Importo (€)</label>
            <input
              type="number"
              className={styles.input}
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              min={0}
              max={10000}
              step={0.01}
              placeholder="0.00"
            />
          </div>
          <div className={styles.field}>
            <label className={styles.label}>Operazione</label>
            <div style={{ display: 'flex', gap: 'var(--space-4)' }}>
              <label style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)', fontSize: '0.8125rem', cursor: 'pointer' }}>
                <input
                  type="radio"
                  checked={operationSign === '+'}
                  onChange={() => setOperationSign('+')}
                />
                Accredito (+)
              </label>
              <label style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)', fontSize: '0.8125rem', cursor: 'pointer' }}>
                <input
                  type="radio"
                  checked={operationSign === '-'}
                  onChange={() => setOperationSign('-')}
                />
                Debito (-)
              </label>
            </div>
          </div>
          <div className={styles.field}>
            <label className={styles.label}>
              Descrizione
              <span className={styles.range}>(obbligatorio, max 255)</span>
            </label>
            <textarea
              className={styles.input}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              maxLength={255}
              rows={3}
              style={{ resize: 'vertical' }}
            />
          </div>
        </div>
        <div className={styles.actions} style={{ marginTop: 'var(--space-5)' }}>
          <button className={styles.secondaryBtn} onClick={() => setModalOpen(false)} type="button">
            Annulla
          </button>
          <button
            className={styles.primaryBtn}
            onClick={handleCreate}
            disabled={createMutation.isPending || !amount || !description}
            type="button"
          >
            {createMutation.isPending ? 'Registrazione...' : 'Registra'}
          </button>
        </div>
      </Modal>
    </>
  );
}

function formatCurrency(value: number): string {
  return new Intl.NumberFormat('it-IT', {
    style: 'currency',
    currency: 'EUR',
    minimumFractionDigits: 2,
  }).format(value);
}

function formatDate(date: string): string {
  return new Intl.DateTimeFormat('it-IT', {
    day: '2-digit',
    month: '2-digit',
    year: '2-digit',
  }).format(new Date(date));
}
