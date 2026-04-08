import { useState, useEffect } from 'react';
import { useToast } from '@mrsmith/ui';
import { useCustomers, useTimooPricing, useUpsertTimooPricing } from '../hooks/useApi';
import { CustomerDropdown, toCustomerOptions } from '../components/shared/CustomerDropdown';
import styles from './FormPage.module.css';

export function TimooPrezziPage() {
  const [customerId, setCustomerId] = useState<string | null>(null);
  const { data: customers, isLoading: customersLoading } = useCustomers();
  const { data: pricing } = useTimooPricing(customerId ? Number(customerId) : null);
  const mutation = useUpsertTimooPricing();
  const { toast } = useToast();

  const [userMonth, setUserMonth] = useState('');
  const [seMonth, setSeMonth] = useState('');

  useEffect(() => {
    if (pricing) {
      setUserMonth(String(pricing.user_month));
      setSeMonth(String(pricing.se_month));
    }
  }, [pricing]);

  async function handleSave() {
    if (!customerId) return;
    try {
      await mutation.mutateAsync({
        customerId: Number(customerId),
        data: {
          user_month: Number(userMonth),
          se_month: Number(seMonth),
        },
      });
      toast('Prezzi Timoo salvati');
    } catch {
      toast('Errore nel salvataggio', 'error');
    }
  }

  return (
    <>
      <div className={styles.pageHeader}>
        <h1 className={styles.pageTitle}>Prezzi Timoo</h1>
        <p className={styles.pageSubtitle}>Gestisci i prezzi per utente e SE del servizio Timoo</p>
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
            <p className={styles.emptyPromptHint}>I prezzi Timoo verranno mostrati dopo la selezione</p>
          </div>
        )}

        {customerId && pricing && (
          <>
            {pricing.is_default && (
              <div className={styles.infoBadge}>
                Valori predefiniti — nessun prezzo personalizzato
              </div>
            )}
            <div className={styles.formGrid}>
              <div className={styles.field}>
                <label className={styles.label}>Prezzo utente/mese (€)</label>
                <input
                  type="number"
                  className={styles.input}
                  value={userMonth}
                  onChange={(e) => setUserMonth(e.target.value)}
                  step="0.01"
                />
              </div>
              <div className={styles.field}>
                <label className={styles.label}>Prezzo SE/mese (€)</label>
                <input
                  type="number"
                  className={styles.input}
                  value={seMonth}
                  onChange={(e) => setSeMonth(e.target.value)}
                  step="0.01"
                />
              </div>
            </div>
            <div className={styles.actions}>
              <button
                className={styles.primaryBtn}
                onClick={handleSave}
                disabled={mutation.isPending}
                type="button"
              >
                {mutation.isPending ? 'Salvataggio...' : 'Salva'}
              </button>
            </div>
          </>
        )}
      </div>
      </div>
    </>
  );
}
