import { Fragment, useCallback, useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { ApiError } from '@mrsmith/api-client';
import { Icon, SingleSelect, Skeleton } from '@mrsmith/ui';
import { downloadRiepilogoRdaExcel, useRiepilogoRda } from '../api/queries';
import type { PeriodPreset, RiepilogoPeriodSelection, RiepilogoRdaBudget, RiepilogoRdaDetail } from '../api/types';
import { useApiClient } from '../api/client';
import { ServiceUnavailable } from '../components/ServiceUnavailable';
import { formatEUR, nz } from '../lib/format';
import s from './RiepilogoRdaPage.module.css';

const DEFAULT_PERIOD: PeriodPreset = 'previous_month';

const PERIOD_OPTIONS: Array<{ value: RiepilogoPeriodSelection; label: string }> = [
  { value: 'this_month', label: 'Mese corrente' },
  { value: 'previous_month', label: 'Mese precedente' },
  { value: 'this_quarter', label: 'Trimestre corrente' },
  { value: 'previous_quarter', label: 'Trimestre precedente' },
  { value: 'current_year', label: 'Anno corrente' },
  { value: 'previous_year', label: 'Anno precedente' },
  { value: 'custom', label: 'Intervallo personalizzato' },
];

const periodValues = new Set<RiepilogoPeriodSelection>(PERIOD_OPTIONS.map((option) => option.value));

const RDA_STATE_LABELS: Record<string, string> = {
  CLOSED: 'Chiuso',
  DELIVERED_AND_COMPLIANT: 'Consegnato e conforme',
  PENDING_CHECK_DOCUMENT: 'In attesa verifica documenti',
  PENDING_CONTRACT_VERIFICATION: 'In verifica contratto',
  PENDING_DISPUTE: 'In gestione contestazione',
  PENDING_ERP_SAVE: 'In attesa registrazione ERP',
  PENDING_LEASING: 'In attesa leasing',
  PENDING_LEASING_ORDER_CREATION: 'Creazione ordine leasing in corso',
  PENDING_PDF_GENERATION: 'Generazione PDF in corso',
  PENDING_PROVIDER_SAVED_IN_ALYANTE: 'Fornitore da registrare in Alyante',
  PENDING_SEND: 'In attesa invio',
  PENDING_VERIFICATION: 'In verifica',
};

function isPeriodSelection(value: string | null): value is RiepilogoPeriodSelection {
  return Boolean(value && periodValues.has(value as RiepilogoPeriodSelection));
}

function isISODate(value: string | null): value is string {
  return Boolean(value && /^\d{4}-\d{2}-\d{2}$/.test(value));
}

function toISODateLocal(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function getDefaultCustomRange() {
  const now = new Date();
  const to = new Date(now.getFullYear(), now.getMonth(), 1);
  const from = new Date(now.getFullYear(), now.getMonth() - 1, 1);
  return { from: toISODateLocal(from), to: toISODateLocal(to) };
}

function formatPeriodDate(value: string | null | undefined): string {
  if (!value) return '—';
  const [year, month, day] = value.slice(0, 10).split('-');
  if (!year || !month || !day) return value;
  return `${day}/${month}/${year}`;
}

function formatDateTime(value: string | null | undefined): string {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value.slice(0, 16).replace('T', ' ');
  return new Intl.DateTimeFormat('it-IT', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date);
}

function safeDateSegment(value: string | null | undefined): string {
  if (!value) return 'data-non-disponibile';
  return value.slice(0, 10).replace(/[^0-9A-Za-z-]/g, '-');
}

function filenameFor(period: RiepilogoPeriodSelection, from?: string | null, to?: string | null): string {
  const safeFrom = safeDateSegment(from);
  const safeTo = safeDateSegment(to);
  return `riepilogo-rda_${period}_${safeFrom}_${safeTo}.xlsx`;
}

function budgetKeyForDetail(detail: RiepilogoRdaDetail): string {
  return detail.budget_id === null ? 'senza-budget' : String(detail.budget_id);
}

function budgetLabel(detail: RiepilogoRdaDetail): string {
  if (!detail.budget_name) return 'Senza budget';
  return detail.budget_year ? `${detail.budget_name} ${detail.budget_year}` : detail.budget_name;
}

function stateLabel(state: string | null | undefined): string {
  if (!state) return '—';
  return RDA_STATE_LABELS[state] ?? state;
}

export function RiepilogoRdaPage() {
  const [params, setParams] = useSearchParams();
  const api = useApiClient();
  const periodParam = params.get('period');
  const period = isPeriodSelection(periodParam) ? periodParam : DEFAULT_PERIOD;
  const customDefaults = useMemo(() => getDefaultCustomRange(), []);
  const customFrom = isISODate(params.get('from')) ? params.get('from')! : customDefaults.from;
  const customTo = isISODate(params.get('to')) ? params.get('to')! : customDefaults.to;
  const riepilogoParams = useMemo(
    () => (period === 'custom' ? { period, from: customFrom, to: customTo } : { period }),
    [customFrom, customTo, period],
  );
  const [selectedBudgetKey, setSelectedBudgetKey] = useState<string | null>(null);
  const [isExporting, setIsExporting] = useState(false);
  const [exportError, setExportError] = useState<string | null>(null);

  const riepilogoQ = useRiepilogoRda(riepilogoParams);
  const data = riepilogoQ.data;

  useEffect(() => {
    if (!isPeriodSelection(periodParam)) {
      setParams((prev) => {
        const next = new URLSearchParams(prev);
        next.set('period', DEFAULT_PERIOD);
        next.delete('from');
        next.delete('to');
        return next;
      }, { replace: true });
      return;
    }

    if (period === 'custom' && (!isISODate(params.get('from')) || !isISODate(params.get('to')))) {
      setParams((prev) => {
        const next = new URLSearchParams(prev);
        next.set('period', 'custom');
        next.set('from', customFrom);
        next.set('to', customTo);
        return next;
      }, { replace: true });
    }
  }, [customFrom, customTo, params, period, periodParam, setParams]);

  useEffect(() => {
    setSelectedBudgetKey(null);
  }, [period, customFrom, customTo]);

  const handlePeriodChange = useCallback(
    (value: RiepilogoPeriodSelection | null) => {
      const nextPeriod = value ?? DEFAULT_PERIOD;
      setParams((prev) => {
        const next = new URLSearchParams(prev);
        next.set('period', nextPeriod);
        if (nextPeriod === 'custom') {
          next.set('from', data?.period.from ?? customFrom);
          next.set('to', data?.period.to ?? customTo);
        } else {
          next.delete('from');
          next.delete('to');
        }
        return next;
      });
    },
    [customFrom, customTo, data?.period.from, data?.period.to, setParams],
  );

  const handleCustomDateChange = useCallback(
    (field: 'from' | 'to', value: string) => {
      setParams((prev) => {
        const next = new URLSearchParams(prev);
        next.set('period', 'custom');
        next.set('from', field === 'from' ? value : customFrom);
        next.set('to', field === 'to' ? value : customTo);
        return next;
      });
    },
    [customFrom, customTo, setParams],
  );

  const handleExport = useCallback(async () => {
    setIsExporting(true);
    setExportError(null);
    try {
      await downloadRiepilogoRdaExcel(
        api,
        riepilogoParams,
        filenameFor(period, data?.period.from, data?.period.to),
      );
    } catch {
      setExportError('Non è stato possibile generare il file Excel.');
    } finally {
      setIsExporting(false);
    }
  }, [api, data?.period.from, data?.period.to, period, riepilogoParams]);

  const maxAmount = useMemo(
    () => Math.max(0, ...(data?.budgets.map((budget) => budget.amount) ?? [])),
    [data?.budgets],
  );

  const visibleDetails = useMemo(() => {
    const details = data?.details ?? [];
    if (!selectedBudgetKey) return details;
    return details.filter((detail) => budgetKeyForDetail(detail) === selectedBudgetKey);
  }, [data?.details, selectedBudgetKey]);

  const selectedBudget = data?.budgets.find((budget) => budget.budget_key === selectedBudgetKey);
  const dateRange = data?.period
    ? `${formatPeriodDate(data.period.from)} – ${formatPeriodDate(data.period.to)} escluso`
    : 'Periodo in caricamento…';

  const isEmpty = Boolean(data && data.details.length === 0);
  const isServiceUnavailable = riepilogoQ.error instanceof ApiError && riepilogoQ.error.status === 503;

  return (
    <main className={`statsRdaPage ${s.page}`}>
      <header className={`pageHeader ${s.header}`}>
        <div className="pageHeaderCopy">
          <span className="pageHeaderEyebrow">
            <Icon name="bar-chart-2" size={14} /> Riepilogo RDA
          </span>
          <h1>Riepilogo RDA</h1>
          <p>Totali degli ordini RDA per budget nel periodo selezionato.</p>
        </div>
      </header>

      <section className={`surface ${s.toolbar}`} aria-label="Filtri riepilogo RDA">
        <div className={s.toolbarField}>
          <label>Periodo</label>
          <SingleSelect
            options={PERIOD_OPTIONS}
            selected={period}
            onChange={handlePeriodChange}
            placeholder="Seleziona periodo"
          />
        </div>
        {period === 'custom' && (
          <>
            <div className={s.toolbarField}>
              <label htmlFor="riepilogo-rda-from">Da</label>
              <input
                id="riepilogo-rda-from"
                className={s.dateInput}
                type="date"
                value={customFrom}
                max={customTo}
                onChange={(event) => handleCustomDateChange('from', event.target.value)}
              />
            </div>
            <div className={s.toolbarField}>
              <label htmlFor="riepilogo-rda-to">A esclusa</label>
              <input
                id="riepilogo-rda-to"
                className={s.dateInput}
                type="date"
                value={customTo}
                min={customFrom}
                onChange={(event) => handleCustomDateChange('to', event.target.value)}
              />
            </div>
          </>
        )}
        <div className={s.rangeText} aria-live="polite">
          <Icon name="calendar" size={16} />
          <span>{dateRange}</span>
        </div>
        <button type="button" className={s.exportButton} onClick={handleExport} disabled={isExporting}>
          <Icon name="download" size={16} />
          {isExporting ? 'Preparazione file Excel…' : 'Scarica Excel'}
        </button>
        {exportError && <p className={s.exportError}>{exportError}</p>}
      </section>

      {riepilogoQ.isLoading && (
        <section className="surface stateCard" aria-live="polite">
          <p className="stateTitle">Caricamento riepilogo RDA…</p>
          <Skeleton rows={5} />
        </section>
      )}

      {!riepilogoQ.isLoading && isServiceUnavailable && <ServiceUnavailable service="Stats RDA" />}

      {!riepilogoQ.isLoading && riepilogoQ.error && !isServiceUnavailable && (
        <section className={s.errorState} role="alert">
          <div className={s.stateIcon}><Icon name="triangle-alert" size={30} /></div>
          <p className="stateTitle">Non è stato possibile caricare il riepilogo RDA.</p>
          <p className="muted">Riprova più tardi.</p>
        </section>
      )}

      {!riepilogoQ.isLoading && !riepilogoQ.error && isEmpty && (
        <section className={s.emptyState}>
          <div className={s.stateIcon}><Icon name="file-text" size={30} /></div>
          <p className="emptyStateTitle">Nessun ordine RDA trovato per il periodo selezionato.</p>
          <p className="emptyStateDesc">Seleziona un periodo diverso per consultare altri ordini RDA.</p>
        </section>
      )}

      {data && !isEmpty && (
        <>
          <section className={s.summaryGrid} aria-label="Indicatori riepilogo RDA">
            <article className={s.metricCard}>
              <span>Totale periodo</span>
              <strong>{formatEUR(data.totals.amount)}</strong>
            </article>
            <article className={s.metricCard}>
              <span>Ordini RDA</span>
              <strong>{data.totals.order_count.toLocaleString('it-IT')}</strong>
            </article>
            <article className={s.metricCard}>
              <span>Budget</span>
              <strong>{data.totals.budget_count.toLocaleString('it-IT')}</strong>
            </article>
          </section>

          <section className={`surface ${s.chartCard}`}>
            <div className={s.cardHeader}>
              <div>
                <h2>Totali per budget</h2>
                <p>Seleziona una barra per filtrare i dettagli degli ordini RDA.</p>
              </div>
              {selectedBudgetKey && (
                <button type="button" className={s.clearButton} onClick={() => setSelectedBudgetKey(null)}>
                  Mostra tutti
                </button>
              )}
            </div>
            <div className={s.barList} role="list" aria-label="Budget ordinati per importo">
              {data.budgets.map((budget) => (
                <BudgetBar
                  key={budget.budget_key}
                  budget={budget}
                  maxAmount={maxAmount}
                  selected={selectedBudgetKey === budget.budget_key}
                  onSelect={() => setSelectedBudgetKey((current) => (current === budget.budget_key ? null : budget.budget_key))}
                />
              ))}
            </div>
          </section>

          <section className={`surface ${s.detailsCard}`}>
            <div className={s.cardHeader}>
              <div>
                <h2>Dettaglio ordini</h2>
                <p>
                  {selectedBudget
                    ? `Filtro budget: ${selectedBudget.budget}`
                    : 'Tutti gli ordini RDA inclusi nel periodo selezionato.'}
                </p>
              </div>
            </div>
            {visibleDetails.length === 0 ? (
              <div className={s.emptyState}>
                <div className={s.stateIcon}><Icon name="file-text" size={30} /></div>
                <p className="emptyStateTitle">Nessun ordine RDA trovato per il periodo selezionato.</p>
              </div>
            ) : (
              <div className={s.tableWrap}>
                <table className={s.table}>
                  <thead>
                    <tr>
                      <th>Ordine</th>
                      <th>Budget</th>
                      <th className={s.numeric}>Importo totale</th>
                      <th>Stato</th>
                      <th>Creato</th>
                    </tr>
                  </thead>
                  <tbody>
                    {visibleDetails.map((detail, idx) => (
                      <Fragment key={`${detail.code}-${idx}`}>
                        <tr className={s.recordMainRow} style={{ animationDelay: `${Math.min(idx * 20, 300)}ms` }}>
                          <td><div className={s.orderKey}>{nz(detail.code)}</div></td>
                          <td>{budgetLabel(detail)}</td>
                          <td className={s.numeric}>{formatEUR(detail.total_price)}</td>
                          <td>{stateLabel(detail.state)}</td>
                          <td>{formatDateTime(detail.created)}</td>
                        </tr>
                        <tr className={s.recordContextRow} style={{ animationDelay: `${Math.min(idx * 20 + 40, 340)}ms` }}>
                          <td colSpan={5}>
                            <dl className={s.contextGrid} aria-label={`Contesto ordine ${nz(detail.code)}`}>
                              <div className={s.metaItem}>
                                <dt>Oggetto</dt>
                                <dd>{nz(detail.object)}</dd>
                              </div>
                              <div className={s.metaItem}>
                                <dt>Progetto</dt>
                                <dd>{nz(detail.project)}</dd>
                              </div>
                              <div className={s.metaItem}>
                                <dt>Centro di costo</dt>
                                <dd>{nz(detail.cost_center)}</dd>
                              </div>
                              <div className={s.metaItem}>
                                <dt>Fornitore</dt>
                                <dd>{nz(detail.company_name)}</dd>
                              </div>
                              <div className={s.metaItem}>
                                <dt>Richiedente</dt>
                                <dd>{nz(detail.requester)}</dd>
                              </div>
                            </dl>
                          </td>
                        </tr>
                      </Fragment>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </section>
        </>
      )}
    </main>
  );
}

function BudgetBar({
  budget,
  maxAmount,
  selected,
  onSelect,
}: {
  budget: RiepilogoRdaBudget;
  maxAmount: number;
  selected: boolean;
  onSelect: () => void;
}) {
  const width = maxAmount > 0 ? Math.max(4, (budget.amount / maxAmount) * 100) : 0;
  const readableAmount = formatEUR(Math.floor(budget.amount));
  const readablePercentage = Math.floor(budget.percentage).toLocaleString('it-IT');
  const tooltip = `${budget.budget} — ${readableAmount} — ${readablePercentage}% — ${budget.order_count} ordini`;

  return (
    <button
      type="button"
      className={`${s.barButton} ${selected ? s.barButtonSelected : ''}`}
      onClick={onSelect}
      title={tooltip}
      aria-pressed={selected}
      aria-label={tooltip}
    >
      <span className={s.barHeader}>
        <span className={s.barBudget}>{budget.budget}</span>
        <span className={s.barAmount}>{readableAmount}</span>
      </span>
      <span className={s.barStats}>
        <span>{readablePercentage}% del totale</span>
        <span>{budget.order_count.toLocaleString('it-IT')} ordini RDA</span>
      </span>
      <span className={s.barTrack} aria-hidden="true">
        <span className={s.barFill} style={{ width: `${width}%` }} />
      </span>
    </button>
  );
}
