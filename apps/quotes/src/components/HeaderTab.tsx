import { useMemo } from 'react';
import { Icon, MultiSelect, SingleSelect } from '@mrsmith/ui';
import type { Quote } from '../api/types';
import {
  useCategories,
  useCustomerOrders,
  useOwners,
  usePaymentMethods,
  useTemplates,
} from '../api/queries';
import { parseReplaceOrders, parseServiceCategoryIds } from '../utils/quoteRules';
import { SegmentedControl } from './SegmentedControl';
import styles from './HeaderTab.module.css';

interface HeaderTabProps {
  quote: Quote;
  onChange: (field: string, value: string | number) => void;
}

export function HeaderTab({ quote, onChange }: HeaderTabProps) {
  const { data: owners } = useOwners();
  const { data: paymentMethods } = usePaymentMethods();
  const categoriesQuery = useCategories();
  const templatesQuery = useTemplates();
  const customerIdStr = quote.customer_id != null ? String(quote.customer_id) : null;
  const customerOrdersQuery = useCustomerOrders(customerIdStr);
  const customerOrders = customerOrdersQuery.data ?? [];

  const selectedOrders = parseReplaceOrders(quote.replace_orders);
  const selectedServiceIds = useMemo(
    () => parseServiceCategoryIds(quote.services).map(Number),
    [quote.services],
  );
  const selectedTemplate = useMemo(
    () => (templatesQuery.data ?? []).find(t => t.template_id === quote.template) ?? null,
    [templatesQuery.data, quote.template],
  );

  const isIaaS = selectedTemplate?.template_type === 'iaas';
  const isSpot = quote.document_type === 'TSC-ORDINE';
  const billingLocked = isIaaS;

  const ownerOptions = useMemo(
    () =>
      (owners ?? []).map(o => ({
        value: String(o.id),
        label: [o.firstname, o.lastname].filter(Boolean).join(' '),
      })),
    [owners],
  );

  const paymentOptions = useMemo(
    () =>
      (paymentMethods ?? []).map(pm => ({ value: pm.code, label: pm.description })),
    [paymentMethods],
  );

  const orderOptions = useMemo(() => {
    const opts = customerOrders.map(o => ({ value: o.name, label: o.name }));
    const knownValues = new Set(opts.map(o => o.value));
    for (const legacy of selectedOrders) {
      if (!knownValues.has(legacy)) {
        opts.push({ value: legacy, label: `${legacy} (legacy)` });
      }
    }
    return opts;
  }, [customerOrders, selectedOrders]);

  const categoryOptions = useMemo(
    () => (categoriesQuery.data ?? []).map(c => ({ value: c.id, label: c.name })),
    [categoriesQuery.data],
  );

  const templateOptions = useMemo(
    () =>
      (templatesQuery.data ?? []).map(t => ({
        value: t.template_id,
        label: t.description,
      })),
    [templatesQuery.data],
  );
  const billMonthsValue = useMemo(() => {
    const raw = String(quote.bill_months);
    if (raw === '1' || raw === '2' || raw === '3' || raw === '6' || raw === '12') return raw;
    return '1';
  }, [quote.bill_months]);

  const dealLabel = useMemo(() => {
    if (quote.deal_number) {
      return `Deal ${quote.deal_number}`;
    }
    return 'Deal';
  }, [quote.deal_number]);

  return (
    <div className={styles.grid}>
      {/* Deal e Proprieta */}
      <div className={styles.section}>
        <div className={styles.sectionTitle}>Deal e Proprieta</div>
        <div className={styles.field}>
          <label className={styles.label}>{dealLabel}</label>
          <input className={styles.readOnly} readOnly value={quote.deal_name ?? '—'} />
        </div>
        <div className={styles.field}>
          <label className={styles.label}>Cliente</label>
          <input className={styles.readOnly} readOnly value={quote.customer_name ?? '—'} />
        </div>
        <div className={styles.field}>
          <label className={styles.label}>Owner</label>
          <SingleSelect<string>
            options={ownerOptions}
            selected={quote.owner || null}
            onChange={v => onChange('owner', v ?? '')}
            placeholder="— Seleziona —"
            allowClear
          />
        </div>
      </div>

      {/* Tipo Proposta */}
      <div className={styles.section}>
        <div className={styles.sectionTitle}>Tipo Proposta</div>
        <div className={styles.field}>
          <label className={styles.label}>Tipo documento</label>
          <div className={styles.radioGroup}>
            <label className={styles.radioLabel}>
              <input
                className={styles.radioInput}
                type="radio" name="document_type" value="TSC-ORDINE-RIC"
                checked={quote.document_type === 'TSC-ORDINE-RIC'}
                onChange={() => onChange('document_type', 'TSC-ORDINE-RIC')}
              />
              Ricorrente
            </label>
            <label className={styles.radioLabel}>
              <input
                className={styles.radioInput}
                type="radio" name="document_type" value="TSC-ORDINE"
                checked={isSpot}
                onChange={() => onChange('document_type', 'TSC-ORDINE')}
              />
              Spot
            </label>
          </div>
        </div>
        <div className={styles.field}>
          <label className={styles.label}>Tipo proposta</label>
          <div className={styles.radioGroup}>
            {(['NUOVO', 'SOSTITUZIONE', 'RINNOVO'] as const).map(pt => (
              <label key={pt} className={styles.radioLabel}>
                <input
                  className={styles.radioInput}
                  type="radio" name="proposal_type" value={pt}
                  checked={quote.proposal_type === pt}
                  onChange={() => onChange('proposal_type', pt)}
                />
                {pt === 'NUOVO' ? 'Nuovo' : pt === 'SOSTITUZIONE' ? 'Sostituzione' : 'Rinnovo'}
              </label>
            ))}
          </div>
        </div>
        {quote.proposal_type === 'SOSTITUZIONE' && (
          <div className={`${styles.field} ${styles.revealEnter}`}>
            <label className={styles.label}>Ordini da sostituire</label>
            <MultiSelect<string>
              options={orderOptions}
              selected={selectedOrders}
              onChange={chosen => onChange('replace_orders', chosen.join(';'))}
              placeholder={
                customerOrdersQuery.isPending
                  ? 'Caricamento ordini...'
                  : customerOrders.length === 0 && selectedOrders.length === 0
                    ? 'Nessun ordine disponibile'
                    : 'Seleziona ordini...'
              }
            />
            {!customerOrdersQuery.isPending && customerOrders.length === 0 && selectedOrders.length === 0 && (
              <div className={styles.emptyHint}>Nessun ordine disponibile per il cliente.</div>
            )}
          </div>
        )}
      </div>

      {/* Condizioni Commerciali */}
      <div className={styles.section}>
        <div className={styles.sectionTitle}>Condizioni Commerciali</div>
        {isIaaS && (
          <div className={styles.infoPill}>
            <Icon name="info" size={14} />
            <span>Valori fissi per offerte IaaS/VCloud — durata e rinnovo non modificabili.</span>
          </div>
        )}
        <div className={styles.field}>
          <label className={styles.label}>Metodo di pagamento</label>
          <SingleSelect<string>
            options={paymentOptions}
            selected={quote.payment_method || null}
            onChange={v => onChange('payment_method', v ?? '')}
            placeholder="— Seleziona —"
            allowClear
          />
        </div>
        <div className={styles.field}>
          <label className={styles.label}>Frequenza di fatturazione</label>
          <SegmentedControl<'1' | '2' | '3' | '6' | '12'>
            value={billMonthsValue}
            onChange={v => onChange('bill_months', Number(v))}
            options={[
              { value: '1', label: 'Mensile', disabled: billingLocked },
              { value: '2', label: 'Bimestrale', disabled: billingLocked },
              { value: '3', label: 'Trimestrale', disabled: billingLocked },
              { value: '6', label: 'Semestrale', disabled: billingLocked },
              { value: '12', label: 'Annuale', disabled: billingLocked },
            ]}
            aria-label="Frequenza di fatturazione"
            size="sm"
          />
          {isIaaS && (
            <div className={styles.emptyHint}>Frequenza derivata dal template IaaS/VCloud.</div>
          )}
        </div>
        <div className={styles.field}>
          <label className={styles.label}>Durata iniziale (mesi)</label>
          <input
            className={`${styles.input} ${isIaaS ? styles.disabled : ''}`}
            type="number" min={1}
            value={quote.initial_term_months}
            onChange={e => onChange('initial_term_months', Number(e.target.value))}
            disabled={isIaaS}
          />
        </div>
        <div className={styles.field}>
          <label className={styles.label}>Durata rinnovo (mesi)</label>
          <input
            className={`${styles.input} ${isIaaS ? styles.disabled : ''}`}
            type="number" min={1}
            value={quote.next_term_months}
            onChange={e => onChange('next_term_months', Number(e.target.value))}
            disabled={isIaaS}
          />
        </div>
        <div className={styles.field}>
          <label className={styles.label}>Tempi di consegna (giorni)</label>
          <input
            className={styles.input}
            type="number" min={0}
            value={quote.delivered_in_days}
            onChange={e => onChange('delivered_in_days', Number(e.target.value))}
          />
        </div>
        <div className={styles.field}>
          <label className={styles.label}>Addebito NRC</label>
          <div className={styles.radioGroup}>
            <label className={styles.radioLabel}>
              <input className={styles.radioInput} type="radio" name="nrc_charge_time" value="1"
                checked={quote.nrc_charge_time === 1}
                onChange={() => onChange('nrc_charge_time', 1)}
              />
              All&apos;ordine
            </label>
            <label className={styles.radioLabel}>
              <input className={styles.radioInput} type="radio" name="nrc_charge_time" value="2"
                checked={quote.nrc_charge_time === 2}
                onChange={() => onChange('nrc_charge_time', 2)}
              />
              All&apos;attivazione
            </label>
          </div>
        </div>
      </div>

      {/* Servizi e Template */}
      <div className={styles.section}>
        <div className={styles.sectionTitle}>Servizi e Template</div>
        <div className={styles.field}>
          <label className={styles.label}>Servizi</label>
          <MultiSelect<number>
            options={categoryOptions}
            selected={selectedServiceIds}
            onChange={chosen => onChange('services', chosen.join(','))}
            placeholder={
              categoriesQuery.isPending ? 'Caricamento servizi...' : 'Seleziona servizi...'
            }
          />
        </div>
        <div className={styles.field}>
          <label className={styles.label}>Template</label>
          <SingleSelect<string>
            options={templateOptions}
            selected={quote.template || null}
            onChange={v => onChange('template', v ?? '')}
            placeholder={
              templatesQuery.isPending ? 'Caricamento template...' : '— Seleziona —'
            }
            allowClear
          />
        </div>
      </div>
    </div>
  );
}
