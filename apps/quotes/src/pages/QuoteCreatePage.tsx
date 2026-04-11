import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { MultiSelect, SingleSelect } from '@mrsmith/ui';
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
import { buildIaaSTrialText, getLanguageCode, getIaaSTemplateRule } from '../utils/quoteRules';
import styles from './QuoteCreatePage.module.css';

const stepNames = ['Deal', 'Configurazione', 'Kit e Prodotti', 'Extra', 'Riepilogo'];

interface WizardState {
  selectedDeal: Deal | null;
  quoteType: 'standard' | 'iaas';
  iaasLanguage: 'ITA' | 'ENG';
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
  trial: string;
  trial_value: number;
  kit_ids: number[];
}

const initialState: WizardState = {
  selectedDeal: null,
  quoteType: 'standard',
  iaasLanguage: 'ITA',
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
  trial: '',
  trial_value: 0,
  kit_ids: [],
};

export function QuoteCreatePage() {
  const navigate = useNavigate();
  const [step, setStep] = useState(0);
  const [state, setState] = useState<WizardState>(initialState);
  const [dealSearch, setDealSearch] = useState('');
  const shouldLoadKits =
    (state.quoteType === 'standard' && step >= 2) ||
    (state.quoteType === 'iaas' && state.template !== '');

  const { data: deals } = useDeals();
  const { data: owners } = useOwners();
  const { data: paymentMethods } = usePaymentMethods();
  // Appsmith parity: standard create uses the Nuova Proposta service list,
  // which excludes only categories 12 and 13.
  const { data: categories } = useCategories({
    excludeIds: [12, 13],
    enabled: state.quoteType === 'standard',
  });
  // Appsmith parity: template list depends on document_type. Spot docs exclude
  // colocation templates; recurring docs allow both colo and non-colo templates
  // (see `TypeDocument.template_suServizio`).
  const templatesParams = useMemo<{ type: string; lang?: string; is_colo?: string }>(() => {
    const params: { type: string; lang?: string; is_colo?: string } = { type: state.quoteType };
    if (state.quoteType === 'iaas') {
      params.lang = getLanguageCode(state.iaasLanguage);
    }
    if (state.quoteType === 'standard' && state.document_type === 'TSC-ORDINE') {
      params.is_colo = 'false';
    }
    return params;
  }, [state.document_type, state.iaasLanguage, state.quoteType]);
  const { data: templates } = useTemplates(templatesParams);
  // Appsmith parity: the `mst_kit` multi-select in "Nuova Proposta" is populated
  // from `list_kit`, grouped by category. Selected kit ids are inserted as
  // quote rows right after `ins_quote` (see `salvaOfferta`).
  const { data: kits, isPending: kitsPending } = useKits({ enabled: shouldLoadKits });
  const createQuote = useCreateQuote();
  const [kitSearch, setKitSearch] = useState('');
  const selectedTemplate = useMemo(
    () => templates?.find(template => template.template_id === state.template) ?? null,
    [state.template, templates]
  );
  const derivedIaaSRule = useMemo(
    () => getIaaSTemplateRule(state.template),
    [state.template]
  );
  const derivedIaaSKit = useMemo(
    () => kits?.find(kit => kit.id === (derivedIaaSRule?.kitId ?? selectedTemplate?.kit_id ?? -1)) ?? null,
    [derivedIaaSRule, kits, selectedTemplate]
  );

  // Appsmith parity: detect COLOCATION service selection via category name.
  // `Service.ServiceChange()` checks `sl_services.selectedOptionLabels.includes('COLOCATION')`.
  const selectedServiceIds = useMemo(
    () => state.services.split(',').map(s => s.trim()).filter(Boolean),
    [state.services]
  );
  const colocationSelected = useMemo(() => {
    if (state.quoteType !== 'standard' || !categories) return false;
    return categories.some(
      c => selectedServiceIds.includes(String(c.id)) && c.name.toUpperCase() === 'COLOCATION'
    );
  }, [categories, selectedServiceIds, state.quoteType]);
  // Appsmith parity: COLOCATION + TSC-ORDINE-RIC → force trimestral billing (3)
  // and disable the billing selector.
  const billingLocked =
    state.quoteType === 'standard' &&
    colocationSelected &&
    state.document_type === 'TSC-ORDINE-RIC';

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

  useEffect(() => {
    if (state.quoteType !== 'iaas') {
      return;
    }
    setState(prev => {
      const nextTemplate = prev.template;
      const nextRule = getIaaSTemplateRule(nextTemplate);
      const nextTrial = buildIaaSTrialText(prev.trial_value, getLanguageCode(prev.iaasLanguage));
      return {
        ...prev,
        document_type: 'TSC-ORDINE-RIC',
        services: nextRule?.services ?? '',
        initial_term_months: 1,
        next_term_months: 1,
        bill_months: 1,
        kit_ids: nextRule ? [nextRule.kitId] : [],
        trial: nextTrial,
      };
    });
  }, [state.iaasLanguage, state.quoteType, state.template, state.trial_value]);

  useEffect(() => {
    if (state.proposal_type === 'SOSTITUZIONE' || state.replace_orders === '') {
      return;
    }
    setState(prev => ({ ...prev, replace_orders: '' }));
  }, [state.proposal_type, state.replace_orders]);

  useEffect(() => {
    if (step === 2 || kitSearch === '') {
      return;
    }
    setKitSearch('');
  }, [kitSearch, step]);

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

  const selectedKitsSummary = useMemo(() => {
    if (state.kit_ids.length === 0) {
      return 'Nessuno';
    }
    if (kitsPending && selectedKits.length === 0) {
      return `${state.kit_ids.length} kit selezionati (caricamento...)`;
    }
    if (selectedKits.length === 0) {
      return `${state.kit_ids.length} kit selezionati`;
    }
    return selectedKits.map(k => k.internal_name).join(', ');
  }, [kitsPending, selectedKits, state.kit_ids.length]);

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
        replace_orders: state.proposal_type === 'SOSTITUZIONE' ? state.replace_orders : '',
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
        trial: state.trial,
        status: 'DRAFT',
        kit_ids: state.kit_ids,
      });
      navigate(`/quotes/${result.id}`);
    } catch {
      // Error handling via mutation
    }
  }, [state, createQuote, navigate]);

  const handleQuoteTypeChange = useCallback((quoteType: WizardState['quoteType']) => {
    setState(prev => {
      if (prev.quoteType === quoteType) {
        return prev;
      }
      if (quoteType === 'iaas') {
        return {
          ...prev,
          quoteType,
          services: '',
          bill_months: 1,
        };
      }
      return {
        ...prev,
        quoteType,
        services: '',
        trial: '',
      };
    });
  }, []);

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
            <TypeSelector value={state.quoteType} onChange={handleQuoteTypeChange} />
            <div className={styles.configGrid}>
              <div className={styles.section}>
                <div className={styles.sectionTitle}>Configurazione</div>
                <div className={styles.field}>
                  <label className={styles.label}>Tipo documento</label>
                  {state.quoteType === 'iaas' ? (
                    <input className={styles.readOnly} readOnly value="Ricorrente" />
                  ) : (
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
                  )}
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
                    <MultiSelect<string>
                      options={customerOrders.map(o => ({ value: o.name, label: o.name }))}
                      selected={state.replace_orders.split(';').map(s => s.trim()).filter(Boolean)}
                      onChange={chosen => update('replace_orders', chosen.join(';'))}
                      placeholder={
                        customerOrdersQuery.isPending
                          ? 'Caricamento ordini...'
                          : customerOrders.length === 0
                            ? 'Nessun ordine disponibile'
                            : 'Seleziona ordini...'
                      }
                    />
                  </div>
                )}
                <div className={styles.field}>
                  <label className={styles.label}>Owner</label>
                  <SingleSelect<string>
                    options={(owners ?? []).map(o => ({
                      value: String(o.id),
                      label: [o.firstname, o.lastname].filter(Boolean).join(' '),
                    }))}
                    selected={state.owner || null}
                    onChange={v => update('owner', v ?? '')}
                    placeholder="— Seleziona —"
                    allowClear
                  />
                </div>
                {state.quoteType === 'standard' && (
                  <div className={styles.field}>
                    <label className={styles.label}>Servizi</label>
                    <MultiSelect<number>
                      options={(categories ?? []).map(c => ({ value: c.id, label: c.name }))}
                      selected={selectedServiceIds.map(Number)}
                      onChange={chosen => update('services', chosen.join(','))}
                      placeholder="Seleziona servizi..."
                    />
                  </div>
                )}
                {state.quoteType === 'iaas' && (
                  <>
                    <div className={styles.field}>
                      <label className={styles.label}>Lingua cliente</label>
                      <div className={styles.radioGroup}>
                        {(['ITA', 'ENG'] as const).map(language => (
                          <label key={language} className={styles.radioLabel}>
                            <input
                              className={styles.radioInput}
                              type="radio"
                              checked={state.iaasLanguage === language}
                              onChange={() => update('iaasLanguage', language)}
                            />
                            {language}
                          </label>
                        ))}
                      </div>
                    </div>
                    <div className={styles.field}>
                      <label className={styles.label}>Trial</label>
                      <input
                        className={styles.input}
                        type="range"
                        min="0"
                        max="200"
                        step="10"
                        value={state.trial_value}
                        onChange={e => update('trial_value', Number(e.target.value))}
                      />
                      <div style={{ fontSize: '0.75rem', color: '#64748b', marginTop: '0.25rem' }}>
                        Trial gratuito: {state.trial_value}€
                      </div>
                      {state.trial && (
                        <div style={{ fontSize: '0.75rem', color: '#475569', marginTop: '0.5rem' }}>
                          {state.trial}
                        </div>
                      )}
                    </div>
                  </>
                )}
                <div className={styles.field}>
                  <label className={styles.label}>Template</label>
                  <SingleSelect<string>
                    options={(templates ?? []).map(t => ({
                      value: t.template_id,
                      label: t.description,
                    }))}
                    selected={state.template || null}
                    onChange={v => update('template', v ?? '')}
                    placeholder={
                      templates && templates.length === 0
                        ? 'Nessun template disponibile'
                        : '— Seleziona —'
                    }
                    allowClear
                  />
                </div>
                {state.quoteType === 'iaas' && (
                  <div className={styles.field}>
                    <label className={styles.label}>Valori derivati</label>
                    <div className={styles.readOnly}>
                      {derivedIaaSKit
                        ? `${derivedIaaSKit.internal_name} • servizi ${derivedIaaSRule?.services ?? '—'} • termini 1/1/1`
                        : 'Seleziona un template IaaS per derivare kit, servizi e termini.'}
                    </div>
                  </div>
                )}
              </div>

              <div className={styles.section}>
                <div className={styles.sectionTitle}>Condizioni</div>
                <div className={styles.field}>
                  <label className={styles.label}>Pagamento</label>
                  <SingleSelect<string>
                    options={(paymentMethods ?? []).map(pm => ({
                      value: pm.code,
                      label: pm.description,
                    }))}
                    selected={state.payment_method || null}
                    onChange={v => update('payment_method', v ?? '')}
                    placeholder="— Seleziona —"
                  />
                </div>
                {state.quoteType === 'iaas' ? (
                  <div className={styles.field}>
                    <label className={styles.label}>Fatturazione canoni</label>
                    <input className={styles.readOnly} readOnly value="Mensile (1)" />
                  </div>
                ) : (
                  <div className={styles.field}>
                    <label className={styles.label}>Fatturazione canoni</label>
                    {billingLocked ? (
                      <input className={styles.readOnly} readOnly value="Trimestrale (3)" />
                    ) : (
                      <SingleSelect<number>
                        options={[
                          { value: 1, label: 'Mensile (1)' },
                          { value: 2, label: 'Bimestrale (2)' },
                          { value: 3, label: 'Trimestrale (3)' },
                          { value: 6, label: 'Semestrale (6)' },
                          { value: 12, label: 'Annuale (12)' },
                        ]}
                        selected={state.bill_months}
                        onChange={v => update('bill_months', v ?? 1)}
                      />
                    )}
                    {billingLocked && (
                      <div style={{ fontSize: '0.75rem', color: '#64748b', marginTop: '0.25rem' }}>
                        COLOCATION ricorrente: fatturazione trimestrale obbligatoria.
                      </div>
                    )}
                  </div>
                )}
                <div className={styles.field}>
                  <label className={styles.label}>Durata iniziale (mesi)</label>
                  <input className={state.quoteType === 'iaas' ? styles.readOnly : styles.input} type="number" value={state.initial_term_months}
                    readOnly={state.quoteType === 'iaas'}
                    onChange={e => update('initial_term_months', Number(e.target.value))} />
                </div>
                <div className={styles.field}>
                  <label className={styles.label}>Durata rinnovo (mesi)</label>
                  <input className={state.quoteType === 'iaas' ? styles.readOnly : styles.input} type="number" value={state.next_term_months}
                    readOnly={state.quoteType === 'iaas'}
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
            {state.quoteType === 'iaas' ? (
              <>
                <p style={{ color: '#64748b', fontSize: '0.8125rem', marginBottom: '0.75rem' }}>
                  Il kit iniziale viene derivato automaticamente dal template IaaS selezionato.
                </p>
                <div className={styles.readOnly}>
                  {derivedIaaSKit
                    ? `${derivedIaaSKit.internal_name} • NRC ${derivedIaaSKit.nrc.toFixed(2)} / MRC ${derivedIaaSKit.mrc.toFixed(2)}`
                    : 'Seleziona un template IaaS valido per generare la riga kit iniziale.'}
                </div>
              </>
            ) : (
              <>
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
                              className={styles.checkInput}
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
              </>
            )}
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
                {selectedKitsSummary}
              </span>
            </div>
            {state.trial && (
              <div className={styles.summaryRow}>
                <span className={styles.summaryLabel}>Trial</span>
                <span className={styles.summaryValue}>{state.trial}</span>
              </div>
            )}
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
