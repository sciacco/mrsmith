import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useDeals, useOwners, usePaymentMethods, useTemplates, useCreateQuote } from '../api/queries';
import type { Deal } from '../api/types';
import { Stepper } from '../components/Stepper';
import { WizardNav } from '../components/WizardNav';
import { DealCard } from '../components/DealCard';
import { TypeSelector } from '../components/TypeSelector';
import styles from './QuoteCreatePage.module.css';

const stepNames = ['Deal', 'Configurazione', 'Kit e Prodotti', 'Extra', 'Riepilogo'];

interface WizardState {
  selectedDeal: Deal | null;
  quoteType: 'standard' | 'iaas';
  document_type: 'TSC-ORDINE-RIC' | 'TSC-ORDINE';
  proposal_type: 'NUOVO' | 'SOSTITUZIONE' | 'RINNOVO';
  replace_orders: string;
  owner: string;
  template: string;
  services: string;
  payment_method: string;
  initial_term_months: number;
  next_term_months: number;
  bill_months: number;
  delivered_in_days: number;
  nrc_charge_time: number;
  description: string;
  notes: string;
  kit_ids: number[];
}

const initialState: WizardState = {
  selectedDeal: null,
  quoteType: 'standard',
  document_type: 'TSC-ORDINE-RIC',
  proposal_type: 'NUOVO',
  replace_orders: '',
  owner: '',
  template: '',
  services: '',
  payment_method: '402',
  initial_term_months: 12,
  next_term_months: 12,
  bill_months: 2,
  delivered_in_days: 60,
  nrc_charge_time: 2,
  description: '',
  notes: '',
  kit_ids: [],
};

export function QuoteCreatePage() {
  const navigate = useNavigate();
  const [step, setStep] = useState(0);
  const [state, setState] = useState<WizardState>(initialState);
  const [dealSearch, setDealSearch] = useState('');

  const { data: deals } = useDeals();
  const { data: owners } = useOwners();
  const { data: paymentMethods } = usePaymentMethods();
  const { data: templates } = useTemplates({ type: state.quoteType });
  const createQuote = useCreateQuote();

  // beforeunload protection
  useEffect(() => {
    const handler = (e: BeforeUnloadEvent) => { e.preventDefault(); };
    window.addEventListener('beforeunload', handler);
    return () => window.removeEventListener('beforeunload', handler);
  }, []);

  const filteredDeals = useMemo(() => {
    if (!deals) return [];
    if (!dealSearch) return deals;
    const q = dealSearch.toLowerCase();
    return deals.filter(d =>
      d.name.toLowerCase().includes(q) ||
      (d.company_name?.toLowerCase().includes(q) ?? false)
    );
  }, [deals, dealSearch]);

  const update = useCallback(<K extends keyof WizardState>(key: K, value: WizardState[K]) => {
    setState(prev => ({ ...prev, [key]: value }));
  }, []);

  const canAdvance = useMemo(() => {
    switch (step) {
      case 0: return state.selectedDeal !== null;
      case 1: return state.template !== '' && state.owner !== '';
      case 2: return true; // Kit step is flexible
      case 3: return true; // Optional
      case 4: return true; // Summary — user confirms
      default: return false;
    }
  }, [step, state]);

  const handleCreate = useCallback(async () => {
    if (!state.selectedDeal) return;
    try {
      const result = await createQuote.mutateAsync({
        customer_id: state.selectedDeal.company_id,
        deal_number: state.selectedDeal.name,
        hs_deal_id: state.selectedDeal.id,
        owner: state.owner,
        document_date: new Date().toISOString().split('T')[0],
        document_type: state.document_type,
        proposal_type: state.proposal_type,
        replace_orders: state.replace_orders,
        template: state.template,
        services: state.services,
        payment_method: state.payment_method,
        initial_term_months: state.initial_term_months,
        next_term_months: state.next_term_months,
        bill_months: state.bill_months,
        delivered_in_days: state.delivered_in_days,
        nrc_charge_time: state.nrc_charge_time,
        description: state.description,
        notes: state.notes,
        status: 'DRAFT',
        kit_ids: state.kit_ids,
      });
      navigate(`/quotes/${result.id}`);
    } catch {
      // Error handling via mutation
    }
  }, [state, createQuote, navigate]);

  const handleNext = () => {
    if (step === 4) {
      void handleCreate();
    } else {
      setStep(s => s + 1);
    }
  };

  return (
    <div className={styles.page}>
      <h1 className={styles.title}>Nuova proposta</h1>
      <Stepper steps={stepNames} current={step} onStepClick={s => setStep(s)} />

      <div className={styles.stepContent} key={step}>
        {step === 0 && (
          <>
            <div className={styles.dealSearch}>
              <input
                className={styles.dealSearchInput}
                placeholder="Cerca deal..."
                value={dealSearch}
                onChange={e => setDealSearch(e.target.value)}
                autoFocus
              />
            </div>
            <div className={styles.dealGrid}>
              {filteredDeals.map(d => (
                <DealCard
                  key={d.id}
                  deal={d}
                  selected={state.selectedDeal?.id === d.id}
                  onClick={() => update('selectedDeal', d)}
                />
              ))}
            </div>
          </>
        )}

        {step === 1 && (
          <>
            <TypeSelector value={state.quoteType} onChange={v => update('quoteType', v)} />
            <div className={styles.configGrid}>
              <div className={styles.section}>
                <div className={styles.sectionTitle}>Configurazione</div>
                <div className={styles.field}>
                  <label className={styles.label}>Tipo documento</label>
                  <div className={styles.radioGroup}>
                    <label className={styles.radioLabel}>
                      <input className={styles.radioInput} type="radio" checked={state.document_type === 'TSC-ORDINE-RIC'}
                        onChange={() => update('document_type', 'TSC-ORDINE-RIC')} /> Ricorrente
                    </label>
                    <label className={styles.radioLabel}>
                      <input className={styles.radioInput} type="radio" checked={state.document_type === 'TSC-ORDINE'}
                        onChange={() => update('document_type', 'TSC-ORDINE')} /> Spot
                    </label>
                  </div>
                </div>
                <div className={styles.field}>
                  <label className={styles.label}>Tipo proposta</label>
                  <div className={styles.radioGroup}>
                    {(['NUOVO', 'SOSTITUZIONE', 'RINNOVO'] as const).map(pt => (
                      <label key={pt} className={styles.radioLabel}>
                        <input className={styles.radioInput} type="radio" checked={state.proposal_type === pt}
                          onChange={() => update('proposal_type', pt)} />
                        {pt === 'NUOVO' ? 'Nuovo' : pt === 'SOSTITUZIONE' ? 'Sostituzione' : 'Rinnovo'}
                      </label>
                    ))}
                  </div>
                </div>
                <div className={styles.field}>
                  <label className={styles.label}>Owner</label>
                  <select className={styles.select} value={state.owner} onChange={e => update('owner', e.target.value)}>
                    <option value="">— Seleziona —</option>
                    {owners?.map(o => (
                      <option key={o.id} value={o.id}>{[o.firstname, o.lastname].filter(Boolean).join(' ')}</option>
                    ))}
                  </select>
                </div>
                <div className={styles.field}>
                  <label className={styles.label}>Template</label>
                  <select className={styles.select} value={state.template} onChange={e => update('template', e.target.value)}>
                    <option value="">— Seleziona —</option>
                    {templates?.map(t => (
                      <option key={t.template_id} value={t.template_id}>{t.description}</option>
                    ))}
                  </select>
                </div>
              </div>

              <div className={styles.section}>
                <div className={styles.sectionTitle}>Condizioni</div>
                <div className={styles.field}>
                  <label className={styles.label}>Pagamento</label>
                  <select className={styles.select} value={state.payment_method} onChange={e => update('payment_method', e.target.value)}>
                    {paymentMethods?.map(pm => (
                      <option key={pm.code} value={pm.code}>{pm.description}</option>
                    ))}
                  </select>
                </div>
                <div className={styles.field}>
                  <label className={styles.label}>Durata iniziale (mesi)</label>
                  <input className={styles.input} type="number" value={state.initial_term_months}
                    onChange={e => update('initial_term_months', Number(e.target.value))} />
                </div>
                <div className={styles.field}>
                  <label className={styles.label}>Durata rinnovo (mesi)</label>
                  <input className={styles.input} type="number" value={state.next_term_months}
                    onChange={e => update('next_term_months', Number(e.target.value))} />
                </div>
                <div className={styles.field}>
                  <label className={styles.label}>Consegna (giorni)</label>
                  <input className={styles.input} type="number" value={state.delivered_in_days}
                    onChange={e => update('delivered_in_days', Number(e.target.value))} />
                </div>
              </div>
            </div>
          </>
        )}

        {step === 2 && (
          <div className={styles.section}>
            <div className={styles.sectionTitle}>Kit e Prodotti</div>
            <p style={{ color: '#64748b', fontSize: '0.875rem' }}>
              I kit verranno configurati dopo la creazione della proposta, nella pagina di dettaglio.
            </p>
          </div>
        )}

        {step === 3 && (
          <div className={styles.section}>
            <div className={styles.sectionTitle}>Extra (opzionale)</div>
            <p style={{ color: '#64748b', fontSize: '0.875rem' }}>
              Descrizione, note e contatti potranno essere aggiunti dopo la creazione.
            </p>
          </div>
        )}

        {step === 4 && (
          <div className={styles.summaryCard}>
            <div className={styles.sectionTitle}>Riepilogo</div>
            <div className={styles.summaryRow}>
              <span className={styles.summaryLabel}>Deal</span>
              <span className={styles.summaryValue}>{state.selectedDeal?.name ?? '—'}</span>
            </div>
            <div className={styles.summaryRow}>
              <span className={styles.summaryLabel}>Cliente</span>
              <span className={styles.summaryValue}>{state.selectedDeal?.company_name ?? '—'}</span>
            </div>
            <div className={styles.summaryRow}>
              <span className={styles.summaryLabel}>Tipo</span>
              <span className={styles.summaryValue}>
                {state.document_type === 'TSC-ORDINE-RIC' ? 'Ricorrente' : 'Spot'} / {state.proposal_type}
              </span>
            </div>
            <div className={styles.summaryRow}>
              <span className={styles.summaryLabel}>Durata</span>
              <span className={styles.summaryValue}>
                {state.initial_term_months}m init / {state.next_term_months}m rinnovo
              </span>
            </div>
          </div>
        )}
      </div>

      <WizardNav
        step={step}
        totalSteps={stepNames.length}
        canAdvance={canAdvance}
        isLastStep={step === 4}
        onBack={() => setStep(s => s - 1)}
        onNext={handleNext}
        isPending={createQuote.isPending}
      />
    </div>
  );
}
