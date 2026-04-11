import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Button, Icon, MultiSelect, SingleSelect, SearchInput } from '@mrsmith/ui';
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
import { CollapsibleSection } from '../components/CollapsibleSection';
import { ContactCard, type ContactFields } from '../components/ContactCard';
import { RichTextEditor } from '../components/RichTextEditor';
import { KitPickerModal } from '../components/KitPickerModal';
import { TrialSlider } from '../components/TrialSlider';
import { buildIaaSTrialText, getLanguageCode, getIaaSTemplateRule } from '../utils/quoteRules';
import { useOptionalAuth } from '../hooks/useOptionalAuth';
import styles from './QuoteCreatePage.module.css';

const stepNames = ['Deal', 'Configurazione', 'Kit', 'Extra', 'Riepilogo'];

const emptyContact: ContactFields = { name: '', tel: '', email: '' };

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
  rif_ordcli: string;
  contactTech: ContactFields;
  contactAltroTech: ContactFields;
  contactAdm: ContactFields;
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
  rif_ordcli: '',
  contactTech: { ...emptyContact },
  contactAltroTech: { ...emptyContact },
  contactAdm: { ...emptyContact },
};

function formatCurrency(value: number): string {
  return value.toLocaleString('it-IT', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
}

function truncateHtml(html: string, max = 80): string {
  const text = html.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim();
  if (text.length === 0) return '';
  return text.length > max ? text.slice(0, max).trimEnd() + '…' : text;
}

function countFilledContacts(state: WizardState): number {
  const contacts = [state.contactTech, state.contactAltroTech, state.contactAdm];
  return contacts.filter(c => c.name || c.tel || c.email).length + (state.rif_ordcli ? 1 : 0);
}

export function QuoteCreatePage() {
  const navigate = useNavigate();
  const [step, setStep] = useState(0);
  const [state, setState] = useState<WizardState>(initialState);
  const [dealSearch, setDealSearch] = useState('');
  const [showKitPicker, setShowKitPicker] = useState(false);
  const [extraOpen, setExtraOpen] = useState<Record<string, boolean>>({
    description: false,
    legalNotes: false,
    contacts: false,
  });
  const shouldLoadKits =
    (state.quoteType === 'standard' && step >= 2) ||
    (state.quoteType === 'iaas' && state.template !== '');

  const { user } = useOptionalAuth();
  const { data: deals } = useDeals();
  const { data: owners } = useOwners();
  const { data: paymentMethods } = usePaymentMethods();
  const { data: categories } = useCategories({
    excludeIds: [12, 13],
    enabled: state.quoteType === 'standard',
  });
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
  const { data: kits, isPending: kitsPending } = useKits({ enabled: shouldLoadKits });
  const createQuote = useCreateQuote();
  const selectedTemplate = useMemo(
    () => templates?.find(t => t.template_id === state.template) ?? null,
    [state.template, templates],
  );
  const derivedIaaSRule = useMemo(
    () => getIaaSTemplateRule(state.template),
    [state.template],
  );
  const derivedIaaSKit = useMemo(
    () => kits?.find(k => k.id === (derivedIaaSRule?.kitId ?? selectedTemplate?.kit_id ?? -1)) ?? null,
    [derivedIaaSRule, kits, selectedTemplate],
  );

  const selectedServiceIds = useMemo(
    () => state.services.split(',').map(s => s.trim()).filter(Boolean),
    [state.services],
  );
  const colocationSelected = useMemo(() => {
    if (state.quoteType !== 'standard' || !categories) return false;
    return categories.some(
      c => selectedServiceIds.includes(String(c.id)) && c.name.toUpperCase() === 'COLOCATION',
    );
  }, [categories, selectedServiceIds, state.quoteType]);
  const customerLang: 'it' | 'en' = useMemo(() => {
    const raw = state.selectedDeal?.company_lingua;
    if (!raw) return 'it';
    return raw.trim().toLowerCase().startsWith('en') ? 'en' : 'it';
  }, [state.selectedDeal]);
  const billingLocked =
    state.quoteType === 'standard' &&
    colocationSelected &&
    state.document_type === 'TSC-ORDINE-RIC';

  const selectedCustomerId = state.selectedDeal?.company_id != null
    ? String(state.selectedDeal.company_id)
    : null;
  const customerPaymentQuery = useCustomerPayment(selectedCustomerId);
  const customerOrdersQuery = useCustomerOrders(selectedCustomerId);
  const customerOrders = customerOrdersQuery.data ?? [];

  useEffect(() => {
    const handler = (e: BeforeUnloadEvent) => { e.preventDefault(); };
    window.addEventListener('beforeunload', handler);
    return () => window.removeEventListener('beforeunload', handler);
  }, []);

  useEffect(() => {
    if (billingLocked) {
      setState(prev => (prev.bill_months === 3 ? prev : { ...prev, bill_months: 3 }));
    }
  }, [billingLocked]);

  useEffect(() => {
    if (!state.template || !templates) return;
    if (!templates.some(t => t.template_id === state.template)) {
      setState(prev => ({ ...prev, template: '' }));
    }
  }, [templates, state.template]);

  useEffect(() => {
    if (state.quoteType !== 'standard') return;
    if (!templates || templates.length === 0) return;

    if (colocationSelected) {
      const match = templates.find(
        t => t.is_colo === true && (t.lang ?? '').toLowerCase() === customerLang,
      );
      if (match && state.template !== match.template_id) {
        const current = templates.find(t => t.template_id === state.template);
        if (!current || current.is_colo === true) {
          setState(prev => ({ ...prev, template: match.template_id }));
        }
      }
    } else {
      const current = templates.find(t => t.template_id === state.template);
      if (current?.is_colo === true) {
        setState(prev => ({ ...prev, template: '' }));
      }
    }
  }, [colocationSelected, customerLang, templates, state.quoteType, state.template]);

  useEffect(() => {
    if (state.quoteType !== 'iaas') return;
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
    if (state.proposal_type === 'SOSTITUZIONE' || state.replace_orders === '') return;
    setState(prev => ({ ...prev, replace_orders: '' }));
  }, [state.proposal_type, state.replace_orders]);

  useEffect(() => {
    if (state.owner !== '' || !owners || !user?.email) return;
    const match = owners.find(o => o.email?.toLowerCase() === user.email?.toLowerCase());
    if (match) {
      setState(prev => (prev.owner === '' ? { ...prev, owner: String(match.id) } : prev));
    }
  }, [owners, user?.email, state.owner]);

  useEffect(() => {
    if (!selectedCustomerId) return;
    if (customerPaymentQuery.isPending) return;
    const resolved = customerPaymentQuery.data?.payment_code;
    const nextCode = resolved && resolved !== '' ? resolved : '402';
    setState(prev => (prev.payment_method === nextCode ? prev : { ...prev, payment_method: nextCode }));
  }, [selectedCustomerId, customerPaymentQuery.isPending, customerPaymentQuery.data, customerPaymentQuery.isError]);

  const selectedKits = useMemo<Kit[]>(() => {
    if (!kits || state.kit_ids.length === 0) return [];
    const order = new Map(state.kit_ids.map((id, i) => [id, i]));
    return kits
      .filter(k => order.has(k.id))
      .sort((a, b) => (order.get(a.id) ?? 0) - (order.get(b.id) ?? 0));
  }, [kits, state.kit_ids]);

  const totals = useMemo(() => {
    let nrc = 0;
    let mrc = 0;
    for (const k of selectedKits) {
      nrc += k.nrc;
      mrc += k.mrc;
    }
    return { nrc, mrc };
  }, [selectedKits]);

  const addKit = useCallback((kitId: number) => {
    setState(prev => (
      prev.kit_ids.includes(kitId)
        ? prev
        : { ...prev, kit_ids: [...prev.kit_ids, kitId] }
    ));
  }, []);

  const removeKit = useCallback((kitId: number) => {
    setState(prev => ({ ...prev, kit_ids: prev.kit_ids.filter(id => id !== kitId) }));
  }, []);

  const filteredDeals = useMemo(() => {
    if (!deals) return [];
    if (!dealSearch) return deals;
    const q = dealSearch.toLowerCase();
    return deals.filter(d =>
      d.name.toLowerCase().includes(q) ||
      (d.company_name?.toLowerCase().includes(q) ?? false),
    );
  }, [deals, dealSearch]);

  const update = useCallback(<K extends keyof WizardState>(key: K, value: WizardState[K]) => {
    setState(prev => ({ ...prev, [key]: value }));
  }, []);

  const updateContact = useCallback(
    (key: 'contactTech' | 'contactAltroTech' | 'contactAdm', patch: Partial<ContactFields>) => {
      setState(prev => ({ ...prev, [key]: { ...prev[key], ...patch } }));
    },
    [],
  );

  const toggleExtra = useCallback((key: string) => {
    setExtraOpen(prev => ({ ...prev, [key]: !prev[key] }));
  }, []);

  const canAdvance = useMemo(() => {
    switch (step) {
      case 0: return state.selectedDeal !== null;
      case 1: return state.template !== '' && state.owner !== '';
      case 2: return state.quoteType === 'iaas' || state.kit_ids.length > 0;
      case 3: return true;
      case 4: return true;
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
        rif_ordcli: state.rif_ordcli || null,
        rif_tech_nom: state.contactTech.name || null,
        rif_tech_tel: state.contactTech.tel || null,
        rif_tech_email: state.contactTech.email || null,
        rif_altro_tech_nom: state.contactAltroTech.name || null,
        rif_altro_tech_tel: state.contactAltroTech.tel || null,
        rif_altro_tech_email: state.contactAltroTech.email || null,
        rif_adm_nom: state.contactAdm.name || null,
        rif_adm_tech_tel: state.contactAdm.tel || null,
        rif_adm_tech_email: state.contactAdm.email || null,
      });
      navigate(`/quotes/${result.id}`);
    } catch {
      // Error handling via mutation
    }
  }, [state, createQuote, navigate]);

  const handleQuoteTypeChange = useCallback((quoteType: WizardState['quoteType']) => {
    setState(prev => {
      if (prev.quoteType === quoteType) return prev;
      if (quoteType === 'iaas') {
        return { ...prev, quoteType, services: '', bill_months: 1 };
      }
      return { ...prev, quoteType, services: '', trial: '' };
    });
  }, []);

  const handleNext = () => {
    if (step === 4) {
      void handleCreate();
    } else {
      setStep(s => s + 1);
    }
  };

  const descriptionSummary = truncateHtml(state.description) || 'Nessuna descrizione';
  const legalNotesSummary = truncateHtml(state.notes) || 'Nessuna pattuizione';
  const filledContactCount = countFilledContacts(state);
  const contactsSummary =
    filledContactCount === 0
      ? 'Nessun contatto'
      : `${filledContactCount} contatt${filledContactCount === 1 ? 'o' : 'i'} inserit${filledContactCount === 1 ? 'o' : 'i'}`;

  const templateLabel = selectedTemplate?.description ?? state.template ?? '—';
  const ownerLabel = useMemo(() => {
    if (!state.owner) return '—';
    const o = owners?.find(item => item.id === state.owner);
    if (!o) return state.owner;
    return [o.firstname, o.lastname].filter(Boolean).join(' ');
  }, [owners, state.owner]);
  const paymentLabel = useMemo(() => {
    if (!state.payment_method) return '—';
    return paymentMethods?.find(p => p.code === state.payment_method)?.description ?? state.payment_method;
  }, [paymentMethods, state.payment_method]);
  const servicesLabel = useMemo(() => {
    if (!categories || selectedServiceIds.length === 0) return '—';
    return selectedServiceIds
      .map(id => categories.find(c => String(c.id) === id)?.name ?? id)
      .join(', ');
  }, [categories, selectedServiceIds]);

  return (
    <div className={styles.page}>
      <header className={styles.pageHeader}>
        <h1 className={styles.title}>Nuova proposta</h1>
        {state.selectedDeal && (
          <p className={styles.subtitle}>
            <span className={styles.subtitleCode}>{state.selectedDeal.name}</span>
            <span>·</span>
            <span>{state.selectedDeal.company_name ?? '—'}</span>
          </p>
        )}
      </header>

      <Stepper steps={stepNames} current={step} onStepClick={s => setStep(s)} />

      <div className={styles.stepContent} key={step}>
        {step === 0 && (
          <div className={styles.stepInner}>
            <div className={styles.dealSearch}>
              <SearchInput
                value={dealSearch}
                onChange={setDealSearch}
                placeholder="Cerca deal per nome o cliente..."
                autoFocus
              />
            </div>
            {filteredDeals.length > 0 ? (
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
            ) : (
              <div className={styles.emptyBlock}>
                {dealSearch ? 'Nessun deal corrispondente alla ricerca.' : 'Nessun deal disponibile.'}
              </div>
            )}
          </div>
        )}

        {step === 1 && (
          <div className={styles.stepInner}>
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
                        <input
                          className={styles.radioInput}
                          type="radio"
                          checked={state.document_type === 'TSC-ORDINE-RIC'}
                          onChange={() => update('document_type', 'TSC-ORDINE-RIC')}
                        />
                        Ricorrente
                      </label>
                      <label className={styles.radioLabel}>
                        <input
                          className={styles.radioInput}
                          type="radio"
                          checked={state.document_type === 'TSC-ORDINE'}
                          onChange={() => update('document_type', 'TSC-ORDINE')}
                        />
                        Spot
                      </label>
                    </div>
                  )}
                </div>
                <div className={styles.field}>
                  <label className={styles.label}>Tipo proposta</label>
                  <div className={styles.radioGroup}>
                    {(['NUOVO', 'SOSTITUZIONE', 'RINNOVO'] as const).map(pt => (
                      <label key={pt} className={styles.radioLabel}>
                        <input
                          className={styles.radioInput}
                          type="radio"
                          checked={state.proposal_type === pt}
                          onChange={() => update('proposal_type', pt)}
                        />
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
                      <TrialSlider
                        value={state.trial_value}
                        onChange={v => update('trial_value', v)}
                        aria-label="Trial"
                      />
                      {state.trial && <div className={styles.hint}>{state.trial}</div>}
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
                  />
                </div>
                {state.quoteType === 'iaas' && (
                  <div className={styles.field}>
                    <label className={styles.label}>Valori derivati</label>
                    <div className={styles.readOnly}>
                      {derivedIaaSKit
                        ? `${derivedIaaSKit.internal_name} · servizi ${derivedIaaSRule?.services ?? '—'} · termini 1/1/1`
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
                      <div className={styles.hint}>
                        COLOCATION ricorrente: fatturazione trimestrale obbligatoria.
                      </div>
                    )}
                  </div>
                )}
                <div className={styles.field}>
                  <label className={styles.label}>Durata iniziale (mesi)</label>
                  <input
                    className={state.quoteType === 'iaas' ? styles.readOnly : styles.input}
                    type="number"
                    value={state.initial_term_months}
                    readOnly={state.quoteType === 'iaas'}
                    onChange={e => update('initial_term_months', Number(e.target.value))}
                  />
                </div>
                <div className={styles.field}>
                  <label className={styles.label}>Durata rinnovo (mesi)</label>
                  <input
                    className={state.quoteType === 'iaas' ? styles.readOnly : styles.input}
                    type="number"
                    value={state.next_term_months}
                    readOnly={state.quoteType === 'iaas'}
                    onChange={e => update('next_term_months', Number(e.target.value))}
                  />
                </div>
                <div className={styles.field}>
                  <label className={styles.label}>Consegna (giorni)</label>
                  <input
                    className={styles.input}
                    type="number"
                    value={state.delivered_in_days}
                    onChange={e => update('delivered_in_days', Number(e.target.value))}
                  />
                </div>
              </div>
            </div>
          </div>
        )}

        {step === 2 && (
          <div className={styles.stepInner}>
            {state.quoteType === 'iaas' ? (
              <div className={styles.section}>
                <div className={styles.sectionTitle}>Kit IaaS</div>
                <p className={styles.hint}>
                  Il kit iniziale viene derivato automaticamente dal template IaaS selezionato.
                </p>
                {derivedIaaSKit ? (
                  <div className={styles.kitCard}>
                    <div className={styles.kitCardLeft}>
                      <span className={styles.kitCardName}>{derivedIaaSKit.internal_name}</span>
                      {derivedIaaSKit.category_name && (
                        <span className={styles.kitCardCategory}>{derivedIaaSKit.category_name}</span>
                      )}
                    </div>
                    <div className={styles.kitCardTotals}>
                      <span>
                        <span className={styles.kitCardTotalLabel}>NRC</span>{' '}
                        {formatCurrency(derivedIaaSKit.nrc)}
                      </span>
                      <span>
                        <span className={styles.kitCardTotalLabel}>MRC</span>{' '}
                        {formatCurrency(derivedIaaSKit.mrc)}
                      </span>
                    </div>
                  </div>
                ) : (
                  <div className={styles.emptyBlock}>
                    Seleziona un template IaaS valido per generare la riga kit iniziale.
                  </div>
                )}
              </div>
            ) : (
              <div className={styles.kitStep}>
                <div className={styles.kitStepHeader}>
                  <div>
                    <div className={styles.sectionTitle}>Kit selezionati</div>
                    <p className={styles.hint}>
                      Aggiungi i kit che saranno le righe iniziali della proposta. Prodotti e
                      obbligatorietà si configurano dopo la creazione.
                    </p>
                  </div>
                  <Button
                    variant="secondary"
                    leftIcon={<Icon name="plus" size={16} />}
                    onClick={() => setShowKitPicker(true)}
                  >
                    Aggiungi kit
                  </Button>
                </div>

                {kitsPending && state.kit_ids.length === 0 && (
                  <div className={styles.hint}>Caricamento catalogo kit...</div>
                )}

                {selectedKits.length === 0 ? (
                  <div className={styles.emptyKits}>
                    <div className={styles.emptyKitsIcon}>
                      <Icon name="package" size={28} strokeWidth={1.5} />
                    </div>
                    <div className={styles.emptyKitsTitle}>Nessun kit selezionato</div>
                    <div className={styles.emptyKitsText}>
                      Apri il catalogo per aggiungere i primi kit alla proposta.
                    </div>
                  </div>
                ) : (
                  <div className={styles.kitList}>
                    {selectedKits.map(k => (
                      <div key={k.id} className={styles.kitCard}>
                        <div className={styles.kitCardLeft}>
                          <span className={styles.kitCardName}>{k.internal_name}</span>
                          {k.category_name && (
                            <span className={styles.kitCardCategory}>{k.category_name}</span>
                          )}
                        </div>
                        <div className={styles.kitCardTotals}>
                          <span>
                            <span className={styles.kitCardTotalLabel}>NRC</span>{' '}
                            {formatCurrency(k.nrc)}
                          </span>
                          <span>
                            <span className={styles.kitCardTotalLabel}>MRC</span>{' '}
                            {formatCurrency(k.mrc)}
                          </span>
                        </div>
                        <button
                          type="button"
                          className={styles.kitCardRemove}
                          onClick={() => removeKit(k.id)}
                          aria-label={`Rimuovi ${k.internal_name}`}
                        >
                          <Icon name="x" size={16} />
                        </button>
                      </div>
                    ))}
                  </div>
                )}

                {selectedKits.length > 0 && (
                  <div className={styles.kitTotals}>
                    <span>
                      <span className={styles.kitTotalsLabel}>NRC Totale</span>{' '}
                      {formatCurrency(totals.nrc)}
                    </span>
                    <span>
                      <span className={styles.kitTotalsLabel}>MRC Totale</span>{' '}
                      {formatCurrency(totals.mrc)}
                    </span>
                  </div>
                )}
              </div>
            )}
          </div>
        )}

        {step === 3 && (
          <div className={styles.stepInner}>
            <p className={styles.extraIntro}>
              Aggiungi descrizione, pattuizioni speciali e contatti di riferimento. Tutte le sezioni
              sono opzionali e possono essere modificate anche dopo la creazione.
            </p>
            <div className={styles.collapseList}>
              <CollapsibleSection
                title="Descrizione"
                summary={descriptionSummary}
                open={extraOpen.description === true}
                onToggle={() => toggleExtra('description')}
              >
                <RichTextEditor
                  value={state.description}
                  onChange={html => update('description', html)}
                  placeholder="Scrivi una breve descrizione della proposta..."
                />
              </CollapsibleSection>

              <CollapsibleSection
                title="Pattuizioni speciali"
                summary={legalNotesSummary}
                open={extraOpen.legalNotes === true}
                onToggle={() => toggleExtra('legalNotes')}
              >
                <div className={styles.approvalBanner}>
                  <Icon name="triangle-alert" size={16} />
                  <span>
                    Se inserisci pattuizioni speciali, la proposta richiederà l&apos;approvazione di un responsabile commerciale.
                  </span>
                </div>
                <RichTextEditor
                  value={state.notes}
                  onChange={html => update('notes', html)}
                  placeholder="Specifica eventuali pattuizioni fuori standard..."
                />
              </CollapsibleSection>

              <CollapsibleSection
                title="Contatti di riferimento"
                summary={contactsSummary}
                open={extraOpen.contacts === true}
                onToggle={() => toggleExtra('contacts')}
              >
                <div className={styles.field}>
                  <label className={styles.label}>Riferimento ordine cliente</label>
                  <input
                    className={styles.input}
                    value={state.rif_ordcli}
                    onChange={e => update('rif_ordcli', e.target.value)}
                    placeholder="Es. PO 2026/0015"
                  />
                </div>
                <div className={styles.contactGrid}>
                  <ContactCard
                    title="Tecnico"
                    icon="settings"
                    value={state.contactTech}
                    onChange={patch => updateContact('contactTech', patch)}
                  />
                  <ContactCard
                    title="Altro tecnico"
                    icon="settings"
                    value={state.contactAltroTech}
                    onChange={patch => updateContact('contactAltroTech', patch)}
                  />
                  <ContactCard
                    title="Amministrativo"
                    icon="mail"
                    value={state.contactAdm}
                    onChange={patch => updateContact('contactAdm', patch)}
                  />
                </div>
              </CollapsibleSection>
            </div>
          </div>
        )}

        {step === 4 && (
          <div className={styles.stepInner}>
            <div className={styles.summaryCard}>
              <div className={styles.summarySection}>
                <div className={styles.summarySectionHeader}>
                  <span>Deal e cliente</span>
                  <button type="button" className={styles.editLink} onClick={() => setStep(0)}>
                    <Icon name="pencil" size={14} />
                    Modifica
                  </button>
                </div>
                <div className={styles.summaryGrid}>
                  <div>
                    <div className={styles.summaryKey}>Deal</div>
                    <div className={styles.summaryVal}>{state.selectedDeal?.name ?? '—'}</div>
                  </div>
                  <div>
                    <div className={styles.summaryKey}>Cliente</div>
                    <div className={styles.summaryVal}>{state.selectedDeal?.company_name ?? '—'}</div>
                  </div>
                </div>
              </div>

              <div className={styles.summarySection}>
                <div className={styles.summarySectionHeader}>
                  <span>Configurazione</span>
                  <button type="button" className={styles.editLink} onClick={() => setStep(1)}>
                    <Icon name="pencil" size={14} />
                    Modifica
                  </button>
                </div>
                <div className={styles.summaryGrid}>
                  <div>
                    <div className={styles.summaryKey}>Tipo</div>
                    <div className={styles.summaryVal}>
                      {state.quoteType === 'iaas' ? 'IaaS' : 'Standard'} · {state.document_type === 'TSC-ORDINE-RIC' ? 'Ricorrente' : 'Spot'} · {state.proposal_type}
                    </div>
                  </div>
                  <div>
                    <div className={styles.summaryKey}>Template</div>
                    <div className={styles.summaryVal}>{templateLabel}</div>
                  </div>
                  <div>
                    <div className={styles.summaryKey}>Owner</div>
                    <div className={styles.summaryVal}>{ownerLabel}</div>
                  </div>
                  <div>
                    <div className={styles.summaryKey}>Pagamento</div>
                    <div className={styles.summaryVal}>{paymentLabel}</div>
                  </div>
                  {state.quoteType === 'standard' && (
                    <div className={styles.summaryColSpan}>
                      <div className={styles.summaryKey}>Servizi</div>
                      <div className={styles.summaryVal}>{servicesLabel}</div>
                    </div>
                  )}
                  <div>
                    <div className={styles.summaryKey}>Termini</div>
                    <div className={styles.summaryVal}>
                      {state.initial_term_months}m iniziale · {state.next_term_months}m rinnovo · bill {state.bill_months}
                    </div>
                  </div>
                  <div>
                    <div className={styles.summaryKey}>Consegna</div>
                    <div className={styles.summaryVal}>{state.delivered_in_days} giorni</div>
                  </div>
                  {state.trial && (
                    <div className={styles.summaryColSpan}>
                      <div className={styles.summaryKey}>Trial</div>
                      <div className={styles.summaryVal}>{state.trial}</div>
                    </div>
                  )}
                </div>
              </div>

              <div className={styles.summarySection}>
                <div className={styles.summarySectionHeader}>
                  <span>Kit</span>
                  <button type="button" className={styles.editLink} onClick={() => setStep(2)}>
                    <Icon name="pencil" size={14} />
                    Modifica
                  </button>
                </div>
                {selectedKits.length === 0 && state.quoteType !== 'iaas' ? (
                  <div className={styles.warningBanner}>
                    <Icon name="triangle-alert" size={16} />
                    <span>Nessun kit selezionato — la proposta verrà creata vuota.</span>
                  </div>
                ) : (
                  <ul className={styles.kitSummaryList}>
                    {(state.quoteType === 'iaas' && derivedIaaSKit
                      ? [derivedIaaSKit]
                      : selectedKits
                    ).map(k => (
                      <li key={k.id} className={styles.kitSummaryRow}>
                        <span className={styles.kitSummaryName}>{k.internal_name}</span>
                        <span className={styles.kitSummaryPrice}>
                          NRC {formatCurrency(k.nrc)} · MRC {formatCurrency(k.mrc)}
                        </span>
                      </li>
                    ))}
                  </ul>
                )}
              </div>

              <div className={styles.summarySection}>
                <div className={styles.summarySectionHeader}>
                  <span>Note e contatti</span>
                  <button type="button" className={styles.editLink} onClick={() => setStep(3)}>
                    <Icon name="pencil" size={14} />
                    Modifica
                  </button>
                </div>
                <div className={styles.summaryList}>
                  <div className={styles.summaryListRow}>
                    <span className={styles.summaryKey}>Descrizione</span>
                    <span className={styles.summaryVal}>{descriptionSummary}</span>
                  </div>
                  <div className={styles.summaryListRow}>
                    <span className={styles.summaryKey}>Pattuizioni speciali</span>
                    <span className={styles.summaryVal}>{legalNotesSummary}</span>
                  </div>
                  <div className={styles.summaryListRow}>
                    <span className={styles.summaryKey}>Contatti</span>
                    <span className={styles.summaryVal}>{contactsSummary}</span>
                  </div>
                </div>
              </div>

              <div className={styles.totalsBar}>
                <div className={styles.totalsItem}>
                  <span className={styles.totalsLabel}>NRC Totale</span>
                  <span className={styles.totalsValue}>{formatCurrency(totals.nrc)}</span>
                </div>
                <div className={styles.totalsDivider} aria-hidden="true" />
                <div className={styles.totalsItem}>
                  <span className={styles.totalsLabel}>MRC Totale</span>
                  <span className={styles.totalsValue}>{formatCurrency(totals.mrc)}</span>
                </div>
              </div>
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

      {state.quoteType === 'standard' && (
        <KitPickerModal
          open={showKitPicker}
          onSelect={addKit}
          onClose={() => setShowKitPicker(false)}
        />
      )}
    </div>
  );
}
