import { useMemo } from 'react';
import type { Quote } from '../api/types';
import {
  useCategories,
  useCustomerOrders,
  useOwners,
  usePaymentMethods,
  useTemplates,
} from '../api/queries';
import { isIaaSTemplate, parseReplaceOrders, parseServiceCategoryIds } from '../utils/quoteRules';
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

  // Stored as `;`-separated string (Appsmith parity); present as multi-select.
  const selectedOrders = parseReplaceOrders(quote.replace_orders);

  const isIaaS = isIaaSTemplate(quote.template);
  const isSpot = quote.document_type === 'TSC-ORDINE';
  const categoryNameById = useMemo(
    () => new Map((categoriesQuery.data ?? []).map(category => [String(category.id), category.name])),
    [categoriesQuery.data]
  );
  const templateDescriptionById = useMemo(
    () => new Map((templatesQuery.data ?? []).map(template => [template.template_id, template.description])),
    [templatesQuery.data]
  );
  const servicesLabel = useMemo(() => {
    const serviceIds = parseServiceCategoryIds(quote.services);
    if (serviceIds.length === 0) {
      return '—';
    }
    if (categoriesQuery.isPending) {
      return 'Risoluzione...';
    }
    return serviceIds
      .map(serviceId => categoryNameById.get(serviceId) ?? `ID ${serviceId} non trovato`)
      .join(', ');
  }, [categoryNameById, categoriesQuery.isPending, quote.services]);
  const templateLabel = useMemo(() => {
    if (!quote.template) {
      return '—';
    }
    if (templatesQuery.isPending) {
      return 'Risoluzione...';
    }
    return templateDescriptionById.get(quote.template) ?? `Template non risolto (${quote.template})`;
  }, [quote.template, templateDescriptionById, templatesQuery.isPending]);

  return (
    <div className={styles.grid}>
      {/* Deal e Proprietà */}
      <div className={styles.section}>
        <div className={styles.sectionTitle}>Deal e Proprietà</div>
        <div className={styles.field}>
          <label className={styles.label}>Deal</label>
          <input className={styles.readOnly} readOnly value={quote.deal_name ?? quote.deal_number ?? '—'} />
        </div>
        <div className={styles.field}>
          <label className={styles.label}>Cliente</label>
          <input className={styles.readOnly} readOnly value={quote.customer_name ?? '—'} />
        </div>
        <div className={styles.field}>
          <label className={styles.label}>Owner</label>
          <select
            className={styles.select}
            value={quote.owner ?? ''}
            onChange={e => onChange('owner', e.target.value)}
          >
            <option value="">— Seleziona —</option>
            {owners?.map(o => (
              <option key={o.id} value={o.id}>
                {[o.firstname, o.lastname].filter(Boolean).join(' ')}
              </option>
            ))}
          </select>
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
            <select
              className={styles.input}
              multiple
              size={Math.min(6, Math.max(3, customerOrders.length))}
              value={selectedOrders}
              disabled={customerOrdersQuery.isPending || customerOrders.length === 0}
              onChange={e => {
                const chosen = Array.from(e.target.selectedOptions, o => o.value);
                onChange('replace_orders', chosen.join(';'));
              }}
            >
              {customerOrders.map(o => (
                <option key={o.name} value={o.name}>{o.name}</option>
              ))}
              {/* Preserve any legacy stored values that are not in the loaded list */}
              {selectedOrders
                .filter(v => !customerOrders.some(o => o.name === v))
                .map(v => (
                  <option key={`legacy-${v}`} value={v}>{v} (legacy)</option>
                ))}
            </select>
            {!customerOrdersQuery.isPending && customerOrders.length === 0 && (
              <div className={styles.emptyHint}>Nessun ordine disponibile per il cliente.</div>
            )}
            {selectedOrders.length > 0 && (
              <div className={styles.emptyHint}>
                Ordini selezionati: {selectedOrders.join(', ')}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Condizioni Commerciali */}
      <div className={styles.section}>
        <div className={styles.sectionTitle}>Condizioni Commerciali</div>
        <div className={styles.field}>
          <label className={styles.label}>Metodo di pagamento</label>
          <select
            className={styles.select}
            value={quote.payment_method ?? ''}
            onChange={e => onChange('payment_method', e.target.value)}
          >
            <option value="">— Seleziona —</option>
            {paymentMethods?.map(pm => (
              <option key={pm.code} value={pm.code}>{pm.description}</option>
            ))}
          </select>
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

      {/* Servizi e Template placeholder */}
      <div className={styles.section}>
        <div className={styles.sectionTitle}>Servizi e Template</div>
        <div className={styles.field}>
          <label className={styles.label}>Servizi</label>
          <input className={styles.readOnly} readOnly value={servicesLabel} />
        </div>
        <div className={styles.field}>
          <label className={styles.label}>Template</label>
          <input className={styles.readOnly} readOnly value={templateLabel} />
        </div>
      </div>
    </div>
  );
}
