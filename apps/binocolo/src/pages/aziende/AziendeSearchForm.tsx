import { Button, Icon, MoneyInput, MultiSelect, SingleSelect, ToggleSwitch } from '@mrsmith/ui';
import { useState } from 'react';
import type { MACompanySearchAreas } from '../../api/types';
import styles from './AziendePage.module.css';

export type CompanySearchNDA = '' | 'any' | 'active' | 'expired_only' | 'none';

const NDA_OPTIONS: { value: CompanySearchNDA; label: string }[] = [
  { value: 'any', label: 'Con NDA registrato' },
  { value: 'active', label: 'Con NDA non scaduto' },
  { value: 'expired_only', label: 'Solo NDA scaduti' },
  { value: 'none', label: 'Senza NDA registrato' },
];

export interface CompanySearchDraft {
  name: string;
  vat: string;
  tax: string;
  annotation: string;
  include: string[];
  exclude: string[];
  turnoverMin: string;
  turnoverMax: string;
  employeesMin: string;
  employeesMax: string;
  turnoverMissing: boolean;
  employeesMissing: boolean;
  nda: CompanySearchNDA;
}

export function emptyCompanySearch(): CompanySearchDraft {
  return { name: '', vat: '', tax: '', annotation: '', include: [], exclude: [], turnoverMin: '', turnoverMax: '', employeesMin: '', employeesMax: '', turnoverMissing: false, employeesMissing: false, nda: '' };
}

export function companySearchParams(draft: CompanySearchDraft): string {
  const params = new URLSearchParams({ mode: 'advanced' });
  for (const [key, value] of Object.entries(draft)) {
    if (Array.isArray(value)) value.forEach((area) => params.append(key, area));
    else if (value !== '' && value !== false) params.set(key, String(value).trim());
  }
  return params.toString();
}

export function territorySummary(draft: Pick<CompanySearchDraft, 'include' | 'exclude'>, areas: MACompanySearchAreas['items']): string {
  const labels = new Map(areas.map((area) => [area.value, area.label]));
  const join = (values: string[]) => values.map((value) => labels.get(value) ?? value.split(':').slice(1).join(' · ')).join(', ');
  const included = draft.include.length ? join(draft.include) : 'Tutta Italia';
  if (!draft.exclude.length) return included;
  if (draft.exclude.length > 1) return `${included}, escluse le aree: ${join(draft.exclude)}`;
  const excluded = join(draft.exclude);
  if (draft.exclude[0]?.startsWith('province:')) return `${included}, esclusa la ${excluded.charAt(0).toLowerCase()}${excluded.slice(1)}`;
  return `${included}, esclusa ${excluded}`;
}

export function AziendeSearchForm({ draft, onChange, onSearch, onReset, areas, areasLoading, areasError, onRetryAreas }: {
  draft: CompanySearchDraft;
  onChange: (draft: CompanySearchDraft) => void;
  onSearch: () => void;
  onReset: () => void;
  areas?: MACompanySearchAreas;
  areasLoading: boolean;
  areasError: boolean;
  onRetryAreas: () => void;
}) {
  const [showExclusions, setShowExclusions] = useState(draft.exclude.length > 0);
  const [submitted, setSubmitted] = useState(false);
  const set = <K extends keyof CompanySearchDraft>(key: K, value: CompanySearchDraft[K]) => onChange({ ...draft, [key]: value });
  const errors: Record<string, string> = {};
  const vat = draft.vat.trim().toUpperCase().replace(/[.\s]/g, '').replace(/^IT/, '');
  const tax = draft.tax.trim().toUpperCase().replace(/[.\s]/g, '');
  if (vat && !/^\d{11}$/.test(vat)) errors.vat = 'Inserisci una P.IVA di 11 cifre.';
  if (tax && !/^(\d{11}|[A-Z0-9]{16})$/.test(tax)) errors.tax = 'Inserisci un codice fiscale di 11 cifre o 16 caratteri.';
  for (const metric of ['turnover', 'employees'] as const) {
    const min = draft[`${metric}Min`];
    const max = draft[`${metric}Max`];
    if (min && max && Number(min) > Number(max)) errors[metric] = 'Il massimo deve essere maggiore o uguale al minimo.';
    const ceiling = metric === 'turnover' ? 1e15 : 2147483647;
    if ([min, max].some((v) => v && (!Number.isFinite(Number(v)) || Number(v) < 0 || Number(v) > ceiling || (metric === 'employees' && !/^\d+$/.test(v))))) {
      errors[metric] = metric === 'employees' ? 'Inserisci un numero intero di dipendenti, maggiore o uguale a zero.' : 'Inserisci un importo valido, maggiore o uguale a zero.';
    }
  }
  function validateAndSearch() {
    setSubmitted(true);
    if (Object.keys(errors).length) return;
    onSearch();
  }
  const fieldError = (key: string) => submitted ? errors[key] : undefined;

  return (
    <form
      className={styles.advancedForm}
      onSubmit={(event) => event.preventDefault()}
      onKeyDown={(event) => {
        // Implicit submission can click the original picker's first chip button.
        if (event.key === 'Enter' && event.target instanceof HTMLInputElement) event.preventDefault();
      }}
      noValidate
    >
      <div className={styles.identityFields}>
        {([['name', 'Ragione sociale'], ['vat', 'P.IVA'], ['tax', 'Codice fiscale']] as const).map(([key, label]) => (
          <div className={styles.field} key={key}>
            <label htmlFor={`company-${key}`}>{label}</label>
            <input id={`company-${key}`} value={draft[key]} maxLength={200} onChange={(event) => set(key, event.target.value)} aria-invalid={!!fieldError(key)} aria-describedby={fieldError(key) ? `company-${key}-error` : undefined} />
            {fieldError(key) && <p id={`company-${key}-error`} className={styles.fieldError}>{fieldError(key)}</p>}
          </div>
        ))}
      </div>

      <fieldset className={styles.territory}>
        <legend>Territorio</legend>
        <MultiSelect options={areas?.items ?? []} selected={draft.include} onChange={(value) => set('include', value)} placeholder="Includi regioni o province" />
        {showExclusions || draft.exclude.length ? (
          <div className={styles.exclusions}>
            <span className={styles.fieldLabel}>Aree escluse</span>
            <MultiSelect options={areas?.items ?? []} selected={draft.exclude} onChange={(value) => set('exclude', value)} placeholder="Escludi regioni o province" />
            <Button type="button" variant="ghost" onClick={() => { set('exclude', []); setShowExclusions(false); }}>Rimuovi esclusioni</Button>
          </div>
        ) : (
          <Button type="button" variant="ghost" leftIcon={<Icon name="plus" size={16} />} onClick={() => setShowExclusions(true)}>Escludi aree</Button>
        )}
        <p className={styles.territorySummary}>{territorySummary(draft, areas?.items ?? [])}</p>
        {areasLoading && <p className={styles.hint}>Caricamento delle aree…</p>}
        {areasError && <div className={styles.fieldError} role="alert">Aree non disponibili. <Button type="button" variant="ghost" onClick={onRetryAreas}>Riprova</Button></div>}
        {areas && !areas.regionsAvailable && <p className={styles.fieldError} role="status">Il repertorio regionale non è disponibile nel corpus. Puoi selezionare le province presenti.</p>}
      </fieldset>

      <div className={styles.metricsFields}>
        <fieldset className={styles.metricField}>
          <legend>Fatturato</legend>
          <div className={styles.range}>
            <MoneyInput id="company-turnover-min" label="Minimo" value={draft.turnoverMin} onChange={(value) => set('turnoverMin', value)} placeholder="Nessun minimo" />
            <MoneyInput id="company-turnover-max" label="Massimo" value={draft.turnoverMax} onChange={(value) => set('turnoverMax', value)} placeholder="Nessun massimo" error={fieldError('turnover')} />
          </div>
          <ToggleSwitch id="company-turnover-missing" checked={draft.turnoverMissing} onChange={(value) => set('turnoverMissing', value)} label="Includi anche aziende con dato non disponibile" />
        </fieldset>
        <fieldset className={styles.metricField}>
          <legend>Dipendenti</legend>
          <div className={styles.range}>
            {(['Min', 'Max'] as const).map((bound) => (
              <div className={styles.field} key={bound}>
                <label htmlFor={`company-employees-${bound}`}>{bound === 'Min' ? 'Minimo' : 'Massimo'}</label>
                <input id={`company-employees-${bound}`} type="number" min="0" step="1" value={draft[`employees${bound}`]} placeholder={bound === 'Min' ? 'Nessun minimo' : 'Nessun massimo'} onChange={(event) => set(`employees${bound}`, event.target.value)} aria-invalid={!!fieldError('employees')} aria-describedby={fieldError('employees') ? 'company-employees-error' : undefined} />
              </div>
            ))}
          </div>
          {fieldError('employees') && <p id="company-employees-error" className={styles.fieldError}>{fieldError('employees')}</p>}
          <ToggleSwitch id="company-employees-missing" checked={draft.employeesMissing} onChange={(value) => set('employeesMissing', value)} label="Includi anche aziende con dato non disponibile" />
        </fieldset>
      </div>
      <p className={styles.hint}>Ogni indicatore usa l’ultimo dato disponibile. Le opzioni sui dati mancanti si applicano solo quando è impostato un minimo o un massimo.</p>
      <div className={styles.field}>
        <label htmlFor="company-annotation">Testo nelle annotazioni</label>
        <input id="company-annotation" value={draft.annotation} maxLength={500} onChange={(event) => set('annotation', event.target.value)} placeholder="Es. passaggio generazionale" />
      </div>
      <div className={styles.field}>
        <span className={styles.fieldLabel}>NDA</span>
        <SingleSelect options={NDA_OPTIONS} selected={draft.nda || null} onChange={(value) => set('nda', value ?? '')} placeholder="Qualsiasi" allowClear clearLabel="Qualsiasi" searchable={false} ariaLabel="NDA" />
        <p className={styles.hint}>Si basa sugli accordi registrati nella scheda azienda, non sulla fase delle card.</p>
      </div>
      <div className={styles.formActions}>
        <Button type="button" onClick={validateAndSearch} leftIcon={<Icon name="search" size={16} />}>Cerca</Button>
        <Button type="button" variant="ghost" onClick={() => { setSubmitted(false); setShowExclusions(false); onReset(); }}>Azzera filtri</Button>
        <p className={styles.hint}>I criteri compilati devono essere tutti soddisfatti.</p>
      </div>
      {submitted && Object.keys(errors).length > 0 && <p className={styles.fieldError} role="alert">Correggi i campi indicati prima di cercare.</p>}
    </form>
  );
}
