import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  useCategories,
  useCustomerOrders,
  useCustomerPayment,
  useDeals,
  useKits,
  useOwners,
  usePaymentMethods,
  useTemplates,
  useCreateQuote,
} from '../api/queries';
import type { Deal, Kit } from '../api/types';
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
  // Appsmith parity: service categories for the standard create flow.
  // Matches `get_product_category` (excludes IaaS/VCloud category ids 12,13,14,15).
  const { data: categories } = useCategories(true);
  // Appsmith parity: template list depends on document_type. Spot docs exclude
  // colocation templates; recurring docs allow both colo and non-colo templates
  // (see `TypeDocument.template_suServizio`).
  const templatesParams: { type: string; is_colo?: string } = { type: state.quoteType };
  if (state.quoteType === 'standard' && state.document_type === 'TSC-ORDINE') {
    templatesParams.is_colo = 'false';
  }
  const { data: templates } = useTemplates(templatesParams);
  // Appsmith parity: the `mst_kit` multi-select in "Nuova Proposta" is populated
  // from `list_kit`, grouped by category. Selected kit ids are inserted as
  // quote rows right after `ins_quote` (see `salvaOfferta`).
  const { data: kits } = useKits();
  const createQuote = useCreateQuote();
  const [kitSearch, setKitSearch] = useState('');

  // Appsmith parity: detect COLOCATION service selection via category name.
  // `Service.ServiceChange()` checks `sl_services.selectedOptionLabels.includes('COLOCATION')`.
  const selectedServiceIds = useMemo(
    () => state.services.split(',').map(s => s.trim()).filter(Boolean),
    [state.services]
  );
  const colocationSelected = useMemo(() => {
    if (!categories) return false;
    return categories.some(
      c => selectedServiceIds.includes(String(c.id)) && c.name.toUpperCase() === 'COLOCATION'
    );
  }, [categories, selectedServiceIds]);
  // Appsmith parity: COLOCATION + TSC-ORDINE-RIC → force trimestral billing (3)
  // and disable the billing selector.
  const billingLocked = colocationSelected && state.document_type === 'TSC-ORDINE-RIC';

  // ERP customer-specific default payment lookup (Appsmith parity: metodoPagDefault)
  const selectedCustomerId = state.selectedDeal?.company_id != null
    ? String(state.selectedDeal.company_id)
    : null;
  const customerPaymentQuery = useCustomerPayment(selectedCustomerId);
  const customerOrdersQuery = useCustomerOrders(selectedCustomerId);
  const customerOrders = customerOrdersQuery.data ?? [];

  // beforeunload protection
  useEffect(() => {
    const handler = (e: BeforeUnloadEvent) => { e.preventDefault(); };
    window.addEventListener('beforeunload', handler);
    return () => window.removeEventListener('beforeunload', handler);
  }, []);

  // Appsmith parity: while COLOCATION is selected for a recurring document,
  // force bill_months to 3 (Trimestrale). When the condition clears, keep the
  // existing value — editability is restored via `billingLocked` on the select.
  useEffect(() => {
    if (billingLocked) {
      setState(prev => (prev.bill_months === 3 ? prev : { ...prev, bill_months: 3 }));
    }
  }, [billingLocked]);

  // Appsmith parity: if the currently selected template disappears from the
  // allowed list (because document_type changed), drop it so the user is
  // forced to pick a still-valid option instead of silently keeping a bad one.
  useEffect(() => {
    if (!state.template || !templates) return;
    if (!templates.some(t => t.template_id === state.template)) {
      setState(prev => ({ ...prev, template: '' }));
    }
  }, [templates, state.template]);

  // When the selected customer resolves, apply the ERP default payment code.
  // Fallback to '402' if the lookup has no value or fails, matching Appsmith.
  useEffect(() => {
    if (!selectedCustomerId) return;
    if (customerPaymentQuery.isPending) return;
    const resolved = customerPaymentQuery.data?.payment_code;
    const nextCode = resolved && resolved !== '' ? resolved : '402';
    setState(prev => (prev.payment_method === nextCode ? prev : { ...prev, payment_method: nextCode }));
  }, [selectedCustomerId, customerPaymentQuery.isPending, customerPaymentQuery.data, customerPaymentQuery.isError]);

  // Appsmith parity: filter kit tree with a single text box (same affordance
  // as `KitPickerModal` in the detail page), then group by category so the
  // user sees the same shape `treeOfKits` produced.
  const filteredKits = useMemo<Kit[]>(() => {
    if (!kits) return [];
    if (!kitSearch) return kits;
    const needle = kitSearch.toLowerCase();
    return kits.filter(k =>
      k.internal_name.toLowerCase().includes(needle) ||
      (k.category_name?.toLowerCase().includes(needle) ?? false)
    );
  }, [kits, kitSearch]);

  const groupedKits = useMemo<[string, Kit[]][]>(() => {
    const map = new Map<string, Kit[]>();
    for (const k of filteredKits) {
      const cat = k.category_name ?? 'Altro';
      const list = map.get(cat) ?? [];
      list.push(k);
      map.set(cat, list);
    }
    return Array.from(map.entries());
  }, [filteredKits]);

  const selectedKits = useMemo<Kit[]>(() => {
    if (!kits || state.kit_ids.length === 0) return [];
    const chosen = new Set(state.kit_ids);
    return kits.filter(k => chosen.has(k.id));
  }, [kits, state.kit_ids]);

  const toggleKit = useCallback((kitId: number) => {
    setState(prev => {
      const exists = prev.kit_ids.includes(kitId);
      const next = exists
        ? prev.kit_ids.filter(id => id !== kitId)
        : [...prev.kit_ids, kitId];
      return { ...prev, kit_ids: next };
    });
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
                {state.proposal_type === 'SOSTITUZIONE' && (
                  <div className={styles.field}>
                    <label className={styles.label}>Ordini da sostituire</label>
                    <select
                      className={styles.input}
                      multiple
                      size={Math.min(6, Math.max(3, customerOrders.length))}
                      value={state.replace_orders.split(';').map(s => s.trim()).filter(Boolean)}
                      disabled={customerOrdersQuery.isPending || customerOrders.length === 0}
                      onChange={e => {
                        const chosen = Array.from(e.target.selectedOptions, o => o.value);
                        update('replace_orders', chosen.join(';'));
                      }}
                    >
                      {customerOrders.map(o => (
                        <option key={o.name} value={o.name}>{o.name}</option>
                      ))}
                    </select>
                    {!customerOrdersQuery.isPending && customerOrders.length === 0 && (
                      <div style={{ fontSize: '0.75rem', color: '#64748b', marginTop: '0.25rem' }}>
                        Nessun ordine disponibile per il cliente.
                      </div>
                    )}
                  </div>
                )}
                <div className={styles.field}>
                  <label className={styles.label}>Owner</label>
                  <select className={styles.select} value={state.owner} onChange={e => update('owner', e.target.value)}>
                    <option value="">— Seleziona —</option>
                    {owners?.map(o => (
                      <option key={o.id} value={o.id}>{[o.firstname, o.lastname].filter(Boolean).join(' ')}</option>
                    ))}
                  </select>
                </div>
                {state.quoteType === 'standard' && (
                  <div className={styles.field}>
                    <label className={styles.label}>Servizi</label>
                    <select
                      className={styles.input}
                      multiple
                      size={Math.min(6, Math.max(3, categories?.length ?? 3))}
                      value={selectedServiceIds}
                      onChange={e => {
                        const chosen = Array.from(e.target.selectedOptions, o => o.value);
                        update('services', chosen.join(','));
                      }}
                    >
                      {categories?.map(c => (
                        <option key={c.id} value={String(c.id)}>{c.name}</option>
                      ))}
                    </select>
                  </div>
                )}
                <div className={styles.field}>
                  <label className={styles.label}>Template</label>
                  <select className={styles.select} value={state.template} onChange={e => update('template', e.target.value)}>
                    <option value="">— Seleziona —</option>
                    {templates?.map(t => (
                      <option key={t.template_id} value={t.template_id}>{t.description}</option>
                    ))}
                  </select>
                  {templates && templates.length === 0 && (
                    <div style={{ fontSize: '0.75rem', color: '#64748b', marginTop: '0.25rem' }}>
                      Nessun template disponibile per la combinazione selezionata.
                    </div>
                  )}
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
                  <label className={styles.label}>Fatturazione canoni</label>
                  <select
                    className={styles.select}
                    value={String(state.bill_months)}
                    disabled={billingLocked}
                    onChange={e => update('bill_months', Number(e.target.value))}
                  >
                    <option value="1">Mensile (1)</option>
                    <option value="2">Bimestrale (2)</option>
                    <option value="3">Trimestrale (3)</option>
                    <option value="6">Semestrale (6)</option>
                    <option value="12">Annuale (12)</option>
                  </select>
                  {billingLocked && (
                    <div style={{ fontSize: '0.75rem', color: '#64748b', marginTop: '0.25rem' }}>
                      COLOCATION ricorrente: fatturazione trimestrale obbligatoria.
                    </div>
                  )}
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
            <p style={{ color: '#64748b', fontSize: '0.8125rem', marginBottom: '0.75rem' }}>
              Seleziona uno o piu kit da inserire nella proposta. I kit scelti diventeranno le righe iniziali.
            </p>
            <div className={styles.field}>
              <input
                className={styles.input}
                placeholder="Cerca kit..."
                value={kitSearch}
                onChange={e => setKitSearch(e.target.value)}
              />
            </div>
            <div className={styles.kitList}>
              {groupedKits.length === 0 && (
                <div style={{ fontSize: '0.75rem', color: '#64748b' }}>
                  Nessun kit disponibile.
                </div>
              )}
              {groupedKits.map(([cat, items]) => (
                <div key={cat} className={styles.kitGroup}>
                  <div className={styles.kitGroupTitle}>{cat}</div>
                  {items.map(k => {
                    const checked = state.kit_ids.includes(k.id);
                    return (
                      <label key={k.id} className={styles.kitRow}>
                        <input
                          type="checkbox"
                          className={styles.radioInput}
                          checked={checked}
                          onChange={() => toggleKit(k.id)}
                        />
                        <span className={styles.kitName}>{k.internal_name}</span>
                        <span className={styles.kitPrice}>
                          NRC {k.nrc.toFixed(2)} / MRC {k.mrc.toFixed(2)}
                        </span>
                      </label>
                    );
                  })}
                </div>
              ))}
            </div>
            <div style={{ marginTop: '0.75rem', fontSize: '0.75rem', color: '#64748b' }}>
              {state.kit_ids.length === 0
                ? 'Nessun kit selezionato.'
                : `${state.kit_ids.length} kit selezionati.`}
            </div>
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
            <div className={styles.summaryRow}>
              <span className={styles.summaryLabel}>Kit selezionati</span>
              <span className={styles.summaryValue}>
                {selectedKits.length === 0
                  ? 'Nessuno'
                  : selectedKits.map(k => k.internal_name).join(', ')}
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
