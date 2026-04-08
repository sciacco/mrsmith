import { useState, useEffect } from 'react';
import { useToast } from '@mrsmith/ui';
import { useGrappaCustomers, useIaaSPricing, useUpsertIaaSPricing } from '../hooks/useApi';
import { CustomerDropdown, toGrappaOptions } from '../components/shared/CustomerDropdown';
import styles from './FormPage.module.css';

const iaasFields = [
  { key: 'charge_cpu', label: 'CPU (€/giorno)', min: 0.05, max: 0.1, step: 0.001 },
  { key: 'charge_ram_kvm', label: 'RAM KVM (€/giorno)', min: 0.05, max: 0.2, step: 0.001 },
  { key: 'charge_ram_vmware', label: 'RAM VMware (€/giorno)', min: 0.18, max: 0.3, step: 0.001 },
  { key: 'charge_pstor', label: 'Disco primario (€/GB)', min: 0.0005, max: 0.002, step: 0.0001 },
  { key: 'charge_sstor', label: 'Disco secondario (€/GB)', min: 0.0005, max: 0.002, step: 0.0001 },
  { key: 'charge_ip', label: 'IP pubblico (€/g)', min: 0.02, max: undefined, step: 0.001 },
] as const;

type FieldKey = (typeof iaasFields)[number]['key'];

export function IaaSPrezziPage() {
  const [customerId, setCustomerId] = useState<string | null>(null);
  const { data: customers, isLoading: customersLoading } = useGrappaCustomers('385');
  const { data: pricing } = useIaaSPricing(customerId ? Number(customerId) : null);
  const mutation = useUpsertIaaSPricing();
  const { toast } = useToast();

  const [form, setForm] = useState<Record<FieldKey, string>>({
    charge_cpu: '',
    charge_ram_kvm: '',
    charge_ram_vmware: '',
    charge_pstor: '',
    charge_sstor: '',
    charge_ip: '',
  });

  useEffect(() => {
    if (pricing) {
      setForm({
        charge_cpu: String(pricing.charge_cpu),
        charge_ram_kvm: String(pricing.charge_ram_kvm),
        charge_ram_vmware: String(pricing.charge_ram_vmware),
        charge_pstor: String(pricing.charge_pstor),
        charge_sstor: String(pricing.charge_sstor),
        charge_ip: String(pricing.charge_ip),
      });
    }
  }, [pricing]);

  function handleChange(key: FieldKey, value: string) {
    setForm((prev) => ({ ...prev, [key]: value }));
  }

  async function handleSave() {
    if (!customerId) return;
    try {
      await mutation.mutateAsync({
        customerId: Number(customerId),
        data: {
          charge_cpu: Number(form.charge_cpu),
          charge_ram_kvm: Number(form.charge_ram_kvm),
          charge_ram_vmware: Number(form.charge_ram_vmware),
          charge_pstor: Number(form.charge_pstor),
          charge_sstor: Number(form.charge_sstor),
          charge_ip: Number(form.charge_ip),
          charge_prefix24: pricing?.charge_prefix24 ?? undefined,
        },
      });
      toast('Prezzi salvati');
    } catch {
      toast('Errore nel salvataggio', 'error');
    }
  }

  return (
    <>
      <div className={styles.pageHeader}>
        <h1 className={styles.pageTitle}>Prezzi IaaS</h1>
        <p className={styles.pageSubtitle}>Gestisci i prezzi delle risorse cloud per cliente</p>
      </div>
      <div className={styles.page}>
      <div className={styles.card}>
        <div className={styles.dropdownRow}>
          <CustomerDropdown
            options={toGrappaOptions(customers)}
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
            <p className={styles.emptyPromptHint}>I prezzi IaaS verranno mostrati dopo la selezione</p>
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
              {iaasFields.map((field) => (
                <div key={field.key} className={styles.field}>
                  <label className={styles.label}>
                    {field.label}
                    <span className={styles.range}>
                      [{field.min} — {field.max ?? '∞'}]
                    </span>
                  </label>
                  <input
                    type="number"
                    className={styles.input}
                    value={form[field.key]}
                    onChange={(e) => handleChange(field.key, e.target.value)}
                    min={field.min}
                    max={field.max}
                    step={field.step}
                  />
                </div>
              ))}
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
